# Cloudflare Workers target — support & limitations

`nextdeploy ship` to Cloudflare Workers (`serverless.provider: cloudflare`)
compiles a Next.js standalone build into a single Worker bundle (`nextcompile`)
and serves static assets from R2. This document is the source of truth for **what
is production-ready and what is not**.

> TL;DR — The Cloudflare target serves static + prerendered pages, dynamic
> server-rendered routes (App Router `ƒ`), API routes, Server Actions,
> middleware, cron triggers, and queue consumers. RSC Flight is rendered to
> HTML via vendored `react-server-dom-webpack` and `react-dom/server.edge`
> bundles. Progressive RSC streaming (Suspense/loading.js), Partial
> Prerendering (PPR), and edge image transcoding are not yet implemented.
>
> Note: a "client-side exception" in the browser is usually the **app's own**
> client code, not the deploy — most often a `NEXT_PUBLIC_*` / auth env var that
> was missing at **build** time (those are inlined into the client bundle), so a
> provider throws on hydration. Check the browser console and the build log.

## ✅ Production-ready on Cloudflare

| Capability | Notes |
|---|---|
| Static / prerendered pages (`○`, `●`) | Served from the Worker + R2 |
| Dynamic server-side rendering (`ƒ`) | App Router pages render server-side via vendored `react-server-dom-webpack/server.edge.mjs` + `react-dom/server.edge.mjs` (M1 — non-streaming). The Flight stream is fully resolved before HTML rendering starts, so the response is complete but not progressively streamed. |
| Client reference manifests | Wired for Next 14 (`.json`) **and** Next 15 (`.js`) — v0.14+. |
| API routes (`ƒ /api/*`) | Dispatched by the Worker |
| Server Actions — execution | Resolved + invoked the way Next does. Bound argument extraction (`$ACTION_ARG_0`, `$ACTION_ARG_1`) and Flight-encoded text-body parsing implemented. Side effects run: mutations, `revalidatePath`/`revalidateTag`, cookies, `redirect()`. **Partial:** responses attempt a Flight-encoded action reply and fall back to JSON when no encoder is available; a Flight reply does not yet guarantee a page re-render after the action. |
| Middleware | Runs ahead of the dispatcher |
| Cron triggers (`scheduled`) | Worker exports `scheduled()` handler; cron expressions applied via `Workers.Scripts.Schedules.Update` |
| Queue consumers (`queue`) | Worker exports `queue()` handler; consumers wired after script upload; batches dispatched to `/api/queue/[name]` |
| Static assets (`/_next/static`, `/public`) | Content-hashed `/_next/static` assets are shared; mutable public and prerendered objects are namespaced in R2 by a deterministic fingerprint of their keys and contents, so Worker rollbacks read matching assets even when a project reuses its Next.js build ID. Old release prefixes are retained for rollback. |
| Custom domains | Auto-attached + re-pointed (`override_existing_origin`). Non-production environments are guarded — previews never hijack production domains. |
| Resource provisioning (KV / Hyperdrive / D1 / Queues / Vectorize / AI Gateway) | Reconciled by `ship` / `apply` (idempotent). `ship --no-provision` resolves `ref:` bindings from the encrypted state manifest. Set `NEXTDEPLOY_CF_STATE_KEY` on all team/CI runners to sync that manifest through R2. |
| Secrets | Folded into the Worker upload as `secret_text` bindings; Cloudflare Secret Store bindings supported |
| Edge protection | IP allow/deny, KV-backed rate limiting, stateless HMAC session-cookie auth — compiled into the Worker |
| Incremental builds, rollback history, smoke verify | Standard; Cloudflare rollback restores the Worker and its build-specific mutable R2 assets. |

## ⚠️ Known limitations — not yet production-ready

These are the remaining "full Next.js App Router on Cloudflare Workers" gaps.

### 1. Progressive RSC streaming with Suspense boundaries (M3)

**Status:** Not implemented. M1 (non-streaming SSR) is complete — the full Flight
stream is resolved before HTML rendering starts, so the response is correct but
not progressively streamed. `loading.tsx` and `<Suspense fallback={...}>` do not
flush immediately; the browser waits for the entire HTML payload.

**What's needed:** Vendor `react` into the SSR condition and wrap the pending
root element in a component calling React's `use()` hook to stream Suspense
boundaries progressively as Flight chunks arrive.

### 2. Partial Prerendering (PPR)

**Status:** Not implemented. When `experimental.ppr` is enabled, `rsc.mjs`
returns `501 Not Implemented` with a clear message.

**What's needed:** Implement the hybrid static shell + dynamic hole streaming
protocol — serve the pre-rendered shell from R2 and stream dynamic components
into matching placeholder holes.

### 3. Edge image transcoding (`/_next/image`)

**Status:** Not implemented. Unless Cloudflare Images (`env.IMAGES`) is
configured, `handleImageRequest` proxies raw upstream images without edge
resizing or format conversion.

**What's needed:** Integrate a lightweight WebAssembly image resizer (e.g.,
Photon Wasm or `@resvg/resvg-wasm`) to transcode, resize, and serve WebP/AVIF
images on-the-fly at the edge.

### 4. Server Action page re-renders

**Status:** Partial. Server Actions execute correctly (mutations, cookies,
redirects, bound args). The runtime attempts a Flight-encoded action reply and
falls back to JSON when no encoder is available, but does not yet guarantee a
Flight-encoded page re-render. Full progressive-enhancement re-render will
arrive alongside RSC streaming improvements (M3).

## Recommendations

- **Docs / marketing / blog / mostly-static apps** → Cloudflare is great.
- **Full-stack App Router apps** (auth-gated dashboards, server-rendered
  React, Server Actions) → Cloudflare now supports dynamic SSR via vendored
  bundles. Progressive streaming (Suspense/loading.js) is the main remaining
  gap — apps that depend on streaming UX should wait for M3 or use the VPS /
  AWS Lambda target.
- **If the page is fully static, consider `output: 'export'`** in the app —
  pure static HTML/CSS/JS sidesteps every runtime gap above.
- **Apps using PPR** → deploy to the **VPS** or **AWS Lambda** target until PPR
  lands.

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
