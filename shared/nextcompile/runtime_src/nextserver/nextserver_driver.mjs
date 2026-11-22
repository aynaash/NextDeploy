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

import { serveStaticFromR2, serveRootPublicFromR2 } from "../serve.mjs";
import { handleImageRequest } from "../image.mjs";
import { serverError } from "../errors.mjs";
import { invokeNodeHandler } from "./http_bridge.mjs";
import { CloudflareCacheHandler } from "./cf_cache_handler.mjs";

let cachedHandlerPromise = null;

/**
 * Install the build-time Virtual Filesystem and static server-chunk loader on
 * globalThis before NextNodeServer boots.
 */
export function installNextServerGlobals({ vfs, loadServerModule, assetVersion }) {
  globalThis.__nextdeployVFS = vfs || { files: {}, dirs: {} };
  globalThis.__nextdeployLoadModule = loadServerModule || (() => undefined);
  globalThis.__nextdeployAssetVersion = assetVersion || "";
  globalThis.__nextdeployCacheHandler = CloudflareCacheHandler;
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
