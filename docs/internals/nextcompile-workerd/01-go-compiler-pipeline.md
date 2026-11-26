# Part 1 — The Go Compiler Pipeline (`shared/nextcompile`)

> Verified against `shared/nextcompile/*.go` and `cli/internal/serverless/nextcompile_bridge.go`.

---

## 1. Boundary & Import Decoupling

`shared/nextcompile` intentionally **does not import** `shared/nextcore` or `cli/internal/...`. Keeping `nextcompile` free of `nextcore` prevents import cycles and isolates the compiler as a pure, filesystem-and-struct-driven library.

At the adapter boundary (`cli/internal/serverless/nextcompile_bridge.go`), `toCompilePayload(meta, cfg)` translates `nextcore.NextCorePayload` field-by-field into `nextcompile.Payload`, and `buildProtectionRuntime(cfg)` normalizes `cfg.Serverless.Cloudflare.Protection` via `protection.BuildRuntime` (`shared/protection`).

### Core Types (`shared/nextcompile/types.go`)

- **`Target`**:
  - `TargetCloudflareWorker` (`"cloudflare-worker"`, default)
  - `TargetAWSLambda` (`"aws-lambda"`)
  - `TargetVPS` (`"vps"`)
- **`Engine`**:
  - `EngineAuto` (`"auto"`, default — picks `nextserver` for Turbopack, `legacy` for Webpack)
  - `EngineNextServer` (`"nextserver"`)
  - `EngineLegacy` (`"legacy"`)
- **`RouteKind`**:
  - `page`, `layout`, `api`, `action`, `middleware`, `proxy`, `static`, `unknown`

### Output Directory Contract

When `CompileOpts.OutDir` is empty, `normalizeOpts` defaults it to `<StandaloneDir>/.nextdeploy-build`. The Cloudflare adapter (`cli/internal/serverless/cloudflare_adapter.go`) passes `<StandaloneDir>/.nextdeploy-cf` and deletes it before every compile (`os.RemoveAll(outDir)`) so stale `dispatch.mjs` entries for deleted routes can never survive across builds.

All generated compiler outputs live under `<OutDir>/_nextdeploy/` with restrictive `0o750` directory and `0o640`/`0o600` file permissions.

---

## 2. The 14 Phases of `Compile()`

**Source:** `shared/nextcompile/compiler.go` (`func Compile(ctx context.Context, opts CompileOpts) (*CompiledBundle, error)`)

```
Compile(ctx, opts)
  ├─ [0] normalizeOpts + ResolveEngine(opts.Engine, opts.StandaloneDir, distDir)
  ├─ [1] DetectVersions()          ──▶ NextVersion, ReactVersion, RuntimeVariant()
  ├─ [2] ScanCompiledServer()      ──▶ []ModuleRef (errgroup, concurrency=32, 4 enrichments)
  ├─ [3] DetectServerActions()     ──▶ *ActionManifest (non-fatal on JSON drift)
  ├─ [4] deriveBindingsStub()      ──▶ []BindingHint ("secret" hints from EnvRefs)
  ├─ [5] elideDeadRoutes()         ──▶ deliberate no-op (returns refs, 0)
  ├─ [6] ensureOutDir()            ──▶ mkdir <OutDir> (0o750)
  ├─ [7] BuildManifest + EmitManifest()         ──▶ _nextdeploy/manifest.json
  ├─ [8] EmitDispatchTable()                    ──▶ _nextdeploy/dispatch.mjs
  ├─ [9] ExtractRuntime()                       ──▶ _nextdeploy/runtime/**
  ├─[10] maybeVendorRSC() [EngineLegacy only]   ──▶ _nextdeploy/runtime/vendor/**
  ├─[11] EmitActionManifest()                   ──▶ _nextdeploy/action_manifest.json
  ├─[12] EmitProtectionConfig()                 ──▶ _nextdeploy/protection.json
  ├─[13] Engine branch:
  │       ├─ EngineNextServer ─▶ EmitVirtualFS()        ──▶ _nextdeploy/vfs.mjs
  │       │                   ─▶ EmitChunkRegistry()    ──▶ _nextdeploy/chunks.mjs
  │       │                   ─▶ EmitNextServerEntry()  ──▶ _nextdeploy/worker_entry.mjs
  │       └─ EngineLegacy     ─▶ EmitWorkerEntry()      ──▶ _nextdeploy/worker_entry.mjs
  └─[14] hashBundle()              ──▶ SHA-256 over normalized manifest + sorted artifacts
```

---

## 3. Phase-by-Phase Deep Dive

### Phase 0 & 1 — Engine Resolution & Version Detection

**Sources:** `shared/nextcompile/nextserver.go` · `shared/nextcompile/version_detect.go`

