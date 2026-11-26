# `nextcompile` — Go Compiler for Next.js Web Apps on Cloudflare `workerd`

> Every claim, symbol name, constant, and code path in this documentation set is verified against the workspace source code.
> File references are relative to the repository root.

---

## 1. What `nextcompile` Is

`nextcompile` (`shared/nextcompile/`) combined with the Cloudflare adapter (`cli/internal/serverless/cloudflare_adapter.go`) is NextDeploy's **pure-Go compiler and bundler pipeline** that transforms a standard `next build` `.next/standalone` directory into a single, self-contained ESM Cloudflare Worker (`worker.mjs`) targeting Cloudflare's `workerd` V8 isolate runtime (`nodejs_compat_v2`).

Unlike external adapter ecosystems (such as `@opennextjs/cloudflare` or `wrangler` wrapper scripts), NextDeploy's compiler:

- Requires **zero third-party adapter npm packages** in the user's Next.js project.
- Runs **entirely in Go** up to the final bundle link step, which embeds **esbuild as a Go library** (`github.com/evanw/esbuild/pkg/api`) directly inside the `nextdeploy` binary — no `npx esbuild` child process is spawned.
- Supports **two execution engines** selected automatically (`ResolveEngine` in `shared/nextcompile/nextserver.go`) or via `serverless.cloudflare.engine` in `nextdeploy.yml`:
  1. **`nextserver`** — boots Next.js's own `NextNodeServer` (`next/dist/server/next-server.js`) inside `workerd` using a build-time Virtual Filesystem, a static Turbopack/Webpack chunk registry, a zero-buffering Web Fetch ↔ Node HTTP streaming bridge, and a Cloudflare R2 + KV `CacheHandler`.
  2. **`legacy`** — uses `nextcompile`'s lightweight hand-written edge dispatcher, segment-trie router, vendored React 18/19 dual-condition RSC + Fizz SSR streaming renderer, and Server Action runner.

---

## 2. Document Map

| Document | Scope | Key Source Files |
|---|---|---|
| [`01-go-compiler-pipeline.md`](./01-go-compiler-pipeline.md) | `nextcore` → `nextcompile` bridge, all 14 phases of `Compile()`, version detection, parallel server scanning, Action manifest extraction, React edge vendoring, `dispatch.mjs` / `manifest.json` / `vfs.mjs` / `chunks.mjs` emission, and deterministic bundle hashing | `shared/nextcompile/*.go`, `cli/internal/serverless/nextcompile_bridge.go` |
| [`02-esbuild-workerd-bundling.md`](./02-esbuild-workerd-bundling.md) | `BuildWorkerBundle`, `esbuild` build options for `nextserver` vs `legacy`, the 5 custom Go `esbuild` plugins (`vfsFsPlugin`, `nextServerTrimPlugin`, `dynamicRequirePatchPlugin`, `reactConditionPlugin`, `nodeBuiltinsPlugin`), bundle top-contributor profiling, and Cloudflare Workers script upload metadata | `cli/internal/serverless/cloudflare_adapter.go`, `cli/internal/serverless/cloudflare_bindings.go`, `cli/internal/serverless/cloudflare.go` |
| [`03-nextserver-workerd-runtime.md`](./03-nextserver-workerd-runtime.md) | `NextNodeServer` inside `workerd`: `ensureProcessCompat()`, `installNextServerGlobals()`, Virtual FS (`vfs.mjs` + `vfs_fs_shim.mjs`), Static Chunk Registry (`chunks.mjs` + `normalizeModuleKey`), Streaming HTTP Bridge (`http_bridge.mjs`), and `CloudflareCacheHandler` (`cf_cache_handler.mjs`) | `shared/nextcompile/nextserver.go`, `shared/nextcompile/runtime_src/nextserver/*.mjs` |
| [`04-legacy-runtime-and-edge-subsystems.md`](./04-legacy-runtime-and-edge-subsystems.md) | `legacy` dispatcher (`dispatcher.mjs`, `route_trie.mjs`, `middleware_match.mjs`), 3-layer RSC + Fizz SSR (`rsc.mjs`, `ssr.mjs`, `metadata.mjs`), Server Actions (`actions.mjs`), ALS request context & `next/*` shims, plus **shared edge subsystems** (`guard.mjs`, `serve.mjs`, `image.mjs`) and test/verification harnesses | `shared/nextcompile/runtime_src/*.mjs`, `shared/nextcompile/runtime_src/next_shims/*.mjs`, `shared/nextcompile/*_test.go`, `cli/internal/serverless/*_test.go` |
| [`05-interview-deep-dive-qa.md`](./05-interview-deep-dive-qa.md) | Systems-engineering interview Q&A & trade-off playbook: exact breakages & fixes (`NextNodeServer` in `workerd`, dynamic `require`, `http_bridge` streaming, ALS concurrency race, L1/R2/KV caching, atomic R2 rollbacks, security boundaries, 30s spoken drills) | All `nextcompile` & `cli/internal/serverless` Worker pipeline files |

