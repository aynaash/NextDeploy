# Part 4 — The `legacy` Runtime Engine, Shared Edge Subsystems & Verification

> Verified against `shared/nextcompile/runtime_src/*.mjs`, `shared/nextcompile/runtime_src/next_shims/*.mjs`, and the Go/JS test suites.

---

## 1. The `legacy` Request Dispatcher (`dispatcher.mjs`)

**Sources:** `shared/nextcompile/runtime_src/dispatcher.mjs` · `route_trie.mjs` · `middleware_match.mjs`

For Webpack-built Next.js apps (`EngineLegacy`), `worker_entry.mjs` (emitted by `EmitWorkerEntry` in `shared/nextcompile/entry.go`) passes the compiled `tables` (`manifest`, `staticTable`, `dynamicTable`, `middlewareRef`, `proxyRef`, `actionManifest`, `actionLoaders`, `guard`, `protection`) to `dispatch(request, env, ctx, tables)`:

### Execution Order

```
Incoming Fetch Request
  ├─ initCacheIndex(manifest.isr) (once per isolate)
  ├─ globalThis.__nextdeployEnv = env; pushEnvIntoProcess(env)
  ├─ stripBasePath(url.pathname, manifest.basePath)
  │
  ├─ [1] Edge Protection Guard (guard.mjs) — ahead of static assets & middleware
  ├─ [2] Pre-middleware short-circuits (tryShortCircuits):
  │       ├─ /_next/image      ─▶ handleImageRequest()
  │       └─ POST + Next-Action ─▶ handleServerAction()
  ├─ [3] Middleware Stack (runMiddlewareStack):
  │       ├─ proxyRef (Next 15+ proxy.ts) evaluated first
  │       └─ middlewareRef (middleware.ts) evaluated second
  ├─ [4] Static R2 Prefix (tryStaticAsset):
  │       └─ /_next/static/* or /public/* ─▶ serveStaticFromR2() (404 on miss)
  ├─ [5] Route Dispatch (tryRouteDispatch):
  │       ├─ tryCachedRender(): SSG/ISR HTML from R2 (skipped when ppr: true, stale, or TTL expired)
  │       ├─ staticTable[pathname] (O(1)) OR matchTrie(pathname, _routeTrie)
  │       └─ Background R2 ISR write-back via ctx.waitUntil (only on stale GET 200 text/html w/o Set-Cookie)
  ├─ [6] Root Public R2 Fallback ─▶ serveRootPublicFromR2() (/robots.txt, /favicon.ico)
  └─ [7] notFound() (404 plain text)
```

### Segment-Trie Router (`route_trie.mjs`)

