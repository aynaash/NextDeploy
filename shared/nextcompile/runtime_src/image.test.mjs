import { test, afterEach } from "node:test";
import assert from "node:assert/strict";

import { handleImageRequest } from "./image.mjs";

const manifest = {
  images: {
    remotePatterns: [{ protocol: "https", hostname: "cdn.example.com" }],
    domains: [],
    formats: ["image/avif"],
  },
};

function imgReq(rawUrlParam) {
  return new Request("https://app.example.com/_next/image?url=" + rawUrlParam);
}

const realFetch = globalThis.fetch;
afterEach(() => {
  globalThis.fetch = realFetch;
});

test("protocol-relative //evil.com is blocked (403)", async () => {
  const res = await handleImageRequest(imgReq("//evil.com/x"), {}, manifest);
  assert.equal(res.status, 403);
});

test("leading-whitespace %20https://evil.com is blocked (403)", async () => {
  const res = await handleImageRequest(imgReq("%20https://evil.com/x"), {}, manifest);
  assert.equal(res.status, 403);
});

test("non-http scheme is rejected (400)", async () => {
  const res = await handleImageRequest(imgReq("data:text/html,x"), {}, manifest);
  assert.equal(res.status, 400);
});

test("same-origin relative path passes and passthrough-fetches", async () => {
  globalThis.fetch = async () =>
    new Response("bytes", { status: 200, headers: { "content-type": "image/png" } });

  const res = await handleImageRequest(imgReq("/local/logo.png"), {}, manifest);
  assert.equal(res.status, 200);
  assert.equal(res.headers.get("x-nextcompile-image"), "passthrough");
});

test("allowlisted remote host passes", async () => {
  globalThis.fetch = async () =>
    new Response("bytes", { status: 200, headers: { "content-type": "image/png" } });

  const res = await handleImageRequest(imgReq("https://cdn.example.com/a.png"), {}, manifest);
  assert.equal(res.status, 200);
});