---

## 3. End-to-End Compilation & Deployment Architecture

**Sources:** `cli/internal/serverless/cloudflare.go` · `cli/internal/serverless/cloudflare_adapter.go` · `shared/nextcompile/compiler.go`

```
next build (output: "standalone")
  │
  ├─ .next/standalone/                 (Next.js server tree, manifests, chunks, node_modules)
  └─ nextcore.NextCorePayload          (routes, middleware/proxy matchers, imageConfig, i18n, buildId)
        │
        ├──▶ DeployStatic()  [cli/internal/serverless/cloudflare.go]
        │      ├─ partitionAssets():
        │      │    ├─ immutable: _next/static/*        ──▶ R2 (_next/static/<hash>/...)
        │      │    └─ mutable:   public/*, .html, .rsc ──▶ assetVersionForAssets() SHA-256
        │      │                                            ──▶ R2 (_nextdeploy/releases/<assetVersion>/...)
        │      └─ sets meta.AssetVersion = version
        │
        └──▶ DeployCompute() [cli/internal/serverless/cloudflare.go]
               └─ BuildWorkerBundle() [cli/internal/serverless/cloudflare_adapter.go]
                    ├─ clean <standaloneDir>/.nextdeploy-cf/
                    ├─ toCompilePayload(meta, cfg)          [nextcompile_bridge.go]
                    ├─ buildProtectionRuntime(cfg)          [nextcompile_bridge.go]
                    ├─ nextcompile.Compile() [14 phases]    [shared/nextcompile/compiler.go]
                    │     ├─ Phase 1:  DetectVersions()
                    │     ├─ Phase 2:  ScanCompiledServer()
                    │     ├─ Phase 3:  DetectServerActions()
                    │     ├─ Phase 4:  deriveBindingsStub()
                    │     ├─ Phase 5:  elideDeadRoutes() (no-op)
                    │     ├─ Phase 6:  ensureOutDir()
                    │     ├─ Phase 7:  BuildManifest() + EmitManifest()
                    │     ├─ Phase 8:  EmitDispatchTable()
                    │     ├─ Phase 9:  ExtractRuntime() (//go:embed all:runtime_src)
                    │     ├─ Phase 10: maybeVendorRSC() (EngineLegacy only)
                    │     ├─ Phase 11: EmitActionManifest()
                    │     ├─ Phase 12: EmitProtectionConfig()
                    │     └─ Phase 13: EngineNextServer ─▶ EmitVirtualFS + EmitChunkRegistry + EmitNextServerEntry
                    │                  EngineLegacy     ─▶ EmitWorkerEntry
                    │     └─ Phase 14: hashBundle() (deterministic SHA-256)
                    │
                    ├─ In-process esbuild (github.com/evanw/esbuild/pkg/api):
                    │     ├─ EngineNextServer ─▶ runEsbuildNextServer() ─▶ worker.mjs
                    │     └─ EngineLegacy     ─▶ runEsbuild()           ─▶ worker.mjs
                    │
                    └─ Cloudflare Workers Upload [cloudflare.go + cloudflare_bindings.go]
                           ├─ buildScriptMetadata() (compat date 2025-04-01, nodejs_compat_v2,
                           │                         ASSETS R2, KV/D1/DO/Hyperdrive/AI/Queues,
                           │                         secret_text bindings, Observability)
                           ├─ computeDeployHash() — skip upload if live deploy hash matches
                           └─ Workers.Scripts.Update() (main_module: "worker.mjs")
```

---

## 4. Dual-Engine Selection (`auto` vs `nextserver` vs `legacy`)

**Sources:** `shared/nextcompile/types.go` · `shared/nextcompile/nextserver.go` · `shared/config/types.go`

`ResolveEngine(requested, standaloneDir, distDir)` chooses between the two Worker runtime strategies:

| Requested Setting (`cloudflare.engine`) | `IsTurbopackTree(...)` Result | Resolved `Engine` | Why |
|---|---|---|---|
| `"auto"` *(default)* or `""` | `true` (`[turbopack]*` or `[root-of-the-server]*` found in `<distDir>/server/chunks{,/ssr}`) | `EngineNextServer` (`"nextserver"`) | Turbopack server chunks emit runtime `require()` calls into `/server/chunks/ssr/...` and do not expose `legacy`'s expected Webpack/page component exports |
| `"auto"` *(default)* or `""` | `false` (classic Webpack build) | `EngineLegacy` (`"legacy"`) | Classic Webpack builds run in the smaller hand-written edge runtime (`dispatcher.mjs` + `rsc.mjs` + `ssr.mjs`) |
| `"nextserver"` | *(ignored)* | `EngineNextServer` (`"nextserver"`) | Runs `NextNodeServer` for both Turbopack and Webpack builds; gives full Next.js router/error boundary/PPR/action parity |
| `"legacy"` | *(ignored)* | `EngineLegacy` (`"legacy"`) | Forces the lightweight custom dispatcher + vendored React edge runtime |

