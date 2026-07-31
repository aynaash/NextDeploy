// Tests for rsc.mjs's diagnostic surface. Run with: node --test rsc.test.mjs
//
// renderRSC itself needs the vendored React builds and so stays
// integration-verified; these cover the pure helpers that decide whether it can
// run at all — the path most likely to be hit on a first real deploy.
import { test } from "node:test";
import assert from "node:assert";

import { resolveComponent, describeMissingComponent } from "./rsc.mjs";

test("resolveComponent finds the component on default/Page/Component", () => {
  const fn = () => null;
  assert.strictEqual(resolveComponent({ default: fn }), fn);
  assert.strictEqual(resolveComponent({ Page: fn }), fn);
  assert.strictEqual(resolveComponent({ Component: fn }), fn);
  assert.strictEqual(resolveComponent({ default: fn, Page: () => 1 }), fn, "default wins");
});

test("resolveComponent is undefined for modules with none of them", () => {
  assert.strictEqual(resolveComponent({}), undefined);
  assert.strictEqual(resolveComponent(null), undefined);
  assert.strictEqual(resolveComponent(undefined), undefined);
});

// The highest-probability first failure of the whole RSC path: a production
// `next build` emitting Next's internal route module rather than a component.
// The message has to name that specifically or it reads as a config error.
test("describeMissingComponent identifies Next's internal route module", () => {
  const out = describeMissingComponent(
    { routeModule: {}, tree: [], workAsyncStorage: {} },
    { compiled: "server/app/page.js" },
  );
  assert.ok(out.includes("INTERNAL route module"), out);
  assert.ok(out.includes("routeModule"), out);
  assert.ok(out.includes("tree"), out);
  assert.ok(out.includes("server/app/page.js"), out);
  assert.ok(out.includes("yussuf.md"), out, "should point at the handoff note");
});

test("describeMissingComponent falls back to a generic explanation", () => {
  const out = describeMissingComponent({ somethingElse: 1 }, { compiled: "server/app/x/page.js" });
  assert.ok(out.includes("Expected a component on `default`"), out);
  assert.ok(out.includes("somethingElse"), out);
  assert.ok(!out.includes("INTERNAL route module"), out);
});

test("describeMissingComponent survives a null module and a missing entry", () => {
  const out = describeMissingComponent(null, undefined);
  assert.ok(out.includes("(none)"), out);
  assert.ok(out.includes("(unknown)"), out);
});
