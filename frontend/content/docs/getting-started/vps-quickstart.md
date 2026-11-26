---
title: VPS Quickstart
lede: Deploy a Next.js app to your own Linux VPS with Caddy auto-TLS, systemd hardening, zero-downtime atomic cutover, and optional Go sidecar acceleration in under 5 minutes.
status: ready
sourceRef: cli/cmd/prepare.go, cli/cmd/ship.go, docs/targets/vps.md
related:
  - title: Requirements
    href: /docs/getting-started/requirements
  - title: VPS Overview
    href: /docs/vps/overview
  - title: Go Sidecar
    href: /docs/vps/go-sidecar
  - title: Zero-Downtime Cutover
    href: /docs/vps/atomic-cutover
  - title: Rollback
    href: /docs/vps/rollback
---

## Prerequisites

Before you start, make sure you have:

- **Linux VPS:** Ubuntu or Debian server with a public IPv4 address and ports `22` (SSH), `80` (HTTP), and `443` (HTTPS) open.
- **Non-root `sudo` user:** A non-root SSH user (e.g. `deploy` or `ubuntu`) with `sudo` privileges. `nextdeploy prepare` refuses `root` SSH logins by default for security (`--allow-root` override is not recommended).
- **SSH private key path:** A local private SSH key (`~/.ssh/id_ed25519` with `chmod 600`) whose public key is in your VPS user's `~/.ssh/authorized_keys`. The CLI requires this `key_path` in `nextdeploy.yml`.
- **Domain name:** A domain or subdomain with an `A` record pointing to your VPS IPv4 address so Caddy can provision Let's Encrypt HTTPS automatically.
- **Secrets strategy (Full-Stack Apps):** **Doppler (recommended)** or a `.env` file synced via `nextdeploy secrets`.

Need help creating a non-root `deploy` user or verifying DNS? See the full [Requirements Guide](/docs/getting-started/requirements).

## 1. Verify SSH Access

Make sure you can reach your VPS as your non-root `sudo` user using your SSH key:

```bash
ssh -i ~/.ssh/id_ed25519 deploy@203.0.113.10
```

## 2. Initialize `nextdeploy.yml`

From the root of your Next.js project:

```bash
nextdeploy init
```

Choose **VPS (SSH)** and fill in your app name, domain, server host, SSH user, and private key path. You can also write `nextdeploy.yml` directly:

```yaml
version: "1.0"
target_type: vps

app:
  name: my-app
  environment: production
  domain: app.example.com
  port: 3000
  gosidecar:
    enabled: true
    image_optimization: true
    isr_cache: true
    max_image_workers: 4

servers:
  - name: prod-01
    host: 203.0.113.10
    username: deploy
    key_path: ~/.ssh/id_ed25519
    port: 22
```

## 3. Bootstrap the Server (`prepare`)

Run once per VPS to install Caddy, `nextdeployd`, systemd units, security hardening, and `/opt/nextdeploy/apps/<app-name>`:

```bash
nextdeploy prepare
```

## 4. Point DNS to Your VPS

Create an `A` record for `@` (or your subdomain) pointing to your VPS IPv4 address (and optionally a `CNAME` for `www`). Caddy automatically provisions Let's Encrypt TLS certificates on the first HTTPS visit.

## 5. Sync Secrets (Full-Stack Apps)

If your app uses environment variables or secrets (`DATABASE_URL`, `AUTH_SECRET`, API keys), sync them before shipping:

- **Doppler (Recommended):**
  ```bash
  doppler run -- nextdeploy secrets sync
  ```
- **Or from a `.env` file:**
  ```bash
  nextdeploy secrets load .env.production
  nextdeploy secrets sync
  ```

## 6. Preview and Ship

```bash
nextdeploy plan
nextdeploy ship
```

*(If using Doppler, run `doppler run -- nextdeploy plan` and `doppler run -- nextdeploy ship`.)*

NextDeploy builds `.next/standalone`, syncs content-hashed assets to the cumulative `shared_static` pool (eliminating `ChunkLoadError`), uploads the release, probes TCP + `/api/health`, and atomically flips the `current` symlink.

## 7. Verify & Tail Logs

```bash
nextdeploy status
nextdeploy logs
```

If anything looks wrong, revert in milliseconds without rebuilding:

```bash
nextdeploy rollback
```
