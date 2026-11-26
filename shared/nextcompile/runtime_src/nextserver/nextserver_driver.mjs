// Worker request driver for NextDeploy's `nextserver` engine.
//
// Runs Next.js's own NextNodeServer inside Cloudflare Workers while keeping
// NextDeploy's fast edge paths ahead of it:
//   1. Edge protection guard (guard.mjs: IP rules, KV rate limit, HMAC auth)
//   2. Static assets (/_next/static/* and /public/*) straight from R2
//   3. /_next/image optimization (image.mjs: R2 + env.IMAGES / cf.image / Wasm)
//   4. Full NextNodeServer request handler via streaming HTTP bridge
//      (App Router, Pages Router, Turbopack, RSC, Suspense, PPR, Server Actions,
//      Middleware/Proxy, API routes, error.tsx, not-found.tsx, ISR via CacheHandler)
//   5. Root public/* fallback from R2 on 404 (e.g. /robots.txt, /favicon.ico)

import { serveStaticFromR2, serveSSGFromR2, serveISRFromR2, serveRootPublicFromR2 } from "../serve.mjs";
import { handleImageRequest } from "../image.mjs";
import { serverError } from "../errors.mjs";
import { invokeNodeHandler } from "./http_bridge.mjs";
import { CloudflareCacheHandler } from "./cf_cache_handler.mjs";
import { loadBuiltinModule } from "./builtin_registry.mjs";

let cachedHandlerPromise = null;

function setProcessProp(proc, key, value) {
  try {
    proc[key] = value;
  } catch {
    try {
      Object.defineProperty(proc, key, {
        value,
        configurable: true,
        writable: true,
      });
    } catch {
      // Non-configurable in this runtime — leave as-is.
    }
  }
}

/**
 * Polyfill Node `process` properties that Cloudflare `workerd`'s
 * `nodejs_compat_v2` omits or leaves as throwing unenv stubs (such as
 * `process.listeners`, `process.on`, `process.cwd`, `process.uptime`,
 * `process.hrtime`, `process.stdout`, `process.stderr`).
 */
export function ensureProcessCompat(proc = globalThis.process) {
  let p = proc;
  if (!p || typeof p !== "object") {
    p = { env: {} };
    globalThis.process = p;
  }
  if (!p.env || typeof p.env !== "object") {
    setProcessProp(p, "env", {});
  }
  if (typeof p.listeners !== "function") {
    setProcessProp(p, "listeners", () => []);
  }
  const chainMethods = [
    "on",
    "once",
    "off",
    "addListener",
    "removeListener",
    "removeAllListeners",
    "prependListener",
    "prependOnceListener",
  ];
  for (const m of chainMethods) {
    if (typeof p[m] !== "function") {
      setProcessProp(p, m, () => p);
    }
  }
  if (typeof p.emit !== "function") {
    setProcessProp(p, "emit", () => false);
  }
  if (typeof p.emitWarning !== "function") {
    setProcessProp(p, "emitWarning", () => {});
  }
  let cwdOk = false;
  if (typeof p.cwd === "function") {
    try {
      cwdOk = typeof p.cwd() === "string";
    } catch {
      cwdOk = false;
    }
  }
  if (!cwdOk) {
    setProcessProp(p, "cwd", () => "/");
  }
  let uptimeOk = false;
  if (typeof p.uptime === "function") {
    try {
      uptimeOk = typeof p.uptime() === "number";
    } catch {
      uptimeOk = false;
    }
  }
  if (!uptimeOk) {
    setProcessProp(p, "uptime", () => 0);
  }
  if (typeof p.hrtime !== "function") {
    const hrtime = (prev) => {
      const ms = typeof performance !== "undefined" ? performance.now() : Date.now();
      const sec = Math.floor(ms / 1000);
      const ns = Math.floor((ms % 1000) * 1e6);
      if (Array.isArray(prev)) {
        let dSec = sec - prev[0];
        let dNs = ns - prev[1];
        if (dNs < 0) {
          dSec -= 1;
          dNs += 1e9;
        }
        return [dSec, dNs];
      }
      return [sec, ns];
    };
    hrtime.bigint = () => {
      const ms = typeof performance !== "undefined" ? performance.now() : Date.now();
      return BigInt(Math.floor(ms * 1e6));
    };
    setProcessProp(p, "hrtime", hrtime);
  } else if (typeof p.hrtime.bigint !== "function") {
    try {
      p.hrtime.bigint = () => {
        const ms = typeof performance !== "undefined" ? performance.now() : Date.now();
        return BigInt(Math.floor(ms * 1e6));
      };
    } catch {
      // Non-extensible
    }
  }
  if (!p.stdout) {
    setProcessProp(p, "stdout", { isTTY: false, write: () => true });
  }
  if (!p.stderr) {
    setProcessProp(p, "stderr", { isTTY: false, write: () => true });
  }
  if (!Array.isArray(p.argv)) {
    setProcessProp(p, "argv", ["node"]);
  }
  if (!p.versions || typeof p.versions !== "object") {
    setProcessProp(p, "versions", { node: "22.0.0" });
  }
  if (typeof p.version !== "string") {
    setProcessProp(p, "version", "v22.0.0");
  }
  if (typeof p.platform !== "string") {
    setProcessProp(p, "platform", "linux");
  }
  return p;
}

/**
 * Install the build-time Virtual Filesystem and static server-chunk loader on
 * globalThis before NextNodeServer boots.
 */
