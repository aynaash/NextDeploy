# Configuration Reference (`nextdeploy.yml`)

The `nextdeploy.yml` file is the central source of truth for your Next.js application infrastructure. It lives in the root directory of your project repository.

---

## Complete Example

```yaml
version: "1.0"

# Target type: "vps" | "serverless"
target_type: vps

# ── App Metadata ────────────────────────────────────────────────
app:
  name: my-app                  # 3-63 lowercase alphanumeric chars and hyphens
  environment: production       # development | staging | production
  domain: app.example.com       # Public custom domain
  port: 3000                    # Internal application port
  gosidecar:
    enabled: true               # Enable Go sidecar for VPS
    image_optimization: true    # Enable Go image transcoding
    isr_cache: true             # Cache prerendered RSC payloads
    max_image_workers: 4

# ── VPS Deployment Target ───────────────────────────────────────
deployment:
  server:
    host: 203.0.113.10          # Server IPv4 or hostname
    user: deploy                # SSH deployment user
    ssh_key: ~/.ssh/id_ed25519  # Local path to private SSH key
    use_sudo: true              # Execute system commands with sudo
  healthcheck:
    path: /api/health           # Healthcheck endpoint
    interval: 30s
    timeout: 5s
    retries: 3
  resources:
    memory_max: 1024M           # Linux cgroup v2 memory limit
    cpu_quota: 150%

# ── Serverless Deployment Target (Optional) ─────────────────────
# Used when target_type is set to "serverless"
serverless:
  provider: cloudflare          # "cloudflare" | "aws"
  cloudflare:
    account_id: ${CLOUDFLARE_ACCOUNT_ID}
    api_token: ${CLOUDFLARE_API_TOKEN}
    zone_name: example.com
    compatibility_date: "2024-09-23"
    resources:
      kv:
        - name: CACHE_KV
      d1:
        - name: primary_db
          binding: DB
      queues:
        - name: email-queue
          binding: QUEUE
  aws:
    region: us-east-1
    memory_size: 1024           # Lambda memory in MB
    timeout: 30                 # Timeout in seconds

# ── Database & Migrations ───────────────────────────────────────
database:
  type: postgres                # postgres | mysql
  host: 10.0.0.5
  port: 5432
  name: app_prod
  username: app_user
  password: ${DB_PASSWORD}
  migrate_on_deploy: true       # Run migrations on deploy

# ── Logging & Monitoring ────────────────────────────────────────
logging:
  enabled: true
  provider: nextdeploy
  stream_logs: true

monitoring:
  enabled: true
  cpu_threshold: 80
  memory_threshold: 80
  alert:
    slack_webhook: ${SLACK_WEBHOOK_URL}
    notify_on:
      - crash
      - healthcheck_failed
```

---

## Field Specifications

### Root Fields

| Field | Type | Description |
| :--- | :--- | :--- |
| `version` | string | Configuration schema version (`"1.0"`). |
| `target_type` | string | Deployment target: `"vps"` or `"serverless"`. |

### `app`

| Field | Type | Required | Description |
| :--- | :--- | :--- | :--- |
| `name` | string | Yes | Unique application identifier (`^[a-z0-9-]+$`, 3-63 characters). |
| `environment` | string | Yes | Environment name (`production`, `staging`, `development`). |
| `domain` | string | Yes | Public domain name where Caddy or CloudFront/Cloudflare will route traffic. |
| `port` | integer | No | Internal port for the Node.js HTTP server (default: `3000`). |
| `gosidecar.enabled` | boolean | No | Enables the high-performance Go sidecar for VPS targets (default: `false`). |
| `gosidecar.image_optimization` | boolean | No | Offload Next.js image resizing and WebP/AVIF generation to Go sidecar. |
| `gosidecar.isr_cache` | boolean | No | Enable Go sidecar caching of static HTML and RSC payloads. |

### `deployment.server` (VPS Target)

| Field | Type | Required | Description |
| :--- | :--- | :--- | :--- |
| `host` | string | Yes | VPS IP address or hostname accessible over SSH. |
| `user` | string | Yes | Remote SSH username (e.g. `ubuntu` or `deploy`). |
| `ssh_key` | string | Yes | Local path to SSH private key. |
| `use_sudo` | boolean | No | Whether commands run with `sudo` (default: `false`). |

### `deployment.healthcheck`

| Field | Type | Default | Description |
| :--- | :--- | :--- | :--- |
| `path` | string | `"/api/health"` | Route to probe before completing atomic cutover. Must return HTTP 200. |
| `interval` | duration | `30s` | Health check interval. |
| `timeout` | duration | `5s` | Timeout for each health check probe. |
| `retries` | integer | `3` | Number of probe retries before marking deploy as failed. |

### `serverless.cloudflare`

| Field | Type | Description |
| :--- | :--- | :--- |
| `account_id` | string | Cloudflare Account ID. Can use `${CLOUDFLARE_ACCOUNT_ID}` env var. |
| `api_token` | string | Cloudflare API Token with Workers, R2, and DNS permissions. |
| `compatibility_date` | string | Cloudflare Workers compatibility date (e.g. `"2024-09-23"`). |
| `resources` | object | Declarative bindings for KV, D1 databases, Queues, Hyperdrive, Vectorize, AI Gateway. |

### Environment Variable Substitution

`nextdeploy.yml` natively supports environment variable interpolation using `${VARIABLE_NAME}` syntax. Unresolved variables can be populated via local `.env` or CI secret stores.
