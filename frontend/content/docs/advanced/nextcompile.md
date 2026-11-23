---
title: NextCompile (Cloudflare Runtime)
lede: NextDeploy's pure-Go compiler and NextServer Worker engine that runs unmodified Next.js 14/15/16 + Turbopack apps directly inside Cloudflare Workers (workerd) — with a build-time Virtual FS, static chunk registry, zero-buffering streaming HTTP bridge, and R2 + KV CacheHandler.
status: ready
sourceRef: shared/nextcompile/, shared/nextcompile/runtime_src/nextserver/, cli/internal/serverless/cloudflare_adapter.go
related:
  - title: Running Next.js 16 Turbopack on Cloudflare (Deep Dive)
    href: /docs/advanced/nextjs-16-turbopack-on-cloudflare
  - title: Workers Runtime
    href: /docs/cloudflare/workers
  - title: Cloudflare Overview
    href: /docs/cloudflare/overview
  - title: Auto-Provisioning
    href: /docs/advanced/auto-provisioning
---

## Why NextCompile Runs `NextNodeServer` Inside `workerd`

Cloudflare Workers execute inside V8 isolates (`workerd`) rather than a Linux container — they cannot spawn a child `node` process or read from a local disk.

Early Cloudflare adapters tried to solve this by reimplementing Next.js's router, RSC Flight serializer, metadata merger, middleware executor, and Server Action dispatcher from scratch. Every Next.js release — especially **Next.js 16's default Turbopack bundler** — breaks custom reimplementations.

NextCompile solves this with two engines controlled by `serverless.cloudflare.engine`:

| Engine | When Used | How It Works |
| :--- | :--- | :--- |
| **`nextserver`** | Default for Turbopack & Next.js 16+ (`engine: auto`), or explicit `engine: nextserver` | Runs Next.js's own `NextNodeServer` inside `workerd` (`nodejs_compat_v2`) via a Virtual FS, static chunk registry, streaming HTTP bridge, and R2+KV `CacheHandler`. |
| **`legacy`** | Default for classic Webpack trees (`engine: auto`), or explicit `engine: legacy` | Lightweight custom Worker dispatcher with vendored React edge runtimes. |

Override per deploy with `NEXTDEPLOY_CF_ENGINE=nextserver|legacy|auto` or in `nextdeploy.yml`:

```yaml
serverless:
  provider: cloudflare
  cloudflare:
    engine: auto # auto | nextserver | legacy
    compatibility_date: "2025-04-01"
    compatibility_flags:
      - nodejs_compat_v2
```

## Request Lifecycle (`nextserver` Engine)

Cloudflare-native fast paths stay ahead of `NextNodeServer` so static assets, edge auth, and pre-rendered HTML never pay Node runtime overhead:

```
Incoming Fetch Request (Cloudflare Edge)
  │
  ├─ 1. Edge Protection Guard (guard.mjs)
  │     IP allow/deny CIDR rules, KV rate limiting, HMAC session auth
  │
  ├─ 2. Image Optimization (/_next/image)
  │     R2 origin + Cloudflare Images / cf.image / Wasm
  │
  ├─ 3. Static Assets (/_next/static/*, /public/*)
  │     Served directly from R2 (env.ASSETS)
  │
  ├─ 3b. Pre-rendered SSG / Fresh ISR HTML Fast Path
  │      Non-RSC GET hits R2 directly (_nextdeploy/releases/<ver>/<route>.html)
  │
  ├─ 4. Next.js NextNodeServer (Streaming HTTP Bridge)
  │     • App Router & Pages Router SSR (Turbopack & Webpack)
  │     • Progressive Suspense & loading.tsx streaming
  │     • Client-side RSC Flight navigations (rsc: 1)
  │     • API Route Handlers, Server Actions, Middleware & proxy.ts
  │     • ISR & revalidateTag via CloudflareCacheHandler (R2 + KV)
  │     • Custom not-found.tsx (true HTTP 404) & error.tsx boundaries
  │
  └─ 5. Root Public Fallback (on 404 GET)
        Checks R2 for /robots.txt, /favicon.ico, /sitemap.xml, etc.
```

## Core Architecture

### 1. Build-Time Virtual Filesystem (`vfs.mjs` + `vfs_fs_shim.mjs`)

`NextNodeServer` synchronously reads `BUILD_ID`, `package.json`, `routes-manifest.json`, `prerender-manifest.json`, and `app-paths-manifest.json` at startup and request time.

- **Compile time (`EmitVirtualFS`)**: Walks `.next/standalone` (skipping `static/`, `cache/`, `trace/`, `.nft.json`, `.map`, and files over 2 MB) and emits all manifests plus a precomputed directory index into `vfs.mjs`.
- **Runtime (`vfs_fs_shim.mjs`)**: Aliases `fs`, `node:fs`, `fs/promises`, and `node:fs/promises` to an in-memory shim supporting `readFileSync`, `existsSync`, `statSync`, `realpathSync`, `readdirSync`, `createReadStream`, and async/promises APIs, normalizing mount prefixes (e.g. `/var/task/.next/...` → `.next/...`) and raising genuine `ENOENT` errors on misses.

