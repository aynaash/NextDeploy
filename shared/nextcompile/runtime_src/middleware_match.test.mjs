import { test } from "node:test";
import assert from "node:assert";
import { middlewareMatches } from "./middleware_match.mjs";

const req = (url, headers = {}) => new Request(url, { headers });

test("empty matchers → runs (no regression)", () => {
  assert.equal(middlewareMatches(req("https://x.com/anything"), []), true);
  assert.equal(middlewareMatches(req("https://x.com/anything"), undefined), true);
  assert.equal(middlewareMatches(req("https://x.com/anything"), null), true);
});

test("path glob gates", () => {
  const m = [{ pathname: "/app/*" }];
  assert.equal(middlewareMatches(req("https://x.com/app/settings"), m), true);
  assert.equal(middlewareMatches(req("https://x.com/app"), m), true);
  assert.equal(middlewareMatches(req("https://x.com/public/logo.png"), m), false);
});

test("compiled regex pattern wins over pathname", () => {
  const m = [{ pathname: "/never/*", pattern: "^/api/.*" }];
  assert.equal(middlewareMatches(req("https://x.com/api/users"), m), true);
  assert.equal(middlewareMatches(req("https://x.com/never/x"), m), false);
});

test("malformed pattern falls back to pathname, does not throw", () => {
  const m = [{ pathname: "/app/*", pattern: "^/api/([" }];
  assert.equal(middlewareMatches(req("https://x.com/app/x"), m), true);
});

test("has cookie present", () => {
  const m = [{ pathname: "/app/*", has: [{ type: "cookie", key: "session" }] }];
  assert.equal(middlewareMatches(req("https://x.com/app", { cookie: "session=abc" }), m), true);
  assert.equal(middlewareMatches(req("https://x.com/app"), m), false);
});

test("cookie parsing handles multiple pairs and spacing", () => {
  const m = [{ pathname: "/*", has: [{ type: "cookie", key: "session", value: "abc" }] }];
  assert.equal(middlewareMatches(req("https://x.com/", { cookie: "a=1; session=abc; b=2" }), m), true);
  assert.equal(middlewareMatches(req("https://x.com/", { cookie: "a=1; sessionx=abc" }), m), false);
});

test("missing cookie enforced", () => {
  const m = [{ pathname: "/login", missing: [{ type: "cookie", key: "session" }] }];
  assert.equal(middlewareMatches(req("https://x.com/login"), m), true);
  assert.equal(middlewareMatches(req("https://x.com/login", { cookie: "session=abc" }), m), false);
});

test("header value exact-match", () => {
  const m = [{ pathname: "/*", has: [{ type: "header", key: "x-env", value: "prod" }] }];
  assert.equal(middlewareMatches(req("https://x.com/", { "x-env": "prod" }), m), true);
  assert.equal(middlewareMatches(req("https://x.com/", { "x-env": "dev" }), m), false);
  assert.equal(middlewareMatches(req("https://x.com/"), m), false);
});

test("query + host conditions", () => {
  const q = [{ pathname: "/*", has: [{ type: "query", key: "beta", value: "1" }] }];
  assert.equal(middlewareMatches(req("https://x.com/?beta=1"), q), true);
  assert.equal(middlewareMatches(req("https://x.com/?beta=0"), q), false);

  const h = [{ pathname: "/*", has: [{ type: "host", key: "", value: "admin.x.com" }] }];
  assert.equal(middlewareMatches(req("https://admin.x.com/"), h), true);
  assert.equal(middlewareMatches(req("https://x.com/"), h), false);
});

test("unknown condition type never satisfies has, never triggers missing", () => {
  const has = [{ pathname: "/*", has: [{ type: "bogus", key: "k" }] }];
  assert.equal(middlewareMatches(req("https://x.com/"), has), false);
  const missing = [{ pathname: "/*", missing: [{ type: "bogus", key: "k" }] }];
  assert.equal(middlewareMatches(req("https://x.com/"), missing), true);
});

test("all legs of a rule must hold (conjunction within a rule)", () => {
  const m = [
    {
      pathname: "/dashboard/*",
      has: [{ type: "cookie", key: "session" }],
      missing: [{ type: "header", key: "x-skip" }],
    },
  ];
  assert.equal(middlewareMatches(req("https://x.com/dashboard", { cookie: "session=a" }), m), true);
  assert.equal(
    middlewareMatches(req("https://x.com/dashboard", { cookie: "session=a", "x-skip": "1" }), m),
    false,
  );
  assert.equal(middlewareMatches(req("https://x.com/other", { cookie: "session=a" }), m), false);
});

test("disjunction across rules", () => {
  const m = [{ pathname: "/a/*" }, { pathname: "/b/*" }];
  assert.equal(middlewareMatches(req("https://x.com/b/1"), m), true);
  assert.equal(middlewareMatches(req("https://x.com/c/1"), m), false);
});
