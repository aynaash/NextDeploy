# Cloudflare Workers target — support & limitations

`nextdeploy ship` compiles a Next.js standalone build into a single Worker
bundle (`nextcompile`) and serves static assets from R2. This document is the
contract for **what is production-ready and what is not**. It is deliberately
pessimistic: an over-claim here costs more than a missing feature.

> **TL;DR** — Static and prerendered pages are solid. Dynamic App Router
> rendering (RSC → SSR → hydration) is **implemented but unverified**: the
> render path has no automated coverage in this repo and a known
> export-condition gap is still open. Treat it as beta, deploy it behind a
> staging environment first, and read *Failure modes* below so you can tell
> which kind of broken you are looking at.
>
> There is no longer an AWS target. The fallback for an app that Workers
> can't run is the **VPS** target, which runs the real Node.js Next.js server.

## Version support

| Next.js | Status |
|---|---|
| 13 | Treated as 14 by `version_detect.go`. Untested. |
| 14 | Supported — `.json` client-reference manifests. |
| 15 | Supported — `.js` client-reference manifests, `proxy.ts` dispatch. |
| 16 | **Untested.** Bucketed as "v15" by version detection, so manifest-shape changes will surface as runtime errors, not as a clear build failure. |

Version detection is a bucketing heuristic (`shared/nextcompile/version_detect.go:194`),
not a compatibility guarantee. Anything above the newest bucket is assumed to
look like the newest bucket.

## Production-ready

| Capability | Notes |
|---|---|
| Static / prerendered pages (`○`, `●`) | Served from the Worker + R2. |
| Static assets (`/_next/static`, `/public`) | Uploaded to R2, content-hash skipped on re-deploy. |
| API routes (`ƒ /api/*`) | Dispatched by the Worker. |
| `middleware.ts` / `proxy.ts` | Both dispatched, proxy first. Matcher evaluation implements Next's disjunction-of-conjunctions semantics including `has`/`missing`. Regex `value` in a condition is **not** supported. |
| Custom domains | Auto-attached and re-pointed (`override_existing_origin`). |
| Resource provisioning (KV / D1 / Hyperdrive / Queues / Vectorize / AI Gateway / DNS) | Reconciled by `plan` / `apply` / `ship`, idempotent. |
| Secrets | Folded into the Worker upload as `secret_text` bindings. |
| Incremental builds, rollback history, smoke verify | Standard. |

## Partial

