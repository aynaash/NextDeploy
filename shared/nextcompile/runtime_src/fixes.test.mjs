// Tests for the deploy-pipeline security/correctness fixes in the Worker
// runtime. Run with: node --test fixes.test.mjs
import { test, afterEach } from "node:test";
import assert from "node:assert/strict";

import { handleImageRequest } from "./image.mjs";
import { serveRootPublicFromR2 } from "./serve.mjs";
import { runGuard } from "./guard.mjs";

// ── R1: image optimizer SSRF ─────────────────────────────────────────────────

const imgManifest = {
  images: {
    remotePatterns: [{ protocol: "https", hostname: "cdn.example.com" }],
    domains: [],
    formats: ["image/avif"],
  },
};

// Build the request WITHOUT re-encoding the payload, so "//evil.com" and
// " https://evil.com" reach the handler verbatim.
function imgReq(rawUrlParam) {
  return new Request("https://app.example.com/_next/image?url=" + rawUrlParam);
}

const realFetch = globalThis.fetch;
afterEach(() => {
  globalThis.fetch = realFetch;
});

test("R1: protocol-relative //evil.com is blocked (403)", async () => {
  const res = await handleImageRequest(imgReq("//evil.com/x"), {}, imgManifest);
  assert.equal(res.status, 403);
});

test("R1: leading-whitespace %20https://evil.com is blocked (403)", async () => {
  const res = await handleImageRequest(imgReq("%20https://evil.com/x"), {}, imgManifest);
  assert.equal(res.status, 403);
});

test("R1: metadata endpoint via protocol-relative is blocked", async () => {
  const res = await handleImageRequest(imgReq("//169.254.169.254/latest/meta-data"), {}, imgManifest);
  assert.equal(res.status, 403);
});

test("R1: non-http scheme is rejected (400)", async () => {
  const res = await handleImageRequest(imgReq("data:text/html,x"), {}, imgManifest);
  assert.equal(res.status, 400);
});

test("R1: same-origin relative path passes and passthrough-fetches", async () => {
  globalThis.fetch = async () => new Response("bytes", { status: 200, headers: { "content-type": "image/png" } });
  const res = await handleImageRequest(imgReq("/local/logo.png"), {}, imgManifest);
  assert.equal(res.status, 200);
  assert.equal(res.headers.get("x-nextcompile-image"), "passthrough");
});

test("R1: allowlisted remote host passes", async () => {
  globalThis.fetch = async () => new Response("bytes", { status: 200, headers: { "content-type": "image/png" } });
  const res = await handleImageRequest(imgReq("https://cdn.example.com/a.png"), {}, imgManifest);
  assert.equal(res.status, 200);
});

// ── R2: prerendered page HTML not served at its bare .html key ────────────────

function fakeAssets(store) {
  return {
    async get(key) {
      if (!(key in store)) return null;
      return {
        body: store[key],
        writeHttpMetadata() {},
        httpEtag: '"x"',
      };
    },
  };
}

const r2Manifest = { routes: { ssg: { "/dashboard": "dashboard.html" }, isr: {} } };

test("R2: /dashboard.html (an SSG page target) is NOT served by root-public", async () => {
  const env = { ASSETS: fakeAssets({ "dashboard.html": "SECRET PAGE", "robots.txt": "ok" }) };
  const res = await serveRootPublicFromR2(env, "/dashboard.html", r2Manifest);
  assert.equal(res, null, "page HTML must not be reachable at its bare .html key (guard bypass)");
});

test("R2: a genuine public/ file is still served", async () => {
  const env = { ASSETS: fakeAssets({ "dashboard.html": "SECRET PAGE", "robots.txt": "ok" }) };
  const res = await serveRootPublicFromR2(env, "/robots.txt", r2Manifest);
  assert.ok(res && res.status === 200, "robots.txt should still serve");
});

// ── R4: rate limiter fails CLOSED when its KV binding is missing ──────────────

test("R4: rate_limit configured but binding unbound → 503 (fail closed)", async () => {
  const cfg = {
    publicPaths: [],
    rateLimit: { kvBinding: "RATE_LIMIT", requestsPerMinute: 1, paths: ["/*"] },
  };
  // env intentionally omits RATE_LIMIT.
  const req = new Request("https://app.example.com/api/x", { headers: { "cf-connecting-ip": "1.2.3.4" } });
  const res = await runGuard(req, {}, null, cfg, 1000);
  assert.equal(res?.status, 503, "missing rate-limit binding must fail closed, not silently un-limit");
});
