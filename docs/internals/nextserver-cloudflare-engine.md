# NextServer Cloudflare Engine

NextDeploy's **`nextserver`** engine runs Next.js's own `NextNodeServer` directly inside Cloudflare Workers (`workerd` with `nodejs_compat_v2`) — achieving native compatibility with **Next.js 14, 15, and 16** and **Turbopack** builds without third-party npm adapters.

---

## Why `nextserver` Exists

Classic Cloudflare Worker adapters reimplement Next.js's router, RSC Flight serializer, metadata merger, middleware executor, and Server Action dispatcher from scratch. Every minor Next.js release — and especially Next.js 16's default **Turbopack** bundler — changes internal manifest formats and runtime chunk layouts, creating endless parity drift.

The `nextserver` engine flips the problem around: instead of reimplementing Next.js, NextDeploy provides the **exact runtime primitives `NextNodeServer` expects** inside a V8 isolate, while keeping Cloudflare-native fast paths ahead of it.

---

## Request Lifecycle

```
Incoming Fetch Request (Cloudflare Edge)
  │
  ├─ 1. Edge Protection Guard (guard.mjs)
  │     IP allow/deny CIDR rules, KV rate limiting, HMAC auth cookies
  │
  ├─ 2. Image Optimization (/_next/image)
  │     R2 origin + Cloudflare Images / cf.image / Wasm
  │
  ├─ 3. Static Assets (/_next/static/*, /public/*)
  │     Served straight from R2 (env.ASSETS) — zero Next.js overhead
  │
  ├─ 3b. Pre-rendered SSG / Fresh ISR HTML Fast Path
  │     Non-RSC GET hits R2 directly (_nextdeploy/releases/<ver>/<route>.html)
  │
  ├─ 4. Next.js NextNodeServer (via Streaming HTTP Bridge)
  │     • App Router & Pages Router SSR
  │     • Progressive Suspense & loading.tsx streaming
  │     • Client-side RSC Flight navigations (rsc: 1)
  │     • API Route Handlers & Server Actions
  │     • Middleware & proxy.ts
  │     • ISR & tag revalidation via CloudflareCacheHandler (R2 + KV)
  │     • Custom not-found.tsx & error.tsx boundaries
  │
  └─ 5. Root Public Fallback (on 404 GET)
        Checks R2 for /robots.txt, /favicon.ico, /sitemap.xml, etc.
```

---

## Core Components

### 1. Engine Auto-Detection (`shared/nextcompile/nextserver.go`)

`ResolveEngine` inspects `.next/standalone/.next/server/chunks` for Turbopack markers (`[turbopack]*` or `[root-of-the-server]*`):

| Setting | Behavior |
| :--- | :--- |
| `engine: auto` *(default)* | Selects `nextserver` for Turbopack trees, `legacy` for classic Webpack trees. |
| `engine: nextserver` | Forces `NextNodeServer` engine for both Turbopack and Webpack builds. |
| `engine: legacy` | Forces the legacy vendored-React edge dispatcher. |

Configure in `nextdeploy.yml` under `serverless.cloudflare.engine` or override per invocation with `NEXTDEPLOY_CF_ENGINE=nextserver`.

### 2. Build-Time Virtual Filesystem (`vfs.mjs` + `vfs_fs_shim.mjs`)

`NextNodeServer` synchronously reads `BUILD_ID`, `package.json`, `routes-manifest.json`, `prerender-manifest.json`, `app-paths-manifest.json`, and related manifests at startup and request time.

- **Compile time (`EmitVirtualFS`)**: Walks `.next/standalone` (skipping `static/`, `cache/`, `standalone/`, `trace/`, `.nft.json`, `.map`, and large files > 2 MB) and packs all manifests into `vfs.mjs` with a precomputed directory index.
- **Runtime (`vfs_fs_shim.mjs`)**: Replaces `fs` / `node:fs` / `node:fs/promises` with an in-memory shim supporting `readFileSync`, `existsSync`, `statSync`, `lstatSync`, `realpathSync`, `readdirSync`, `createReadStream`, async callbacks, and `fs.promises`, normalizing arbitrary mount prefixes (e.g. `/var/task/.next/...` → `.next/...`) and raising genuine `ENOENT` errors on misses.

### 3. Static Server Chunk Registry (`chunks.mjs`)

Cloudflare Workers forbid runtime `eval()`, `new Function()`, and dynamic filesystem `require(expr)`.

- **Compile time (`EmitServerChunkRegistry`)**: Walks `.next/server` and generates a static lookup table mapping every server JS/CJS/MJS chunk and manifest module to a static `() => require(...)` thunk that esbuild bundles into the single Worker file.
- **Path normalization**: Turbopack emits runtime chunk requests like `/server/chunks/ssr/[root-of-the-server]__*.js` (omitting `.next/`). `loadServerModule` normalizes leading slashes, `DIST_PREFIX` (`.next/`), and `SERVER_PREFIX` (`.next/server/`).
- **Dynamic `require` patching (`cli/internal/serverless/cloudflare/adapter/adapter.go`)**: An esbuild `OnLoad` plugin rewrites non-literal `require(expr)` calls in `/next/dist/server/` and `/[turbopack]` modules to `__ndDynRequire(expr)`, falling back to JSON VFS lookups and throwing `MODULE_NOT_FOUND` cleanly on optional probes (like `instrumentation.js`).