1. **`ResolveEngine`**:
   - If `opts.Engine` is explicitly `EngineNextServer` or `EngineLegacy`, returns it verbatim.
   - Otherwise (`EngineAuto` or unset), calls `IsTurbopackTree(standaloneDir, distDir)`, which checks `<standaloneDir>/<distDir>/server/chunks` and `<standaloneDir>/<distDir>/server/chunks/ssr` for any filename prefixed with `[turbopack]` or `[root-of-the-server]`.
2. **`DetectVersions(standaloneDir)`** (`version_detect.go`):
   - `readDepVersions` probes in strict order:
     1. `<standaloneDir>/package.json`
     2. `<standaloneDir>/node_modules/next/package.json`
     3. Ancestor directories walking up from `filepath.Dir(standaloneDir)` to the filesystem root (essential because `<project>/.next/standalone` puts `<project>/package.json` two levels up, not one).
   - `readPackageJSON` handles both a direct `next` package (`pkg.Name == "next"`) and a consumer app (`dependencies["next"]` / `devDependencies["next"]`).
   - If no `next` version resolves, returns `ErrVersionNotFound` (fatal).
   - `parseSemver` strips leading `^~>= `, separates pre-release/build metadata (`-`, `+`), parses `Major`, `Minor`, `Patch`, and keeps `Raw` intact. Missing/unparseable `react` version is non-fatal (falls back to `ReactVersion{Raw: reactRaw}`).
   - `NextVersion.RuntimeVariant()` maps `>= 15` → `"v15"`, `14` → `"v14"`, `13` → `"v13"`, default → `"v14"`.

### Phase 2 — Parallel Server Tree Scanner (`scanner.go`)

**Source:** `shared/nextcompile/scanner.go`

`ScanCompiledServer(ctx, standaloneDir, payload)` scans `<standaloneDir>/<distDir>/server`:

1. **`collectCompiledFiles(serverDir)`**:
   - Recursively walks `serverDir` collecting `.js` and `.mjs` files sorted lexicographically.
   - Skips **only** the top-level `<serverDir>/chunks` directory (`filepath.Rel(serverDir, path) == "chunks"`), never a user route named `chunks` (e.g. `server/app/chunks/page.js`).