`buildRouteTrie(dynamicTable)` replaces linear regex scanning (`route_match.mjs`'s `matchDynamic`) with an `O(path depth × local branching)` radix segment trie. At every segment, `walk()` enforces Next.js's exact priority order with backtracking:

1. **Literal** (`node.literals.get(seg)`)
2. **Single dynamic segment** (`node.dyn` for `[slug]`, non-empty)
3. **Required catch-all** (`node.catchAll` for `[...slug]`, 1+ segments)
4. **Optional catch-all** (`node.optCatchAll` for `[[...slug]]`, 0+ segments)

`zipParams` decodes captured values via `decodeURIComponent`, falling back to raw strings on malformed `%` sequences.

### Middleware & Proxy Matchers (`middleware_match.mjs`)

`middlewareMatches(request, matchers)` implements Next.js's disjunction-of-conjunctions semantics:
- Empty/absent matcher list returns `true` (runs broadly, matching Next's default).
- A request matches if **any** rule matches (`matchers.some`), where a rule matches iff:
  - Path matches (`new RegExp(rule.pattern)` if present, else glob `pathMatches(pathname, rule.pathname)`), **AND**
  - Every `rule.has` condition is present, **AND**
  - Every `rule.missing` condition is absent.
- Supported condition types: `header`, `cookie`, `query`, `host` (presence-only when `value` is empty, exact match otherwise).

### API Routes & Legacy Pages (`invokeAPI` / `invokeLegacyPage`)

- **`invokeAPI`**: Checks `mod.routeModule?.userland || mod.routeModule?.methods || mod` for the uppercase HTTP method (auto-mapping `HEAD` → `GET` if `HEAD` is absent; returning `405` otherwise). Wraps `params` as a dual object (`Object.assign(Promise.resolve(rawParams), rawParams)`) so both Next 14 sync `params.id` and Next 15 `await params` code work identically.
- **`coerceToResponse`**: Coerces `Response`, `null`/`undefined` (`204`), `string` (`text/html`), or JSON values and merges any headers/cookies recorded on the `AsyncLocalStorage` `RequestContext`.

---

## 2. Three-Layer RSC + Progressive Fizz SSR (`rsc.mjs`, `ssr.mjs`, `metadata.mjs`)

**Sources:** `shared/nextcompile/runtime_src/rsc.mjs` · `ssr.mjs` · `metadata.mjs`

For App Router pages (`entry.usesRSC === true`) in `EngineLegacy`, `renderRSC()` drives Next's three-layer pipeline without Next's `app-render`:

1. **Parallel Module Load**: Loads `entry.load()`, `entry.loadClientManifest()`, and `entry.loadLayouts` concurrently inside `runWithContext(reqCtx, ...)`.
2. **Component & Layout Composition**:
   - `resolveComponent` unwraps `default`, `Page`, `Component`, `routeModule.userland`, or walks `loaderTree` (`extractComponentFromTree`). If no component is found, `describeMissingComponent` returns an explicit diagnostic naming Next's internal `routeModule`/`tree` exports.
   - `buildLayoutTree` invokes layouts root→leaf around `PageComponent({ params, searchParams })`, catching `NEXT_HTTP_ERROR_FALLBACK;404` and `NEXT_REDIRECT;...` digests.
3. **Layer 1 — RSC Flight Encoding**: Calls vendored `react-server-dom-webpack/server.edge.mjs`'s `renderToReadableStream(tree, bundlerConfig)`.
4. **Metadata Synthesis (`metadata.mjs`)**: Because `app-render` is bypassed, `resolveMetadata` walks `[...layoutModules, pageUserland]` root→leaf, calls `generateMetadata(props, parent)` or reads `metadata`, applies `title.template` / `title.absolute`, resolves relative OG/Twitter/canonical URLs against `metadataBase`, and escapes all text/attributes (`renderMetadataTags`).
5. **Layer 2 — Progressive Streaming SSR (`ssr.mjs`)**:
   - Tees `flightStream` into `[primary, fallback]` upfront (`flightStream.tee()`).
   - `renderToHtml(primary, ...)` tees again into `[forSSR, forInline]`:
     - **M3 Progressive Streaming** (when standard `react` with `React.use` is vendored): `renderToHtmlStreaming` wraps `deps.client.createFromReadableStream(forSSR, { ssrManifest })` inside `createStreamingRoot(React, rootPromise)` (`() => React.use(rootPromise)`) and passes it to `react-dom/server.edge`'s `renderToReadableStream`. `<Suspense>` fallbacks and `loading.tsx` flush before async server components settle.
     - **M1 Fallback** (if standard `react` wasn't vendored): awaits `createFromReadableStream` before calling `renderToReadableStream`.
   - `buildSSRManifest` picks `firstNonEmpty(m.edgeSSRModuleMapping, m.ssrModuleMapping)` — avoiding empty `edgeSSRModuleMapping: {}` on Webpack builds.
   - `composeHtmlWithFlight` merges the HTML stream and inline Flight stream:
     - `createHeadInjector` buffers up to `65536` bytes to splice `metadata` + stylesheet `<link>` tags right after `<head>`.
     - The `htmlStarted` promise gates `pumpFlight` so no `<script>` tag can ever be emitted before `<!DOCTYPE html>` (which would trigger browser quirks mode).
     - `flightChunkScript` escapes `&`, `>`, `<`, `\u2028`, `\u2029` (`htmlEscapeJsonString` — preventing `</script>` XSS breakouts) and emits idempotent `(self.__next_f=self.__next_f||[]).push([1,...])` blocks.
   - If SSR deps are missing or throw, `renderHtmlOrShell` cancels `primary` and falls back to `wrapFlightInHtmlShell(fallback, ...)` rather than returning a 500.
   - **PPR Routes**: Pages with `entry.ppr === true` skip R2 SSG static-shell short-circuiting and stream dynamically through `renderRSC` with header `x-nextcompile-ppr: dynamic-streaming-fallback`.

---

## 3. Server Actions (`actions.mjs`)

**Source:** `shared/nextcompile/runtime_src/actions.mjs`

Triggered on `POST` requests carrying a `Next-Action` header:

1. **CSRF Check (`originMatchesHost`)**: Requires an `Origin` header whose `host` and `protocol` match `request.url` (403 otherwise).
2. **Lookup**: Looks up `actionId` in `action_manifest.json`, loads the module via `actionLoaders[entry.module]()`, and resolves `mod.__next_app__.require(entry.export)[actionId]` (`resolveActionFn`, unwrapping CJS `.default` via `pageExports`).
3. **Bounded Body Parsing (`parseArgs`, `MAX_BODY_BYTES = 2 * 1024 * 1024`)**:
   - Rejects declared `Content-Length > 2 MiB` immediately and enforces the 2 MiB limit **during** streaming (`readBoundedBody`) so oversized multipart/JSON payloads cannot OOM the isolate (`BodyTooLargeError` → HTTP 413).
   - Supports `multipart/form-data`, `application/x-www-form-urlencoded` (extracting `$ACTION_ARG_N` bound arguments and stripping `$ACTION_` / `1_$ACTION_` markers), `application/json`, and `text/plain` (including Flight `0:[...]` payloads).
4. **Execution & Reply**:
   - Invokes `fn(...args)` inside `runWithContext(reqCtx, ...)`.
   - Passes through `Response` objects (with context headers merged) or encodes return values via `encodeFlightReply` (`text/x-component`) using the route's `clientModules`, falling back to `application/json` with `x-nextcompile-action-response: json`.
   - Propagates `x-nextjs-revalidate: 1` and `x-nextcompile-revalidated-paths`, and schedules background ISR R2 pre-warming (`queueISRWarmups`) via `ctx.waitUntil`.

---

## 4. Per-Request Context, Caching & `next/*` Shims

**Sources:** `shared/nextcompile/runtime_src/context.mjs` · `cache.mjs` · `next_shims/*.mjs`

- **`context.mjs` (via `node:async_hooks` `AsyncLocalStorage`)**:
  - `runWithContext(ctx, fn)` / `getContext()` (throws if called outside a request scope).
  - Implements Next 15 async `cookies()` (`RequestCookiesImpl` with buffered `setCookies`), `headers()` (`ReadonlyHeadersImpl` throwing on `append`/`set`/`delete`), `draftMode()` (`__prerender_bypass` cookie), and `after(fn)` (delegating to `ctx.workerCtx.waitUntil`).
- **`cache.mjs`**:
  - `revalidatePath(path)` / `revalidateTag(tag)` stamp a 60s self-healing in-memory `Map` (`STALE_WINDOW_MS = 60_000`) and persist `rev:<path>` / `revTag:<tag>` keys to KV `env.NEXTCOMPILE_CACHE` (`expirationTtl: 24h`).
  - `unstable_cache(fn, key, opts)` checks an in-isolate `memoCache` and KV `uc:<fullKey>` (enforcing Cloudflare KV's 60s minimum TTL; defaults to `3600s` when `revalidate` is unset or `< 60`).
- **`next_shims/*.mjs`** (wired via `runEsbuild` `Alias` map):
  - `next/headers` → `headers.mjs`
  - `next/cache` → `cache.mjs`
  - `next/server` → `server.mjs` (`after`, `NextResponse`, `NextRequest` with CF `cf-connecting-ip`/`cf-ipcountry`/`cf-ipcity`/`cf-region`, `userAgent`, `ipAddress`)
  - `work-async-storage.external`, `action-async-storage.external`, `after-task-async-storage.external` → `async_storages.mjs`
  - `fast-set-immediate.external` → `fast_set_immediate.mjs`

---

## 5. Shared Edge Subsystems (Active in Both `nextserver` and `legacy`)

### A. Edge Protection Guard (`guard.mjs` + `protection.go`)

**Sources:** `shared/nextcompile/runtime_src/guard.mjs` · `shared/nextcompile/protection.go`

Runs **first** on every request before static assets, routing, or middleware:

1. **`publicPaths` Bypass**: Glob match (`pathMatches`, where `/app/*` matches both `/app` and `/app/x`).
2. **IP Extraction (`clientIP`) & CIDR Gate (`ipMatches`, `ipDenied`)**:
   - `cf-connecting-ip` is trusted unconditionally (stripped/overwritten by Cloudflare).
   - `x-forwarded-for` / `x-real-ip` are read **only** when `protection.json` sets `trustForwardedFor: true`.
   - `ipMatches` supports exact IPs and IPv4 CIDR masks (`/0`–`/32`, special-casing `/0` because JS `<< 32` is a no-op). `deny` wins over `allow`; a non-empty `allow` list fails closed (`403`) on unknown (`""`) IPs.
3. **KV Rate Limiting (`checkRateLimit`)**:
   - Fixed 60s per-IP window (`rl:<ip>:<windowId>`, `expirationTtl: 120`).
   - Fails **closed** (`503 Service Unavailable`) if `rateLimit.kvBinding` is missing from `env`; returns `429` with `retry-after: 60` when exceeded.
4. **Stateless HMAC Session Auth (`verifySessionCookie`)**:
   - Reads `cfg.auth.cookieName`, verifies `<payloadB64url>.<sigB64url>` using Web Crypto `HMAC-SHA256` (`crypto.subtle`) and constant-time `timingSafeEqual`.
   - Requires a valid JSON payload with numeric `exp` (unix seconds) > `nowMs` (prevents immortal tokens without a DB check). Redirects HTML requests (`302` to `loginPath?next=...`) or returns `401` for API calls.

### B. R2 Static, SSG & Release-Versioned Serving (`serve.mjs`)

**Source:** `shared/nextcompile/runtime_src/serve.mjs`

- **`/_next/static/*`**: Stored **unversioned** in R2 at `_next/static/...` (URL-decoded via `safeDecode` so `%5B...%5D` dynamic-segment chunks resolve cleanly without `ChunkLoadError`), served with `public, max-age=31536000, immutable`.
- **`/public/*` & Mutable Assets**: Versioned under `_nextdeploy/releases/<assetVersion>/<key>` so rollbacks serve the exact public files, SSG HTML, and `.rsc` payloads belonging to that release even if `BUILD_ID` was reused.
- **`serveRootPublicFromR2`**: Only probes R2 on 404 GETs whose final path segment contains `.` (avoiding R2 GETs on arbitrary 404s) and checks `isPrerenderedPageKey(key, manifest)` so protected prerendered pages like `/dashboard.html` can never bypass `guard.mjs` via the root-public fallback.
- **`serveISRFromR2`**: Enforces `manifest.isr.intervals[pathname]` against R2 `customMetadata.revalidatedAt` (falling back to `obj.uploaded`).

### C. Edge Image Optimization (`image.mjs`)

**Source:** `shared/nextcompile/runtime_src/image.mjs`

Handles `GET /_next/image?url=<src>&w=<width>&q=<quality>[&fm=<format>]`:

1. **SSRF Defense-in-Depth**:
   - Resolves `new URL(src, request.url)` **first** before validation (defeating `//evil.com`, `%20https://...`, `http:evil.com`).
   - Rejects non-`http:`/`https:` schemes (`400`).
   - `isBlockedHost()` blocks `localhost`, `0.0.0.0`, `::1`, `127.*`, `10.*`, `192.168.*`, `169.254.*` (cloud metadata endpoint), and `172.16.0.0/12` (`403`).
   - Cross-origin URLs must match `manifest.images.domains` or `remotePatterns` (`**.example.com`, `*.example.com`, `/**`, `/*`).
2. **Loop-Free Origin Loading (`loadUpstreamImage`)**: Same-origin images read directly from `env.ASSETS` (R2) via `serveStaticFromR2` / `serveRootPublicFromR2` rather than fetching back into the Worker.
3. **3-Tier Transform Pipeline**:
   - `env.IMAGES` (Cloudflare Images binding) → native edge transcoding (`transform` + `output`).
   - `globalThis.__nextdeployWasmImageTransform` → optional Wasm transform hook cached in `caches.default`.
   - Fallback `passthroughFetch` → passes `cf: { image: { width, quality, format } }` (active on zones with Cloudflare Image Resizing; raw passthrough otherwise) tagged via `x-nextcompile-image`.

---

## 6. Verification & Test Suites

| Suite | Command | What It Covers |
|---|---|---|
| **Go compiler unit tests** | `go test ./shared/nextcompile/...` | `Compile()`, `ScanCompiledServer`, `DetectVersions`, `DetectServerActions` (Next 14/15 fixtures), `VendorRSC`, `EmitDispatchTable`, `EmitVirtualFS`, `EmitChunkRegistry`, `Protection` |
| **Adapter & esbuild tests** | `go test ./cli/internal/serverless -run 'TestBuildWorkerBundle\|TestAssetVersion\|TestBuildScriptMetadata'` | Full `BuildWorkerBundle` on synthetic Webpack and Next 16 Turbopack trees, `nodejs_compat_v2` bindings, R2 asset versioning |
| **Real standalone smoke test** | `NEXTDEPLOY_TEST_STANDALONE=/path/to/.next/standalone go test -v -count=1 ./cli/internal/serverless -run TestBuildWorkerBundle_RealStandaloneSmoke` | Compiles and links an arbitrary real Next.js standalone build end-to-end |
| **Canary Next.js drift check** | `go test -tags canary ./cli/internal/serverless -run TestCanary` | Guards against upstream Next.js compiler/manifest drift |
| **Embedded JS runtime tests** | `node --test shared/nextcompile/runtime_src/**/*.test.mjs` | `nextserver` VFS/HTTP bridge/CacheHandler, `guard`, `route_trie`, `middleware_match`, `rsc`, `ssr`, `ssr_streaming`, `actions`, `metadata`, `image`, `serve` |
