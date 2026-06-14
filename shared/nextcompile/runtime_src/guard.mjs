// nextcompile runtime — edge protection guard.
//
// Runs ahead of the app's own proxy.ts / middleware.ts (and all routing).
// Driven by the build-time _nextdeploy/protection.json (generated from
// nextdeploy.yml `cloudflare.protection`). Enforces, in order:
//
//   1. IP deny / allow  — exact-match lists
//   2. Rate limit        — KV-backed, fixed 60s per-IP window
//   3. Auth              — stateless HMAC session cookie (no DB round-trip, so
//                          it works identically for D1 or bring-your-own Postgres)
//
// runGuard() returns a Response to short-circuit the request, or null to let it
// proceed to the app. Every exported helper is pure (or takes its effects as
// arguments) so the whole policy can be unit-tested without a Worker.

/** Best-effort client IP from Cloudflare / proxy headers. */
export function clientIP(request) {
  return (
    request.headers.get("cf-connecting-ip") ||
    request.headers.get("x-forwarded-for") ||
    request.headers.get("x-real-ip") ||
    ""
  );
}

/**
 * Anchored glob match for a single pathname pattern. "*" matches any run of
 * characters (including "/"); a trailing "/*" also matches the bare prefix, so
 * "/app/*" matches both "/app" and "/app/x/y". Patterns are full-path anchored.
 */
export function pathMatches(path, pattern) {
  if (pattern === path) return true;
  // "/app/*" should also match "/app"
  if (pattern.endsWith("/*") && path === pattern.slice(0, -2)) return true;
  const re = new RegExp(
    "^" +
      pattern
        .split("*")
        .map((s) => s.replace(/[.*+?^${}()|[\]\\]/g, "\\$&"))
        .join(".*") +
      "$",
  );
  return re.test(path);
}

export function matchesAny(path, patterns) {
  return Array.isArray(patterns) && patterns.some((p) => pathMatches(path, p));
}

/**
 * IP gating. An allowlist, when non-empty, is exclusive: anything not on it is
 * denied. The denylist always blocks. Deny wins over allow.
 */
export function ipDenied(ip, cfg) {
  if (cfg.deny && cfg.deny.includes(ip)) return true;
  if (cfg.allow && cfg.allow.length > 0 && !cfg.allow.includes(ip)) return true;
  return false;
}

/** Read one cookie value from a Cookie header string. */
export function readCookie(cookieHeader, name) {
  if (!cookieHeader) return "";
  for (const pair of cookieHeader.split(/;\s*/)) {
    const eq = pair.indexOf("=");
    if (eq < 0) continue;
    if (pair.slice(0, eq).trim() === name) return pair.slice(eq + 1).trim();
  }
  return "";
}

// --- base64url + HMAC (Web Crypto: available in Workers and Node 18+) ---------

function b64urlDecodeToBytes(s) {
  const b64 = s.replace(/-/g, "+").replace(/_/g, "/") + "=".repeat((4 - (s.length % 4)) % 4);
  const bin = atob(b64);
  const out = new Uint8Array(bin.length);
  for (let i = 0; i < bin.length; i++) out[i] = bin.charCodeAt(i);
  return out;
}

function bytesToB64url(bytes) {
  let bin = "";
  for (const b of bytes) bin += String.fromCharCode(b);
  return btoa(bin).replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/, "");
}

async function hmacSign(message, secret) {
  const enc = new TextEncoder();
  const key = await crypto.subtle.importKey(
    "raw",
    enc.encode(secret),
    { name: "HMAC", hash: "SHA-256" },
    false,
    ["sign"],
  );
  const sig = await crypto.subtle.sign("HMAC", key, enc.encode(message));
  return bytesToB64url(new Uint8Array(sig));
}

/** Constant-time string compare to avoid signature timing leaks. */
export function timingSafeEqual(a, b) {
  if (a.length !== b.length) return false;
  let diff = 0;
  for (let i = 0; i < a.length; i++) diff |= a.charCodeAt(i) ^ b.charCodeAt(i);
  return diff === 0;
}

