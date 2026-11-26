# Part 5 — Systems-Engineering Interview Deep-Dive Q&A & Trade-Off Playbook

> Every symbol, file path, constant, and architectural trade-off below is verified against the workspace source code.
> Designed for senior infrastructure / runtime / platform interviews (e.g., Fly.io, Cloudflare, Vercel) where interviewers push past headlines ("filesystem breaks") into exact mechanisms, failure modes, and what you would change next.

---

## How to Answer Principal/Staff Systems Follow-ups

Whenever an interviewer asks **"How does X work?"** or **"What breaks if you do Y?"**, use a 3-beat structure:

1. **Exact Mechanism** — Name the concrete constraint in `workerd` / V8 / R2 and the exact Go compile-time transform or JS runtime primitive you emitted to solve it.
2. **Failure Mode** — State what crashes or silently corrupts if a naive solution misses an edge case (e.g., `ENOENT`, Turbopack missing `.next/` prefix, `ctx.waitUntil`).
3. **Honest Trade-Off + Evolution** — Name the real limitation still in the code today and how you'd fix it at the next scale tier.

---

## Round 1 — Bundling Unmodified Next.js 16 Turbopack `NextNodeServer` into `workerd`

> **Interviewer Question:** *"If you take a real Next.js 16 Turbopack `.next/standalone` build and try to bundle `NextNodeServer` directly into a Cloudflare Worker, name specific things that immediately break in `workerd` — and walk through what your Go compiler emits to solve dynamic `require(chunkPath)`."*

### 6 Specific Breakages (Not Just "Filesystem and Network")

1. **Synchronous `fs.*` calls during `NextNodeServer` boot and request handling**
   - *Breakage:* `NextNodeServer` synchronously calls `fs.readFileSync`, `existsSync`, `statSync`, `lstatSync`, `realpathSync`, and `readdirSync` to read `BUILD_ID`, `package.json`, `routes-manifest.json`, `prerender-manifest.json`, `app-paths-manifest.json`, and `required-server-files.json`. In `workerd`, `node:fs` is either unavailable or a stub.
   - *Fix (`shared/nextcompile/nextserver.go` + `cli/internal/serverless/cloudflare_adapter.go`):*
     - **Compile time (`EmitVirtualFS`)**: Walks `.next/standalone` (skipping `static/`, `cache/`, `standalone/`, `trace/`, `.nft.json`, `.map`, and files `> 2 MiB`) and packs manifests + a precomputed `dirs` index into `_nextdeploy/vfs.mjs` alongside `nextConfig` from `required-server-files.json`.
     - **Bundle time (`vfsFsPlugin`)**: Intercepts `^(node:)?fs$` and `^(node:)?fs/promises$` ahead of `nodeBuiltinsPlugin` and routes them to `vfs_fs_shim.mjs` / `vfs_fs_promises_shim.mjs`.
     - **Runtime detail (`normalizeVFSPath`)**: Strips arbitrary host prefixes (e.g., `/var/task/.next/...` → `.next/...`) and raises authentic `ENOENT` errors (`err.code = "ENOENT"`, `err.errno = -2`) on misses — critical because Next.js probes optional files inside `try/catch` checking `e.code === "ENOENT"`.

2. **Runtime dynamic `require(chunkPath)` in Turbopack & Next server internals**
   - *Breakage:* Turbopack emits `[turbopack]_runtime.js` and `[root-of-the-server]__*.js` that load route handlers and SSR chunks via `require(variable)`. `esbuild` cannot statically bundle a non-literal `require(expr)`, and `workerd` has no disk loader at runtime.
   - *Fix (`EmitChunkRegistry` in `shared/nextcompile/nextserver.go` + `dynamicRequirePatchPlugin` in `cli/internal/serverless/cloudflare_adapter.go`):*
     - **Compile time (`EmitChunkRegistry`)**: Walks `<standaloneDir>/<distDir>/server/` collecting every `.js`, `.cjs`, and `.mjs` file and emits `_nextdeploy/chunks.mjs` as an `Object.freeze({...})` table mapping each relative path to a static `() => require("../.next/server/...")` thunk. Because every `require` argument is a string literal, in-process `esbuild` bundles every server chunk into `worker.mjs`.
     - **Bundle time (`dynamicRequirePatchPlugin`)**: Scans every `.js`/`.cjs` matching `/[turbopack]` or `/next/dist/server/` (excluding `/compiled/`) and rewrites non-literal `require(expr)` calls via `dynamicRequireExprRe` to `__ndDynRequire(expr)` (and `await dynamicImportEsmDefault(` to `await __ndDynImport(`).
     - **Runtime (`loadServerModule` + `normalizeModuleKey` in `chunks.mjs`)**: Turbopack often requests paths omitting `.next/` (like `/server/chunks/ssr/[root-of-the-server]__*.js`). `normalizeModuleKey` cleans `.`/`..`, tries exact match, `SERVER_PREFIX` slice, `DIST_PREFIX` (`.next/`) prepend, `SERVER_PREFIX` (`.next/server/`) prepend, and `.js`/`.cjs`/`.mjs` extension probing. `__ndDynRequire` falls back to `JSON.parse(globalThis.__nextdeployVFS.files[key])` for `.json` requires and throws `err.code = "MODULE_NOT_FOUND"` on misses so optional Next probes (like `instrumentation.js`) degrade cleanly.

