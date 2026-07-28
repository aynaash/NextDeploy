// Tests for the edge protection guard. Run with: node --test guard.test.mjs
// Uses Node's built-in test runner + Web Crypto (global in Node 18+), matching
// the Workers runtime the guard actually runs in.

import { test } from "node:test";
import assert from "node:assert/strict";

import {
  pathMatches,
  matchesAny,
  ipDenied,
  ipMatches,
  readCookie,
  clientIP,
  timingSafeEqual,
  signSessionCookie,
  verifySessionCookie,
  checkRateLimit,
  runGuard,
} from "./guard.mjs";

// --- pathMatches -------------------------------------------------------------

test("pathMatches: exact", () => {
  assert.ok(pathMatches("/login", "/login"));
  assert.ok(!pathMatches("/login", "/signin"));
});

test("pathMatches: trailing /* matches prefix and children", () => {
  assert.ok(pathMatches("/app", "/app/*"));
  assert.ok(pathMatches("/app/settings", "/app/*"));
  assert.ok(pathMatches("/app/a/b", "/app/*"));
  assert.ok(!pathMatches("/application", "/app/*"));
});

test("pathMatches: bare wildcard and middle wildcard", () => {
  assert.ok(pathMatches("/anything/here", "/*"));
  assert.ok(pathMatches("/api/webhooks/stripe", "/api/*/stripe"));
  assert.ok(!pathMatches("/api/webhooks/paypal", "/api/*/stripe"));
});

test("pathMatches: dots are literal, not regex", () => {
  assert.ok(pathMatches("/favicon.ico", "/favicon.ico"));
  assert.ok(!pathMatches("/faviconxico", "/favicon.ico"));
});

test("matchesAny", () => {
  assert.ok(matchesAny("/_next/static/x.js", ["/_next/*", "/login"]));
  assert.ok(!matchesAny("/app", ["/_next/*", "/login"]));
  assert.ok(!matchesAny("/app", undefined));
});

// --- ipDenied ----------------------------------------------------------------

test("ipDenied: denylist blocks", () => {
  assert.ok(ipDenied("1.2.3.4", { deny: ["1.2.3.4"] }));
  assert.ok(!ipDenied("1.2.3.5", { deny: ["1.2.3.4"] }));
});

test("ipDenied: non-empty allowlist is exclusive", () => {
  assert.ok(!ipDenied("9.9.9.9", { allow: ["9.9.9.9"] }));
  assert.ok(ipDenied("8.8.8.8", { allow: ["9.9.9.9"] }));
});

test("ipDenied: deny wins over allow", () => {
  assert.ok(ipDenied("9.9.9.9", { allow: ["9.9.9.9"], deny: ["9.9.9.9"] }));
});

test("ipDenied: empty config allows all", () => {
  assert.ok(!ipDenied("1.1.1.1", {}));
});

test("ipDenied: CIDR entries match, not just exact IPs", () => {
  // Operators write allow: ["10.0.0.0/8"] and expect it to work; before CIDR
  // support this silently matched nothing, so the allowlist denied everyone.
  assert.ok(!ipDenied("10.4.5.6", { allow: ["10.0.0.0/8"] }));
  assert.ok(ipDenied("11.4.5.6", { allow: ["10.0.0.0/8"] }));
  assert.ok(ipDenied("192.168.1.7", { deny: ["192.168.0.0/16"] }));
  assert.ok(!ipDenied("192.169.1.7", { deny: ["192.168.0.0/16"] }));
});

test("ipDenied: unknown IP cannot satisfy an allowlist", () => {
  // "" means no trustworthy header was available. It must fail closed against
  // a non-empty allowlist rather than sneak through as a non-match.
  assert.ok(ipDenied("", { allow: ["1.2.3.4"] }));
  assert.ok(!ipDenied("", {}));
});

test("ipMatches: boundaries and malformed rules", () => {
  assert.ok(ipMatches("1.2.3.4", "1.2.3.4"));
  assert.ok(ipMatches("1.2.3.4", "1.2.3.4/32"));
  assert.ok(ipMatches("255.255.255.255", "0.0.0.0/0"));
  assert.ok(ipMatches("10.255.255.255", "10.0.0.0/8"));
  assert.ok(!ipMatches("10.0.0.1", "10.0.0.0/33")); // out-of-range prefix
  assert.ok(!ipMatches("10.0.0.1", "10.0.0.0/abc"));
  assert.ok(!ipMatches("10.0.0.1", "not-an-ip/8"));
  assert.ok(!ipMatches("999.0.0.1", "999.0.0.0/8")); // octet > 255
  assert.ok(!ipMatches("", "10.0.0.0/8"));
  // IPv6 has no CIDR support here; exact match still works.
  assert.ok(ipMatches("2001:db8::1", "2001:db8::1"));
  assert.ok(!ipMatches("2001:db8::2", "2001:db8::/32"));
});

