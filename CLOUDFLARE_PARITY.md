# Cloudflare Workers target — support & limitations

`nextdeploy ship` to Cloudflare Workers (`serverless.provider: cloudflare`)
compiles a Next.js standalone build into a single Worker bundle (`nextcompile`)
and serves static assets from R2. This document is the source of truth for **what
is production-ready and what is not**.

> TL;DR — The Cloudflare target serves static + prerendered pages (with their
> client chunks and RSC Flight payload) correctly. It does **not** yet support
> apps that rely on **dynamic server-side rendering** of the React tree; use the
> **VPS** or **AWS Lambda** target for those until the SSR runtime gap closes.
>
> Note: a "client-side exception" in the browser is usually the **app's own**
> client code, not the deploy — most often a `NEXT_PUBLIC_*` / auth env var that
> was missing at **build** time (those are inlined into the client bundle), so a
> provider throws on hydration. Check the browser console and the build log.

## ✅ Production-ready on Cloudflare

| Capability | Notes |
|---|---|
| Static / prerendered pages (`○`, `●`) | Served from the Worker + R2 |
| Client reference manifests | Wired for Next 14 (`.json`) **and** Next 15 (`.js`) — v0.14+. (Manifest plumbing only; whether a given app *hydrates* also depends on its own client code + build-time env.) |
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

- **Client reference manifests — wired (v0.14+).** `nextcompile` now reads both
  Next 14 `.json` and Next 15 `.js` manifests (previously the `.js` form was
  ignored, leaving `loadClientManifest` null). This is the manifest *plumbing*;
  it does not by itself guarantee a given app hydrates — that also needs the
  app's client code + build-time `NEXT_PUBLIC_*`/auth env to be correct.
- **Dynamic SSR (limitation 1) — open.** Needs a `workerd`-compatible Next server
  runtime (shim or vendored) so the externalized `app-page.runtime.prod.js` /
  `react-dom/server.edge` resolve at runtime. Tracked as engine work.

Until the SSR gap lands, this document is the contract. Don't paper over it with
per-app hacks in production — fix it in `nextcompile` or pick a different target.
