// Tests for Server Action body parsing. Run with: node --test actions.test.mjs
import { test } from "node:test";
import assert from "node:assert/strict";

import { parseArgs, BodyTooLargeError } from "./actions.mjs";

const CAP = 2 * 1024 * 1024;

function multipartReq(contentLength) {
  const headers = { "content-type": "multipart/form-data; boundary=x" };
  if (contentLength != null) headers["content-length"] = String(contentLength);
  // Body is never read for the over-cap / missing-length cases — the gate
  // rejects first.
  return new Request("https://app.example.com/act", { method: "POST", headers, body: "--x--" });
}

test("multipart over the cap throws BodyTooLargeError", async () => {
  await assert.rejects(
    () => parseArgs(multipartReq(CAP + 1)),
    (err) => err instanceof BodyTooLargeError && err.status === 413,
  );
});

test("multipart with no content-length is rejected", async () => {
  await assert.rejects(
    () => parseArgs(multipartReq(null)),
    (err) => err instanceof BodyTooLargeError,
  );
});

test("multipart under the cap parses to a single args object", async () => {
  const form = new FormData();
  form.append("name", "ada");
  const req = new Request("https://app.example.com/act", { method: "POST", body: form });
  // FormData bodies set their own content-length; assert it's present + small.
  assert.ok(Number(req.headers.get("content-length")) <= CAP);
  const args = await parseArgs(req);
  assert.deepEqual(args, [{ name: "ada" }]);
});

test("json over the cap also throws BodyTooLargeError", async () => {
  const big = "x".repeat(CAP + 1);
  const req = new Request("https://app.example.com/act", {
    method: "POST",
    headers: { "content-type": "application/json" },
    body: JSON.stringify([big]),
  });
  await assert.rejects(() => parseArgs(req), (err) => err instanceof BodyTooLargeError);
});