3. **`vm.runInNewContext` inside `next/dist/server/load-manifest.external.js`**
   - *Breakage:* Next's `evalManifest()` evaluates `.js` manifests (`globalThis.__RSC_MANIFEST[...] = ...`) inside `vm.runInNewContext`, which throws in `workerd` because V8 dynamic code generation (`eval` / `vm`) is forbidden by Worker CSP.
   - *Fix (`dynamicRequirePatchPlugin`):* Replaces `next/dist/server/load-manifest.external.js` on load so `loadManifest` reads via shimmed `fs.readFileSync` and `evalManifest` invokes `globalThis.__nextdeployLoadModule(manifestPath)` from the pre-bundled `chunks.mjs` registry (throwing `ENOENT` on miss).

4. **`Module.prototype.require` monkey-patching in `next/dist/server/require-hook.js`**
   - *Breakage:* `require-hook.js` patches Node's `Module.prototype.require` for alias overrides, which does not exist in an ESM Worker bundle.
   - *Fix (`dynamicRequirePatchPlugin`):* Replaces `require-hook.js` with a minimal no-op `hookPropertyMap = new Map()`, `defaultOverrides = {}`, `addHookAliases()`.

5. **`node-environment.js` and incomplete `unenv` `process` stubs**
   - *Breakage:* `next/dist/server/node-environment.js` calls `process.listeners("unhandledRejection")` and monkey-patches `node:crypto`, and `workerd`'s `nodejs_compat_v2` `unenv` shim leaves methods like `process.listeners`, `process.cwd()`, `process.uptime()`, and `process.hrtime.bigint()` missing or throwing `not implemented`.
   - *Fix:* `dynamicRequirePatchPlugin` replaces `node-environment.js` with just `globalThis.AsyncLocalStorage` from `node:async_hooks`, and `ensureProcessCompat()` (`shared/nextcompile/runtime_src/nextserver/nextserver_driver.mjs`) polyfills `listeners`, EventEmitter chaining methods, `cwd` (`"/"`), `uptime` (`0`), `hrtime`/`hrtime.bigint`, `stdout`/`stderr`, and `versions` using `setProcessProp` (handling non-writable `unenv` descriptors).

6. **Node `(req, res)` HTTP protocol vs Web Fetch `(Request) => Promise<Response>`**
   - *Breakage:* `NextNodeServer.getRequestHandler()` writes to a Node `ServerResponse` rather than returning a Web `Response`.
   - *Fix (`shared/nextcompile/runtime_src/nextserver/http_bridge.mjs`):* Bridges `Request` → Node `Readable` (`IncomingMessage`-compatible) and `ServerResponse` → Web `ReadableStream` `Response` without buffering.

### Honest Trade-Offs to Volunteer

