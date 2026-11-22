// Unit tests for the nextserver Worker runtime (VFS, HTTP bridge, CacheHandler, driver).
// Run with: node --test shared/nextcompile/runtime_src/nextserver/nextserver.test.mjs

import { test, beforeEach } from "node:test";
import assert from "node:assert/strict";

import fsShim, { normalizeVFSPath, readFileSync, existsSync, statSync, readdirSync, promises as fsPromises } from "./vfs_fs_shim.mjs";
import { invokeNodeHandler } from "./http_bridge.mjs";
import { CloudflareCacheHandler, resetLocalCacheForTests } from "./cf_cache_handler.mjs";
import {
  ensureProcessCompat,
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

test("CloudflareCacheHandler loads build-time prerendered HTML and RSC payloads from R2 release assets", async () => {
  const releaseStore = new Map([
    ["_nextdeploy/releases/v1/docs/advanced/nextcompile.html", { body: "<h1>NextCompile</h1>" }],
    ["_nextdeploy/releases/v1/docs/advanced/nextcompile.rsc", { body: "0:[\"$\",\"h1\",null,{\"children\":\"NextCompile\"}]\n" }],
    ["_nextdeploy/releases/v1/_not-found.html", { body: "<h1>Not Found</h1>" }],
  ]);
  globalThis.__nextdeployEnv = {
    ASSETS: {
      get: async (k) => releaseStore.get(k) || null,
    },
  };
  const handler = new CloudflareCacheHandler({ assetVersion: "v1" });
  const hit = await handler.get("/docs/advanced/nextcompile", { kind: "APP_PAGE" });
  assert.equal(hit?.value?.kind, "APP_PAGE");
  assert.equal(hit?.value?.html, "<h1>NextCompile</h1>");
  assert.equal(hit?.value?.status, undefined);
  assert.ok(Buffer.isBuffer(hit?.value?.rscData));
  assert.equal(hit.value.rscData.toString("utf8"), "0:[\"$\",\"h1\",null,{\"children\":\"NextCompile\"}]\n");

  // /_not-found must preserve HTTP 404 status so NextNodeServer never returns 200 for missing routes.
  const notFoundHit = await handler.get("/_not-found", { kind: "APP_PAGE" });
  assert.equal(notFoundHit?.value?.status, 404);

  // FETCH cache lookups must never collide with release HTML files.
  resetLocalCacheForTests();
  assert.equal(await handler.get("/docs/advanced/nextcompile", { kind: "FETCH" }), null);
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

test("dispatchNextServer serves SSG and fresh ISR HTML from R2, bypassing for RSC Flight and expired ISR", async () => {
  const freshDate = new Date();
  const staleDate = new Date(Date.now() - 120_000);
  const r2Assets = {
    "_nextdeploy/releases/v1/docs/advanced/nextcompile.html": {
      body: "<h1>Pre-rendered SSG Doc</h1>",
      writeHttpMetadata() {},
    },
    "_nextdeploy/releases/v1/blog/fresh.html": {
      body: "<h1>Fresh ISR</h1>",
      uploaded: freshDate,
      writeHttpMetadata() {},
    },
    "_nextdeploy/releases/v1/blog/stale.html": {
      body: "<h1>Stale ISR</h1>",
      uploaded: staleDate,
      writeHttpMetadata() {},
    },
  };
  const env = {
    ASSETS: {
      get: async (k) => r2Assets[k] || null,
    },
  };
  const manifest = {
    assetVersion: "v1",
    routes: {
      ssg: {
        "/docs/advanced/nextcompile": "/docs/advanced/nextcompile.html",
      },
      isr: {
        "/blog/fresh": "/blog/fresh.html",
        "/blog/stale": "/blog/stale.html",
      },
    },
    isr: {
      intervals: {
        "/blog/fresh": 60,
        "/blog/stale": 60,
      },
    },
  };

  let serverHits = 0;
  const createServer = async () => ({
    prepare: async () => {},
    getRequestHandler: () => async (req, res) => {
      serverHits++;
      res.statusCode = 200;
      res.setHeader("content-type", "text/html");
      res.end(`server:${req.url}`);
    },
  });

  // 1. SSG route GET hits R2 directly, 0 NextNodeServer hits.
  const ssgRes = await dispatchNextServer(
    new Request("https://app.example.com/docs/advanced/nextcompile"),
    env,
    {},
    { manifest, createServer },
  );
  assert.equal(ssgRes.status, 200);
  assert.equal(await ssgRes.text(), "<h1>Pre-rendered SSG Doc</h1>");
  assert.equal(serverHits, 0);

  // 2. RSC Flight request on the same SSG route falls through to NextNodeServer.
  const rscRes = await dispatchNextServer(
    new Request("https://app.example.com/docs/advanced/nextcompile", {
      headers: { rsc: "1" },
    }),
    env,
    {},
    { manifest, createServer },
  );
  assert.equal(rscRes.status, 200);
  assert.equal(await rscRes.text(), "server:/docs/advanced/nextcompile");
  assert.equal(serverHits, 1);

  // 3. Fresh ISR serves straight from R2 without invoking NextNodeServer.
  const freshRes = await dispatchNextServer(
    new Request("https://app.example.com/blog/fresh"),
    env,
    {},
    { manifest, createServer },
  );
  assert.equal(freshRes.status, 200);
  assert.equal(await freshRes.text(), "<h1>Fresh ISR</h1>");
  assert.equal(serverHits, 1);

  // 4. Expired ISR falls through to NextNodeServer for regeneration.
  const staleRes = await dispatchNextServer(
    new Request("https://app.example.com/blog/stale"),
    env,
    {},
    { manifest, createServer },
  );
  assert.equal(staleRes.status, 200);
  assert.equal(await staleRes.text(), "server:/blog/stale");
  assert.equal(serverHits, 2);
});

test("ensureProcessCompat polyfills minimal workerd process and replaces throwing unenv stubs", () => {
  const workerdLikeProcess = {
    env: {},
    cwd() {
      throw new Error("[unenv] process.cwd is not implemented yet!");
    },
    uptime() {
      throw new Error("[unenv] process.uptime is not implemented yet!");
    },
  };
  ensureProcessCompat(workerdLikeProcess);
  assert.deepEqual(workerdLikeProcess.listeners("unhandledRejection"), []);
  assert.equal(workerdLikeProcess.on("exit", () => {}), workerdLikeProcess);
  assert.equal(workerdLikeProcess.cwd(), "/");
  assert.equal(workerdLikeProcess.uptime(), 0);
  assert.equal(typeof workerdLikeProcess.hrtime.bigint(), "bigint");
  assert.equal(workerdLikeProcess.stdout.isTTY, false);
  assert.equal(workerdLikeProcess.versions.node, "22.0.0");
});