/**
 * Mint a stateless session cookie value: "<payloadB64url>.<HMAC-SHA256(payload)>".
 * The app's login handler (or better-auth adapter) uses this to issue the cookie
 * the guard later verifies. Pass an `exp` (unix seconds) in payloadObj to expire.
 */
export async function signSessionCookie(payloadObj, secret) {
  const payload = bytesToB64url(new TextEncoder().encode(JSON.stringify(payloadObj)));
  const sig = await hmacSign(payload, secret);
  return `${payload}.${sig}`;
}

/**
 * Verify a stateless session cookie of the form "<payloadB64url>.<sigB64url>"
 * where sig = HMAC-SHA256(payloadB64url, secret). If the decoded payload is JSON
 * with a numeric `exp` (unix seconds), expiry is enforced too.
 */
export async function verifySessionCookie(value, secret, nowMs = Date.now()) {
  if (!value || !secret) return false;
  const dot = value.lastIndexOf(".");
  if (dot <= 0) return false;
  const payload = value.slice(0, dot);
  const sig = value.slice(dot + 1);
  const expected = await hmacSign(payload, secret);
  if (!timingSafeEqual(sig, expected)) return false;
  try {
    const data = JSON.parse(new TextDecoder().decode(b64urlDecodeToBytes(payload)));
    if (data && typeof data.exp === "number" && data.exp * 1000 < nowMs) return false;
  } catch {
    // Payload isn't JSON — signature already proved authenticity, so accept.
  }
  return true;
}

/**
 * Fixed-window per-IP rate limit. Counter key is bucketed per 60s so it
 * self-expires; we set a 120s TTL as a safety margin. Returns whether the
 * request is over the limit (it still counts the request that trips it).
 */
export async function checkRateLimit(kv, ip, limit, nowMs) {
  const windowId = Math.floor(nowMs / 60000);
  const key = `rl:${ip}:${windowId}`;
  const current = parseInt((await kv.get(key)) || "0", 10) || 0;
  if (current >= limit) return { limited: true, count: current };
  await kv.put(key, String(current + 1), { expirationTtl: 120 });
  return { limited: false, count: current + 1 };
}

/**
 * Apply the protection policy to a request.
 * @returns {Promise<Response|null>} Response to short-circuit, or null to continue.
 */
export async function runGuard(request, env, ctx, cfg, nowMs = Date.now()) {
  if (!cfg) return null;
  const url = new URL(request.url);
  const path = url.pathname;

  // Public paths bypass everything (also covers /_next/*, login, etc.).
  if (matchesAny(path, cfg.publicPaths)) return null;

  const ip = clientIP(request);

  // 1. IP gate.
  if (ipDenied(ip, cfg)) return new Response("Forbidden", { status: 403 });

  // 2. Rate limit.
  if (cfg.rateLimit && matchesAny(path, cfg.rateLimit.paths)) {
    const kv = env?.[cfg.rateLimit.kvBinding];
    if (kv) {
      const { limited } = await checkRateLimit(kv, ip, cfg.rateLimit.requestsPerMinute, nowMs);
      if (limited) {
        return new Response("Too Many Requests", {
          status: 429,
          headers: { "retry-after": "60" },
        });
      }
    }
  }

  // 3. Auth.
  if (cfg.auth && matchesAny(path, cfg.auth.protectedPaths)) {
    const secret = env?.[cfg.auth.secretEnv] ?? globalThis.process?.env?.[cfg.auth.secretEnv];
    const cookie = readCookie(request.headers.get("cookie") || "", cfg.auth.cookieName);
    const ok = await verifySessionCookie(cookie, secret, nowMs);
    if (!ok) {
      const accept = request.headers.get("accept") || "";
      if (accept.includes("text/html")) {
        const to = new URL(cfg.auth.loginPath, url.origin);
        to.searchParams.set("next", path);
        return Response.redirect(to.toString(), 302);
      }
      return new Response("Unauthorized", { status: 401 });
    }
  }

  return null;
}
