// Server Action EXECUTION tests — the recipe verified against real Next 14.2 /
// 15.1 builds: mod.__next_app__.require(moduleId)[actionId] resolves the action,
// and mod.renderToReadableStream encodes the Flight reply. Run: node --test
import { test } from "node:test";
import assert from "node:assert/strict";

import { resolveActionFn, encodeFlightReply, handleServerAction } from "./actions.mjs";

// A module shaped exactly like a compiled Next page module: __next_app__.require
// returns the action worker module (keyed by actionId), plus a Flight encoder.
function fakePageModule({ wrapInDefault = false } = {}) {
  const actionWorker = {
    "act-add": async (a, b) => ({ sum: a + b }),
    "act-greet": async (formData) => ({ hello: formData.get("name") }),
  };
  const exportsObj = {
    __next_app__: { require: (id) => (id === "42" ? actionWorker : {}) },
    // Minimal stand-in for react-server-dom-webpack's renderToReadableStream:
    // emit the value as a Flight row-0 line, which is what the real encoder does.
    renderToReadableStream: (value) =>
      new Response(`0:${JSON.stringify(value)}\n`).body,
  };
  return wrapInDefault ? { default: exportsObj } : exportsObj;
}

test("resolveActionFn: require() shape (direct exports)", () => {
  const mod = fakePageModule();
  assert.equal(typeof resolveActionFn(mod, "42", "act-add"), "function");
});

test("resolveActionFn: import() shape (exports under .default)", () => {
  const mod = fakePageModule({ wrapInDefault: true });
  assert.equal(typeof resolveActionFn(mod, "42", "act-add"), "function");
});

test("resolveActionFn: unknown moduleId / actionId / missing __next_app__ → undefined", () => {
  const mod = fakePageModule();
  assert.equal(resolveActionFn(mod, "999", "act-add"), undefined);
  assert.equal(resolveActionFn(mod, "42", "nope"), undefined);
  assert.equal(resolveActionFn({}, "42", "act-add"), undefined);
});

test("encodeFlightReply: encodes via the module's renderToReadableStream", async () => {
  const mod = fakePageModule();
  const stream = encodeFlightReply(mod, { sum: 5 });
  assert.ok(stream, "expected a stream");
  const text = await new Response(stream).text();
  assert.equal(text, '0:{"sum":5}\n');
});

test("encodeFlightReply: null when the encoder is absent", () => {
  assert.equal(encodeFlightReply({ __next_app__: {} }, { x: 1 }), null);
});

test("handleServerAction: resolves, executes, and returns a Flight reply", async () => {
  const actionId = "act-add";
  const manifest = { actions: { [actionId]: { module: "app/page", export: "42", runtime: "node" } } };
  const loaders = { "app/page": async () => fakePageModule({ wrapInDefault: true }) };

  // Same-origin POST with the Next-Action header and a JSON arg body.
  const req = new Request("https://app.example.com/", {
    method: "POST",
    headers: {
      "next-action": actionId,
      "content-type": "application/json",
      origin: "https://app.example.com",
      host: "app.example.com",
    },
    body: JSON.stringify([2, 3]),
  });

  const res = await handleServerAction(req, {}, { waitUntil() {} }, manifest, loaders);
  assert.equal(res.status, 200);
  assert.equal(res.headers.get("content-type"), "text/x-component");
  const text = await res.text();
  assert.equal(text, '0:{"sum":5}\n', "action executed and its result was Flight-encoded");
});
