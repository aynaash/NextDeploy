# Cloudflare Workers target — support & limitations

`nextdeploy ship` to Cloudflare Workers (`serverless.provider: cloudflare`)
compiles a Next.js standalone build into a single Worker bundle (`nextcompile`)
and serves static assets from R2. This document is the source of truth for **what
is production-ready and what is not**.

> TL;DR — The Cloudflare target serves static + prerendered pages, ISR routes
> (with time-based expiry and background R2 write-back), dynamic server-rendered
> routes with progressive RSC streaming (M3 — `<Suspense>` and `loading.tsx`),
> API routes, Server Actions, independent `proxy.ts` + `middleware.ts` matchers,
> cron triggers, and queue consumers. RSC Flight is streamed to HTML via vendored
> `react-server-dom-webpack`, `react-dom/server.edge`, and dual-condition `react`
> bundles. Partial Prerendering (PPR) routes degrade cleanly to dynamic streaming
> SSR (`x-nextcompile-ppr: dynamic-streaming-fallback`) rather than 501, and
> `/_next/image` supports Cloudflare Images (`env.IMAGES`), direct R2 reads for
> same-origin assets, `cf.image` subrequest resizing, and an optional Wasm +
> `caches.default` transform hook.
>
> Note: a "client-side exception" in the browser is usually the **app's own**
> client code, not the deploy — most often a `NEXT_PUBLIC_*` / auth env var that
> was missing at **build** time (those are inlined into the client bundle), so a
> provider throws on hydration. Check the browser console and the build log.

## ✅ Production-ready on Cloudflare

| Capability | Notes |
|---|---|
| Worker Runtime Engines (`cloudflare.engine`) | Supports `auto` (default), `nextserver`, and `legacy` (also overridable via `NEXTDEPLOY_CF_ENGINE`). **`nextserver`** runs Next.js's own `NextNodeServer` inside the Worker with a build-time Virtual Filesystem (`vfs.mjs`), static Turbopack/Webpack chunk registry (`chunks.mjs`), streaming Web↔Node HTTP bridge, and Cloudflare R2+KV `CacheHandler` — enabling **native Next 16 Turbopack builds** and full App Router parity (`error.tsx`, `not-found.tsx`, parallel/intercepting routes, PPR, Server Actions). **`auto`** picks `nextserver` for Turbopack builds and `legacy` for Webpack builds. |
| Static / prerendered pages (`○`, `●`) | Served from the Worker + R2 |
| ISR & Revalidation | Enforces both time-based `manifest.isr.intervals` (via R2 `customMetadata.revalidatedAt` / `obj.uploaded`) and on-demand `revalidatePath`/`revalidateTag`. Fresh `200 OK` HTML responses write back to R2 asynchronously via `ctx.waitUntil` and clear stale markers; `unstable_cache` persists across isolates when `env.NEXTCOMPILE_CACHE` KV is bound. |
| Progressive RSC streaming with Suspense boundaries (M3) | App Router pages render and stream progressively via vendored `react-server-dom-webpack/server.edge.mjs`, `react-dom/server.edge.mjs`, and standard-condition `react` (`React.use(rootPromise)`). `<Suspense fallback={...}>` and `loading.tsx` flush as soon as the shell is ready; falls back to M1 awaited rendering if standard `react` was not vendored. |
| Client reference manifests | Wired for Next 14 (`.json`) **and** Next 15 (`.js`) — v0.14+. |
| API routes (`ƒ /api/*`) | Dispatched by the Worker (O(depth) segment-trie dynamic routing) |
| Server Actions — execution | Resolved + invoked the way Next does. Bound argument extraction (`$ACTION_ARG_0`, `$ACTION_ARG_1`) and Flight-encoded text-body parsing implemented. Passes route `clientModules` into `encodeFlightReply`, propagates `x-nextjs-revalidate` + `x-nextcompile-revalidated-paths` headers, and triggers background `ctx.waitUntil` ISR R2 pre-warming when an action revalidates an ISR path. |
| Middleware & `proxy.ts` | Runs ahead of the dispatcher; `proxy.ts` and `middleware.ts` matchers are parsed and evaluated independently. |
| Cron triggers (`scheduled`) | Worker exports `scheduled()` handler; cron expressions applied via `Workers.Scripts.Schedules.Update` |
| Queue consumers (`queue`) | Worker exports `queue()` handler; consumers wired after script upload; batches dispatched to `/api/queue/[name]` |
| Static assets (`/_next/static`, `/public`) | Content-hashed `/_next/static` assets are shared; mutable public and prerendered objects are namespaced in R2 by a deterministic fingerprint of their keys and contents, so Worker rollbacks read matching assets even when a project reuses its Next.js build ID. Old release prefixes are retained for rollback. |
| Custom domains | Auto-attached + re-pointed (`override_existing_origin`) with Mozilla Public Suffix List apex detection (`.co.uk`, `.com.au`, etc.). Non-production environments are guarded — previews never hijack production domains. |
| Resource provisioning (KV / Hyperdrive / D1 / Queues / Vectorize / AI Gateway / Zone Settings) | Reconciled by `ship` / `apply` (idempotent) and previewed in `nextdeploy plan` (including `zone_settings` TTL overrides). `ship --no-provision` resolves `ref:` bindings from the encrypted state manifest. Set `NEXTDEPLOY_CF_STATE_KEY` on all team/CI runners to sync that manifest through R2 (with exponential backoff + jitter on concurrent writes). |
| Automated CI/CD Pipelines (`generate-ci`) | Target-aware GitHub Actions workflow with dry-run planning on PRs (`--detailed-exitcode`), ephemeral preview stacks (`--preview`), cleanup on PR close, encrypted state synchronization via `NEXTDEPLOY_CF_STATE_KEY`, and atomic shipping on push to `main`. |
| Secrets | Folded into the Worker upload as `secret_text` bindings; Cloudflare Secret Store bindings supported |
| Edge protection | IP allow/deny, KV-backed rate limiting, stateless HMAC session-cookie auth — compiled into the Worker |
| Incremental builds, rollback history, smoke verify | Standard; supports both step-based and commit-targeted (`--to <commit>`) rollbacks via Worker deployment annotations, restoring both the Worker and its build-specific mutable R2 assets. |

