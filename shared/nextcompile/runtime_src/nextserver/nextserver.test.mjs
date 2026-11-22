// Unit tests for the nextserver Worker runtime (VFS, HTTP bridge, CacheHandler, driver).
// Run with: node --test shared/nextcompile/runtime_src/nextserver/nextserver.test.mjs

import { test, beforeEach } from "node:test";
import assert from "node:assert/strict";

import fsShim, { normalizeVFSPath, readFileSync, existsSync, statSync, readdirSync, promises as fsPromises } from "./vfs_fs_shim.mjs";
import { invokeNodeHandler } from "./http_bridge.mjs";
import { CloudflareCacheHandler, resetLocalCacheForTests } from "./cf_cache_handler.mjs";
import {
  installNextServerGlobals,
  dispatchNextServer,
  resetNextServerHandlerForTests,
} from "./nextserver_driver.mjs";

beforeEach(() => {
  resetLocalCacheForTests();
  resetNextServerHandlerForTests();
  installNextServerGlobals({
    vfs: {
      files: {
        "package.json": '{"name":"demo"}',
        ".next/BUILD_ID": "build-turbo-1",
        ".next/routes-manifest.json": '{"version":3}',
      },
      dirs: {
        "": ["package.json", ".next"],
        ".next": ["BUILD_ID", "routes-manifest.json"],
      },
    },
    assetVersion: "v1",
  });
});

// ── VFS tests ────────────────────────────────────────────────────────────────

test("VFS reads inlined manifest files with relative, leading-slash, and mounted paths", async () => {
  assert.equal(normalizeVFSPath("/.next/BUILD_ID"), ".next/BUILD_ID");
  assert.equal(normalizeVFSPath("/var/task/.next/routes-manifest.json"), ".next/routes-manifest.json");
  assert.equal(existsSync(".next/BUILD_ID"), true);
  assert.equal(readFileSync("/.next/BUILD_ID", "utf8"), "build-turbo-1");
  assert.equal(await fsPromises.readFile("/.next/routes-manifest.json", "utf8"), '{"version":3}');
  assert.equal(statSync(".next/BUILD_ID").isFile(), true);
  assert.equal(statSync(".next").isDirectory(), true);
  assert.deepEqual(readdirSync(".next"), ["BUILD_ID", "routes-manifest.json"]);
});

test("VFS throws ENOENT for missing files", () => {
  assert.equal(existsSync(".next/does-not-exist.json"), false);
  assert.throws(
    () => fsShim.readFileSync(".next/does-not-exist.json", "utf8"),
    (err) => err.code === "ENOENT",
  );
});

// ── HTTP bridge tests ────────────────────────────────────────────────────────

test("http_bridge streams progressive chunks before handler finishes", async () => {
  let releaseSecondChunk;
  const secondChunkGate = new Promise((resolve) => {
    releaseSecondChunk = resolve;
  });

  const handler = async (req, res) => {
    assert.equal(req.method, "POST");
    assert.equal(req.url, "/api/echo?x=1");
    assert.equal(req.headers["x-test"], "yes");

    const bodyParts = [];
    for await (const chunk of req) bodyParts.push(chunk);
    const reqBody = Buffer.concat(bodyParts).toString("utf8");

    res.statusCode = 200;
    res.setHeader("content-type", "text/html; charset=utf-8");
    res.appendHeader("set-cookie", "a=1; Path=/");
    res.appendHeader("set-cookie", "b=2; Path=/");
    res.write(`<shell>${reqBody}</shell>`);
    await secondChunkGate;
    res.end("<resolved>done</resolved>");
  };

  const waitUntilPromises = [];
  const req = new Request("https://app.example.com/api/echo?x=1", {
    method: "POST",
    headers: { "x-test": "yes", "content-type": "text/plain" },
    body: "hello",
  });

  const response = await invokeNodeHandler(handler, req, {
    waitUntil: (p) => waitUntilPromises.push(p),
  });

  assert.equal(response.status, 200);
  const reader = response.body.getReader();
  const decoder = new TextDecoder();

  // First chunk MUST arrive before secondChunkGate resolves (true streaming!).
  const first = await reader.read();
  assert.equal(first.done, false);
  assert.equal(decoder.decode(first.value), "<shell>hello</shell>");

  releaseSecondChunk();
  const second = await reader.read();
  assert.equal(decoder.decode(second.value), "<resolved>done</resolved>");
  const end = await reader.read();
  assert.equal(end.done, true);
  await Promise.all(waitUntilPromises);
});

