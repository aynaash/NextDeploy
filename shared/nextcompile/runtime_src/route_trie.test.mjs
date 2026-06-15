// Tests for the segment-trie route matcher. Run with:
//   node --test route_trie.test.mjs
//
// Strategy: the existing linear scan (matchDynamic) is the oracle. We port
// dispatch.go's routeToRegex + dynamicSpecificity to JS to build a realistic,
// specificity-sorted dynamicTable, then assert matchTrie returns the SAME
// entry + params as matchDynamic over a corpus. The route corpus is
// deliberately non-overlapping (no path matches two routes), so the result is
// independent of how ties are sorted — which is what makes trie==oracle a
// sound equivalence claim rather than a coincidence of sort order.

import { test } from "node:test";
import assert from "node:assert/strict";

import { matchDynamic } from "./route_match.mjs";
import { buildRouteTrie, matchTrie } from "./route_trie.dev.mjs";

// --- faithful JS ports of dispatch.go ---------------------------------------

function escapeRegexLiteral(s) {
  return s.replace(/[.+*?()$^|]/g, (c) => "\\" + c);
}

// Port of routeToRegex: Next route pattern -> { re, paramNames }.
function routeToRegex(route) {
  const parts = route.replace(/^\/|\/$/g, "").split("/").filter(Boolean);
  const paramNames = [];
  let src = "^";
  for (const part of parts) {
    src += "\\/";
    if (part.startsWith("[[...") && part.endsWith("]]")) {
      src = src.slice(0, -2); // undo the just-written \/
      src += "(?:\\/(.+?))?";
      paramNames.push(part.slice(5, -2));
    } else if (part.startsWith("[...") && part.endsWith("]")) {
      src += "(.+?)";
      paramNames.push(part.slice(4, -1));
    } else if (part.startsWith("[") && part.endsWith("]")) {
      src += "([^/]+)";
      paramNames.push(part.slice(1, -1));
    } else {
      src += escapeRegexLiteral(part);
    }
  }
  src += "\\/?$";
  return { re: new RegExp(src), paramNames };
}

// Port of dynamicSpecificity.
function dynamicSpecificity(route) {
  const parts = route.replace(/^\/|\/$/g, "").split("/").filter(Boolean);
  let score = parts.length * 10;
  for (const p of parts) {
    if (p.startsWith("[[...")) score -= 5;
    else if (p.startsWith("[...")) score -= 4;
    else if (p.startsWith("[")) score -= 1;
  }
  return score;
}

// Build a dynamicTable entry array from route strings, sorted most-specific
// first, matching what dispatch.go emits.
function buildTable(routes) {
  return routes
    .map((route) => {
      const { re, paramNames } = routeToRegex(route);
      return { route, pattern: re, paramNames, load: () => route };
    })
    .sort((a, b) => dynamicSpecificity(b.route) - dynamicSpecificity(a.route));
}

// Non-overlapping route corpus: no pathname matches more than one route.
const ROUTES = [
  "/blog/[slug]",
  "/blog/[slug]/comments",
  "/shop/[...path]",
  "/docs/[[...slug]]",
  "/users/[id]/settings",
  "/api/[version]/[resource]",
  "/files/[...path]",
];

const PATHS = [
  "/blog/hello-world",
  "/blog/hello/comments",
  "/shop/a/b/c",
  "/shop/single",
  "/docs",
  "/docs/intro/setup",
  "/users/42/settings",
  "/api/v2/posts",
  "/files/x/y",
  "/blog/hello/", // optional trailing slash
  "/blog/hello%20world", // percent-encoded segment
  "/nonexistent", // no match
  "/blog", // dynamic needs a slug -> no match
  "/blog/a/b/c", // too deep -> no match
  "/api/v2", // missing a segment -> no match
];

// --- equivalence: trie == linear scan ---------------------------------------

test("matchTrie matches matchDynamic across the corpus", () => {
  const table = buildTable(ROUTES);
  const trie = buildRouteTrie(table);

  for (const path of PATHS) {
    const want = matchDynamic(path, table);
    const got = matchTrie(path, trie);

    if (want === null) {
      assert.equal(got, null, `expected no match for ${path}`);
      continue;
    }
    assert.notEqual(got, null, `expected a match for ${path}`);
    // Same entry object and identical params.
    assert.equal(got.entry, want.entry, `entry mismatch for ${path}`);
    assert.deepEqual(got.params, want.params, `params mismatch for ${path}`);
  }
});

// --- trie-specific semantics -------------------------------------------------

test("optional catch-all matching zero segments yields no param", () => {
  const trie = buildRouteTrie(buildTable(["/docs/[[...slug]]"]));
  const r = matchTrie("/docs", trie);
  assert.deepEqual(r.params, {});
});

test("catch-all joins remaining segments into one decoded param", () => {
  const trie = buildRouteTrie(buildTable(["/shop/[...path]"]));
  const r = matchTrie("/shop/a/b%2Fc", trie);
  assert.equal(r.params.path, "a/b/c"); // %2F decodes inside the joined value
});

test("literal beats dynamic at the same position (deterministic tie-break)", () => {
  // These two routes both match /a/b. The trie deterministically prefers the
  // literal edge ('a') -> /a/[x]; the linear scan would resolve this tie by
  // unstable sort, so this is the trie's documented improvement.
  const trie = buildRouteTrie(buildTable(["/a/[x]", "/[y]/b"]));
  const r = matchTrie("/a/b", trie);
  assert.equal(r.entry.route, "/a/[x]");
  assert.deepEqual(r.params, { x: "b" });
});

test("multi-param route extracts params in route order", () => {
  const trie = buildRouteTrie(buildTable(["/api/[version]/[resource]"]));
  const r = matchTrie("/api/v3/users", trie);
  assert.deepEqual(r.params, { version: "v3", resource: "users" });
});