- **Single-bundle ceiling:** Emitting every server chunk in `chunks.mjs` into one `worker.mjs` means cold-start V8 parse/compile cost scales with total app routes, and massive monorepos can approach Cloudflare's 3 MB (free) / 10 MB (paid) gzipped Worker limit. *Evolution:* Split route subtrees into multiple Workers behind Cloudflare Service Bindings using `dispatch.mjs`'s route partition.
- **Regex vs AST rewrite:** `dynamicRequireExprRe` (`require\(\s*([^'"`\s][^)]*)\)`) in `cli/internal/serverless/cloudflare_adapter.go:633` is a regex pass, not a JS AST transform. If a future Next.js/Turbopack release emits a nested function call inside `require(...)` (e.g., `require(path.join(dir, name))`), `[^)]*` stops at the inner `)`. *Evolution:* Replace `patchDynamicRequires` with an esbuild/SWC AST visitor if nested expressions appear.

---

## Round 2 — Zero-Buffering Streaming Bridge & Isolate Lifetime

> **Interviewer Question:** *"In `workerd`, if you return `new Response(stream)` as soon as headers flush, the Worker runtime can freeze or tear down the isolate as soon as `fetch()` resolves. How do you stream Next.js App Router `<Suspense>` chunks without buffering the page or getting killed mid-stream?"*

### Exact Mechanism (`shared/nextcompile/runtime_src/nextserver/http_bridge.mjs`)

- `createStreamingServerResponse(method)` pairs a Node `EventEmitter`-based `ServerResponse` shim with a Web `ReadableStream` (`bodyStream`) and a `responsePromise`.
- `commitHeaders()` fires **only once** on the first `writeHead()`, `flushHeaders()`, `write()`, or `end()`. It immediately resolves `responsePromise` with `new Response(bodyStream, ...)` (or `new Response(null, ...)` for `HEAD` and `NULL_BODY_STATUSES = {101, 204, 205, 304}`), giving immediate TTFB for the shell.
- Subsequent `res.write(chunk)` calls enqueue `Uint8Array` bytes straight into `streamController.enqueue(bytes)`.
- `invokeNodeHandler(handler, request, ctx)` passes the full `handlerPromise` (which only settles after `handler(req, res)` finishes and `res.end()` closes `bodyStream`) to **`ctx.waitUntil(handlerPromise)`**. That explicitly tells `workerd` to keep the isolate alive until the stream completes.
- If `handler` throws *before* `headersSent`, `invokeNodeHandler` writes a `500` plain response; if *after* headers committed, it calls `res.destroy(err)` which errors `bodyStream`.

### Honest Trade-Off & Evolution

- **No stream backpressure:** In `http_bridge.mjs:285-292`, `res.write()` unconditionally returns `true` and does not inspect `streamController.desiredSize` or emit Node's `"drain"` event. A handler streaming megabytes faster than a slow mobile client consumes can buffer chunks in V8 isolate memory. *Evolution:* Return `streamController.desiredSize > 0` from `write()` and wire `ReadableStream`'s `pull()` hook to emit `"drain"` on the `EventEmitter`.

---

## Round 3 — Concurrency & Isolate Global State Races

> **Interviewer Question:** *"V8 isolates handle multiple concurrent requests via `async/await` interleaving. Where does your `nextserver` driver store per-request `env` and `ctx`, and what happens under concurrent requests?"*

### Exact Mechanism & Real Bug Surface (`shared/nextcompile/runtime_src/nextserver/nextserver_driver.mjs`)

In `dispatchNextServer(request, env, ctx, opts)` (`nextserver_driver.mjs:236-239`):

```js
globalThis.__nextdeployEnv = env;
globalThis.__nextdeployWorkerCtx = ctx || null;
globalThis.__nextdeployAssetVersion = manifest?.assetVersion || "";
pushEnvIntoProcess(env);
```

- `env` bindings (R2 `ASSETS`, KV `NEXTCOMPILE_CACHE`, secrets) are identical across requests within a single deployed Worker version, so overwriting `globalThis.__nextdeployEnv` with the same bindings is benign.
- **However**, `ctx` (`ExecutionContext`, carrying `ctx.waitUntil`) is **per-request** in `workerd`. When `CloudflareCacheHandler.set()` or `revalidateTag()` calls `getWorkerCtx()` (`cf_cache_handler.mjs:22-24, 189-194`) after an `await` (for example, after an R2 `get` or async SSR render), a concurrent Request B entering `dispatchNextServer` has overwritten `globalThis.__nextdeployWorkerCtx` with Request B's `ctx`. If Request B finishes before Request A's R2 `put` settles, Request A's background write was registered on Request B's `ctx` (or `ctx.waitUntil` may throw/drop if Request B already settled).

### How You'd Fix It

- Notice that `EngineLegacy` (`shared/nextcompile/runtime_src/context.mjs`) **already** solves this cleanly using `node:async_hooks` `AsyncLocalStorage` (`runWithContext(reqCtx, fn)`, storing `workerCtx: ctx` inside the ALS store).
- *Evolution for `nextserver_driver.mjs`:* Wrap `invokeNodeHandler` in an `AsyncLocalStorage` store carrying `{ env, ctx }` and have `cf_cache_handler.mjs`'s `getEnv()` / `getWorkerCtx()` read `als.getStore()` first, falling back to `globalThis` only for unit tests.

---

## Round 4 — Multi-Tier Edge Caching, `CloudflareCacheHandler`, and Tag Invalidation

> **Interviewer Question:** *"How do you make Next.js ISR, `generateStaticParams` RSC navigations, and `revalidateTag()` work across distributed Cloudflare Worker isolates without a shared filesystem?"*

### Exact Mechanism (`shared/nextcompile/runtime_src/nextserver/cf_cache_handler.mjs` + `nextserver_driver.mjs`)

1. **Global Handler Registration (avoiding dynamic `import()`):**
   - `createNextServer()` deliberately sets `cacheHandler: undefined` and `cacheMaxMemorySize: 0` (`nextserver.go:368-369`) so `NextNodeServer` never calls dynamic `import(filePath)`.
   - `installNextServerGlobals()` registers `CloudflareCacheHandler` on `globalThis[Symbol.for("@next/cache-handlers")].FetchCache` and `globalThis.__nextdeployCacheHandler`.
2. **Three Lookup Tiers in `CloudflareCacheHandler.get(key, ctx)`:**
   - **L1 (`l1Cache`):** Per-isolate `Map` for sub-millisecond warm hits.
   - **L2 (Cloudflare R2 `env.ASSETS`):** Reads `_nextdeploy/cache/<assetVersion>/<key>.json`. Because JSON serialization turns Node `Buffer`s into `{ type: "Buffer", data: [...] }`, `reviveSerializedBuffers()` converts `value.rscData` back into a live `Buffer`.
   - **Release Fallback (for `ctx.kind !== "FETCH"` and `key.startsWith("/")`):** Reads build-time prerendered `_nextdeploy/releases/<assetVersion>/<route>.html` and `<route>.rsc` in parallel from R2 and constructs an `APP_PAGE` entry (setting `status: 404` for `/_not-found`). This lets `generateStaticParams` routes serve both HTML and client-side RSC Flight navigations without triggering Next's `NoFallbackError`.
3. **Cross-Isolate Tag Invalidation (`revalidateTag`):**
   - Records `localTagRevalidatedAt.set(tag, now)`, purges matching L1 entries, and writes `tag:<tag>` = `String(now)` to KV (`env.NEXTCOMPILE_CACHE`, `expirationTtl: 7 days`) via `ctx.waitUntil`. Both L1 and L2 reads check `isEntryInvalidatedByTags(tags, lastModified, env)`.

### Honest Trade-Offs & Evolution

- **Unbounded L1 `Map`:** `l1Cache` (`cf_cache_handler.mjs:15`) has no max-entry count or LRU eviction. High-cardinality dynamic routes cached in L1 can accumulate until the isolate hits `workerd`'s 128 MB memory limit. *Evolution:* Bound `l1Cache` with a small LRU (e.g., 100–256 entries or byte-budgeted eviction) and rely on R2 for the long tail.
- **KV eventual consistency & best-effort R2 writes:** KV writes can take up to ~60s to propagate globally across Cloudflare PoPs, and failed R2 `put` calls inside `waitUntil` are logged without retry. *Evolution:* Route background revalidations through a Cloudflare Queue consumer with DLQ for guaranteed R2 write-back.

---

## Round 5 — Shrinking Next.js 16 + React 19 to ~1.60 MB Gzipped

> **Interviewer Question:** *"A normal `.next/standalone` `node_modules` directory is tens of megabytes. How do you fit `NextNodeServer` plus a full Next 16 app inside Cloudflare Workers' bundle limits without breaking runtime behavior?"*

### Exact Mechanisms (`cli/internal/serverless/cloudflare_adapter.go` + `shared/nextcompile/nextserver.go`)

1. **`nextServerTrimPlugin` (`trimFilter`):** Replaces build-only, native-binary, dev, and experimental modules with a zero-byte recursive CJS `Proxy` stub (`next-trim-stub`): `sharp`, `webpack`, `terser`, `typescript`, `@next/swc-*`, `@babel/*`, `styled-jsx/macro`, `caniuse-lite`, `browserslist`, `@vercel/og`, `react-dom/static`, `edge-runtime*`, `*.runtime.dev.js`, `*-experimental.runtime.*`, `next/dist/telemetry/*`, `next/dist/trace/*`, `next/dist/build/*`.
2. **Selective VFS Packing (`shouldInlineInVFS`):** Excludes `static/`, `cache/`, `standalone/`, `trace/`, `*.nft.json`, `*.map`, and any file `> 2 MiB` from `vfs.mjs`.
3. **Syntax + Whitespace Minification Without Mangling (`cloudflare_adapter.go:509-511`):** Enables `MinifySyntax`, `MinifyWhitespace`, and `LegalCommentsNone`, intentionally keeping `MinifyIdentifiers: false` so `workerd` tail logs and stack traces remain readable.
4. **Builtin Split (`nodeBuiltinsPlugin`):** Marks 37 `workerd`-native `node:*` modules external (wrapping CJS `require()` calls in `"node-native"` ESM facades) and stubs 25 unavailable modules (`child_process`, `vm`, `worker_threads`, etc.) via `"node-stub"`.
5. **Observability (`logBundleTopContributors`):** Emits `esbuild-meta.json` and logs the top 10 byte contributors on every build so operators know immediately if a user dependency blew the budget.

---

## Round 6 — Atomic Deploys, R2 Consistency & Instant Rollbacks

> **Interviewer Question:** *"If you deploy a new Worker version and new static/prerendered assets to R2, how do you prevent a mid-deploy race where a live page references a JS chunk that isn't in R2 yet — or a rollback reading stale HTML from a previous build with the same `BUILD_ID`?"*

### Exact Mechanism (`cli/internal/serverless/cloudflare.go:540-639` + `shared/nextcompile/runtime_src/serve.mjs`)

1. **Two-Phase R2 Upload (`DeployStatic`):**
   - `partitionAssets()` splits assets into **immutable** (`_next/static/*`, content-hashed filenames) and **mutable** (`public/*`, SSG `.html`, `.rsc`).
   - Phase 1 uploads all immutable `_next/static/*` chunks and waits on `wg.Wait()` as a hard barrier. If any chunk upload fails, `DeployStatic` aborts before any mutable HTML or Worker script is updated.
   - Phase 2 computes `assetVersionForAssets(mutable)` — a deterministic SHA-256 over sorted key lengths, key names, and file byte digests — and namespaces every mutable object under `_nextdeploy/releases/<assetVersion>/...`.
2. **Worker Coupling:** `meta.AssetVersion` flows into `manifest.json` (`assetVersion`) bundled inside `worker.mjs`. `DeployCompute` uploads `worker.mjs` *after* `DeployStatic` succeeds, with `computeDeployHash` skipping unchanged uploads.
3. **Why not `BUILD_ID`?** Next.js apps can reuse or pin `BUILD_ID` across builds; content-hashing the mutable assets guarantees every rollback restores the exact matching `.html`/`.rsc`/`public/*` snapshot.

### Honest Trade-Off & Evolution

- **R2 storage growth:** Old `_nextdeploy/releases/<assetVersion>/` prefixes are intentionally retained so `nextdeploy rollback` is instant. Over hundreds of deploys, stale release prefixes accumulate in R2. *Evolution:* Track active/retained release versions in the encrypted R2 state manifest and prune prefixes older than the retention window (matching the VPS 5-release ring buffer).

---

## Round 7 — Dual-Condition React 19 in `EngineLegacy`

> **Interviewer Question:** *"React 19 publishes CJS-only builds and requires the `react-server` export condition to encode RSC Flight streams, but standard `react` (without `react-server`) to run Fizz SSR (`React.use()`). How does `EngineLegacy` bundle both in one Worker?"*

### Exact Mechanisms (`shared/nextcompile/vendor.go` + `cloudflare_adapter.go` + `ssr.mjs`)

1. **React 19 CJS Vendoring (`VendorRSC` + `findEdgeBuild`):**
   - React 19's flat `server.edge.js` is a dynamic conditional shim (`require("./cjs/...production.js")`). Copying it alone leaves a dangling require. `findEdgeBuild` ranks concrete `esm/...` and `cjs/<pkg>-<entry>.production.js` **above** `<entry>.js`, writes CJS payloads as `<entry>.cjs`, and generates a static ESM facade (`<entry>.mjs`: `export * from "./<entry>.cjs"; export { default } from "./<entry>.cjs";`).
   - Vendors 4 builds: `react-server-dom-webpack/server.edge`, `client.edge`, `react-dom/server.edge`, and standard `react`.
2. **Per-Importer Condition Splitting (`reactConditionPlugin`):**
   - `runEsbuild` sets `Conditions: ["react-server", "workerd", "worker", "node"]`, so normal page/RSC imports get `react.react-server`.
   - `reactConditionPlugin` intercepts `^react$` and redirects SSR importers (`/vendor/react-dom/`, `/vendor/react-server-dom-webpack/client.edge`, `/runtime/ssr.mjs`) to `_nextdeploy/runtime/vendor/react/index.mjs` (standard React with `React.use()`).
3. **Streaming Composition (`ssr.mjs`):**
   - Tees the Flight stream twice (`flightStream.tee()` for SSR-vs-fallback, then `forSSR`/`forInline`), passes `React.use(rootPromise)` into Fizz for M3 progressive `<Suspense>` flushing, uses `createHeadInjector` (64 KB budget) to splice metadata + CSS right after `<head>`, and gates `pumpFlight` on `htmlStarted` so no `<script>` precedes `<!DOCTYPE html>`.

---

## Round 8 — Edge Security & Isolation Boundaries

> **Interviewer Question:** *"What stops a client from spoofing rate-limit IPs, bypassing route auth via static fallback, triggering SSRF on `/_next/image`, or OOMing the Worker via Server Actions?"*

| Attack Vector | Exact Defense in Code |
|---|---|
| **IP allowlist / rate-limit spoofing** | `clientIP()` (`guard.mjs:31-43`) trusts `cf-connecting-ip` unconditionally; `x-forwarded-for` / `x-real-ip` are ignored unless `trustForwardedFor: true`. Unknown IP (`""`) fails closed (`403`) against non-empty `allow` lists (`ipDenied`) and shares one `"rl::..."` rate-limit bucket. Missing KV binding fails closed (`503`). |
| **Auth bypass via root public fallback** | `serveRootPublicFromR2` (`serve.mjs:49-93`) checks `isPrerenderedPageKey(key, manifest)` so `/dashboard.html` can never bypass `guard.mjs` protecting `/dashboard`. |
| **`/_next/image` SSRF & Worker self-fetch loops** | `handleImageRequest` (`image.mjs:42-65, 178-187`) resolves `new URL(src, request.url)` *first* (defeating `//evil.com` and `%20https://`), requires `http:`/`https:`, blocks `localhost`/`127.*`/`10.*`/`172.16-31.*`/`192.168.*`/`169.254.169.254` (`isBlockedHost`), enforces `remotePatterns`/`domains`, and reads same-origin images directly from R2 (`env.ASSETS`). |
| **Server Action OOM DoS & CSRF** | `actions.mjs` enforces same-origin `Origin` vs `request.url` (`originMatchesHost`) and enforces `MAX_BODY_BYTES = 2 MiB` *incrementally* while reading `request.body.getReader()` (`readBoundedBody`), cancelling the stream and returning `413` before buffering unbounded multipart uploads. |
| **Flight XSS injection** | `htmlEscapeJsonString` (`ssr.mjs:243-254`) escapes `&`, `>`, `<`, `\u2028`, `\u2029` before embedding chunks into `<script>(self.__next_f=...).push(...)</script>`. |
| **Accidental secret wipe on deploy** | `DeployCompute` (`cloudflare.go:1027-1034`) calls `refuseSecretWipe` because Cloudflare Worker script uploads replace rather than merge `secret_text` bindings. |

---

## Round 9 — Why Pure Go + Embedded `esbuild` Instead of a JS/TS CLI?

> **Interviewer Question:** *"Why write the compiler in Go if you still invoke `esbuild` to produce `worker.mjs`?"*

- **Single self-contained binary:** `shared/nextcompile/runtime.go` uses `//go:embed all:runtime_src`, and `cloudflare_adapter.go` links `github.com/evanw/esbuild/pkg/api` directly in-process. `nextdeploy` needs zero global npm installs, zero adapter version skew in the user's `package.json`, and zero `npx` startup overhead.
- **Bounded parallel analysis & determinism:** `ScanCompiledServer` uses a bounded `errgroup` (`scanConcurrency = 32`) to scan standalone trees without hitting macOS FD limits (`256`), and `hashBundle` computes a deterministic SHA-256 digest (zeroing `Manifest.GeneratedAt` and using `baseDir`-relative POSIX paths) so CI and local deploys produce identical content hashes.
- **Honest trade-off:** Next.js itself still runs in Node during `next build` (and `nextcore` invokes Node to evaluate `next.config.*`), and upgrading `esbuild` requires releasing a new `nextdeploy` binary rather than bumping an npm package.

---

## Quick 30-Second Spoken Drills (Ready for Live Interviews)

### Drill A — Answering the Fly.io `nextcompile` Follow-up Directly
> *"At runtime, Turbopack chunks and `NextNodeServer` fail in `workerd` on two coupled mechanisms: synchronous `fs` reads for manifests, and runtime `require(variable)` / `vm.runInNewContext` for server chunks — both forbidden in V8 isolates.*
> *To solve that without reimplementing Next.js, our Go compiler runs two compile-time passes over `.next/standalone`: `EmitVirtualFS` packs all `.json`/`.meta`/`BUILD_ID` manifests and a directory index into `vfs.mjs` (shimmed onto `node:fs` with authentic `ENOENT` codes via `vfsFsPlugin`), and `EmitChunkRegistry` walks `.next/server` to emit `chunks.mjs` — a frozen map of static `() => require(...)` thunks so in-process Go `esbuild` bundles every server chunk into one `worker.mjs`.*
> *Then an esbuild `OnLoad` plugin (`dynamicRequirePatchPlugin`) rewrites non-literal `require(expr)` calls in Next server and `[turbopack]` modules to `__ndDynRequire(expr)` and replaces `load-manifest.external.js`'s `vm.runInNewContext` with a registry lookup.*
> *The honest trade-off is that bundling every chunk into one Worker increases cold-start parse size and `patchDynamicRequires` uses a regex rather than an AST pass — at larger app scales I'd split route groups across multiple Workers behind Service Bindings and move the `require` rewriter to an AST transform."*

### Drill B — Streaming & Isolate Lifetime
> *"NextNodeServer speaks `(req, res)` while `workerd` speaks `Promise<Response>`. In `http_bridge.mjs`, `commitHeaders()` resolves the Web `Response` on the first `writeHead`/`write` backed by a `ReadableStream`, while `ctx.waitUntil(handlerPromise)` keeps the isolate alive until `res.end()` closes the stream — so App Router `<Suspense>` chunks flush with zero buffering. The trade-off I'd fix next is backpressure: `res.write()` currently returns `true` unconditionally instead of checking `streamController.desiredSize` and emitting `drain`."*

### Drill C — Concurrency & Shared Isolate State
> *"`env` bindings are static per deployment, so sharing them on `globalThis` is safe — **except** `nextserver_driver.mjs` also stashes `ctx` (`ExecutionContext.waitUntil`) on `globalThis.__nextdeployWorkerCtx`. Under concurrent requests inside one isolate, Request B overwrites `ctx` while Request A is awaiting R2/KV in `CloudflareCacheHandler`. Our `legacy` engine already fixes this with `AsyncLocalStorage` (`context.mjs`), and migrating `nextserver_driver.mjs` to store `{ env, ctx }` in `AsyncLocalStorage` is the exact fix."*

### Drill D — Atomic R2 + Worker Cutover
> *"`DeployStatic` runs a two-phase upload with a hard `wg.Wait()` barrier: immutable `_next/static/*` chunks land in R2 first, then mutable `.html`/`.rsc`/`public/*` files are SHA-256 hashed (`assetVersionForAssets`) and uploaded under `_nextdeploy/releases/<assetVersion>/` before `worker.mjs` (carrying that `assetVersion` in `manifest.json`) is uploaded. That prevents mixed-version `ChunkLoadError`s and makes rollbacks atomic even if `BUILD_ID` is reused, at the cost of needing R2 lifecycle GC for old release prefixes."*
