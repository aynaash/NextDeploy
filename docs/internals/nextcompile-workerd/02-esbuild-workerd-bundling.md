# Part 2 — In-Process `esbuild` Bundling & `workerd` Packaging

> Verified against `cli/internal/serverless/cloudflare_adapter.go`, `cli/internal/serverless/cloudflare_bindings.go`, and `cli/internal/serverless/cloudflare.go`.

---

## 1. `BuildWorkerBundle` & Output Profiling

**Source:** `cli/internal/serverless/cloudflare_adapter.go`

`BuildWorkerBundle(ctx, standaloneDir, meta, cfg, log)` is the bridge from `.next/standalone` to the final `worker.mjs` uploaded to Cloudflare:

1. **Clean Build Tree**: Removes `<standaloneDir>/.nextdeploy-cf` before invoking `nextcompile.Compile` so stale dispatch tables from removed routes cannot linger.
2. **Compile**: Calls `nextcompile.Compile` with `TargetCloudflareWorker`, `Engine(cfCfg.ResolveEngine())`, `toCompilePayload(meta, cfg)`, and `buildProtectionRuntime(cfg)`.
3. **Summary Report (`logBundleSummary`)**: Logs resolved engine, Next/React versions, route/action counts, `ManifestFeatures` flags (`rsc`, `actions`, `middleware`, `proxy`, `isr`, `imgopt`, `i18n`, `ppr`, `after`), vendored RSC info, and the 12-char `ContentHash`.
4. **In-Process `esbuild` Link**:
   - `EngineNextServer` → `runEsbuildNextServer(...)`
   - `EngineLegacy` → `runEsbuild(...)`
   - Both compile directly via `github.com/evanw/esbuild/pkg/api` (`api.Build(...)`) inside the Go process, producing `<standaloneDir>/.nextdeploy-cf/worker.mjs` and `<standaloneDir>/.nextdeploy-cf/_nextdeploy/esbuild-meta.json`.
5. **Top-Contributor Size Audit (`logBundleTopContributors`)**: Parses `esbuild-meta.json`, sorts `outputs[worker.mjs].inputs` by `bytesInOutput`, and prints the top 10 modules and percentages so operators immediately see what dominates Worker bundle size.

---

## 2. `api.BuildOptions`: `nextserver` vs `legacy`

| Option | `runEsbuildNextServer` (`EngineNextServer`) | `runEsbuild` (`EngineLegacy`) |
|---|---|---|
| `Platform` | `api.PlatformNode` | `api.PlatformNode` |
| `Format` | `api.FormatESModule` | `api.FormatESModule` |
| `Target` | `api.ESNext` | `api.ESNext` |
| `MainFields` | `["module", "main"]` | `["module", "main"]` |
| `Conditions` | `["workerd", "worker", "node", "default"]` | `["react-server", "workerd", "worker", "node"]` |
| `External` | `node:*`, `cloudflare:*`, `optionalExternalPackages` | `node:*`, `cloudflare:*`, `optionalExternalPackages` |
| `Define` | `process.env.NODE_ENV = "production"` | `process.env.NODE_ENV = "production"` |
| `Minification` | `MinifySyntax: true`, `MinifyWhitespace: true`, `LegalCommentsNone` *(identifiers preserved for readable `workerd` stack traces)* | Same |
| `Banner` (`js`) | Sets `__dirname = "/"`, `__filename = "/worker.mjs"`, `process.env.NODE_ENV = "production"`, `NEXT_RUNTIME \|\|= "nodejs"` | Sets `__dirname = "/"`, `__filename = "/worker.mjs"`, `NEXT_RUNTIME \|\|= "edge"` |
| `Alias` | `@opentelemetry/api` → `node_modules/@opentelemetry/api` or `next/dist/compiled/@opentelemetry/api` | `next/cache`, `next/headers`, `next/server`, Next async-storage internals, `fast-set-immediate.external`, `react-dom`, `@opentelemetry/api` |
| `Plugins` | `vfsFsPlugin`, `nextServerTrimPlugin`, `dynamicRequirePatchPlugin`, `nodeBuiltinsPlugin` | `reactConditionPlugin`, `nodeBuiltinsPlugin` |

### `optionalExternalPackages`

`optionalExternalPackages` (`cloudflare_adapter.go:277-289`) marks packages Next.js conditionally probes inside `try { require(...) } catch {}` as external so `esbuild` does not fail the build when they are absent from `node_modules`:

