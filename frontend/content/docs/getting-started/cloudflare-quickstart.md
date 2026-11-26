---
title: Cloudflare Quickstart
lede: Deploy an unmodified Next.js 14, 15, or 16 (Turbopack) app to Cloudflare Workers + R2 in under 5 minutes with zero third-party adapters.
status: ready
sourceRef: cli/internal/serverless/cloudflare*.go, shared/nextcompile/, docs/targets/cloudflare.md
related:
  - title: Requirements
    href: /docs/getting-started/requirements
  - title: Cloudflare Overview
    href: /docs/cloudflare/overview
  - title: Workers Runtime
    href: /docs/cloudflare/workers
  - title: Bindings
    href: /docs/cloudflare/bindings
  - title: Running Next.js 16 Turbopack on Cloudflare
    href: /docs/advanced/nextjs-16-turbopack-on-cloudflare
---

## Prerequisites

- **Cloudflare Account & API Token:** `CLOUDFLARE_ACCOUNT_ID` and `CLOUDFLARE_API_TOKEN` with Workers, R2, Account Settings, Zone, DNS, and Cache Purge permissions.
- **Standalone Output:** `output: 'standalone'` enabled in `next.config.mjs`.
- **Domain on Cloudflare DNS:** Your custom domain must belong to a Cloudflare Zone in your account.
- **Secrets Synchronization:** Sync runtime secrets via **Doppler (recommended)** (`doppler run -- nextdeploy secrets sync`), **`.env`** (`nextdeploy secrets load .env.production && nextdeploy secrets sync`), or Cloudflare Secret Store bindings.

See the full [Requirements Guide](/docs/getting-started/requirements) for details.

## 1. Export Cloudflare Credentials

Create a Cloudflare **User API Token** with:
- **Account → Workers Scripts (Edit)**
- **Account → Workers R2 Storage (Edit)**
- **Account → Account Settings (Read)**
- **Zone → DNS (Edit)** & **Zone → Zone (Read)**
- **Zone → Cache Purge (Purge)** *(required for automatic edge cache invalidation)*

For the complete guide and troubleshooting, see [Cloudflare API Token Permissions](/docs/cloudflare/tokens).

```bash
export CLOUDFLARE_ACCOUNT_ID="your-account-id"
export CLOUDFLARE_API_TOKEN="your-api-token"
```

*(You can also inject these via Doppler: `doppler run -- nextdeploy ship`.)*

## 2. Configure `nextdeploy.yml`

Run `nextdeploy init` and select **Serverless → Cloudflare**, or add `nextdeploy.yml`:

```yaml
version: "1.0"
target_type: serverless

app:
  name: my-edge-app
  domain: app.example.com
  environment: production

serverless:
  provider: cloudflare
  cloudflare:
    account_id: ${CLOUDFLARE_ACCOUNT_ID}
    api_token: ${CLOUDFLARE_API_TOKEN}
    zone_name: example.com
    engine: auto # auto selects nextserver for Turbopack & Next.js 16+
    compatibility_date: "2025-04-01"
    compatibility_flags:
      - nodejs_compat_v2
```

Ensure `output: 'standalone'` is enabled in `next.config.mjs`.

## 3. Preview Changes (`plan`)

```bash
nextdeploy plan
```

`plan` checks Cloudflare APIs and shows R2 buckets, KV/D1/Queue/Hyperdrive resources, bindings, and custom domains to create or update without touching production.

## 4. Build & Deploy (`ship`)

```bash
nextdeploy ship
```

NextDeploy:

1. Runs `next build` (native Turbopack or Webpack) and extracts route/ISR/asset metadata.
2. Compiles `.next/standalone` into a single `worker.mjs` (~1.60 MB gzipped for a full Next.js 16 Turbopack app) using `nextcompile` + in-process `esbuild`.
3. Auto-provisions declared R2, KV, D1, Queue, and Hyperdrive resources.
4. Uploads static and prerendered `.html`/`.rsc` assets to R2 under versioned release prefixes.
5. Uploads the Worker + secret bindings, attaches your custom domain, and runs `SmokeVerify` against your live URLs.

## 5. Tail Live Edge Logs

```bash
nextdeploy logs
```