`IsTurbopackTree(standaloneDir, distDir)` inspects two directories (`<standaloneDir>/<distDir>/server/chunks` and `<standaloneDir>/<distDir>/server/chunks/ssr`) and returns `true` if any entry filename begins with `[turbopack]` or `[root-of-the-server]`.

### Engine Capability & Tradeoff Matrix

| Dimension | `EngineNextServer` (`nextserver`) | `EngineLegacy` (`legacy`) |
|---|---|---|
| **Core execution model** | Real `NextNodeServer` (`next/dist/server/next-server.js`) over `http_bridge.mjs` | Custom JS `dispatcher.mjs` + `rsc.mjs` + `ssr.mjs` |
| **Turbopack (Next 16+) support** | Native | Not supported for Turbopack chunk graphs |
| **Filesystem access** | Build-time `vfs.mjs` + `vfs_fs_shim.mjs` (`fs`/`node:fs`/`fs/promises`) | None needed; all imports statically emitted in `dispatch.mjs` |
| **Dynamic `require()` resolution** | `chunks.mjs` static thunk map + `dynamicRequirePatchPlugin` (`__ndDynRequire`) | Static ESM `() => import(...)` thunks in `dispatch.mjs` |
| **React bundling** | Uses Next's internal `next/dist/compiled/react-server-dom-{webpack,turbopack}` + `NextNodeServer` | Vendors 4 React builds from `node_modules` via `VendorRSC` + `reactConditionPlugin` |
| **`next/headers`, `next/cache`, `next/server`** | Handled natively inside `NextNodeServer` | Redirected via esbuild `Alias` to `next_shims/*.mjs` + `context.mjs` + `cache.mjs` |
| **Shared edge fast paths** | `guard.mjs`, `/_next/image`, R2 static & SSG/ISR pre-rendered GET fast path, root `public/*` 404 fallback | Same shared `guard.mjs`, `image.mjs`, `serve.mjs`, `cache.mjs` |
| **ISR / Cache handler** | `CloudflareCacheHandler` registered on `globalThis[Symbol.for("@next/cache-handlers")].FetchCache` (L1 `Map` + L2 R2 + KV tags + R2 release fallback) | `tryCachedRender` + `ctx.waitUntil` R2 HTML write-back + `cache.mjs` (`rev:`/`revTag:` in KV) |
| **Worker entrypoint `NEXT_RUNTIME`** | `"nodejs"` (via esbuild banner) | `"edge"` (via esbuild banner) |

---

## 5. Complete Source Inventory

### Go Compiler (`shared/nextcompile/`)

| File | Responsibility |
|---|---|
| `types.go` | `Target`, `Engine`, `RouteKind`, `CompileOpts`, `Payload`, `ModuleRef`, `BindingHint`, `CompileStats`, `CompiledBundle`, `Logger` |
| `compiler.go` | `Compile()` 14-phase orchestrator, `maybeVendorRSC`, `emitVendorStubs`, `hashBundle`, `deriveBindingsStub`, `elideDeadRoutes` |
| `version_detect.go` | `DetectVersions`, `readDepVersions`, `parseSemver`, `NextVersion.RuntimeVariant()` |
| `scanner.go` | `ScanCompiledServer` (parallel `errgroup`), lexical regex extraction, route/kind mapping, client manifest/layout/bootstrap/CSS passes |
| `actions.go` | `DetectServerActions`, `parseUpstreamManifest` (Next 14 + Next 15 shapes), `stabilize`, `EmitActionManifest` |
| `manifest.go` | `BuildManifest`, `AssetVersionForBuild`, `buildFeatures`, `buildManifestISR`, `EmitManifest` |
| `dispatch.go` | `EmitDispatchTable`, `importPrefixFor`, `dynamicSpecificity`, `routeToRegex`, `staticTable` / `dynamicTable` / `actionLoaders` generation |
| `runtime.go` | `//go:embed all:runtime_src`, `ExtractRuntime`, `RuntimeSourceFiles`, `isDevOnlyRuntimeFile` |
| `vendor.go` | `VendorRSC`, `locateRSCPackage`, `findEdgeBuild`, `detectModuleFormat`, `writeInteropShim`, `vendorStandardReact` |
| `protection.go` | `EmitProtectionConfig` (serializes `protection.Runtime` or `null`) |
| `entry.go` | `EmitWorkerEntry` (`legacy` `fetch` / `scheduled` / `queue` Worker template) |
| `nextserver.go` | `IsTurbopackTree`, `ResolveEngine`, `EmitVirtualFS`, `EmitChunkRegistry`, `EmitNextServerEntry` |

