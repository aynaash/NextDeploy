# Part 3 — The `nextserver` Runtime on `workerd`

> Verified against `shared/nextcompile/nextserver.go` and `shared/nextcompile/runtime_src/nextserver/*.mjs`.

---

## 1. Boot & Request Lifecycle

**Source:** `shared/nextcompile/runtime_src/nextserver/nextserver_driver.mjs`

At module evaluation time, `_nextdeploy/worker_entry.mjs` (emitted by `EmitNextServerEntry` in `shared/nextcompile/nextserver.go`) executes:

```js
installNextServerGlobals({
  vfs,
  loadServerModule,
  assetVersion: manifest?.assetVersion || "",
});
```

Inside `fetch(request, env, ctx)` → `dispatchNextServer(request, env, ctx, driverOpts)`:

```
Incoming Request
  ├─ bind globalThis.__nextdeployEnv = env
  ├─ bind globalThis.__nextdeployWorkerCtx = ctx
  ├─ pushEnvIntoProcess(env)  (string bindings → process.env, NODE_ENV="production")
  ├─ stripBasePath(url.pathname, manifest.basePath)
  │
  ├─ [1] guard && protection ─▶ runGuard(request, env, ctx, protection)   [short-circuit 403/429/302/401]
  ├─ [2] /_next/image        ─▶ handleImageRequest(request, env, manifest) [R2 + IMAGES/Wasm/cf.image]
  ├─ [3] /_next/static/* | /public/* ─▶ serveStaticFromR2(env, pathname, manifest)
  ├─ [3b] Pre-rendered SSG / fresh ISR HTML fast path:
  │       (only when method === "GET" && env.ASSETS && !rsc header && !_rsc query)
  │       ├─ manifest.routes.ssg[pathname] ─▶ serveSSGFromR2(...)
  │       └─ manifest.routes.isr[pathname] ─▶ serveISRFromR2(..., ttl) (returns on !expired)
  ├─ [4] NextNodeServer (lazy singleton):
  │       getOrCreateRequestHandler(createServer)
  │         └─ import("next/dist/server/next-server.js")
  │            new NextNodeServer({ dir: "/", dev: false, conf: { ...nextConfig, distDir,
  │                                 cacheHandler: undefined, cacheMaxMemorySize: 0 },
  │                                 customServer: false, minimalMode: false })
  │            await server.prepare() → server.getRequestHandler()
  │       invokeNodeHandler(requestHandler, request, ctx)
  └─ [5] 404 GET fallback ─▶ serveRootPublicFromR2(env, pathname, manifest)
```

> **Why `cacheHandler: undefined`?** Passing a file-path string in `conf.cacheHandler` makes `NextNodeServer` execute a dynamic ESM `import(cacheHandlerPath)`. Instead, `installNextServerGlobals()` registers `CloudflareCacheHandler` directly on `globalThis[Symbol.for("@next/cache-handlers")].FetchCache` and `globalThis.__nextdeployCacheHandler`, while `nextServerTrimPlugin` / `dynamicRequirePatchPlugin` route internal lookups straight to `CloudflareCacheHandler`.

`getOrCreateRequestHandler` caches `cachedHandlerPromise` at isolate scope and clears it on rejection so a transient boot error does not permanently poison the isolate.

---

## 2. Polyfilling `workerd` `process` (`ensureProcessCompat`)

**Source:** `shared/nextcompile/runtime_src/nextserver/nextserver_driver.mjs:43-171`

Even with `nodejs_compat_v2`, Cloudflare's `unenv` `process` shim leaves several properties missing or throwing `not implemented` errors when `NextNodeServer` boots. `ensureProcessCompat()` uses `setProcessProp()` (trying direct assignment first, falling back to `Object.defineProperty(..., { configurable: true, writable: true })`) to guarantee:

| Property | Behavior Provided |
|---|---|
| `process.env` | `{}` if missing |
| `process.listeners` | `() => []` |
| `on`, `once`, `off`, `addListener`, `removeListener`, `removeAllListeners`, `prependListener`, `prependOnceListener` | Chainable no-ops returning `process` |
| `emit`, `emitWarning` | No-ops (`false` / `undefined`) |
| `process.cwd()` | Probes `p.cwd()` in `try/catch`; replaces with `() => "/"` if missing or throwing |
| `process.uptime()` | Probes `p.uptime()` in `try/catch`; replaces with `() => 0` if missing or throwing |
| `process.hrtime` & `process.hrtime.bigint` | Synthesizes `[sec, ns]` (with `prev` delta support) and `BigInt` nanoseconds from `performance.now()` / `Date.now()` |
| `process.stdout`, `process.stderr` | `{ isTTY: false, write: () => true }` |
| `process.argv`, `versions`, `version`, `platform` | `["node"]`, `{ node: "22.0.0" }`, `"v22.0.0"`, `"linux"` |

---

