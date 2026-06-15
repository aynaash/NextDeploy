// Tests for static R2 asset serving. Run with: node --test serve.test.mjs
import { test } from "node:test";
import assert from "node:assert";

import { serveStaticFromR2, serveRootPublicFromR2 } from "./serve.mjs";

// Records the key passed to ASSETS.get so we can assert what R2 was queried for.
function mockEnv(stored = {}) {
  const calls = [];
  return {
    calls,
    env: {
      ASSETS: {
        get: async (key) => {
          calls.push(key);
          return stored[key] ? { body: stored[key], writeHttpMetadata() {} } : null;
        },
      },
    },
  };
}

// The regression: route-group / dynamic-segment chunks are requested URL-encoded
// (".../[...slug]/..." → ".../%5B...slug%5D/..."), but R2 stores the literal key.
// Without decoding, the GET misses → 404 → ChunkLoadError → client-side exception.
test("serveStaticFromR2 decodes %5B...%5D before the R2 lookup", async () => {
  const { env, calls } = mockEnv();
  const encoded = "/_next/static/chunks/app/(docs)/docs/%5B...slug%5D/page-abc.js";
  await serveStaticFromR2(env, encoded);
  assert.strictEqual(
    calls[0],
    "_next/static/chunks/app/(docs)/docs/[...slug]/page-abc.js",
  );
});

test("serveStaticFromR2 serves a decoded key and 200s", async () => {
  const key = "_next/static/chunks/app/[id]/page-x.js";
  const { env } = mockEnv({ [key]: "console.log(1)" });
  const res = await serveStaticFromR2(env, "/_next/static/chunks/app/%5Bid%5D/page-x.js");
  assert.ok(res, "expected a Response for an existing (encoded-request) asset");
  assert.strictEqual(res.status, 200);
});

test("serveRootPublicFromR2 also decodes the key", async () => {
  const { env, calls } = mockEnv();
  await serveRootPublicFromR2(env, "/my%20file.txt");
  assert.strictEqual(calls[0], "my file.txt");
});

test("a malformed percent-escape falls back to the raw key, never throws", async () => {
  const { env, calls } = mockEnv();
  const res = await serveStaticFromR2(env, "/_next/static/chunks/bad%E0%A4%A.js");
  assert.strictEqual(res, null); // miss, but no throw
  assert.strictEqual(calls[0], "_next/static/chunks/bad%E0%A4%A.js");
});
