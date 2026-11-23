---
title: Bindings
lede: Declare Cloudflare R2, KV, D1, Hyperdrive, Queues, Durable Objects, Vectorize, Workers AI, and secret_text bindings in nextdeploy.yml and access them cleanly inside Next.js.
status: ready
sourceRef: cli/internal/serverless/cloudflare_bindings.go, shared/config/
related:
  - title: R2 Storage
    href: /docs/cloudflare/r2
  - title: Hyperdrive
    href: /docs/cloudflare/hyperdrive
  - title: Queues
    href: /docs/cloudflare/queues
  - title: KV / Vectorize / AI
    href: /docs/cloudflare/kv-vectorize-ai
---

## Declarative Bindings in `nextdeploy.yml`

Every Cloudflare binding your Next.js app needs is declared once in `nextdeploy.yml`. NextDeploy auto-provisions missing resources, checks drift on `nextdeploy plan`, and wires every binding into the Worker metadata on `nextdeploy ship`:

```yaml
serverless:
  provider: cloudflare
  cloudflare:
    account_id: ${CLOUDFLARE_ACCOUNT_ID}
    api_token: ${CLOUDFLARE_API_TOKEN}
    zone_name: example.com
    engine: auto
    compatibility_date: "2025-04-01"
    compatibility_flags:
      - nodejs_compat_v2
    resources:
      kv:
        - name: RATE_LIMIT_KV
          binding: RATE_LIMIT
      d1:
        - name: app_prod_db
          binding: DB
      r2:
        - name: uploads-bucket
          binding: UPLOADS
      queues:
        - name: jobs-queue
          binding: JOBS
      hyperdrive:
        - name: pg-primary
          binding: HYPERDRIVE
          origin: ${DATABASE_URL}
```

## Auto-Injected Bindings

- **`ASSETS` (R2):** Automatically created and bound for your app's static assets, prerendered `.html`/`.rsc` pages, and `CloudflareCacheHandler` L2 storage unless you declare your own `ASSETS` binding.
- **`secret_text` bindings:** Runtime secrets from `.env` / Doppler / secret providers are folded automatically into the Worker upload and mirrored onto `process.env`. Duplicate binding names (e.g. an `.env` key colliding with `ASSETS` or `DB`) fail fast before upload.

## Accessing Bindings in Next.js Route Handlers & Server Components

NextDeploy exposes the active Cloudflare `env` object on `globalThis.__nextdeployEnv` on every request:

```ts
// lib/env.ts
export interface CloudflareEnv {
  DB?: any
  ASSETS?: any
  RATE_LIMIT?: any
  JOBS?: any
  HYPERDRIVE?: { connectionString: string }
  AI?: any
}

export function getCloudflareEnv(): CloudflareEnv {
  return ((globalThis as any).__nextdeployEnv ?? {}) as CloudflareEnv
}
```
