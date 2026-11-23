---
title: Configuration Examples
lede: Ready-to-copy nextdeploy.yml templates for Cloudflare Workers (Next.js 16 Turbopack + Edge Guard + D1/KV/R2), Linux VPS with Go Sidecar, and AWS Serverless.
status: ready
sourceRef: sample.nextdeploy.yml, docs/guide/configuration.md
related:
  - title: YAML Reference
    href: /docs/config/reference
  - title: Cloudflare Quickstart
    href: /docs/getting-started/cloudflare-quickstart
  - title: VPS Quickstart
    href: /docs/getting-started/vps-quickstart
---

## 1. Cloudflare Workers (Next.js 16 Turbopack + D1 + KV + Edge Auth)

```yaml
version: "1.0"
target_type: serverless

app:
  name: nextdeploy-frontend
  environment: production
  domain: nextdeploy.org

serverless:
  provider: cloudflare
  cloudflare:
    account_id: ${CLOUDFLARE_ACCOUNT_ID}
    api_token: ${CLOUDFLARE_API_TOKEN}
    zone_name: nextdeploy.org
    engine: auto
    compatibility_date: "2025-04-01"
    compatibility_flags:
      - nodejs_compat_v2
    resources:
      d1:
        - name: app_prod_db
          binding: DB
      kv:
        - name: rate_limit_kv
          binding: RATE_LIMIT
        - name: cache_tags_kv
          binding: NEXTCOMPILE_CACHE
    protection:
      enabled: true
      public_paths:
        - /
        - /docs/*
        - /login
        - /api/health
      rate_limit:
        kv_binding: RATE_LIMIT
        requests_per_minute: 120
      auth:
        secret_env: AUTH_SECRET
        cookie_name: nd_session
        protected_paths:
          - /app/*
        login_path: /login
```

## 2. Linux VPS with Go Sidecar & cgroup v2 Sandboxing

```yaml
version: "1.0"
target_type: vps

app:
  name: my-store
  environment: production
  domain: store.example.com
  port: 3000
  gosidecar:
    enabled: true
    image_optimization: true
    isr_cache: true
    max_image_workers: 4

deployment:
  server:
    host: 203.0.113.10
    user: ubuntu
    ssh_key: ~/.ssh/id_ed25519
    use_sudo: true
  healthcheck:
    path: /api/health
    interval: 30s
    timeout: 5s
    retries: 3
  resources:
    memory_max: 1024M
    cpu_quota: 150%
```

## 3. AWS Serverless (Lambda + S3 + CloudFront)

```yaml
version: "1.0"
target_type: serverless

app:
  name: cloud-store
  environment: production
  domain: store.example.com

serverless:
  provider: aws
  aws:
    region: us-east-1
    memory_size: 1024
    timeout: 30
    cloudfront:
      price_class: PriceClass_100
```
