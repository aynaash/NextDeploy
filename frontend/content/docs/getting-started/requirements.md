---
title: Requirements
lede: Everything you need before deploying Next.js full-stack applications with NextDeploy — VPS host setup, non-root sudo user, SSH key path, DNS, and secrets synchronization.
status: ready
sourceRef: cli/cmd/prepare.go, cli/cmd/secrets.go, shared/config/types.go
related:
  - title: Installation
    href: /docs/getting-started/installation
  - title: VPS Quickstart
    href: /docs/getting-started/vps-quickstart
  - title: AWS Quickstart
    href: /docs/getting-started/aws-quickstart
  - title: Cloudflare Quickstart
    href: /docs/getting-started/cloudflare-quickstart
  - title: Secrets Overview
    href: /docs/secrets/overview
  - title: Doppler Integration
    href: /docs/secrets/doppler
---

Before running `nextdeploy prepare` or `nextdeploy ship`, verify that your Next.js application, local CLI environment, and target infrastructure satisfy the requirements below.

---

## 1. Next.js App & Local Machine Requirements

- **Supported Next.js Versions:** Next.js 14, 15, or 16 (App Router or Pages Router, Turbopack or Webpack) that builds cleanly with `next build`.
- **Standalone Output:** Enable `output: 'standalone'` in `next.config.mjs` (required for AWS and Cloudflare, and recommended for minimal VPS bundles):

```js
/** @type {import('next').NextConfig} */
const nextConfig = {
  output: 'standalone',
}

export default nextConfig
```

- **Git Repository:** Initialize Git (`git init`) so `nextdeploy build` can record deployment provenance in `metadata.json`.
- **NextDeploy CLI:** Installed locally (`nextdeploy version`).

---

## 2. Linux VPS Requirements (Recommended for Full-Stack Apps)

Deploying full-stack Next.js applications (SSR, Server Actions, API routes, databases, background jobs) to a Linux VPS requires four host prerequisites plus a secrets sync strategy.

### 1. Linux VPS & Open Firewall Ports
- **OS:** Ubuntu (22.04 / 24.04) or Debian (12+) with a public IPv4 address.
- **Resources:** At least **1 GB RAM** (2+ GB recommended if building/running full-stack SSR apps with image optimization).
- **Firewall Ports:** Allow inbound TCP traffic on:
  - **`22`** — SSH / SFTP access for the CLI
  - **`80`** — HTTP & Let's Encrypt ACME HTTP-01 challenges
  - **`443`** — HTTPS traffic served by Caddy

### 2. Non-Root User with `sudo` Privileges (Required by Default)
For security and compliance, `nextdeploy prepare` **refuses to provision over a `root` SSH login by default** (unless `--allow-root` is explicitly passed, which is not recommended).

