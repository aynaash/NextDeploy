---
title: Running Next.js 16 Turbopack on Cloudflare Workers
lede: How we built NextDeploy's pure-Go NextServer engine to run unmodified Next.js 16.0.10 Turbopack + React 19 inside Cloudflare Workers — zero third-party adapters, 1.60 MB gzipped, and 10 real-world workerd lessons learned taking nextdeploy.org live.
status: ready
sourceRef: shared/nextcompile/nextserver.go, shared/nextcompile/runtime_src/nextserver/, cli/internal/serverless/cloudflare_adapter.go
related:
  - title: NextCompile (full reference)
    href: /docs/advanced/nextcompile
  - title: Workers Runtime
    href: /docs/cloudflare/workers
  - title: Cloudflare Overview
    href: /docs/cloudflare/overview
---

## The Problem: Why Reimplementing Next.js Always Loses

Most serverless adapters for Next.js start with the same tempting idea:

> *"Let's parse `.next/routes-manifest.json`, import each compiled `page.js` directly, and write our own tiny Worker dispatcher + React SSR renderer."*

That works on a toy app with 3 static pages. In production — and especially once **Next.js 16 made Turbopack the default production bundler** — it breaks down fast:

1. **Turbopack's chunk graph is not Webpack's.** Turbopack emits `[turbopack]_runtime.js` and `[root-of-the-server]__<hash>._.js` with runtime `require()` calls into `/server/chunks/ssr/...`. Custom dispatchers that only understand Webpack's `__webpack_require__` crash or miss chunks.
2. **React 19 dual-condition imports.** App Router RSC encoding requires the `react-server` build of React, while HTML streaming requires the client/edge build of `react-dom/server`. Maintaining vendored copies across React 19.x releases is a constant source of hydration bugs.
3. **Feature whack-a-mole.** Next.js ships `proxy.ts`, nested `generateMetadata`, `not-found.tsx` boundaries, `Suspense` streaming, `generateStaticParams` + RSC Flight navigations, `revalidateTag`, `after()`, and new manifest shapes every few months. Reimplementing them in parallel means permanently chasing upstream.

So we asked a simpler question:

> **Cloudflare Workers (`workerd` + `nodejs_compat_v2`) now ships native `node:async_hooks`, `node:stream`, `node:buffer`, `node:crypto`, `node:zlib`, `node:events`, `node:url`, and `node:util`. What happens if we run Next.js's *actual* `NextNodeServer` inside the Worker?**