// --- readCookie / clientIP ---------------------------------------------------

test("readCookie", () => {
  assert.equal(readCookie("a=1; session=abc.def; b=2", "session"), "abc.def");
  assert.equal(readCookie("", "session"), "");
  assert.equal(readCookie("other=1", "session"), "");
});

test("clientIP prefers cf-connecting-ip regardless of trust config", () => {
  const req = new Request("https://x/", {
    headers: { "cf-connecting-ip": "1.1.1.1", "x-forwarded-for": "2.2.2.2" },
  });
  assert.equal(clientIP(req, {}), "1.1.1.1");
  assert.equal(clientIP(req, { trustForwardedFor: true }), "1.1.1.1");
});

test("clientIP ignores spoofable headers by default", () => {
  // Off-Cloudflare, x-forwarded-for is set by the client. Believing it lets an
  // attacker forge an allowlisted IP or rotate out of a rate-limit bucket.
  const req = new Request("https://x/", {
    headers: { "x-forwarded-for": "2.2.2.2", "x-real-ip": "3.3.3.3" },
  });
  assert.equal(clientIP(req, {}), "");
  assert.equal(clientIP(req, undefined), "");
  assert.equal(clientIP(req, { trustForwardedFor: false }), "");
});

test("clientIP honours x-forwarded-for only when opted in", () => {
  const req = new Request("https://x/", {
    headers: { "x-forwarded-for": "1.1.1.1, 2.2.2.2, 3.3.3.3" },
  });
  // Left-most entry is the originating client.
  assert.equal(clientIP(req, { trustForwardedFor: true }), "1.1.1.1");
});

test("clientIP falls back to x-real-ip under opt-in", () => {
  const req = new Request("https://x/", { headers: { "x-real-ip": " 4.4.4.4 " } });
  assert.equal(clientIP(req, { trustForwardedFor: true }), "4.4.4.4");
  assert.equal(clientIP(req, {}), "");
});

// --- timingSafeEqual ---------------------------------------------------------

test("timingSafeEqual", () => {
  assert.ok(timingSafeEqual("abc", "abc"));
  assert.ok(!timingSafeEqual("abc", "abd"));
  assert.ok(!timingSafeEqual("abc", "abcd"));
});

// --- session cookie sign/verify ---------------------------------------------

test("sign then verify round-trips", async () => {
  // exp is now mandatory — a validly-signed but exp-less cookie is rejected
  // (no expiry = no revocation path for a stateless session).
  const cookie = await signSessionCookie({ sub: "user-1", exp: 9999999999 }, "s3cret");
  assert.ok(await verifySessionCookie(cookie, "s3cret"));
});

test("valid signature but no exp → rejected", async () => {
  const cookie = await signSessionCookie({ sub: "user-1" }, "s3cret");
  assert.ok(!(await verifySessionCookie(cookie, "s3cret")));
});

test("verify fails with wrong secret", async () => {
  const cookie = await signSessionCookie({ sub: "user-1" }, "s3cret");
  assert.ok(!(await verifySessionCookie(cookie, "other")));
});

test("verify fails on tampered payload", async () => {
  const cookie = await signSessionCookie({ sub: "user-1" }, "s3cret");
  const [, sig] = cookie.split(".");
  const forged = `${Buffer.from('{"sub":"admin"}').toString("base64url")}.${sig}`;
  assert.ok(!(await verifySessionCookie(forged, "s3cret")));
});

test("verify enforces exp", async () => {
  const past = await signSessionCookie({ sub: "u", exp: 1000 }, "s3cret");
  assert.ok(!(await verifySessionCookie(past, "s3cret", 2000 * 1000)));
  const future = await signSessionCookie({ sub: "u", exp: 9999999999 }, "s3cret");
  assert.ok(await verifySessionCookie(future, "s3cret"));
});

test("verify rejects malformed / empty", async () => {
  assert.ok(!(await verifySessionCookie("", "s")));
  assert.ok(!(await verifySessionCookie("nodot", "s")));
  assert.ok(!(await verifySessionCookie("a.b", "")));
});

