# Running Unmodified Next.js 16 Turbopack on Cloudflare Workers — Zero Third-Party Adapters, 1.60 MB Gzipped

How we replaced a fragile custom Next.js reimplementation in **NextDeploy** with a pure-Go compiler pipeline that runs Next.js's real `NextNodeServer` directly inside Cloudflare's `workerd` V8 isolate — and every single edge case we hit taking a live Next.js 16.0.10 Turbopack + React 19 app to production on `https://nextdeploy.org`.

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

Spoiler: It runs **unmodified Next.js 16.0.10 Turbopack + React 19.2.0** in a **6.43 MB raw / 1.60 MB gzipped** bundle (small enough for Cloudflare Workers' 3 MB free tier!), with zero third-party npm adapters.

Getting there uncovered 10 fascinating `workerd` + Next.js 16 internals traps. Here is the full architecture, real code, and every lesson learned.

---

## Architecture Overview

Instead of replacing Next.js, NextDeploy's `nextserver` engine wraps `NextNodeServer` with Cloudflare-native edge fast paths in front and Workers-safe primitives underneath:

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

In `nextdeploy.yml`, `engine: auto` automatically detects Turbopack trees (`[turbopack]*` or `[root-of-the-server]*` in `.next/server/chunks`) and selects `nextserver`, while keeping `legacy` available as a fallback:

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

## The 4 Runtime Primitives That Make `NextNodeServer` Work in `workerd`

### 1. Build-Time Virtual Filesystem (`vfs.mjs` + `vfs_fs_shim.mjs`)

`NextNodeServer` calls `fs.readFileSync`, `fs.existsSync`, `fs.statSync`, and `fs.readdirSync` synchronously during boot and on every request to read `BUILD_ID`, `routes-manifest.json`, `prerender-manifest.json`, `app-paths-manifest.json`, and `package.json`.

Cloudflare Workers have no filesystem. At compile time, NextDeploy walks `.next/standalone` (skipping static client assets, source maps, `.nft.json` traces, and files over 2 MB) and packs every manifest into a single ES module `vfs.mjs` alongside a directory listing index. At bundle time, esbuild aliases `fs`, `node:fs`, `fs/promises`, and `node:fs/promises` to `vfs_fs_shim.mjs`:

```js
// shared/nextcompile/runtime_src/nextserver/vfs_fs_shim.mjs
export function normalizeVFSPath(rawPath) {
  // Resolves `.` and `..`, strips arbitrary mount prefixes like
  // "/var/task/.next/routes-manifest.json" → ".next/routes-manifest.json".
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

Returning authentic `ENOENT` errors (`err.code = "ENOENT"`) on misses is critical — Next.js uses `try { readFileSync(...) } catch (e) { if (e.code === 'ENOENT') ... }` everywhere to probe optional features like `middleware-manifest.json` or `pages/500.html`.

### 2. Static Server Chunk Registry (`chunks.mjs`)

Both Webpack and Turbopack load route handlers and SSR chunks dynamically at runtime via `require(chunkPath)`. In a bundled Cloudflare Worker, runtime `require(variable)` fails because esbuild cannot statically trace an arbitrary expression.

At compile time, `EmitServerChunkRegistry` walks `.next/server` and emits a frozen map of static `() => require(...)` thunks:

```js
// Generated _nextdeploy/chunks.mjs
export const serverModules = Object.freeze({
  ".next/server/app/api/health/route.js": () => require("../.next/server/app/api/health/route.js"),
  ".next/server/chunks/ssr/[root-of-the-server]__6e1ea091._.js": () => require("../.next/server/chunks/ssr/[root-of-the-server]__6e1ea091._.js"),
  // ...every compiled server chunk and manifest JS file
});
```

Then an esbuild `OnLoad` plugin patches non-literal `require(expr)` calls in `/next/dist/server/` and `/[turbopack]` files to call `__ndDynRequire(expr)`, which looks up the normalized path in `serverModules` (and falls back to JSON files in the VFS).

### 3. Zero-Buffering Web ↔ Node Streaming HTTP Bridge (`http_bridge.mjs`)

`NextNodeServer.getRequestHandler()` speaks Node's `(req: IncomingMessage, res: ServerResponse)` protocol, while Cloudflare Workers speak Web Fetch `(request: Request) => Promise<Response>`.

Our bridge turns `request.body` into a Node `Readable` and creates a `ServerResponse`-compatible EventEmitter backed by a Web `ReadableStream`. As soon as Next.js commits headers (first `writeHead`, `flushHeaders`, or `write`), `responsePromise` resolves immediately with `new Response(bodyStream, ...)` while `ctx.waitUntil(handlerPromise)` keeps the Worker isolate alive until `res.end()` closes the stream:

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

That gives true progressive React `Suspense` and `loading.tsx` streaming with zero response buffering.

### 4. R2 + KV `CloudflareCacheHandler` (`cf_cache_handler.mjs`)

Next.js's `IncrementalCache` expects a `CacheHandler` implementing `get(key, ctx)`, `set(key, data, ctx)`, and `revalidateTag(tags)`.

Our handler layers:
- **L1**: Per-isolate `Map` for instant warm hits.
- **L2**: R2 (`env.ASSETS`) under `_nextdeploy/cache/<assetVersion>/<key>.json` with non-blocking `ctx.waitUntil` writes and automatic `Buffer` revival for serialized `rscData`.
- **Build-time R2 release fallback**: Reads pre-rendered `.html` + `.rsc` files from `_nextdeploy/releases/<assetVersion>/<route>.{html,rsc}` so `generateStaticParams` routes work seamlessly for both full HTML requests and client-side `<Link>` RSC Flight requests.
- **Tag invalidation**: Writes `revalidateTag` timestamps to Cloudflare KV (`env.NEXTCOMPILE_CACHE`) so invalidations propagate across every edge isolate worldwide.

---

## 10 Real-World Lessons Learned (From First Compile to Live `https://nextdeploy.org`)

Testing against unit-test fixtures was easy. Testing against a real **Next.js 16.0.10 + Turbopack + React 19.2.0** production app — and then deploying it to real Cloudflare `workerd` — surfaced 10 concrete gotchas that would have been invisible otherwise:

### Lesson 1: Turbopack omits `.next/` in server chunk paths
Turbopack's `[turbopack]_runtime.js` calls `require("/server/chunks/ssr/[root-of-the-server]__*.js")` — omitting the `.next/` prefix. Normalizing `DIST_PREFIX` (`.next/`) and `SERVER_PREFIX` (`.next/server/`) inside `chunks.mjs` made every Turbopack chunk resolve on the first try.

### Lesson 2: `evalManifest` uses `vm.runInNewContext` (forbidden in `workerd`)
Next.js loads `page_client-reference-manifest.js` via `load-manifest.external.js`'s `evalManifest()` and `evalManifestFromRelativePath()`, which execute manifest JS using Node's `vm.runInNewContext`. In Cloudflare Workers, dynamic code evaluation is blocked by CSP.

**Fix:** Replace `next/dist/server/load-manifest.external.js` at bundle time with a shim exporting all 5 Next.js `16.4+` functions (`loadManifest`, `evalManifest`, `evalManifestFromRelativePath`, `loadManifestFromRelativePath`, `clearManifestCache`) with `handleMissing` support, executing pre-bundled manifest thunks from our static chunk registry instead of `vm`.

### Lesson 3: `next/dist/server/require-hook.js` crashes in ESM bundles
`require-hook.js` monkey-patches `Module.prototype.require`, which is `undefined` inside an esbuild ESM bundle. Replacing it with a tiny no-op shim (`addHookAliases`, `defaultOverrides`, `hookPropertyMap`) avoids the startup crash cleanly.

### Lesson 4: CJS `node-stub` modules must export `module.exports = stub`
Exporting `export default stub` from an esbuild virtual stub module makes `require("tty").isatty(1)` throw `TypeError: r2(...).isatty is not a function` because esbuild wraps ESM default exports under `{ default: stub }`. Switching the stub loader to `module.exports = stub` with explicit safe methods (`isatty: () => false`, `os.cpus`, `os.tmpdir`, `performance: globalThis.performance`) fixed every CJS Node builtin probe.

### Lesson 5: Next.js 16 silently falls back to `FileSystemCache` across module boundaries
Even when we configured `conf.cacheHandler`, Next.js 16 instantiated a second `IncrementalCache` inside its compiled route module without passing `CurCacheHandler` — causing it to silently fall back to `FileSystemCache`, miss on our read-only VFS, and throw `NoFallbackError` on `generateStaticParams` routes like `/docs/advanced/nextcompile`!

Inspecting `next/dist/server/lib/incremental-cache/index.js` revealed Next.js 16's official cross-boundary hook:

```js
// shared/nextcompile/runtime_src/nextserver/nextserver_driver.mjs
const cacheHandlersSym = Symbol.for("@next/cache-handlers");
globalThis[cacheHandlersSym] = { FetchCache: CloudflareCacheHandler };
```

Registering `CloudflareCacheHandler` on `Symbol.for("@next/cache-handlers").FetchCache` — and setting `conf.cacheHandler = undefined` so compiled Next runtimes never attempt a dynamic `import()` of a file path — made **every** `IncrementalCache` instance inside Next.js 16 use our R2+KV handler natively.

### Lesson 6: Never hardcode `status: 200` on cached `/_not-found` entries
When Next.js serves a missing URL like `/nonexistent-404`, it reads `/_not-found` from `IncrementalCache`. When our R2 release fallback hardcoded `status: 200`, missing pages returned `200 OK` with the 404 page HTML!

Setting `status: key === "/_not-found" ? 404 : undefined` lets Next.js apply `prerender-manifest.json`'s `initialStatus` (404) while keeping normal routes at 200.

### Lesson 7: Node.js tests pass while live `workerd` crashes on `process.listeners`
Everything passed locally in Node.js — and the Worker uploaded cleanly to Cloudflare — yet our very first live hit to `https://nextdeploy.org/` returned:

```
TypeError: process.listeners is not a function
    at installUnhandledRejectionFilter (next/dist/server/node-environment-extensions/unhandled-rejection.js)
    at next/dist/server/node-environment.js
```

`workerd`'s `nodejs_compat_v2` provides a minimal `process` object without `listeners`, `on`, `off`, `emit`, `hrtime`, and with throwing `[unenv]` stubs for `cwd()` / `uptime()`.

**Fix (two layers):**
1. Shim `next/dist/server/node-environment.js` to keep only the `AsyncLocalStorage` baseline (dropping 9 Node-only global monkey-patches, eliminating 7 noisy `node:crypto` warnings, and shrinking the bundle by 72 KB).
2. Run `ensureProcessCompat()` before importing `NextNodeServer` to safely polyfill any missing or throwing `process.*` property, and add a unit + smoke test that explicitly replaces `globalThis.process` with a stripped `workerd`-like object.

### Lesson 8: Trimming unused Next.js runtimes cuts the bundle under the 3 MB free tier
Next.js's `next-server.js` references dev, experimental, edge-runtime, and `react-dom/static` bundles that are never executed in production. Trimming `*.runtime.dev.js`, `*-experimental.runtime.*`, `edge-runtime`, `react-dom/static`, `caniuse-lite`, and `@swc/helpers` brought the full Next.js 16 Turbopack Worker down from **8.1 MB** to **6.43 MB raw / 1.60 MB gzipped** — well under both Cloudflare Workers' **3 MB gzipped free tier** and **10 MB paid tier**.

### Lesson 9: Always test RSC Flight navigations (`rsc: 1`), not just initial HTML loads
Serving pre-rendered `.html` from R2 makes direct URL visits fast, but client-side `<Link>` navigation in Next.js App Router sends an `rsc: 1` header expecting `text/x-component`. Skipping the HTML fast path on `rsc` requests and loading `.rsc` payloads inside `CloudflareCacheHandler` ensured client-side navigation between all 50+ SSG doc pages returned authentic `200 text/x-component` payloads.

### Lesson 10: Smoke checks should skip Next.js internal error routes
Next.js includes `/_global-error` and `/_not-found` in `routes-manifest.json`'s `staticRoutes`. Post-deploy smoke verification must skip `/_*`, `/404`, and `/500` so it probes real user-facing routes (`/`, `/docs`, `/login`) instead of expecting 200 OK on `/_not-found`.

---

## Next.js 16.4+ Compatibility Update

Upgrading from Next.js `16.0.10` to `16.4.6` surfaced three additional runtime requirements now handled in `cli/internal/serverless/cloudflare/adapter/adapter.go`:

1. **`evalManifestFromRelativePath` + `handleMissing` (`load-manifest.external.js`)**: `NextNodeServer.loadManifests()` calls `evalManifestFromRelativePath` with `handleMissing: true`. Exporting all 5 manifest helpers and returning `undefined` when `handleMissing` is true fixes `TypeError: l2 is not a function`.
2. **Safe `fast-set-immediate.external.js` shim**: Next.js `16.4+` runs `install()` at module load and mutates `require("node:timers").setImmediate`, which throws `TypeError: Cannot set property setImmediate of #<Object> which has only a getter` on esbuild's read-only ESM namespace wrapper. Replacing it with safe exports (`DANGEROUSLY_runPendingImmediatesAfterCurrentTask`, `expectNoPendingImmediates`, `unpatchedSetImmediate`, `ImmediateTracker`, `runWithNativeImmediateTracking`) avoids mutating `node:timers`.
3. **`process.env.__NEXT_DEV_SERVER` Define**: Defining `process.env.__NEXT_DEV_SERVER: ""` at bundle time lets esbuild dead-code eliminate Next.js `16.4+` dev-only imports.

You can verify any real standalone build locally before deploying:

```bash
NEXTDEPLOY_CF_ENGINE=nextserver NEXTDEPLOY_TEST_STANDALONE=/path/to/app/.next/standalone \
  go test -v ./cli/internal/serverless/cloudflare/adapter -run TestBuildWorkerBundle_RealStandaloneSmoke
```

---

## Live Production Results (`https://nextdeploy.org`)

Deployed straight from `nextdeployfrontend` (Next.js 16.0.10, Turbopack, React 19.2.0, 52 routes, Tailwind v4, Geist fonts, edge auth protection, R2 assets) using `doppler run -- nextdeploy ship`:

- **Compile time (`nextcompile` + in-process esbuild):** ~170 ms total
- **Worker bundle size:** **6,432.6 KB raw / 1.60 MB gzipped**
- **Live edge responses on Cloudflare Workers:**

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

---

## Key Source Files

- Engine auto-detection, VFS emitter, static chunk registry, Worker entrypoint: `shared/nextcompile/nextserver.go`
- In-memory `node:fs` VFS shim: `shared/nextcompile/runtime_src/nextserver/vfs_fs_shim.mjs`
- Streaming Web ↔ Node HTTP bridge: `shared/nextcompile/runtime_src/nextserver/http_bridge.mjs`
- R2 + KV `CloudflareCacheHandler`: `shared/nextcompile/runtime_src/nextserver/cf_cache_handler.mjs`
- Request driver & `workerd` `process` polyfill: `shared/nextcompile/runtime_src/nextserver/nextserver_driver.mjs`
- esbuild plugins (`require` rewriter, `load-manifest`/`fast-set-immediate`/`node-environment` shims, bundle trimmer): `cli/internal/serverless/cloudflare/adapter/adapter.go`
- Post-deploy smoke verification: `cli/internal/serverless/smoke.go`
