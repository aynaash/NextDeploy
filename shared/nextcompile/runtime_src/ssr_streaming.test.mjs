// Tests for the M3 streaming scaffold. Run with: node --test ssr_streaming.test.mjs
//
// The scaffold is unwired by design, so these assert its CONTRACT: that it
// suspends rather than awaits, and that it refuses to run without the vendored
// React that M3 must add first. A fake React stands in — the real one isn't
// vendored yet, which is precisely what M3 is blocked on.
import { test } from "node:test";
import assert from "node:assert";

import { createStreamingRoot, renderToHtmlStreaming } from "./ssr_streaming.dev.mjs";

function fakeReact() {
  return {
    use: (p) => ({ __used: p }),
    createElement: (type) => ({ __element: type }),
  };
}

test("createStreamingRoot builds an element that use()s the pending root", () => {
  const React = fakeReact();
  const pending = Promise.resolve("root");
  const el = createStreamingRoot(React, pending);
  assert.ok(el.__element, "expected a React element");
  assert.deepStrictEqual(el.__element(), { __used: pending });
});

test("createStreamingRoot rejects a React without use() — the M3 prerequisite", () => {
  assert.throws(() => createStreamingRoot(null, Promise.resolve()), /vendored standard-condition React/);
  assert.throws(
    () => createStreamingRoot({ createElement: () => {} }, Promise.resolve()),
    /use\(\)/,
  );
});

// The whole point of M3: the Flight root must NOT be awaited before Fizz starts,
// or Suspense boundaries have nothing left to stream.
test("renderToHtmlStreaming does not await the flight root before rendering", async () => {
  let resolveFlight;
  const neverYet = new Promise((r) => {
    resolveFlight = r;
  });
  let renderCalled = false;

  const deps = {
    client: { createFromReadableStream: () => neverYet },
    dom: {
      renderToReadableStream: async () => {
        renderCalled = true;
        return "html-stream";
      },
    },
  };

  const out = await renderToHtmlStreaming(deps, fakeReact(), null, { scripts: [] }, {});
  assert.strictEqual(out, "html-stream");
  assert.ok(renderCalled, "renderToReadableStream must run while Flight is still pending");
  resolveFlight("late");
});

test("renderToHtmlStreaming forwards bootstrap scripts and omits an empty list", async () => {
  const seen = [];
  const deps = {
    client: { createFromReadableStream: () => Promise.resolve("r") },
    dom: {
      renderToReadableStream: async (_el, opts) => {
        seen.push(opts);
        return "s";
      },
    },
  };
  await renderToHtmlStreaming(deps, fakeReact(), null, { scripts: ["/a.js"] }, {});
  assert.deepStrictEqual(seen[0].bootstrapScripts, ["/a.js"]);

  await renderToHtmlStreaming(deps, fakeReact(), null, { scripts: [] }, {});
  assert.strictEqual(seen[1].bootstrapScripts, undefined, "empty list must be omitted, not passed");
});
