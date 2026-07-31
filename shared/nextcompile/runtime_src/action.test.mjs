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

// A string body makes the runtime set Content-Length for you, so exercising
// the streaming path needs a ReadableStream — which is also what a real
// chunked upload looks like.
function chunkedReq(contentType, chunks) {
  const body = new ReadableStream({
    start(c) {
      for (const chunk of chunks) c.enqueue(new TextEncoder().encode(chunk));
      c.close();
    },
  });
  return new Request("https://app.example.com/act", {
    method: "POST",
    headers: { "content-type": contentType },
    body,
    duplex: "half",
  });
}

// A chunked upload carries no Content-Length, so the header gate can't see it.
// The cap must be enforced while streaming — and it must abort mid-read rather
// than buffer the whole thing and check afterwards, which is what makes it a
// DoS guard rather than a report.
test("chunked body over the cap is rejected without buffering it all", async () => {
  const chunk = "x".repeat(64 * 1024);
  const chunkCount = Math.ceil(CAP / chunk.length) + 4;
  let produced = 0;
  const body = new ReadableStream({
    pull(c) {
      if (produced >= chunkCount) return void c.close();
      produced++;
      c.enqueue(new TextEncoder().encode(chunk));
    },
  });
  const req = new Request("https://app.example.com/act", {
    method: "POST",
    headers: { "content-type": "text/plain" },
    body,
    duplex: "half",
  });
  assert.equal(req.headers.get("content-length"), null, "precondition: no content-length");

  await assert.rejects(
    () => parseArgs(req),
    (err) => err instanceof BodyTooLargeError && err.status === 413,
  );
  // It stopped early instead of draining the producer.
  assert.ok(produced < chunkCount, `read ${produced}/${chunkCount} chunks — should have aborted early`);
});

// The flip side: a legitimate chunked body under the cap must still parse.
// Rejecting every request without a Content-Length would break real traffic.
test("chunked body under the cap parses normally", async () => {
  const req = chunkedReq("application/json", ['[{"name":', '"ada"}]']);
  assert.equal(req.headers.get("content-length"), null, "precondition: no content-length");
  assert.deepEqual(await parseArgs(req), [{ name: "ada" }]);
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