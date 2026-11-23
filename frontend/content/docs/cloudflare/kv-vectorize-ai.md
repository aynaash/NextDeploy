---
title: KV / Vectorize / AI
lede: Provision and bind Cloudflare KV namespaces, Vectorize indexes, D1 databases, and Workers AI directly in nextdeploy.yml.
status: ready
sourceRef: cli/internal/serverless/cloudflare_bindings.go, cli/internal/serverless/cloudflare_resources.go
related:
  - title: Bindings
    href: /docs/cloudflare/bindings
  - title: Workers Runtime
    href: /docs/cloudflare/workers
---

## Cloudflare KV

Use KV for edge rate limiting, session metadata, and cross-isolate `revalidateTag()` invalidation (`NEXTCOMPILE_CACHE`):

```yaml
serverless:
  provider: cloudflare
  cloudflare:
    resources:
      kv:
        - name: RATE_LIMIT_KV
          binding: RATE_LIMIT
        - name: NEXTCOMPILE_CACHE_KV
          binding: NEXTCOMPILE_CACHE
```

When `NEXTCOMPILE_CACHE` is bound, NextDeploy's `CloudflareCacheHandler` automatically propagates `revalidateTag()` timestamps across every Cloudflare edge isolate worldwide.

## Cloudflare D1 (SQLite at the Edge)

```yaml
    resources:
      d1:
        - name: app_prod_db
          binding: DB
```

Query D1 inside Route Handlers or Server Components via `globalThis.__nextdeployEnv.DB.prepare(...)`.

## Workers AI & Vectorize

Bind Cloudflare's serverless GPU inference (`AI`) and vector database (`VECTORIZE`) in the same `nextdeploy.yml`:

```yaml
    bindings:
      ai:
        binding: AI
      vectorize:
        - binding: DOCS_INDEX
          index_name: docs-embeddings
```

```ts
const env = (globalThis as any).__nextdeployEnv
const result = await env.AI.run('@cf/meta/llama-3.1-8b-instruct', {
  prompt: 'Summarize NextDeploy in one sentence.',
})
```
