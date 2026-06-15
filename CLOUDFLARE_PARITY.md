# Cloudflare Workers target — support & limitations

`nextdeploy ship` to Cloudflare Workers (`serverless.provider: cloudflare`)
compiles a Next.js standalone build into a single Worker bundle (`nextcompile`)
and serves static assets from R2. This document is the source of truth for **what
is production-ready and what is not**.

> TL;DR — The Cloudflare target is production-ready for **static-heavy** Next.js
> sites (marketing, docs, blogs, API routes). It does **not** yet fully support
> apps that rely on **client-component hydration** or **dynamic server-side
> rendering** of the React tree. For those, use the **VPS** or **AWS Lambda**
> target until the App Router runtime gaps below are closed.

## ✅ Production-ready on Cloudflare

| Capability | Notes |
|---|---|
| Static / prerendered pages (`○`, `●`) | Served from the Worker + R2 |
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

### 1. Client-component hydration (`"use client"`)

**Symptom:** pages render their HTML but throw a *client-side exception* in the
browser during hydration (e.g. `Application error: a client-side exception has
occurred`).

**Cause:** Next.js 15 emits `page_client-reference-manifest.**js**` files (a
side-effecting module: `globalThis.__RSC_MANIFEST[...] = {...}`). `nextcompile`
currently only wires the older `page_client-reference-manifest.**json**` form, so
`loadClientManifest` resolves to `null` and `"use client"` boundaries are never
mapped to their client bundles → hydration fails.

### 2. Dynamic server-side rendering

**Symptom:** dynamically server-rendered routes (`ƒ`) return `500`.

**Cause:** to make the Worker bundle build, `nextcompile` externalizes Next's
server runtime — `next/dist/compiled/next-server/app-page.runtime.prod.js`,
`react-dom/server.edge`, and the `*-async-storage.external.js` modules. Those are
not available at runtime on `workerd`, so on-demand React rendering throws. (See
`optionalExternalPackages` in `cloudflare_adapter.go`.)

### 3. React Server Components (RSC) streaming

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

Closing limitations 1–2 means teaching `nextcompile` to (a) read Next 15's `.js`
client-reference-manifests and (b) provide a `workerd`-compatible Next server
runtime (shim or vendored). Both are tracked as engine work; until they land,
this document is the contract. Do not paper over these with per-app hacks in
production — fix them in `nextcompile` or pick a different deploy target.