### 4. Streaming Web ↔ Node HTTP Bridge (`http_bridge.mjs`)

Converts Cloudflare's Web Fetch `Request` into a Node `IncomingMessage`-compatible `Readable` and exposes a Node `ServerResponse`-compatible writable object backed by a Web `ReadableStream`.

- Headers commit on the first `writeHead()`, `flushHeaders()`, or `write()` call, immediately resolving `Promise<Response>` so progressive React `Suspense` chunks stream to the browser without buffering.
- Background completion is registered with `ctx.waitUntil(handlerPromise)` so the isolate stays alive until the stream finishes.

### 5. R2 + KV `CloudflareCacheHandler` (`cf_cache_handler.mjs`)

Implements Next.js's official `IncrementalCache` handler contract:

- **L1**: Per-isolate in-memory `Map` for sub-millisecond warm hits.
- **L2**: Cloudflare R2 (`env.ASSETS`) under `_nextdeploy/cache/<assetVersion>/<key>.json` with non-blocking `ctx.waitUntil` writes and automatic `Buffer` revival for serialized `rscData`.
- **Release Fallback**: On cache miss for non-`FETCH` keys, loads build-time prerendered `.html` + `.rsc` assets from `_nextdeploy/releases/<assetVersion>/<route>.{html,rsc}` (preserving `status: 404` for `/_not-found`), allowing `generateStaticParams` routes to serve both full HTML and client-side RSC Flight navigations without `NoFallbackError`.
- **Cross-Isolate Tag Invalidation**: Records `revalidateTag()` timestamps in KV (`env.NEXTCOMPILE_CACHE` under `tag:<name>`).
- **Global Registration**: Registered on `globalThis[Symbol.for("@next/cache-handlers")].FetchCache` inside `installNextServerGlobals()` so every `IncrementalCache` instance inside Next.js 15/16 uses `CloudflareCacheHandler` without falling back to `FileSystemCache` or triggering dynamic `import()`.

### 6. `workerd` Shims & Bundle Trimming (`cli/internal/serverless/cloudflare/adapter/adapter.go`)

| Target | Why It's Shimmed / Trimmed |
| :--- | :--- |
| `next/dist/server/require-hook.js` | Monkey-patches `Module.prototype.require` (unavailable in ESM / `workerd`). Replaced with a no-op hook map. |
| `next/dist/server/load-manifest.external.js` | `evalManifest` calls `vm.runInNewContext` (blocked by `workerd` CSP). Replaced with VFS + static chunk-registry lookup exporting `loadManifest`, `evalManifest`, `evalManifestFromRelativePath`, `loadManifestFromRelativePath`, and `clearManifestCache` with `handleMissing` support (Next.js `16.4+`). |
| `next/dist/server/node-environment-extensions/fast-set-immediate.external.js` | Calls `install()` on import and assigns `require("node:timers").setImmediate = ...`, throwing on esbuild's read-only ESM namespace wrapper in Next.js `16.4+`. Replaced with safe no-op exports (`DANGEROUSLY_runPendingImmediatesAfterCurrentTask`, `expectNoPendingImmediates`, `unpatchedSetImmediate`, `ImmediateTracker`, `runWithNativeImmediateTracking`). |
| `next/dist/server/node-environment.js` | Calls `process.listeners("unhandledRejection")` and monkey-patches `node:crypto`. Replaced with just the `AsyncLocalStorage` baseline. |
| `process.env.__NEXT_DEV_SERVER` | Defined as `""` in esbuild so Next.js `16.4+` dead-code eliminates dev-only `require()` branches (e.g. `hot-reloader-app`). |
| `ensureProcessCompat()` | Polyfills missing/throwing `[unenv]` `process` methods in `workerd` (`listeners`, `on`/`off`/`emit`, `cwd`, `uptime`, `hrtime.bigint`, `stdout`, `stderr`, `versions`). |
| Dev / experimental / static runtimes | Trims `*.runtime.dev.js`, `*-experimental.runtime.*`, `edge-runtime`, `react-dom/static`, `caniuse-lite`, `@swc/helpers` (~1.60 MB gzipped final bundle). |

---

## Local Verification Against Any Real Standalone Build

An opt-in Go smoke test compiles and executes any real Next.js standalone build directly:

```bash
NEXTDEPLOY_CF_ENGINE=nextserver NEXTDEPLOY_TEST_STANDALONE=/path/to/app/.next/standalone \
  go test -v -count=1 ./cli/internal/serverless/cloudflare/adapter -run TestBuildWorkerBundle_RealStandaloneSmoke
```