- `"critters"` (only used when `experimental.optimizeCss` is enabled)
- `"next/dist/compiled/@ampproject/toolbox-optimizer"` (AMP optimizer)
- `"react-dom/server.edge"` (bare specifier required by Next's `pages.runtime.prod.js`; in `legacy`, `ssr.mjs` imports the vendored copy via relative path `./vendor/react-dom/server.edge.mjs` so this external does not shadow it)

---

## 3. The 5 Custom Go `esbuild` Plugins

### Plugin 1: `vfsFsPlugin` (`nextserver` only)

Intercepts `^(node:)?fs$` and `^(node:)?fs/promises$` in `OnResolve` and redirects them to:
- `runtime/nextserver/vfs_fs_shim.mjs`
- `runtime/nextserver/vfs_fs_promises_shim.mjs`

This runs **ahead** of `nodeBuiltinsPlugin`, ensuring Next.js's synchronous and async filesystem calls read from the build-time Virtual Filesystem instead of hitting the `node-stub` empty proxy.

### Plugin 2: `nextServerTrimPlugin` (`nextserver` only)

Handles three resolution responsibilities:

1. **CacheHandler Bridge**: Resolves bare `__nextdeploy_cf_cache_handler__` to `runtime/nextserver/cf_cache_handler.mjs`.
2. **Bundled RSC Runtimes**: Intercepts `^react-server-dom-(webpack|turbopack)/` and calls `resolveNextCompiledReactServer`, probing `<node_modules>/next/dist/compiled/<spec>{,.js,.edge.js,.node.js}`. If absent, falls back to the `next-trim-stub` namespace.
3. **Dead / Build-Time / Native Package Trimming (`trimFilter`)**: Redirects build-only, native, dev, and experimental runtimes into `next-trim-stub`, which synthesizes a self-chaining CJS `Proxy`:
   - Native/build tools: `sharp`, `webpack`, `webpack/*`, `terser`, `typescript`, `@next/swc-*`, `@babel/*`, `styled-jsx/macro`, `caniuse-lite`, `caniuse-lite/*`, `browserslist`, `@vercel/og`
   - Unused Next runtimes: `react-dom/static`, `next/dist/compiled/webpack/*`, `next/dist/compiled/edge-runtime*`, `next/dist/compiled/next-server/*-experimental.runtime.*`, `next/dist/compiled/next-server/*.runtime.dev.js`, `next/dist/telemetry/*`, `next/dist/trace/*`, `next/dist/build/*`

The `next-trim-stub` proxy handles property access, function calls, and `new` construction without throwing during module evaluation if dead branches reference it.

### Plugin 3: `dynamicRequirePatchPlugin` (`nextserver` only)

Intercepts `\.(c?js)$` loads in `OnLoad` to neutralize three `workerd`-incompatible Next.js modules and patch dynamic `require(expr)` calls:

| Target Path Suffix | Why Stock Next.js Fails in `workerd` | Replacement Behavior |
|---|---|---|
| `/next/dist/server/require-hook.js` | Monkey-patches `Module.prototype.require` (unavailable in ESM / `workerd`) | Exports a no-op `hookPropertyMap = new Map()`, `defaultOverrides = {}`, `addHookAliases()` |
| `/next/dist/server/node-environment.js` | Calls `process.listeners("unhandledRejection")` and monkey-patches `node:crypto` | Ensures `globalThis.AsyncLocalStorage` is set from `node:async_hooks` and exports `{}` |
| `/next/dist/server/load-manifest.external.js` | `evalManifest()` calls `vm.runInNewContext(...)` (blocked by `workerd` V8 CSP) | `loadManifest` reads via shimmed `fs.readFileSync`; `evalManifest` invokes `globalThis.__nextdeployLoadModule(manifestPath)` from `chunks.mjs` (throws authentic `ENOENT` on miss) and returns `{ __RSC_MANIFEST: globalThis.__RSC_MANIFEST }` |

For all other modules matching `/[turbopack]` or `/next/dist/server/` (excluding `/compiled/`):
- `patchDynamicRequires` rewrites non-literal `require(expr)` calls (`dynamicRequireExprRe`) to `__ndDynRequire(expr)` (while leaving `function require(...)` declarations untouched) and `await dynamicImportEsmDefault(` to `await __ndDynImport(`.
- Prepends `dynamicRequireHelper`, which:
  1. Returns `globalThis.__nextdeployCacheHandler` when `spec` contains `__nextdeploy_cf_cache_handler__`.
  2. Resolves server JS chunks synchronously via `globalThis.__nextdeployLoadModule?.(spec)` (`chunks.mjs`).
  3. Falls back to `JSON.parse(globalThis.__nextdeployVFS.files[key])` for `.json` requires (normalizing `.next/` prefixes).
  4. Throws an `Error` with `err.code = "MODULE_NOT_FOUND"` on misses so optional Next probes (such as `instrumentation.js`) degrade cleanly inside `try/catch`.

### Plugin 4: `reactConditionPlugin` (`legacy` only)

Because `runEsbuild` sets `Conditions: ["react-server", "workerd", "worker", "node"]`, bare `import "react"` normally resolves to `react.react-server.js` (which omits client/SSR hooks like `React.use` needed by Fizz SSR).

`reactConditionPlugin` intercepts `^react$` in `OnResolve` and branches on `args.Importer`:
- **SSR importers** (`/vendor/react-dom/`, `/vendor/react-server-dom-webpack/client.edge`, or `/runtime/ssr.mjs`) → routed to `_nextdeploy/runtime/vendor/react/index.mjs` (standard production React).
- **All other importers** (pages, layouts, `server.edge`) → routed to `reactServerPath` (`react/react.react-server.js` or `react/cjs/react.react-server.production.js`).

This allows a single `worker.mjs` bundle to contain both React Server and React Fizz SSR graphs simultaneously without symbol collisions.

### Plugin 5: `nodeBuiltinsPlugin` (both engines)

Handles bare and `node:`-prefixed Node built-in modules:

1. **`nativeNodeModules` (37 modules supported by `workerd` `nodejs_compat_v2`)**:
   - `_stream_duplex`, `_stream_passthrough`, `_stream_readable`, `_stream_transform`, `_stream_writable`, `_tls_common`, `_tls_wrap`, `assert`, `assert/strict`, `async_hooks`, `buffer`, `constants`, `crypto`, `diagnostics_channel`, `dns`, `dns/promises`, `events`, `module`, `net`, `path`, `path/posix`, `path/win32`, `querystring`, `stream`, `stream/consumers`, `stream/promises`, `stream/web`, `string_decoder`, `sys`, `timers`, `timers/promises`, `tls`, `url`, `util`, `util/types`, `zlib`.
   - For ESM `import` (`!= ResolveJSRequireCall`), marks `node:<mod>` as `External: true`.
   - For CJS `require()` (`ResolveJSRequireCall`), routes to the `"node-native"` virtual namespace which emits an ESM bridge:
     ```js
     import * as mod from "node:<mod>";
     export default (mod.default ?? mod);
     export * from "node:<mod>";
     ```
     This keeps CJS code inside compiled Next.js/Turbopack chunks working seamlessly inside an ESM Worker bundle.

2. **`stubNodeModules` (25 modules unavailable or disabled in `workerd`)**:
   - `child_process`, `cluster`, `console`, `dgram`, `domain`, `fs`, `fs/promises`, `http`, `http2`, `https`, `inspector`, `inspector/promises`, `os`, `perf_hooks`, `punycode`, `readline`, `readline/promises`, `repl`, `trace_events`, `tty`, `v8`, `vm`, `wasi`, `worker_threads`.
   - Note: In `nextserver`, `vfsFsPlugin` runs earlier in the `Plugins` slice and claims `fs` / `fs/promises` before `nodeBuiltinsPlugin` sees them.
   - All remaining `stubNodeModules` resolve to `"node-stub"`, returning a `Proxy` with realistic defaults (`STATUS_CODES`, `METHODS`, `isatty: () => false`, `platform: () => "linux"`, `arch: () => "x64"`, `tmpdir: () => "/tmp"`, `cpus`, `totalmem`, `freemem`, `performance`, `console`) so dead code paths and startup probes never crash Worker upload validation.

---

## 4. `workerd` Script Metadata & R2 Versioning Contract

**Sources:** `cli/internal/serverless/cloudflare_bindings.go` · `cli/internal/serverless/cloudflare.go`

When `DeployCompute` uploads `worker.mjs` to Cloudflare:

- **Default `workerd` Runtime Flags** (`cloudflare_bindings.go:14-18`):
  - `compatibility_date`: `"2025-04-01"` (overridable via `serverless.cloudflare.compatibility_date`)
  - `compatibility_flags`: `["nodejs_compat_v2"]` (overridable via `serverless.cloudflare.compatibility_flags`)
- **Entry Module**: `main_module: "worker.mjs"` (`format: "esm"`).
- **Bindings (`buildScriptMetadata`)**:
  - Auto-injects R2 binding `ASSETS` pointing at the app's asset bucket if the user did not declare an explicit `ASSETS` R2 binding.
  - Translates KV, D1, Hyperdrive (resolving `ref:` from provisioned state), Queue producers & consumers (with optional `dead_letter_queue`), Vectorize, Workers AI, AI Gateway, Durable Objects (with `Migrations`), Secret Store, and plain-text env vars.
  - Folds all runtime secrets in as `secret_text` bindings (with `refuseSecretWipe` guard because Cloudflare script updates are replace-not-merge).
  - Pre-validates duplicate binding names before multi-MB upload.
  - Enables Workers `Observability` (`Logs.Enabled: true`, `InvocationLogs: true`, `HeadSamplingRate: 1.0`) unless explicitly disabled.
- **Idempotent Deploys & Atomic Rollbacks (`cloudflare.go`)**:
  - `DeployStatic` uploads immutable `/_next/static/*` chunks first (hard `wg.Wait()` barrier), then computes `assetVersionForAssets` (SHA-256 over sorted key lengths, key names, and file contents) and uploads mutable assets (`public/*`, SSG `.html`, `.rsc`) under `_nextdeploy/releases/<assetVersion>/...`.
  - `meta.AssetVersion` flows through `toCompilePayload` into `manifest.json` (`assetVersion`) inside `worker.mjs`.
  - `computeDeployHash(bundleSum, scriptMeta, secrets)` checks live Worker annotations/hashes and skips the Worker upload if nothing changed.