export function installNextServerGlobals({ vfs, loadServerModule, assetVersion }) {
  ensureProcessCompat();
  globalThis.__nextdeployVFS = vfs || { files: {}, dirs: {} };
  globalThis.__nextdeployLoadModule = loadServerModule || (() => undefined);
  globalThis.__nextdeployLoadBuiltin = loadBuiltinModule;
  globalThis.__nextdeployAssetVersion = assetVersion || "";
  globalThis.__nextdeployCacheHandler = CloudflareCacheHandler;
  // Next.js 15/16 reads Symbol.for("@next/cache-handlers").FetchCache across
  // module boundaries when creating per-route IncrementalCache instances;
  // registering CloudflareCacheHandler here prevents silent fallback to
  // FileSystemCache (which would fail on the read-only Worker VFS).
  const cacheHandlersSym = Symbol.for("@next/cache-handlers");
  const existing = globalThis[cacheHandlersSym];
  if (!existing || typeof existing !== "object") {
    globalThis[cacheHandlersSym] = { FetchCache: CloudflareCacheHandler };
  } else if (!existing.FetchCache) {
    existing.FetchCache = CloudflareCacheHandler;
  }
}

/**
 * Reset the cached handler (for unit tests).
 */
export function resetNextServerHandlerForTests() {
  cachedHandlerPromise = null;
}

async function getOrCreateRequestHandler(createServer) {
  if (!cachedHandlerPromise) {
    cachedHandlerPromise = (async () => {
      const server = await createServer({ CacheHandler: CloudflareCacheHandler });
      if (typeof server.prepare === "function") {
        await server.prepare();
      }
      return server.getRequestHandler();
    })().catch((err) => {
      cachedHandlerPromise = null;
      throw err;
    });
  }
  return cachedHandlerPromise;
}

function pushEnvIntoProcess(env) {
  if (typeof globalThis.process === "undefined") {
    globalThis.process = { env: {} };
  }
  if (!globalThis.process.env) {
    globalThis.process.env = {};
  }
  globalThis.process.env.NODE_ENV = "production";
  if (!env) return;
  for (const key of Object.keys(env)) {
    const val = env[key];
    if (typeof val === "string") {
      globalThis.process.env[key] = val;
    }
  }
}

function stripBasePath(pathname, basePath) {
  if (!basePath) return pathname;
  if (pathname === basePath) return "/";
  if (pathname.startsWith(basePath + "/")) return pathname.slice(basePath.length);
  return pathname;
}

/**
 * Main fetch dispatcher for the `nextserver` Worker engine.
 *
 * @param {Request} request
 * @param {Record<string, any>} env
 * @param {{ waitUntil?: (p: Promise<any>) => void }} ctx
 * @param {{
 *   manifest: any,
 *   guard?: Function,
 *   protection?: any,
 *   createServer: (opts: { CacheHandler: any }) => Promise<{ prepare?: Function, getRequestHandler: Function }>,
 * }} opts
 */
export async function dispatchNextServer(request, env, ctx, opts) {
  const { manifest, guard, protection, createServer } = opts;

  globalThis.__nextdeployEnv = env;
  globalThis.__nextdeployWorkerCtx = ctx || null;
  globalThis.__nextdeployAssetVersion = manifest?.assetVersion || "";
  pushEnvIntoProcess(env);

  const url = new URL(request.url);
  const pathname = stripBasePath(url.pathname, manifest?.basePath);

  try {
    // 1. Edge protection runs ahead of everything (IP / rate-limit / auth).
    if (guard && protection) {
      const blocked = await guard(request, env, ctx, protection);
      if (blocked) return blocked;
    }

    // 2. /_next/image handled natively at the edge (R2 + IMAGES / cf.image / Wasm).
    if (pathname === "/_next/image" || pathname.startsWith("/_next/image?")) {
      return await handleImageRequest(request, env, manifest);
    }

    // 3. Static assets (/_next/static/* and /public/*) served straight from R2.
    if (pathname.startsWith("/_next/static/") || pathname.startsWith("/public/")) {
      const staticRes = await serveStaticFromR2(env, pathname, manifest);
      if (staticRes) return staticRes;
    }

    // 3b. Pre-rendered SSG / fresh ISR HTML served straight from R2 (fast path
    //     for generateStaticParams routes like /docs/[...slug], falling through
    //     to NextNodeServer on miss, expiry, RSC Flight requests, or non-GET).
    const isRSCFlightReq = request.headers.has("rsc") || url.searchParams.has("_rsc");
    if (request.method === "GET" && env?.ASSETS && !isRSCFlightReq) {
      const ssgTarget = manifest?.routes?.ssg?.[pathname];
      if (ssgTarget) {
        const ssgRes = await serveSSGFromR2(env, ssgTarget, manifest);
        if (ssgRes) return ssgRes;
      }
      const isrTarget = manifest?.routes?.isr?.[pathname];
      if (isrTarget) {
        const ttl = manifest?.isr?.intervals?.[pathname] || 0;
        const { response: isrRes, expired } = await serveISRFromR2(env, isrTarget, manifest, ttl);
        if (!expired && isrRes) return isrRes;
      }
    }

    // 4. Delegate dynamic / SSR / RSC / API / Action / Middleware / ISR traffic
    //    to Next.js's real NextNodeServer via our streaming HTTP bridge.
    const requestHandler = await getOrCreateRequestHandler(createServer);
    const response = await invokeNodeHandler(requestHandler, request, ctx);

    // 5. If NextNodeServer returned 404 for a dotted root file (/robots.txt,
    //    /favicon.ico, etc.), check R2 public assets before returning 404.
    if (response.status === 404 && request.method === "GET") {
      const publicRoot = await serveRootPublicFromR2(env, pathname, manifest);
      if (publicRoot) return publicRoot;
    }

    return response;
  } catch (err) {
    return serverError(err);
  }
}