// ── CloudflareCacheHandler tests ─────────────────────────────────────────────

test("CloudflareCacheHandler persists to R2 and invalidates via revalidateTag", async () => {
  const r2Store = new Map();
  const kvStore = new Map();
  const waitUntilPromises = [];

  globalThis.__nextdeployEnv = {
    ASSETS: {
      get: async (k) => (r2Store.has(k) ? { body: r2Store.get(k), json: async () => JSON.parse(r2Store.get(k)) } : null),
      put: async (k, v) => {
        r2Store.set(k, v);
      },
    },
    NEXTCOMPILE_CACHE: {
      get: async (k) => kvStore.get(k) ?? null,
      put: async (k, v) => {
        kvStore.set(k, v);
      },
    },
  };
  globalThis.__nextdeployWorkerCtx = {
    waitUntil: (p) => waitUntilPromises.push(p),
  };

  const handler = new CloudflareCacheHandler({ assetVersion: "v1" });
  await handler.set("/blog", { kind: "APP_PAGE", html: "<h1>v1</h1>" }, { tags: ["posts"] });
  await Promise.all(waitUntilPromises);

  // Confirm persisted to R2 and readable even after clearing L1.
  resetLocalCacheForTests();
  const hit = await handler.get("/blog");
  assert.equal(hit?.value?.html, "<h1>v1</h1>");

  // Revalidate tag — subsequent get must miss.
  await new Promise((r) => setTimeout(r, 5));
  await handler.revalidateTag("posts");
  await Promise.all(waitUntilPromises);
  assert.equal(await handler.get("/blog"), null);
});

// ── NextServer driver tests ──────────────────────────────────────────────────

test("dispatchNextServer runs guard, serves R2 static assets, and delegates dynamic routes to NextNodeServer", async () => {
  const r2Assets = {
    "_next/static/chunks/app.js": "console.log('static')",
    "_nextdeploy/releases/v1/robots.txt": "User-agent: *",
  };
  const env = {
    ASSETS: {
      get: async (k) => (r2Assets[k] ? { body: r2Assets[k], writeHttpMetadata() {} } : null),
    },
  };
  const manifest = { assetVersion: "v1" };

  let serverHits = 0;
  const createServer = async () => ({
    prepare: async () => {},
    getRequestHandler: () => async (req, res) => {
      serverHits++;
      if (req.url === "/robots.txt") {
        res.statusCode = 404;
        res.end("not found");
        return;
      }
      res.statusCode = 200;
      res.setHeader("content-type", "text/html");
      res.end(`rendered:${req.url}`);
    },
  });

  // Static asset bypasses NextNodeServer completely.
  const staticRes = await dispatchNextServer(
    new Request("https://app.example.com/_next/static/chunks/app.js"),
    env,
    {},
    { manifest, createServer },
  );
  assert.equal(staticRes.status, 200);
  assert.equal(await staticRes.text(), "console.log('static')");
  assert.equal(serverHits, 0);

  // Dynamic route hits NextNodeServer.
  const pageRes = await dispatchNextServer(
    new Request("https://app.example.com/docs/intro"),
    env,
    {},
    { manifest, createServer },
  );
  assert.equal(pageRes.status, 200);
  assert.equal(await pageRes.text(), "rendered:/docs/intro");
  assert.equal(serverHits, 1);

  // Root public file falls back to R2 when NextNodeServer returns 404.
  const robotsRes = await dispatchNextServer(
    new Request("https://app.example.com/robots.txt"),
    env,
    {},
    { manifest, createServer },
  );
  assert.equal(robotsRes.status, 200);
  assert.equal(await robotsRes.text(), "User-agent: *");
});