## ⚠️ Known limitations — partial or degraded behavior

These are the remaining nuances compared to a full Node.js Next.js server.

### 1. Partial Prerendering (PPR) static-shell splicing

**Status:** Degraded to dynamic streaming SSR. Routes marked with `experimental.ppr` bypass R2 static-shell short-circuiting and stream cleanly through M3 `renderRSC` with `x-nextcompile-ppr: dynamic-streaming-fallback` rather than returning `501 Not Implemented`.

**What's still missing:** Serving the pre-rendered static shell directly from R2 and splicing dynamic Suspense holes inline without re-rendering the static shell on the Worker.

### 2. Edge image transcoding (`/_next/image`)

**Status:** Native with `env.IMAGES` or zone Image Resizing; opt-in Wasm hook otherwise. Same-origin images are read directly from `env.ASSETS` (R2) to avoid Worker self-fetch loops. When `env.IMAGES` is unbound, `handleImageRequest` supports:
- an optional zero-binding Wasm transcoding hook (`globalThis.__nextdeployWasmImageTransform`) cached in `caches.default`, and
- Cloudflare `cf.image` subrequest resizing options on passthrough fetches (active on zones with Cloudflare Image Resizing enabled; passes through raw bytes otherwise).

### 3. Server Action full-page Flight re-renders

**Status:** Partial. Server Actions execute mutations, cookies, redirects, bound arguments, route `clientModules` Flight reply encoding, `x-nextjs-revalidate` header propagation, and background ISR R2 warm-ups. Returning a full re-rendered RSC page tree inline within the action POST response (rather than letting the client revalidate/refetch) is not yet implemented.

## Recommendations

- **Docs / marketing / blog / mostly-static apps** → Cloudflare is great (SSG + ISR R2 write-back).
- **Full-stack App Router apps** (auth-gated dashboards, server-rendered React with `<Suspense>` / `loading.tsx`, Server Actions) → Production-ready on Cloudflare with M3 progressive streaming.
- **If the page is fully static, consider `output: 'export'`** in the app — pure static HTML/CSS/JS sidesteps every runtime cost.
- **Apps relying on PPR static-shell TTFB** → PPR routes work on Cloudflare via dynamic streaming SSR fallback; use VPS or AWS Lambda if true static-shell splicing is required.

## Debugging hydration issues

A "client-side exception" in the browser is usually the **app's own** client
code, not the deploy — most often a `NEXT_PUBLIC_*` / auth env var that was
missing at **build** time (those are inlined into the client bundle), so a
provider throws on hydration. Check the browser console and the build log.

A `ChunkLoadError` on a `_next/static/.../%5B...%5D/...` URL was a URL-decode
bug (fixed — the worker now URL-decodes R2 asset keys). A library/`undefined`
error is usually app code — often a `NEXT_PUBLIC_*` env missing at **build**
time (inlined into the client bundle).

When the vendored SSR bundles are absent (a bundle compiled before SSR
vendoring landed), the renderer degrades to a blank but non-erroring Flight-only
shell rather than returning 500. If you see a blank page, verify that
`shared/nextcompile/runtime_src/vendor/` contains the expected bundles and
re-run `nextdeploy build`.