Your VPS username in `nextdeploy.yml` should be a **non-root user with `sudo` access** (for example `deploy` or your cloud provider's default `ubuntu` user).

If your VPS only gave you `root`, create a `deploy` user and copy your authorized SSH keys:

```bash
# Run on your VPS as root:
adduser --disabled-password --gecos "" deploy
usermod -aG sudo deploy

# Allow passwordless sudo for automated provisioning (recommended)
echo "deploy ALL=(ALL) NOPASSWD:ALL" | tee /etc/sudoers.d/deploy
chmod 440 /etc/sudoers.d/deploy

# Copy your SSH public key to the new non-root user
mkdir -p /home/deploy/.ssh
cp /root/.ssh/authorized_keys /home/deploy/.ssh/authorized_keys
chown -R deploy:deploy /home/deploy/.ssh
chmod 700 /home/deploy/.ssh
chmod 600 /home/deploy/.ssh/authorized_keys
```

### 3. SSH Private Key Path for the CLI
The `nextdeploy` CLI connects to your VPS over SSH and SFTP for `prepare`, `ship`, `status`, `logs`, `rollback`, and `secrets`. You must provide the path to your local **private** SSH key (`key_path` in `nextdeploy.yml`):

```yaml
servers:
  - name: prod-01
    host: 203.0.113.10
    username: deploy
    key_path: ~/.ssh/id_ed25519
    port: 22
```

If you don't have a keypair yet, generate one locally and copy the public key to the VPS:

```bash
ssh-keygen -t ed25519 -C "nextdeploy" -f ~/.ssh/id_ed25519
chmod 600 ~/.ssh/id_ed25519
ssh-copy-id -i ~/.ssh/id_ed25519.pub deploy@203.0.113.10
```

### 4. Domain Name Pointed to Your VPS IP
Caddy automatically provisions and renews Let's Encrypt TLS certificates for your app's `domain`. Before running `nextdeploy ship`, create DNS records at your domain registrar pointing to your VPS public IPv4 address:

| Record Type | Host / Name | Value | Notes |
| :--- | :--- | :--- | :--- |
| **`A`** | `@` (or `app`) | `203.0.113.10` | Must resolve to your VPS IP before TLS issuance |
| **`CNAME`** *(optional)* | `www` | `@` (or `app.example.com`) | Redirects `www` traffic |

*(If using Cloudflare DNS in front of a VPS, set the proxy status to **DNS Only / grey cloud** during initial Let's Encrypt certificate issuance.)*

### 5. Secrets Synchronization (Doppler Recommended, or `.env`)
Full-stack Next.js apps depend on runtime environment variables and secrets (`DATABASE_URL`, `AUTH_SECRET`, `STRIPE_SECRET_KEY`, OAuth keys, etc.). NextDeploy does not commit `.env` files into build artifacts—you must sync secrets using one of two methods:

- **Option A — Doppler (Recommended):** Keeps zero plaintext `.env` files on disk, supports separate `dev`/`stg`/`prd` configs, and injects secrets into memory at deploy time:
  ```bash
  doppler run -- nextdeploy secrets sync
  doppler run -- nextdeploy ship
  ```
- **Option B — `.env` File / Local Encrypted Vault:** Load a local `.env` file into NextDeploy's ChaCha20-Poly1305 encrypted store and sync it to your VPS daemon:
  ```bash
  nextdeploy secrets load .env.production
  nextdeploy secrets sync
  ```
  Or set individual keys:
  ```bash
  nextdeploy secrets set DATABASE_URL="postgres://..." AUTH_SECRET="..."
  ```

### 6. Databases & External Services
NextDeploy deploys and manages your Next.js compute layer, reverse proxy, and secrets—not managed database instances. If your full-stack app uses PostgreSQL, MySQL, or Redis:
- Provision a managed database (Neon, Supabase, RDS, Upstash) or run it on your server/private network.
- Supply its connection string (`DATABASE_URL`, `REDIS_URL`, etc.) via **Doppler** or **`nextdeploy secrets load`** before running `nextdeploy ship`.

---

## 3. AWS Serverless Requirements

- **IAM Credentials:** An IAM user or role with permissions for Lambda, S3, CloudFront, ACM, SQS, IAM, and Secrets Manager (`AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_REGION`).
  - Note: CloudFront ACM SSL certificates must be requested in **`us-east-1`**.
- **Standalone Output:** `output: 'standalone'` in `next.config.mjs`.
- **Domain & DNS Registrar Access:** You will need to add the ACM validation CNAME records and point your domain to the generated `xxxx.cloudfront.net` distribution (written to `.nextdeploy/dns.md`).
- **Secrets Synchronization:** Sync secrets to AWS Secrets Manager via **Doppler (recommended)** (`doppler run -- nextdeploy secrets sync`) or **`.env`** (`nextdeploy secrets load .env.production && nextdeploy secrets sync`).

---

## 4. Cloudflare Workers Requirements

- **Cloudflare Account & User API Token:** Export `CLOUDFLARE_ACCOUNT_ID` and `CLOUDFLARE_API_TOKEN` with `Workers Scripts: Edit`, `Workers R2 Storage: Edit`, `Account Settings: Read`, `Zone: Read`, `DNS: Edit`, and `Cache Purge: Purge`. See [API Token Permissions](/docs/cloudflare/tokens).
- **Standalone Output:** `output: 'standalone'` in `next.config.mjs`.
- **Domain on Cloudflare DNS:** Your domain must be onboarded to a Cloudflare Zone to attach custom domains automatically.
- **Secrets Synchronization:** Sync Worker secrets via **Doppler (recommended)** (`doppler run -- nextdeploy secrets sync`), **`.env`** (`nextdeploy secrets load .env.production`), or **Cloudflare Secret Store** bindings in `nextdeploy.yml`.

---

## 5. Pre-Flight Readiness Check

Before running `nextdeploy ship`, verify everything is wired properly:

```bash
# 1. Verify non-root SSH key login and sudo access (VPS)
ssh -i ~/.ssh/id_ed25519 deploy@203.0.113.10 "sudo -n true && echo OK"

# 2. Verify DNS resolves to your VPS IP (VPS)
dig +short app.example.com

# 3. Verify cloud credentials (AWS / Cloudflare)
nextdeploy creds

# 4. Dry-run preflight validation (checks config, routes, secrets, and drift)
nextdeploy plan
```
