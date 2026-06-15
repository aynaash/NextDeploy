# Cloudflare Workers target — support & limitations

`nextdeploy ship` to Cloudflare Workers (`serverless.provider: cloudflare`)
compiles a Next.js standalone build into a single Worker bundle (`nextcompile`)
and serves static assets from R2. This document is the source of truth for **what
is production-ready and what is not**.

> TL;DR — The Cloudflare target is production-ready for **static + client-side**
> Next.js apps (marketing, docs, blogs, API routes, `"use client"` interactivity).
> It does **not** yet fully support apps that rely on **dynamic server-side
> rendering** of the React tree. For those, use the **VPS** or **AWS Lambda**
> target until the SSR runtime gap below is closed.

## ✅ Production-ready on Cloudflare

| Capability | Notes |
|---|---|
| Static / prerendered pages (`○`, `●`) | Served from the Worker + R2 |
| Client-component hydration (`"use client"`) | Reference manifests wired for Next 14 (`.json`) **and** Next 15 (`.js`) — v0.14+ |
| API routes (`ƒ /api/*`) | Dispatched by the Worker |
| Middleware | Runs ahead of the dispatcher |
| Static assets (`/_next/static`, `/public`) | Uploaded to R2, content-hash skipped |
| Custom domains | Auto-attached + re-pointed (`override_existing_origin`) |
| Resource provisioning (KV / Hyperdrive / D1) | Reconciled by `ship` (idempotent) |
| Secrets | Folded into the Worker upload as `secret_text` bindings |
| Incremental builds, rollback history, smoke verify | Standard |

## ⚠️ Known limitations — NOT recommended for production

These are the "full Next.js App Router on Cloudflare Workers" gaps. They are the
problem space tools like OpenNext exist to solve, and `nextcompile` does not
fully cover them yet. **Don't ship apps that depend on them to a production
Cloudflare deploy.**

### 1. Dynamic server-side rendering

**Symptom:** dynamically server-rendered routes (`ƒ`) return `500`.

**Cause:** to make the Worker bundle build, `nextcompile` externalizes Next's
server runtime — `next/dist/compiled/next-server/app-page.runtime.prod.js`,
`react-dom/server.edge`, and the `*-async-storage.external.js` modules. Those are
not available at runtime on `workerd`, so on-demand React rendering throws. (See
`optionalExternalPackages` in `cloudflare_adapter.go`.)

### 2. React Server Components (RSC) streaming

Partial. RSC vendoring (`react-server-dom-webpack`) only kicks in when detected,
and the Flight runtime on `workerd` is not exercised by the cases above. Treat
full RSC streaming as unsupported on the Cloudflare target for now.

## Recommendations

- **Docs / marketing / blog / mostly-static apps** → Cloudflare is great. Prefer
  statically-rendered routes; keep dynamic behavior in API routes + client-side
  fetches rather than server-rendered React.
- **If the page is fully static, consider `output: 'export'`** in the app — pure
  static HTML/CSS/JS sidesteps every runtime gap above.
- **Full-stack App Router apps** (auth-gated dashboards, server-rendered React,
  RSC) → deploy to the **VPS** or **AWS Lambda** target, which run the real
  Node.js Next.js server.

## Status of the gaps

- **Client-component hydration — resolved (v0.14+).** Two general fixes: (1) the
  worker now URL-decodes R2 asset keys, so client chunks under `[param]` /
  route-group dirs (requested as `%5B...%5D`) stop 404'ing with a `ChunkLoadError`;
  (2) `nextcompile` reads both Next 14 `.json` and Next 15 `.js`
  client-reference-manifests. With both, `"use client"` boundaries hydrate.

  > Debugging a browser "client-side exception": check the console. A
  > `ChunkLoadError` on a `_next/static/.../%5B...%5D/...` URL was the decode bug
  > (fixed). A library/`undefined` error is usually app code — often a
  > `NEXT_PUBLIC_*` env missing at **build** time (inlined into the client bundle).
- **Dynamic SSR (limitation 1) — open.** Needs a `workerd`-compatible Next server
  runtime (shim or vendored) so the externalized `app-page.runtime.prod.js` /
  `react-dom/server.edge` resolve at runtime. Tracked as engine work.

Until the SSR gap lands, this document is the contract. Don't paper over it with
per-app hacks in production — fix it in `nextcompile` or pick a different target.
