---
title: Cloudflare Workers
lede: Deploy unmodified Next.js 14, 15, and 16 (including native Turbopack) globally to your own Cloudflare account — Workers + R2 + D1 + KV + Hyperdrive + Queues + Vectorize + AI + Durable Objects, all configured from a single nextdeploy.yml.
status: ready
sourceRef: cli/internal/serverless/cloudflare*.go, shared/nextcompile/, docs/running-nextjs-16-turbopack-on-cloudflare-workers.md
related:
  - title: Cloudflare Quickstart
    href: /docs/getting-started/cloudflare-quickstart
  - title: Workers Runtime
    href: /docs/cloudflare/workers
  - title: NextCompile (runtime)
    href: /docs/advanced/nextcompile
  - title: Running Next.js 16 Turbopack on Cloudflare (Deep Dive)
    href: /docs/advanced/nextjs-16-turbopack-on-cloudflare
  - title: Auto-Provisioning
    href: /docs/advanced/auto-provisioning
---

## Why Cloudflare Workers with NextDeploy

Deploy directly into your own Cloudflare account with zero per-seat markup, global edge execution, zero cold-start Node containers, and native bindings to R2, D1, KV, Hyperdrive, Queues, Vectorize, Workers AI, and Durable Objects — all declared in `nextdeploy.yml` and provisioned automatically via `nextdeploy plan` / `nextdeploy ship`.

This documentation site (`https://nextdeploy.org`) itself runs **Next.js 16.0.10 + Turbopack + React 19.2.0** on Cloudflare Workers via NextDeploy's `nextserver` engine.

## Production Capabilities Matrix

| Capability | Status | Notes |
| :--- | :--- | :--- |
| **Next.js 14 / 15 / 16 & Native Turbopack** | ✅ **Ready** | Auto-selects the `nextserver` engine for Turbopack builds (no `--webpack` workaround needed). |
| **App Router & Pages Router SSR (`ƒ`)** | ✅ **Ready** | Executed by Next.js's own `NextNodeServer` inside `workerd` (`nodejs_compat_v2`). |
| **Static & SSG (`○`, `●`, `generateStaticParams`)** | ✅ **Ready** | Served from R2 (`env.ASSETS`) for both HTML and client-side RSC Flight navigations (`rsc: 1`). |
| **Progressive RSC Streaming (`Suspense`, `loading.tsx`)** | ✅ **Ready** | Zero-buffering Web `ReadableStream` ↔ Node `ServerResponse` HTTP bridge. |
| **ISR & On-Demand Revalidation** | ✅ **Ready** | R2 + KV `CloudflareCacheHandler` wired globally via `Symbol.for("@next/cache-handlers")`. |
| **API Routes & Server Actions** | ✅ **Ready** | Full App Router Route Handlers, mutations, `revalidatePath`, `revalidateTag`, cookies, and redirects. |
| **Middleware & Proxy (`proxy.ts`)** | ✅ **Ready** | Runs ahead of route rendering for auth, headers, redirects, and rewrites. |
| **Custom `not-found.tsx` & `error.tsx`** | ✅ **Ready** | Preserves true HTTP `404` / `500` status codes with custom error pages. |
| **Edge Protection Guard** | ✅ **Ready** | IP allow/deny CIDR rules, KV rate limiting, and HMAC session cookie auth ahead of Next.js. |
| **Cron Triggers (`scheduled`) & Queues (`queue`)** | ✅ **Ready** | Native Worker exports routed to `/api/cron` and `/api/queue/[name]`. |
| **Custom Domains & Atomic Releases** | ✅ **Ready** | Automatic custom domain attachment and versioned R2 release prefixes. |

## Architecture at a Glance

```
next build (Turbopack or Webpack, output: "standalone")
  │
  ▼
nextcompile (Go) + in-process esbuild (~170 ms)
  ├── Virtual FS (vfs.mjs) — all .next/*.json manifests + BUILD_ID
  ├── Static Server Chunk Registry (chunks.mjs) — Turbopack/Webpack thunks
  ├── Streaming HTTP Bridge + CloudflareCacheHandler (R2 + KV)
  └── Single ESM Worker bundle (worker.mjs, ~1.60 MB gzipped)
  │
  ▼
nextdeploy ship
  ├── Auto-provisions R2, D1, KV, Queues, Hyperdrive (drift-checked via plan)
  ├── Syncs static & prerendered .html/.rsc assets to R2 (versioned keys)
  ├── Uploads Worker + secret bindings + custom domains via Cloudflare API
  └── Runs post-deploy SmokeVerify on live routes
```

## How NextDeploy Compares to Wrangler / OpenNext

- **No `wrangler.toml` or Node adapter packages required:** NextDeploy compiles `.next/standalone` directly in Go, bundles via in-process `esbuild`, uploads straight to the Cloudflare API, and reconciles bindings + secrets + custom domains from one `nextdeploy.yml`.
- **Unified multi-target workflow:** Switch between **VPS**, **AWS Lambda**, and **Cloudflare Workers** using the same CLI (`init`, `plan`, `build`, `ship`, `status`, `logs`, `rollback`).

## Where to Go Next

- [Cloudflare Quickstart](/docs/getting-started/cloudflare-quickstart) — deploy your first app in 5 minutes.
- [Workers Runtime](/docs/cloudflare/workers) — compatibility flags, bundle sizes, and `workerd` shims.
- [NextCompile Reference](/docs/advanced/nextcompile) — full engine internals.
- [Running Next.js 16 Turbopack on Cloudflare Workers](/docs/advanced/nextjs-16-turbopack-on-cloudflare) — engineering deep dive with code and 10 lessons learned.