// --- rate limit --------------------------------------------------------------

function fakeKV() {
  const store = new Map();
  return {
    store,
    async get(k) {
      return store.has(k) ? store.get(k) : null;
    },
    async put(k, v) {
      store.set(k, v);
    },
  };
}

test("checkRateLimit counts up then trips at the limit", async () => {
  const kv = fakeKV();
  const now = 60_000 * 100; // fixed window
  let res;
  res = await checkRateLimit(kv, "1.1.1.1", 2, now);
  assert.deepEqual([res.limited, res.count], [false, 1]);
  res = await checkRateLimit(kv, "1.1.1.1", 2, now);
  assert.deepEqual([res.limited, res.count], [false, 2]);
  res = await checkRateLimit(kv, "1.1.1.1", 2, now);
  assert.equal(res.limited, true);
});

test("checkRateLimit isolates by IP and window", async () => {
  const kv = fakeKV();
  const now = 60_000 * 100;
  await checkRateLimit(kv, "1.1.1.1", 1, now);
  // different IP, same window — fresh counter
  let res = await checkRateLimit(kv, "2.2.2.2", 1, now);
  assert.equal(res.limited, false);
  // same IP, next window — fresh counter
  res = await checkRateLimit(kv, "1.1.1.1", 1, now + 60_000);
  assert.equal(res.limited, false);
});

// --- runGuard orchestration --------------------------------------------------

const baseCfg = {
  version: 1,
  publicPaths: ["/_next/*", "/login"],
  rateLimit: { kvBinding: "RATE_LIMIT", requestsPerMinute: 1, paths: ["/*"] },
  auth: { secretEnv: "AUTH_SECRET", cookieName: "session", loginPath: "/login", protectedPaths: ["/app/*"] },
};

function req(path, headers = {}) {
  return new Request("https://example.com" + path, { headers });
}

test("runGuard: public path bypasses all", async () => {
  const out = await runGuard(req("/login"), {}, null, baseCfg);
  assert.equal(out, null);
});

test("runGuard: denied IP → 403", async () => {
  const cfg = { ...baseCfg, deny: ["6.6.6.6"] };
  const out = await runGuard(req("/app/x", { "cf-connecting-ip": "6.6.6.6" }), {}, null, cfg);
  assert.equal(out.status, 403);
});

test("runGuard: over rate limit → 429", async () => {
  const env = { RATE_LIMIT: fakeKV() };
  const now = 60_000 * 200;
  // public path for auth (so we isolate the rate-limit branch): use "/" not in protectedPaths
  const first = await runGuard(req("/", { "cf-connecting-ip": "1.1.1.1" }), env, null, baseCfg, now);
  assert.equal(first, null); // under limit, not an auth-protected path
  const second = await runGuard(req("/", { "cf-connecting-ip": "1.1.1.1" }), env, null, baseCfg, now);
  assert.equal(second.status, 429);
});

test("runGuard: protected path without cookie → 401 for API", async () => {
  const env = { RATE_LIMIT: fakeKV(), AUTH_SECRET: "s3cret" };
  const out = await runGuard(req("/app/data", { "cf-connecting-ip": "1.1.1.1", accept: "application/json" }), env, null, baseCfg, 60_000 * 300);
  assert.equal(out.status, 401);
});

test("runGuard: protected path without cookie → 302 redirect for HTML", async () => {
  const env = { RATE_LIMIT: fakeKV(), AUTH_SECRET: "s3cret" };
  const out = await runGuard(req("/app/data", { "cf-connecting-ip": "1.1.1.1", accept: "text/html" }), env, null, baseCfg, 60_000 * 301);
  assert.equal(out.status, 302);
  assert.match(out.headers.get("location"), /\/login\?next=%2Fapp%2Fdata/);
});

test("runGuard: protected path with valid cookie → passes", async () => {
  const env = { RATE_LIMIT: fakeKV(), AUTH_SECRET: "s3cret" };
  const cookie = await signSessionCookie({ sub: "u1", exp: 9999999999 }, "s3cret");
  const out = await runGuard(
    req("/app/data", { "cf-connecting-ip": "1.1.1.1", accept: "text/html", cookie: `session=${cookie}` }),
    env,
    null,
    baseCfg,
    60_000 * 302,
  );
  assert.equal(out, null);
});

test("runGuard: null cfg is a no-op", async () => {
  assert.equal(await runGuard(req("/app/x"), {}, null, null), null);
});