Result: **unmodified Next.js 16.0.10 Turbopack + React 19.2.0** running live on `https://nextdeploy.org` in a **6.43 MB raw / 1.60 MB gzipped** bundle (small enough for Cloudflare Workers' 3 MB free tier), with zero third-party npm adapters.

Here is the architecture, real code, and all 10 production lessons learned getting it live.

---

## Architecture Overview

Instead of replacing Next.js, NextDeploy's `nextserver` engine keeps Cloudflare-native edge fast paths in front and provides the exact runtime primitives `NextNodeServer` expects underneath:

```
Request → Edge Guard (IP / KV rate limit / HMAC auth)
        → /_next/image (R2 + Cloudflare Images)
        → /_next/static/* & /public/* (direct from R2)
        → Pre-rendered SSG / fresh ISR HTML (direct from R2 on non-RSC GET)
        → NextNodeServer via Streaming HTTP Bridge
            ├── Virtual FS shim (node:fs → in-memory build-time manifests)
            ├── Static Chunk Registry (dynamic require() → esbuild-bundled thunks)
            └── CloudflareCacheHandler (L1 Map + L2 R2 + KV tags + R2 release fallback)
        → Root public/* fallback on 404 (/robots.txt, /favicon.ico)
```

In `nextdeploy.yml`, `engine: auto` detects Turbopack trees (`[turbopack]*` or `[root-of-the-server]*` in `.next/server/chunks`) and automatically selects `nextserver`:

```go
// shared/nextcompile/nextserver.go
func IsTurbopackTree(standaloneDir, distDir string) bool {
	if distDir == "" {
		distDir = ".next"
	}
	candidates := []string{
		filepath.Join(standaloneDir, distDir, "server", "chunks"),
		filepath.Join(standaloneDir, distDir, "server", "chunks", "ssr"),
	}
	return slices.ContainsFunc(candidates, dirHasTurbopackChunk)
}
```

---

## The 4 Runtime Primitives

### 1. Build-Time Virtual Filesystem (`vfs.mjs` + `vfs_fs_shim.mjs`)

`NextNodeServer` calls `fs.readFileSync`, `fs.existsSync`, `fs.statSync`, and `fs.readdirSync` synchronously during boot and on every request to read `BUILD_ID`, `routes-manifest.json`, `prerender-manifest.json`, `app-paths-manifest.json`, and `package.json`.

At compile time, `EmitVirtualFS` walks `.next/standalone` (skipping static assets, source maps, `.nft.json`, and files > 2 MB) and emits all manifests + directory index into `vfs.mjs`. At bundle time, esbuild aliases `fs` and `node:fs` to `vfs_fs_shim.mjs`:

```js
// shared/nextcompile/runtime_src/nextserver/vfs_fs_shim.mjs
export function normalizeVFSPath(rawPath) {
  const joined = cleanSegments(rawPath);
  const vfs = getVFS();
  if (hasOwn(vfs.files, joined) || hasOwn(vfs.dirs, joined)) return joined;
  const nextIdx = joined.indexOf(".next/");
  if (nextIdx > 0) {
    const sub = joined.slice(nextIdx);
    if (hasOwn(vfs.files, sub) || hasOwn(vfs.dirs, sub)) return sub;
  }
  return joined;
}
```

Returning authentic `ENOENT` errors (`err.code = "ENOENT"`) on misses is essential — Next.js uses `try { readFileSync(...) } catch (e) { if (e.code === 'ENOENT') ... }` everywhere to probe optional manifests.

### 2. Static Server Chunk Registry (`chunks.mjs`)

Both Webpack and Turbopack load route modules and SSR chunks via runtime `require(chunkPath)`. At compile time, `EmitServerChunkRegistry` walks `.next/server` and emits a frozen map of static `() => require(...)` thunks so esbuild bundles every chunk into the single Worker file:

```js
// Generated _nextdeploy/chunks.mjs
export const serverModules = Object.freeze({
  ".next/server/app/api/health/route.js": () => require("../.next/server/app/api/health/route.js"),
  ".next/server/chunks/ssr/[root-of-the-server]__6e1ea091._.js": () => require("../.next/server/chunks/ssr/[root-of-the-server]__6e1ea091._.js"),
});
```

An esbuild `OnLoad` plugin patches dynamic `require(expr)` calls in `/next/dist/server/` and `/[turbopack]` files to call `__ndDynRequire(expr)` against `serverModules` (falling back to VFS JSON files).

### 3. Zero-Buffering Web ↔ Node Streaming HTTP Bridge (`http_bridge.mjs`)

`NextNodeServer.getRequestHandler()` speaks Node's `(req: IncomingMessage, res: ServerResponse)`, while Cloudflare Workers speak Web Fetch `(request: Request) => Promise<Response>`.

Our bridge turns `request.body` into a Node `Readable` and creates a `ServerResponse`-compatible EventEmitter backed by a Web `ReadableStream`. As soon as Next.js commits headers (`writeHead`, `flushHeaders`, or first `write`), `responsePromise` resolves immediately while `ctx.waitUntil(handlerPromise)` keeps the Worker isolate alive until `res.end()` closes the stream:

```js
// shared/nextcompile/runtime_src/nextserver/http_bridge.mjs
const handlerPromise = Promise.resolve()
  .then(() => handler(req, res))
  .then(() => { if (!res.writableEnded) res.end(); })
  .catch((err) => { /* ... */ });

if (typeof ctx?.waitUntil === "function") {
  ctx.waitUntil(handlerPromise);
}
return await responsePromise;
```

That gives true progressive React `Suspense` and `loading.tsx` streaming without buffering.

### 4. R2 + KV `CloudflareCacheHandler` (`cf_cache_handler.mjs`)

Layers an in-isolate L1 `Map`, R2 L2 (`_nextdeploy/cache/<assetVersion>/<key>.json` with `ctx.waitUntil` writes and `Buffer` revival for `rscData`), build-time R2 release fallback (`_nextdeploy/releases/<assetVersion>/<route>.{html,rsc}`), and KV tag invalidation (`tag:<name>`).

---

## 10 Real-World Lessons Learned

Testing against unit fixtures was easy. Running a real **Next.js 16.0.10 + Turbopack + React 19.2.0** app — and deploying it to live Cloudflare `workerd` — uncovered 10 concrete edge cases:

### 1. Turbopack omits `.next/` in server chunk paths
Turbopack's runtime requests `/server/chunks/ssr/[root-of-the-server]__*.js` without `.next/`. Normalizing `DIST_PREFIX` (`.next/`) and `SERVER_PREFIX` (`.next/server/`) inside `chunks.mjs` made every Turbopack chunk resolve cleanly.

### 2. `evalManifest` uses `vm.runInNewContext` (blocked in `workerd`)
Next.js loads `page_client-reference-manifest.js` via `load-manifest.external.js`'s `evalManifest()` / `evalManifestFromRelativePath()`, which execute JS through `vm.runInNewContext`. Replacing `load-manifest.external.js` at bundle time with a VFS + static chunk-registry shim exporting all 5 Next.js `16.4+` helpers (`loadManifest`, `evalManifest`, `evalManifestFromRelativePath`, `loadManifestFromRelativePath`, `clearManifestCache`) with `handleMissing` support keeps manifest loading 100% CSP-compliant.

### 3. `next/dist/server/require-hook.js` crashes in ESM bundles
`require-hook.js` patches `Module.prototype.require`, which does not exist in ESM Worker bundles. Replacing it with a tiny no-op shim fixes startup cleanly.

### 4. CJS `node-stub` modules must export `module.exports = stub`
Exporting `export default stub` from an esbuild virtual stub module caused `require("tty").isatty(1)` to throw `TypeError: r2(...).isatty is not a function`. Switching to CJS `module.exports = stub` with explicit safe helpers (`isatty: () => false`, `os.cpus`, `performance: globalThis.performance`) fixed every CJS Node builtin probe.

### 5. Next.js 16 silently falls back to `FileSystemCache` across module boundaries
Even with `conf.cacheHandler` set, Next.js 16 created a second `IncrementalCache` inside its compiled route module without passing `CurCacheHandler`, silently falling back to `FileSystemCache` and throwing `NoFallbackError` on `generateStaticParams` routes like `/docs/advanced/nextcompile`.

Registering `CloudflareCacheHandler` on Next.js 16's official cross-boundary symbol — and leaving `conf.cacheHandler = undefined` so compiled runtimes never attempt a dynamic `import()` of a file path — solved it globally:

```js
// shared/nextcompile/runtime_src/nextserver/nextserver_driver.mjs
const cacheHandlersSym = Symbol.for("@next/cache-handlers");
globalThis[cacheHandlersSym] = { FetchCache: CloudflareCacheHandler };
```

### 6. Preserve HTTP `404` on cached `/_not-found` entries
When Next.js serves `/nonexistent-404`, it reads `/_not-found` from `IncrementalCache`. Hardcoding `status: 200` in the R2 release fallback caused missing routes to return `200 OK`. Setting `status: key === "/_not-found" ? 404 : undefined` preserves true HTTP `404` while keeping normal routes at `200`.

### 7. Node.js tests pass while live `workerd` crashes on `process.listeners`
Our first live hit to `https://nextdeploy.org/` threw:

```
TypeError: process.listeners is not a function
    at installUnhandledRejectionFilter (next/dist/server/node-environment-extensions/unhandled-rejection.js)
    at next/dist/server/node-environment.js
```

`workerd`'s `nodejs_compat_v2` ships a minimal `process` object without `listeners`, `on`/`off`/`emit`, `hrtime`, and with throwing `[unenv]` stubs for `cwd()` / `uptime()`.

**Fix:** Shim `next/dist/server/node-environment.js` down to just the `AsyncLocalStorage` baseline (saving 72 KB and silencing 7 `node:crypto` warnings) and run `ensureProcessCompat()` before importing `NextNodeServer`.

### 8. Trimming dev & experimental runtimes fits the 3 MB free tier
Trimming `*.runtime.dev.js`, `*-experimental.runtime.*`, `edge-runtime`, `react-dom/static`, `caniuse-lite`, and `@swc/helpers` dropped the full Next.js 16 Turbopack bundle from **8.1 MB** to **6.43 MB raw / 1.60 MB gzipped** — well under Cloudflare's 3 MB free tier and 10 MB paid tier.

### 9. Test RSC Flight navigations (`rsc: 1`), not just initial HTML
Client-side `<Link>` transitions send `rsc: 1` expecting `text/x-component`. Skipping the HTML fast path for RSC requests and loading `.rsc` payloads inside `CloudflareCacheHandler` kept all 50+ SSG doc pages working on both hard reloads and client-side navigations.

### 10. Exclude internal error routes from post-deploy smoke checks
Next.js lists `/_global-error` and `/_not-found` in `routes-manifest.json`'s `staticRoutes`. Post-deploy `SmokeVerify` now skips `/_*`, `/404`, and `/500` so it probes real user-facing routes (`/`, `/docs`, `/login`).

---

## Next.js 16.4+ Compatibility Notes

Upgrading from Next.js `16.0.10` to `16.4.6` introduced three additional runtime requirements handled automatically in `cli/internal/serverless/cloudflare/adapter/adapter.go`:

1. **`evalManifestFromRelativePath` + `handleMissing` (`load-manifest.external.js`)**: `NextNodeServer.loadManifests()` calls `evalManifestFromRelativePath` with `handleMissing: true` when probing optional manifests. Missing this export caused `TypeError: l2 is not a function`; the shim exports all 5 manifest functions and returns `undefined` on missing optional manifests.
2. **Safe `fast-set-immediate.external.js` shim**: Next.js `16.4+` runs `install()` on import and assigns `require("node:timers").setImmediate = ...`, throwing `TypeError: Cannot set property setImmediate of #<Object> which has only a getter` against esbuild's read-only ESM namespace wrapper. NextDeploy shims `fast-set-immediate.external.js` (and `shared/nextcompile/runtime_src/next_shims/fast_set_immediate.mjs`) to export `DANGEROUSLY_runPendingImmediatesAfterCurrentTask`, `expectNoPendingImmediates`, `unpatchedSetImmediate`, `ImmediateTracker`, and `runWithNativeImmediateTracking` without mutating `node:timers`.
3. **Eliminating dev-only imports (`process.env.__NEXT_DEV_SERVER`)**: Next.js `16.4+` guards dev-only `require()` calls (such as `hot-reloader-app`) behind `process.env.__NEXT_DEV_SERVER`. Setting `process.env.__NEXT_DEV_SERVER: ""` in esbuild's `Define` drops those branches at bundle time.

Verify any local standalone build before shipping:

```bash
NEXTDEPLOY_CF_ENGINE=nextserver NEXTDEPLOY_TEST_STANDALONE=/path/to/app/.next/standalone \
  go test -v ./cli/internal/serverless/cloudflare/adapter -run TestBuildWorkerBundle_RealStandaloneSmoke
```

---

## Live Production Results (`https://nextdeploy.org`)

Deployed from `nextdeployfrontend` (Next.js 16.0.10, Turbopack, React 19.2.0, 52 routes, Tailwind v4, Geist fonts, edge auth guard, R2 assets) via `nextdeploy ship`:

- **Compile time (`nextcompile` + in-process esbuild):** ~170 ms
- **Worker bundle size:** **6,432.6 KB raw / 1.60 MB gzipped**

| Live URL | Status | Content-Type | Size |
| :--- | :--- | :--- | :--- |
| `https://nextdeploy.org/` | **200** | `text/html; charset=utf-8` | 37,910 B |
| `https://nextdeploy.org/api/health` | **200** | `application/json` | 166 B |
| `https://nextdeploy.org/docs` | **200** | `text/html; charset=utf-8` | 185,888 B |
| `https://nextdeploy.org/docs/advanced/nextcompile` | **200** | `text/html; charset=utf-8` | 41,539 B |
| `https://nextdeploy.org/docs/advanced/nextcompile` (`rsc: 1`) | **200** | `text/x-component` | 16,880 B |
| `https://nextdeploy.org/login` | **200** | `text/html; charset=utf-8` | 19,398 B |
| `https://nextdeploy.org/app` | **401** | `text/plain;charset=UTF-8` | 12 B *(edge auth guard)* |
| `https://nextdeploy.org/nonexistent-404` | **404** | `text/html; charset=utf-8` | 17,904 B *(real `not-found.tsx`)* |