| Capability | What works | What doesn't |
|---|---|---|
| **Server Actions** | Resolution and invocation the way Next does it, origin-checked (Next's CSRF model), 2 MB body cap, side effects run — mutations, `revalidatePath`/`revalidateTag`, cookies, `redirect()`. Return values are Flight-encoded (`text/x-component`) via the action module's own `renderToReadableStream`. | Flight encoding needs that export on the compiled module; when it isn't there the reply falls back to JSON and the Next client logs a format warning. Full progressive-enhancement re-render of the page after an action is coupled to the streaming gap below. |
| **App Router metadata** | `export const metadata` and `generateMetadata`, merged root → leaf, including `metadataBase` for og/twitter/canonical URLs. | File-convention metadata (`opengraph-image.tsx` et al), `alternates.languages`, `manifest`, `appleWebApp`, verification tokens. Unknown keys are ignored, never fatal. |
| **ISR / revalidation** | On-demand invalidation works: `revalidatePath`, `revalidateTag` and `unstable_cache` are wired through `cache.mjs` (aliased over `next/cache`). Tier 1 is a time-boxed in-memory stale set per isolate; Tier 2 writes `rev:<path>` / `revTag:<tag>` markers to KV when a `NEXTCOMPILE_CACHE` binding exists, so other isolates on the deployment see it. | **Time-based revalidation does not fire.** `isStale` only consults explicit invalidation markers, so `revalidate: 3600` never expires on its own. There is no R2 rewrite — a stale path falls through to a live render, which for App Router means the unverified SSR path below. Global invalidation (Queue + consumer + R2 rewrite) is a separate milestone. `unstable_cache` is a per-isolate `Map`, not a persistent cache. |

## Not implemented

| Capability | Behavior |
|---|---|
| **PPR** (`experimental.ppr`) | Explicit **501** with the route named. Remove `experimental_ppr` from the route and redeploy. |
| **Streaming / Suspense / `loading.js`** | The Flight stream is fully resolved before HTML rendering starts (`ssr.mjs`, M1 scope). The page is correct but arrives in one shot — `loading.js` fallbacks never paint. `ssr_streaming.dev.mjs` is the scaffold for this; it is not wired in. |
| **`opengraph-image.tsx` and friends** | Needs a build-time scan that doesn't exist yet. |

## Dynamic SSR — implemented, unverified

The SSR layer landed in `4fe82fb`: `rsc.mjs` composes the layout chain and
encodes a Flight stream via the vendored `react-server-dom-webpack/server.edge`,
`ssr.mjs` renders that stream to HTML via the vendored
`react-server-dom-webpack/client.edge` + `react-dom/server.edge`, and emits the
same `self.__next_f` bootstrap Next's own hydrator expects. Layer 3 — hydration
— is Next's own client runtime, uploaded to R2 unchanged.

**Two things keep this at beta, and both are in the repo, not in speculation:**

1. **The render path is not tested.** `ssr.test.mjs` and `rsc.test.mjs` both
   state that the render itself "needs the vendored React builds" and stay out
   of scope. What is covered is the helpers around it — manifest building,
   bootstrap URL ordering, component resolution, and the degradation path when
   the vendor bundle is absent. Nothing in CI proves that an App Router page
   renders HTML on `workerd`.

2. **The `react-server` export condition is still open.** React's Flight
   *server* build expects the `react-server` condition; the Worker bundle is
   built with `--conditions=workerd,worker,node`
   (`cloudflare_adapter.go:328`), and `client.edge` needs that condition
   **off**. Vendoring copies the concrete build by relative path, which
   sidesteps package `exports` resolution for the vendored file itself — but
   not for what React requires internally. See
   `runtime_src/vendor/README.md`, "Known gap for the SSR renderer".

Until someone deploys an App Router app with dynamic routes and confirms real
HTML comes back, this section is a description of intent, not of verified
behavior. **If you run that test, update this file with the result.**

## Failure modes — how to tell what broke

| What you see | What it means |
|---|---|
| **501**, "Partial Prerendering (PPR) is not yet implemented" | The route is PPR-marked. Remove `experimental_ppr`. |
| **501**, "React Server Components runtime not vendored" | The build did not vendor `react-server-dom-webpack/server.edge`, or it threw at module load. The response includes the load error — that is where the `react-server` condition problem would surface. |
| **500**, "nextcompile RSC render failure" + stack | The Flight encode threw. Real stack, read it. |
| **500**, "layout composition failed" | A layout in the chain isn't a plain `{ children }` component. |
| **500**, describing a missing component | The compiled module has no `default`/`Page`/`Component` export — usually a Next internal route module that shouldn't have been dispatched. |
| **200 with a blank page** | The *quietest* failure: the RSC encode succeeded but the SSR builds were missing, so the response degraded to the Flight-only shell. Non-erroring by design. Check for `self.__next_f` with no server-rendered markup around it. |
| `ChunkLoadError` in the browser on a `%5B…%5D` URL | The R2 asset-key decode bug. Fixed in v0.14+; if you see it, your bundle predates the fix. |
| A library or `undefined` error in the console | Usually the app's own code — most often a `NEXT_PUBLIC_*` or auth env var missing at **build** time, since those are inlined into the client bundle. Not a deploy bug. |

## Recommendations

- **Docs, marketing, blog, mostly-static apps** → Cloudflare is a good fit today.
  Prefer statically-rendered routes; keep dynamic behavior in API routes and
  client-side fetches.
- **Fully static** → consider `output: 'export'` in the app. Pure static
  HTML/CSS/JS sidesteps every runtime gap above.
- **Full-stack App Router** (auth-gated dashboards, server-rendered React, RSC,
  streaming) → ship to a staging environment first and verify the pages render.
  If they don't, the **VPS** target runs the real Node.js Next.js server and is
  the supported fallback.
- **Anything on Next 16** → verify before you rely on it. Version detection
  buckets it with 15.

Don't paper over a gap with a per-app hack in production — fix it in
`nextcompile`, or pick the VPS target. When a gap closes, this file changes in
the same commit.
