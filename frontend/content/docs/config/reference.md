---
title: YAML Configuration Reference
lede: Complete schema reference for nextdeploy.yml across VPS, Cloudflare Workers, and AWS Serverless targets, including ${ENV_VAR} interpolation.
status: ready
sourceRef: shared/config/, sample.nextdeploy.yml, docs/guide/configuration.md
related:
  - title: Configuration Examples
    href: /docs/config/examples
  - title: CLI Reference
    href: /docs/cli/reference
  - title: Cloudflare Bindings
    href: /docs/cloudflare/bindings
---

## Root & App Fields

| Field | Type | Description |
| :--- | :--- | :--- |
| `version` | string | Schema version (`"1.0"`). |
| `target_type` | string | `"vps"` or `"serverless"`. |
| `app.name` | string | Lowercase slug (`^[a-z0-9-]+$`, 3–63 chars). |
| `app.environment` | string | `"production"`, `"staging"`, or `"development"`. |
| `app.domain` | string | Primary custom domain (e.g. `nextdeploy.org`). |
| `app.port` | integer | Internal Node.js port for VPS targets (default: `3000`). |
| `app.gosidecar.enabled` | boolean | Enable the Go sidecar on VPS targets. |
| `app.gosidecar.image_optimization` | boolean | Offload `/_next/image` transcoding to the Go sidecar. |
| `app.gosidecar.isr_cache` | boolean | Serve prerendered HTML/RSC directly from Go sidecar memory. |
| `app.gosidecar.max_image_workers` | integer | Concurrent Go image worker pool size (default: `4`). |

## VPS Target (`servers` & `app.resources`)

| Field | Type | Default | Description |
| :--- | :--- | :--- | :--- |
| `servers[].name` | string | — | Identifier for the VPS server. |
| `servers[].host` | string | — | VPS IPv4 address or hostname. |
| `servers[].username` | string | — | SSH username (e.g. `ubuntu`, `deploy`). |
| `servers[].key_path` | string | — | Local path to SSH private key (`~/.ssh/id_ed25519`). |
| `servers[].port` | integer | `22` | SSH port. |
| `app.resources.memory_max` | string | — | Linux `cgroup v2` memory cap (e.g. `1024M`). |
| `app.resources.memory_high` | string | — | systemd `MemoryHigh` soft throttle (e.g. `800M`). |
| `app.resources.cpu_quota` | string | — | systemd `CPUQuota` limit (e.g. `150%`). |

## Cloudflare Target (`serverless.cloudflare.*`)

| Field | Type | Default | Description |
| :--- | :--- | :--- | :--- |
| `serverless.provider` | string | — | `"cloudflare"` or `"aws"`. |
| `serverless.cloudflare.account_id` | string | — | Cloudflare Account ID (`${CLOUDFLARE_ACCOUNT_ID}`). |
| `serverless.cloudflare.api_token` | string | — | Cloudflare API Token (`${CLOUDFLARE_API_TOKEN}`). |
| `serverless.cloudflare.zone_name` | string | — | DNS zone name for custom domain attachment & cache purge. |
| `serverless.cloudflare.engine` | string | `"auto"` | `"auto"` (`nextserver` for Turbopack, `legacy` for Webpack), `"nextserver"`, or `"legacy"`. |
| `serverless.cloudflare.compatibility_date` | string | `"2025-04-01"` | Cloudflare `workerd` compatibility date. |
| `serverless.cloudflare.compatibility_flags` | list | `["nodejs_compat_v2"]` | Cloudflare Worker runtime flags. |
| `serverless.cloudflare.resources` | object | — | Declarative `kv`, `d1`, `r2`, `queues`, and `hyperdrive` resources. |
| `serverless.cloudflare.protection` | object | — | Edge guard: IP allow/deny CIDRs, KV rate limits, HMAC cookie auth. |

## AWS Target (`serverless.*`)

| Field | Type | Default | Description |
| :--- | :--- | :--- | :--- |
| `serverless.provider` | string | — | Set to `"aws"`. |
| `serverless.region` | string | `"us-east-1"` | AWS deployment region. |
| `serverless.memory_size` | integer | `1024` | Lambda memory allocation in MB. |
| `serverless.timeout` | integer | `30` | Lambda timeout in seconds. |
| `serverless.cloudfront_id` | string | — | CloudFront distribution ID for cache invalidation. |
| `serverless.isr_revalidation` | boolean | `false` | Enable ISR revalidation SQS and worker Lambda. |
| `serverless.image_optimization` | boolean | `false` | Enable image optimization Lambda. |
| `serverless.warmer` | boolean | `false` | Deploy EventBridge warmer cron. |

## Environment Variable Interpolation

Any string value in `nextdeploy.yml` supports `${VARIABLE_NAME}` interpolation from your shell, `.env`, or `doppler run --`.