## 3. Build-Time Virtual Filesystem (`vfs.mjs` + `vfs_fs_shim.mjs`)

**Sources:** `shared/nextcompile/nextserver.go:66-213` · `shared/nextcompile/runtime_src/nextserver/vfs_fs_shim.mjs` · `shared/nextcompile/runtime_src/nextserver/vfs_fs_promises_shim.mjs`

### Compile-Time Packing (`EmitVirtualFS`)

`collectVFSFiles(standaloneDir, distDir)` embeds `<standaloneDir>/package.json` plus selected files inside `<standaloneDir>/<distDir>`:
- **Skipped directories**: `static`, `cache`, `standalone`, `trace` (client/static assets live in R2, never in Worker memory).
- **Included files**: `BUILD_ID`, `package.json`, `*.json`, `*.meta` — **excluding** `*.nft.json` and `*.map`, and skipping any file over `maxVFSFileBytes` (`2 * 1024 * 1024` bytes).
- **Directory index (`buildVFSDirectoryIndex`)**: Precomputes sorted child names per directory path (`vfs.dirs`) so `readdirSync` works without scanning `vfs.files` on every call.
- **`nextConfig` extraction (`extractNextConfigFromVFS`)**: Parses `.next/required-server-files.json`'s `config` object and exports it directly from `vfs.mjs` for `createNextServer()`.

### Runtime Filesystem Shim (`vfs_fs_shim.mjs`)

`vfsFsPlugin` binds `fs`, `node:fs`, `fs/promises`, and `node:fs/promises` to `vfs_fs_shim.mjs` and `vfs_fs_promises_shim.mjs`:

- **Path Normalization (`normalizeVFSPath`)**:
  1. Accepts `string`, `URL` (`url.pathname`), or `Buffer`.
  2. Converts backslashes to `/`, resolves `.` and `..` segments.
  3. Checks direct hit in `vfs.files` / `vfs.dirs`.
  4. Strips arbitrary leading mount prefixes before `.next/` (e.g. `var/task/.next/routes-manifest.json` → `.next/routes-manifest.json`) and maps `*/package.json` → `package.json`.
- **Authentic `ENOENT` Errors (`makeENOENT`)**: Every missing path throws an `Error` with `err.code = "ENOENT"`, `err.errno = -2`, `err.syscall`, and `err.path`. Next.js relies on `e.code === "ENOENT"` when probing optional manifests (`middleware-manifest.json`, `pages/500.html`, etc.).
- **Implemented APIs**:
  - Sync: `existsSync`, `readFileSync` (returns UTF-8 string when encoding passed, `Buffer` / `Uint8Array` otherwise), `statSync` (supports `{ throwIfNoEntry: false }`), `lstatSync`, `realpathSync` + `realpathSync.native` (returns `"/" + key`), `readdirSync` (supports `{ withFileTypes: true }`), `accessSync`, `createReadStream` (via `Readable.from([data])`).
  - Read-only no-ops: `mkdirSync`, `writeFileSync`, `unlinkSync`, `rmSync` (and async promise counterparts).
  - Async callbacks (`readFile`, `stat`, `lstat`, `access`, `readdir`, `realpath`) scheduled via `queueMicrotask` + `promises` object.

---

## 4. Static Server Chunk Registry (`chunks.mjs`)

**Sources:** `shared/nextcompile/nextserver.go:215-333` · `cli/internal/serverless/cloudflare_adapter.go:633-799`

Because `workerd` prohibits dynamic `require(variable)` and runtime `eval()`/`vm`:

1. **`EmitChunkRegistry`** walks `<standaloneDir>/<distDir>/server/` collecting all `.js`, `.cjs`, and `.mjs` files sorted deterministically (`collectServerChunkPaths`) and emits `_nextdeploy/chunks.mjs`:
   ```js
   export const serverModules = Object.freeze({
     ".next/server/app/api/health/route.js": () => require("../../.next/server/app/api/health/route.js"),
     ".next/server/chunks/ssr/[root-of-the-server]__6e1ea091._.js": () => require("../../.next/server/chunks/ssr/[root-of-the-server]__6e1ea091._.js"),
   });
   ```
   Every entry uses a static string literal inside `require(...)`, so `esbuild` bundles the entire server chunk graph into `worker.mjs`.

2. **`normalizeModuleKey(raw)`** handles the path variations emitted by Turbopack and Webpack at runtime:
   - Resolves `.` / `..` segments and checks exact match in `serverModules`.
   - Slices from `SERVER_PREFIX` (`.next/server/`) if a full host mount path was passed.
   - Strips leading `/` and retries with `DIST_PREFIX` (`.next/`) prepended — handles Turbopack requests like `/server/chunks/ssr/[root-of-the-server]__*.js`.
   - Retries with `SERVER_PREFIX` (`.next/server/`) prepended.
   - If not ending in `.js`, probes `.js`, `.cjs`, `.mjs` across all three candidate bases.