2. **Parallel Module Analysis (`scanConcurrency = 32`)**:
   - Uses `golang.org/x/sync/errgroup` capped at 32 concurrent file reads (preventing macOS EMFILE exhaustion on 50k+ file standalone trees).
   - `analyzeModule` classifies paths relative to `classifyRoot` (`<standaloneDir>/<distDir>`) and applies fast lexical regexes instead of a JS AST parser:
     - `envRefPattern`: ``process\.env\.([A-Z][A-Z0-9_]*)`` → `EnvRefs`
     - `fetchURLPattern`: ``\bfetch\(\s*['"]([a-z][a-z0-9+.-]*://[^'"]+)['"]`` → `FetchTargets` (template literals excluded to avoid false positives)
     - `usesRSCPattern`: `"use client"|react-server-dom-|registerClientReference|AppPageRouteModule` (plus any `page`/`layout` under `server/app/`) → `UsesRSC`
     - `hasActionsPattern`: `"use server"|__next_internal_action_entry_do_not_use__` → `HasActions`
     - `pprPattern`: `experimental_ppr\s*=\s*true|__NEXT_PPR_STATIC_` → `PPREnabled`
     - `afterPattern`: `\bunstable_after\b|__next_after__` → `UsesAfter`

3. **Route & Kind Mapping**:
   - `kindFromCompiled`: identifies `middleware.{js,mjs}` (`RouteKindMiddleware`), `proxy.{js,mjs}` (`RouteKindProxy`), `*/route.{js,mjs}` and `pages/api/*` (`RouteKindAPI`), `*/page.{js,mjs}` and `pages/*` (`RouteKindPage`), `*/layout.{js,mjs}` (`RouteKindLayout`).
   - `routePathFromCompiled`: maps `server/middleware.js` → `/_middleware`, `server/proxy.js` → `/_proxy`, `server/app/layout.js` → `/_root_layout`, `server/pages/index.js` → `/`, and strips App Router route groups (`stripRouteGroups` removes `(marketing)` segments while preserving `[[...]]` / `[...]` / `[slug]`).
   - `classifyRefs`: upgrades any non-middleware/non-proxy module whose `RoutePath` is in `Payload.Routes.APIRoutes` to `RouteKindAPI`.

4. **Four Second-Pass Enrichments** (run on confirmed refs after classification):
   - **`attachClientManifests`**: For each `RouteKindPage`, checks for `<stem>_client-reference-manifest.json`. In Next 15+, Next emits a `.js` assignment instead:
     ```js
     globalThis.__RSC_MANIFEST=(globalThis.__RSC_MANIFEST||{});globalThis.__RSC_MANIFEST["<key>"]={...}
     ```
     `extractRSCManifestJSON` cuts after the first `]=`, trims `;`, validates `json.Valid`, and materializes the `.json` sibling so static `import(..., { with: { type: "json" } })` works in `dispatch.mjs`.
   - **`attachLayoutChains`**: Maps `RouteKindLayout` directories and walks from `pageDir` up to `server/app` to build root→leaf `LayoutChain` (noting production builds inline layouts into `page.js` and omit `layout.js`).
   - **`attachBootstrapChunks`**: Reads `.next/app-build-manifest.json` keyed by `appManifestKey(compiledPath)` (which **retains** `(group)` and `@slot` segments, unlike `RoutePath`) and concatenates ancestor `/layout` chunks + page chunks in root→leaf order. When `app-build-manifest.json` is absent (Next 16+ standalone builds), falls back to `build-manifest.json`'s `rootMainFiles` plus `.js` chunks extracted from each route's `page_client-reference-manifest.{json,js}` `clientModules`.
   - **`attachStylesheets`**: Reads `entryCSSFiles` from `ClientManifestPath` using a streaming `json.NewDecoder` so key iteration order (**CSS cascade order**) is preserved, accepting both bare string paths and Next 15+ `{inlined, path}` objects (`cssEntry.UnmarshalJSON`).

### Phase 3 — Server Actions Manifest (`actions.go`)

**Source:** `shared/nextcompile/actions.go`

`DetectServerActions(standaloneDir, distDir)` checks `<standaloneDir>/<distDir>/server/server-reference-manifest.json` then `<standaloneDir>/server/server-reference-manifest.json`:

- Missing manifest returns `ErrNoActionManifest` (expected; `Compile` continues). Any non-ENOENT I/O error fails the build rather than silently dropping actions.
- Parse failures log a warning and degrade `actionManifest = nil` so manifest-shape drift in newer Next releases doesn't brick non-action deploys.
- `moduleIDFromWorker` handles both Next 14 bare strings (`"9459"`) and Next 15 objects (`{"moduleId":"1149","async":false}`).
- `parseUpstreamManifest` flattens `node` then `edge` entries (`node` wins if both exist) and calls `stabilize()` to sort module keys lexicographically per action ID so map iteration randomness never causes hash drift.

### Phase 4 & 5 — Binding Hints & Dead Route Elision (`compiler.go`)

- **`deriveBindingsStub`**: Emits one `BindingHint{Kind: "secret", Name: env, Sources: ...}` per unique `EnvRef` across all `ModuleRef`s.
- **`elideDeadRoutes`**: Deliberately a no-op returning `(refs, 0)`. Dynamic segments, middleware rewrites, and manifest fallbacks make static reachability unsafe; shipping an extra route is far safer than 404ing a live URL.

### Phase 7 & 8 — Manifest & Dispatch Table Emission (`manifest.go`, `dispatch.go`)

- **`BuildManifest` (`manifest.go`)**: Builds `Manifest` (`schemaVersion: "1"`). Computes `AssetVersion` from `p.AssetVersion` or falls back to `AssetVersionForBuild(p.BuildID)` (hex SHA-256 of `BuildID`). Derives conservative boolean capability flags (`ManifestFeatures`: `rsc`, `serverActions`, `middleware`, `proxy`, `isr`, `imageOptimization`, `i18n`, `ppr`, `after`), tag-indexed ISR metadata (`ManifestISR.Tags` and `Intervals`), and sorts all slices for byte-identical output.
- **`EmitDispatchTable` (`dispatch.go`)**: Writes `<OutDir>/_nextdeploy/dispatch.mjs`:
  - Computes `importPrefixFor(dispatchDir, standaloneDir)` so imports inside nested `.nextdeploy-cf/_nextdeploy/dispatch.mjs` walk back out (`../../`) to reach `<standaloneDir>/.next/...`.
  - **`staticTable`**: `Object.freeze({...})` keyed by exact route path for O(1) lookup.
  - **`dynamicTable`**: `Object.freeze([...])` sorted by descending `dynamicSpecificity(route)` (`len(parts)*10`, subtracting `1` per `[slug]`, `4` per `[...rest]`, `5` per `[[...opt]]`). Each entry carries a pre-compiled JS RegExp literal (`routeToRegex` — e.g. `/blog/[slug]` → `/^\/blog\/([^/]+)\/?$/`) and `paramNames`.
  - Every table entry carries `kind`, `usesRSC`, `hasActions`, `ppr`, `compiled`, `load: () => import(...)`, `loadClientManifest`, `loadLayouts`, `bootstrap` (URL strings, **never** `import()` thunks), and `css` (URL strings).
  - **`middlewareRef` / `proxyRef`**: `{ compiled, load } | null`.
  - **`actionLoaders`**: `Object.freeze({ "<moduleID>": () => import("<prefix><distDir>/server/<moduleID>.js") })`. Literal `import()` calls are hoisted here so esbuild includes them in `worker.mjs` without bundling via dynamic string concatenation.

### Phase 9 & 10 — Runtime Extraction & React Edge Vendoring (`runtime.go`, `vendor.go`)

1. **`ExtractRuntime(outDir)` (`runtime.go`)**:
   - Uses `//go:embed all:runtime_src` so the CLI binary is 100% self-contained (`go install` leaves no sibling files).
   - Walks `runtime_src` and skips dev-only files (`*.test.mjs`, `*.dev.mjs` via `isDevOnlyRuntimeFile`) before writing to `<OutDir>/_nextdeploy/runtime/`.
2. **`maybeVendorRSC` / `VendorRSC` (`vendor.go`)**:
   - Executed **only for `EngineLegacy`** on `TargetCloudflareWorker` (`EngineNextServer` uses Next's own bundled server React internals).
   - Resolves up to **4 React builds** by walking up to 5 ancestor directories from `standaloneDir` and `os.Getwd()` (checking both `node_modules/<pkg>` and `node_modules/next/dist/compiled/<pkg>`):
     1. `react-server-dom-webpack/server.edge` (primary Flight encoder)
     2. `react-server-dom-webpack/client.edge` (Flight decoder for Fizz SSR)
     3. `react-dom/server.edge` (HTML stream renderer)
     4. `react` standard production build (`vendorStandardReact` — provides `React.use` for M3 progressive `<Suspense>` streaming)
   - **`findEdgeBuild` ranking** is critical for React 19: React 19 publishes CJS-only builds where the flat `<entry>.js` file is a dynamic conditional shim `require("./cjs/<pkg>-<entry>.production.js")`. Ranking concrete `esm/<pkg>-<entry>.production.js` and `cjs/<pkg>-<entry>.production.js` **ahead** of `<entry>.js` prevents copying a dangling shim.
   - When `detectModuleFormat` classifies a file as `"cjs"`, `vendorEdgeBuild` writes `<entry>.cjs` and generates a sibling `<entry>.mjs` interop facade (`export * from "./<entry>.cjs"; export { default } from "./<entry>.cjs";`) so runtime imports always use stable `.mjs` specifiers.
   - If any required edge build is missing and `manifest.Features.RSC == true`, `maybeVendorRSC` fails with an actionable install error; if `RSC == false`, `emitVendorStubs` writes throwing `.mjs` stubs so esbuild still links cleanly.

### Phase 11–13 — Action Manifest, Protection Config, and Entrypoints

- **`EmitActionManifest` (`actions.go`)**: Always writes `_nextdeploy/action_manifest.json` (empty `actions: {}` when none exist) so `worker_entry.mjs`'s static JSON import never fails.
- **`EmitProtectionConfig` (`protection.go`)**: Writes `_nextdeploy/protection.json` (`null` when protection is unset/disabled).
- **Phase 13 Engine Branch**:
  - `EngineNextServer` (`nextserver.go`): Emits `_nextdeploy/vfs.mjs`, `_nextdeploy/chunks.mjs`, and `_nextdeploy/worker_entry.mjs` (`EmitNextServerEntry`).
  - `EngineLegacy` (`entry.go`): Emits `_nextdeploy/worker_entry.mjs` (`EmitWorkerEntry`).
  - Both entrypoints export Cloudflare Workers' 3 standard event handlers:
    - `fetch(request, env, ctx)` → main HTTP dispatcher
    - `scheduled(event, env, ctx)` → dispatches synthetic `POST http://localhost/api/cron` with `x-cron-trigger` and JSON `{ cron, scheduledTime }` via `ctx.waitUntil`
    - `queue(batch, env, ctx)` → dispatches synthetic `POST http://localhost/api/queue/<batch.queue>` with `x-queue-name` and `{ queue, messages }` via `ctx.waitUntil`

### Phase 14 — Deterministic Content Hashing (`compiler.go`)

`hashBundle(baseDir, &manifest, hashInputs)` computes a SHA-256 digest (`CompileStats.ContentHash`) used for reproducible-build checks:

1. Clones `manifest` and sets `norm.GeneratedAt = ""` so the wall-clock timestamp does not churn the hash.
2. Feeds `"manifest\x00"` + marshaled normalized manifest bytes into SHA-256.
3. Sorts `hashInputs` (which includes `dispatch.mjs`, `action_manifest.json`, `protection.json`, `worker_entry.mjs`, all extracted `runtime/*.mjs`, `vfs.mjs`, `chunks.mjs`, and every vendored `.cjs`/`.mjs` file + shim) and hashes `<relPath>\x00<bytes>` using `baseDir`-relative POSIX paths rather than machine-specific absolute paths.