### Embedded JS Runtime (`shared/nextcompile/runtime_src/`)

| File | Engine(s) | Responsibility |
|---|---|---|
| `nextserver/nextserver_driver.mjs` | `nextserver` | `ensureProcessCompat`, `installNextServerGlobals`, `dispatchNextServer` fast paths + `NextNodeServer` invocation |
| `nextserver/vfs_fs_shim.mjs` | `nextserver` | Synchronous & callback `fs` / `node:fs` shim over `globalThis.__nextdeployVFS` |
| `nextserver/vfs_fs_promises_shim.mjs` | `nextserver` | `fs/promises` / `node:fs/promises` re-exports from `vfs_fs_shim.mjs` |
| `nextserver/http_bridge.mjs` | `nextserver` | `requestToIncomingMessage`, `createStreamingServerResponse`, `invokeNodeHandler` |
| `nextserver/cf_cache_handler.mjs` | `nextserver` | `CloudflareCacheHandler` (`get`, `set`, `revalidateTag`) backed by L1 `Map`, R2, and KV |
| `dispatcher.mjs` | `legacy` | 9-stage request router, short-circuits, middleware/proxy stack, ISR R2 write-back, API/page invocation |
| `route_trie.mjs` | `legacy` | Segment radix trie (`literal > [dyn] > [...catchAll] > [[...optCatchAll]]`) |
| `route_match.mjs` | `legacy` | `buildRouteContext` and legacy linear `matchDynamic` regex matcher |
| `middleware_match.mjs` | `legacy` | `middlewareMatches` (`pattern`/`pathname` + `has`/`missing` conditions) |
| `rsc.mjs` | `legacy` | `renderRSC`, `resolveComponent`, `buildLayoutTree`, `renderHtmlOrShell`, `wrapFlightInHtmlShell` |
| `ssr.mjs` | `legacy` | `loadSSRDeps`, `buildSSRManifest`, M3 `renderToHtmlStreaming` (`React.use`), `createHeadInjector`, `composeHtmlWithFlight` |
| `metadata.mjs` | `legacy` | `resolveMetadata`, `mergeMetadata`, `renderMetadataTags`, `metadataBase` resolution |
| `actions.mjs` | `legacy` | `isServerAction`, `handleServerAction`, `originMatchesHost`, `readBoundedBody` (2 MiB cap), `resolveActionFn`, `encodeFlightReply` |
| `context.mjs` | `legacy` | `AsyncLocalStorage` request scope (`cookies`, `headers`, `draftMode`, `after`) |
| `cache.mjs` | `legacy` | `revalidatePath`, `revalidateTag`, `isStale`, `clearStale`, `unstable_cache` |
| `next_shims/*.mjs` | `legacy` | `headers.mjs`, `cache.mjs`, `server.mjs`, `async_storages.mjs`, `fast_set_immediate.mjs` |
| `guard.mjs` | **Both** | IPv4 CIDR allow/deny, KV fixed-window rate limit, Web Crypto HMAC-SHA256 stateless session auth |
| `serve.mjs` | **Both** | R2 asset serving (`/_next/static/*`, `/public/*`, SSG/ISR `.html`, root `public/*` fallback with `isPrerenderedPageKey` guard) |
| `image.mjs` | **Both** | `/_next/image` SSRF guard, R2 same-origin read, `env.IMAGES` / Wasm+Cache / `cf.image` passthrough |
| `errors.mjs` | **Both** | Minimal `notFound()` (404) and `serverError(err)` (500, `cache-control: no-store`) |

### Cloudflare Adapter & Upload Layer (`cli/internal/serverless/`)

| File | Responsibility |
|---|---|
| `nextcompile_bridge.go` | `toCompilePayload` (`nextcore.NextCorePayload` → `nextcompile.Payload`) and `buildProtectionRuntime` |
| `cloudflare_adapter.go` | `BuildWorkerBundle`, `runEsbuild`, `runEsbuildNextServer`, 5 custom esbuild plugins, `logBundleSummary`, `logBundleTopContributors` |
| `cloudflare_bindings.go` | `buildScriptMetadata`, `compatibility_date`/`compatibility_flags`, binding translation, `secret_text` folding, duplicate-name guard |
| `cloudflare.go` | `DeployStatic` (2-phase R2 upload + `assetVersionForAssets`), `DeployCompute` (SHA-256 skip check + multipart Worker script upload) |