3. **`dynamicRequirePatchPlugin`** rewrites non-literal `require(expr)` calls in Next server internals and `[turbopack]` modules to `__ndDynRequire(expr)`, which invokes `globalThis.__nextdeployLoadModule(spec)` (`loadServerModule`), falls back to `JSON.parse(vfs.files[key])` for `.json` paths, or throws `err.code = "MODULE_NOT_FOUND"`.

---

## 5. Zero-Buffering Web Fetch ↔ Node HTTP Streaming Bridge (`http_bridge.mjs`)

**Source:** `shared/nextcompile/runtime_src/nextserver/http_bridge.mjs`

`NextNodeServer.getRequestHandler()` expects Node's `(req: IncomingMessage, res: ServerResponse)` signature while `workerd` expects `Promise<Response>`.

```
Fetch Request ──▶ requestToIncomingMessage(request)
                    ├─ folds headers (set-cookie kept as string[], synthetic host)
                    ├─ lazy Readable pulling request.body.getReader() chunks
                    │    wrapped via Buffer.from(val.buffer, val.byteOffset, val.byteLength)
                    └─ mock req.socket / req.connection (cf-connecting-ip, encrypted=https:)

ServerResponse ◀── createStreamingServerResponse(request.method)
                    ├─ EventEmitter + headerMap (case-insensitive)
                    ├─ ReadableStream bodyStream (cancel() emits "close")
                    ├─ commitHeaders() on first writeHead() / flushHeaders() / write() / end()
                    │    └─ immediately resolves responsePromise with new Response(bodyStream | null)
                    └─ enqueues Uint8Array chunks directly (suppressed for HEAD & 101/204/205/304)

invokeNodeHandler(handler, request, ctx)
  ├─ handlerPromise = Promise.resolve().then(() => handler(req, res))...
  ├─ ctx.waitUntil(handlerPromise)   ← keeps isolate alive after headers flush!
  └─ return await responsePromise    ← TTFB as soon as Next flushes shell headers
```

Key design properties:
- **Zero-buffering progressive SSR**: `<Suspense>` fallbacks and `loading.tsx` shells stream to the browser immediately on the first `res.write()`, while `ctx.waitUntil(handlerPromise)` prevents `workerd` from tearing down the isolate before async RSC boundaries resolve and call `res.end()`.
- **Pre-header vs post-header errors**: If the handler throws before headers commit, `invokeNodeHandler` sends a `500 Internal Server Error` text response; if headers already flushed, it calls `res.destroy(err)` to error the `ReadableStream`.

---

## 6. Multi-Tier `CloudflareCacheHandler` (`cf_cache_handler.mjs`)

**Source:** `shared/nextcompile/runtime_src/nextserver/cf_cache_handler.mjs`

Implements Next.js's `IncrementalCache` handler interface (`get(key, ctx)`, `set(key, data, ctx)`, `revalidateTag(tags)`):

1. **L1 (`l1Cache` in-isolate `Map`)**: Sub-millisecond hit on warm isolates, validated against `isEntryInvalidatedByTags(tags, lastModified, env)`.
2. **L2 (Cloudflare R2 `env.ASSETS`)**:
   - Key format: `_nextdeploy/cache/<assetVersion>/<key>.json` (or `_nextdeploy/cache/<key>.json` when `assetVersion` is empty).
   - `set()` updates L1 synchronously and performs the R2 `put()` asynchronously via `ctx.waitUntil`, stamping `customMetadata.revalidatedAt`.
   - `get()` parses JSON from R2 and calls `reviveSerializedBuffers()` to restore `{ type: "Buffer", data: [...] }` payloads on `value.rscData` back into real `Buffer` instances before caching in L1.
3. **Build-Time Pre-Rendered Release Fallback**:
   - On L1+L2 miss when `ctx.kind !== "FETCH"` and `key.startsWith("/")`, looks up `_nextdeploy/releases/<assetVersion>/<clean>.html` and `<clean>.rsc` in parallel (`"/"` → `"index"`).
   - Synthesizes an `APP_PAGE` entry (`{ kind: "APP_PAGE", html, rscData, status: key === "/_not-found" ? 404 : undefined }`).
   - This allows `generateStaticParams` routes to serve both full HTML and client-side RSC Flight navigations cleanly inside `NextNodeServer` without hitting `NoFallbackError`.
4. **Cross-Isolate Tag Invalidation**:
   - `revalidateTag(tags)` stamps `localTagRevalidatedAt.set(tag, now)`, purges matching entries from `l1Cache`, and writes `tag:<tag>` = `String(now)` to KV (`env.NEXTCOMPILE_CACHE`, `expirationTtl: 7 days`) via `ctx.waitUntil`.
   - Every L1 and L2 `get()` compares `entry.lastModified` against both `localTagRevalidatedAt` and KV `tag:<tag>` timestamps.