### 2. Static Server Chunk Registry (`chunks.mjs`)

Cloudflare Workers forbid `eval()`, `new Function()`, and dynamic filesystem `require(expr)`.

- **Compile time (`EmitServerChunkRegistry`)**: Walks `.next/server` and generates a static lookup table mapping every server chunk and manifest module to a static `() => require(...)` thunk bundled by esbuild.
- **Turbopack path normalization**: Turbopack emits runtime chunk requests like `/server/chunks/ssr/[root-of-the-server]__*.js` (omitting `.next/`). `loadServerModule` normalizes `DIST_PREFIX` (`.next/`) and `SERVER_PREFIX` (`.next/server/`).
- **Dynamic `require` patching**: An in-process esbuild `OnLoad` plugin rewrites non-literal `require(expr)` calls in `/next/dist/server/` and `/[turbopack]` modules to `__ndDynRequire(expr)` with VFS JSON fallback and proper `MODULE_NOT_FOUND` errors for optional probes like `instrumentation.js`.

### 3. Zero-Buffering Streaming HTTP Bridge (`http_bridge.mjs`)

Converts Web Fetch `Request` into a Node `IncomingMessage`-compatible `Readable`, and pairs a Node `ServerResponse`-compatible EventEmitter with a Web `ReadableStream`.

- Headers commit on the first `writeHead()`, `flushHeaders()`, or `write()`, immediately resolving `Promise<Response>` so React `Suspense` boundaries stream progressively to the browser.
- `ctx.waitUntil(handlerPromise)` keeps the Worker isolate alive until `res.end()` closes the stream.

### 4. R2 + KV `CloudflareCacheHandler` (`cf_cache_handler.mjs`)

Implements Next.js's official `IncrementalCache` handler contract across three layers:

- **L1**: Per-isolate in-memory `Map` for sub-millisecond warm hits.
- **L2**: Cloudflare R2 (`env.ASSETS`) under `_nextdeploy/cache/<assetVersion>/<key>.json` with non-blocking `ctx.waitUntil` writes and automatic `Buffer` revival for serialized `rscData`.
- **Release Asset Fallback**: On cache miss for non-`FETCH` keys, loads build-time prerendered `.html` and `.rsc` files from `_nextdeploy/releases/<assetVersion>/<route>.{html,rsc}` (preserving `status: 404` for `/_not-found`), allowing `generateStaticParams` routes to serve both full HTML and client-side RSC Flight payloads without `NoFallbackError`.
- **Global Registration**: Registered on `globalThis[Symbol.for("@next/cache-handlers")].FetchCache` inside `installNextServerGlobals()` so every per-route `IncrementalCache` instance in Next.js 15/16 uses it natively without falling back to `FileSystemCache` or invoking dynamic `import()`.

### 5. `workerd` Shims & Bundle Trimming

| Module / Surface | Treatment |
| :--- | :--- |
| `next/dist/server/require-hook.js` | Replaced with a no-op hook map (avoids `Module.prototype.require` crash in ESM). |
| `next/dist/server/load-manifest.external.js` | Replaced with a VFS + static chunk-registry shim (eliminates `vm.runInNewContext`). |
| `next/dist/server/node-environment.js` | Replaced with the `AsyncLocalStorage`-only baseline (eliminates `process.listeners` crash and `node:crypto` warnings). |
| `ensureProcessCompat()` | Polyfills missing/throwing `[unenv]` `process` properties (`listeners`, `on`/`off`/`emit`, `cwd`, `uptime`, `hrtime.bigint`, `stdout`, `stderr`, `versions`). |
| Dev / experimental / static runtimes | Trims `*.runtime.dev.js`, `*-experimental.runtime.*`, `edge-runtime`, `react-dom/static`, `caniuse-lite`, `@swc/helpers`. |

## Bundle Size & Compile Speed

Measured on this exact documentation site (`nextdeployfrontend` — Next.js 16.0.10, Turbopack, React 19.2.0, 52 routes, Tailwind v4):

- **`nextcompile` + in-process esbuild:** ~170 ms total
- **Final `worker.mjs` size:** **6,432.6 KB raw / 1.60 MB gzipped** (fits even Cloudflare Workers' **3 MB gzipped free tier**)

## Local Smoke Testing Against Real Builds

You can compile and execute any real Next.js standalone build locally using the opt-in Go smoke test:

```bash
NEXTDEPLOY_TEST_STANDALONE=/path/to/app/.next/standalone \
  go test -v -count=1 ./cli/internal/serverless -run TestBuildWorkerBundle_RealStandaloneSmoke
```
