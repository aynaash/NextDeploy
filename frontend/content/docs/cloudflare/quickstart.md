---
title: Cloudflare Quickstart
lede: Deploy an unmodified Next.js 14, 15, or 16 (Turbopack) app to Cloudflare Workers + R2 with automatic resource provisioning and custom domain attachment.
status: ready
sourceRef: cli/internal/serverless/cloudflare*.go, shared/nextcompile/, docs/targets/cloudflare.md
related:
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

1. A Next.js app with `output: 'standalone'` in `next.config.mjs`.
2. A Cloudflare account ID and an API token (see the full [Cloudflare API Token Permissions](/docs/cloudflare/tokens) guide):
   - **Account → Workers Scripts: Edit**
   - **Account → Workers R2 Storage: Edit**
   - **Account → Workers KV Storage / D1 / Queues / Hyperdrive: Edit** *(for any bindings you declare)*
   - **Zone → DNS: Edit** & **Zone → Zone: Read** *(for custom domains)*
   - **Zone → Cache Purge: Purge** *(required for automatic edge cache purge)*

> [!TIP]
> Ensure you create a **User API Token** (`My Profile → API Tokens`) rather than an Account API Token to avoid `401 Unauthorized` errors during cache purge. See [API Token Permissions](/docs/cloudflare/tokens) for step-by-step setup.

## 1. Export Credentials

```bash
export CLOUDFLARE_ACCOUNT_ID="your-account-id"
export CLOUDFLARE_API_TOKEN="your-api-token"
```

## 2. Configure `nextdeploy.yml`

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

## 3. Plan & Ship

```bash
nextdeploy plan   # Preview resources, bindings, and drift
nextdeploy ship   # Build, compile worker.mjs (~1.60 MB gzipped), sync R2, deploy, smoke verify
```

Deployment reports are saved to `.nextdeploy/reports/` (gitignored), and live logs can be tailed with `nextdeploy logs`.
