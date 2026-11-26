---
title: Security Architecture & Trust Model
lede: "Comprehensive security architecture: zero-SaaS trust boundaries, ChaCha20-Poly1305 AEAD secret encryption, bare-metal systemd sandboxing, and edge protection guards."
status: ready
sourceRef: shared/secrets/, shared/protection/, daemon/, cli/internal/serverless/
related:
  - title: Secrets Overview
    href: /docs/secrets/overview
  - title: Secrets Encryption
    href: /docs/secrets/encryption
  - title: systemd Hardening
    href: /docs/vps/systemd-hardening
---

## 1. Zero-SaaS Trust Boundary

Unlike managed PaaS providers (Vercel, Render, Railway), NextDeploy has **no central SaaS control plane**:
- No central servers store your database passwords, API tokens, or source code.
- Builds, secret decryption, SSH command execution, and cloud API calls execute **directly from your local workstation or CI/CD runner** straight to your own VPS, Cloudflare account, or AWS account.
- If NextDeploy's GitHub repository or website ever experienced an outage, your deployed infrastructure continues operating with **zero dependency**.

```mermaid
flowchart LR
    subgraph Boundary ["Developer / CI Runner (Your Boundary)"]
        CLI["NextDeploy CLI"]
        Vault["Encrypted Vault .secrets"]
    end

    subgraph Targets ["Direct Cloud Targets (Your Accounts)"]
        VPS["Your Linux VPS (SSH / nextdeployd)"]
        CF["Your Cloudflare Account (Workers / R2)"]
        AWS["Your AWS Account (Lambda / CloudFront)"]
    end

    CLI -->|"Direct SSH / HMAC-SHA256"| VPS
    CLI -->|"Direct Cloudflare API"| CF
    CLI -->|"Direct AWS SDK / SigV4"| AWS
```

---

## 2. Secrets & Cryptographic Integrity

- **Authenticated Encryption at Rest**: Secrets stored locally or in CI/CD are encrypted with **ChaCha20-Poly1305** or **AES-256-GCM** using keys derived via **Argon2id** (64MB memory, 3 iterations, 4 parallelism).
- **Unique Per-Write Nonces**: Every secret write generates a new cryptographically secure 96-bit nonce via `crypto/rand`. Tampered ciphertexts are rejected before decryption.
- **In-Memory Injection**: Secrets are injected directly into process memory or mounted via the AWS Secrets Manager Extension daemon, preventing plaintext leakage into `/etc/environment` or shell history files.

---

## 3. VPS Bare-Metal Process Hardening

Running applications directly on a Linux server without Docker containers requires strict kernel sandboxing. NextDeploy configures systemd with container-grade security:

- **Unprivileged `DynamicUser=yes`**: Allocates an ephemeral UID/GID (`61184-65519`) per service instance. The process has no login shell and does not exist in `/etc/passwd`.
- **Filesystem Lockdown**: `ProtectSystem=strict` mounts `/usr`, `/boot`, and `/etc` read-only. `ProtectHome=yes` completely hides `/root` and `/home`.
- **Private Namespaces**: `PrivateTmp=yes` places temporary files in an isolated namespace; `PrivateDevices=yes` blocks raw device node access.
- **Linux `cgroup v2` Memory Limits**: Caps application memory (`MemoryMax=1G`). If a Node.js process experiences an out-of-memory leak, the kernel restarts only that worker without impacting Caddy or the host OS.

---

## 4. Cloudflare Edge Protection Guard (`shared/protection/`)

For edge deployments on Cloudflare Workers, NextDeploy embeds an Edge Protection Guard ahead of the Next.js routing pipeline:

### A. IP Allowlist & Denylist
Supports IPv4 and IPv6 CIDR matching. Traffic from blocked ranges returns `403 Forbidden` at Cloudflare's edge before running Next.js server code.

### B. Fail-Closed KV Rate Limiting
Enforces per-IP fixed 60-second request windows (`rl:<ip>:<windowId>`) backed by Cloudflare KV. Traffic uses the trusted `cf-connecting-ip` header while ignoring untrusted `x-forwarded-for` headers by default, and fails closed (`503 Service Unavailable`) if the declared KV binding is missing.

#### KV Write Coalescing & Contention Safety
Cloudflare Workers KV enforces a strict **1 write (`PUT`) per second per key** limit (and **1,000 `PUT`s/day** on the Free plan). Without coalescing, simultaneous Next.js page navigations and background `<Link>` RSC prefetches for the same visitor IP hit the same window key inside 1 second and trigger `Error: KV PUT failed: 429 Too Many Requests`.

NextDeploy's edge guard (`shared/nextcompile/runtime_src/guard.mjs`) prevents KV contention crashes automatically:
- **Per-isolate write coalescing (`1,100ms` cooldown):** Burst requests keep incrementing the isolate-local counter while persisting `kv.put` at most once every `1,100ms` per IP window key via `ctx.waitUntil(...)`.
- **Non-fatal KV error handling:** Any KV `429` or transient `get`/`put` failure is logged and swallowed instead of crashing the Worker with `500 Internal Server Error`, while isolate-local counting continues enforcing the configured `requests_per_minute`.
- **Route scoping:** Scope `rate_limit.paths` and `public_paths` in `nextdeploy.yml` so static documentation and RSC prefetches do not consume KV write quota:

```yaml
    protection:
      enabled: true
      public_paths: ["/", "/login", "/docs", "/docs/*"]
      rate_limit:
        kv_binding: RATE_LIMIT
        requests_per_minute: 60
        paths: ["/api/*", "/app/*"]
```

### C. HMAC-Signed Session Cookie Authentication
Enforces edge authentication on protected routes (e.g. `/app/*`, `/admin/*`) using HMAC-SHA256 signed session cookies with constant-time verification. Unauthorized requests receive an instant `401 Unauthorized` (for API routes) or `302 Redirect` to `/login` (for browser page requests).

### D. SSRF & URL Guard Protections
NextDeploy's image optimization and asset dispatcher enforce strict URL sanitation:
- Protocol-relative URLs (`//evil.com`) and non-HTTP schemes (`file://`, `gopher://`) are rejected with `400 Bad Request`.
- Remote image requests are clamped to hostnames explicitly allowlisted in `images.remotePatterns` in `next.config.mjs`.
- Bare `.html` keys are blocked from root asset lookups to prevent prerendered page bypass.
