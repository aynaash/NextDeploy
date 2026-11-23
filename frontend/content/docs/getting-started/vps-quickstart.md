---
title: VPS Quickstart
lede: Deploy a Next.js app to your own Linux VPS with Caddy auto-TLS, systemd hardening, zero-downtime atomic cutover, and optional Go sidecar acceleration in under 5 minutes.
status: ready
sourceRef: cli/cmd/prepare.go, cli/cmd/ship.go, docs/targets/vps.md
related:
  - title: VPS Overview
    href: /docs/vps/overview
  - title: Go Sidecar
    href: /docs/vps/go-sidecar
  - title: Zero-Downtime Cutover
    href: /docs/vps/atomic-cutover
  - title: Rollback
    href: /docs/vps/rollback
---

## 1. Verify SSH Access

Make sure you can reach your VPS using your SSH key:

```bash
ssh -i ~/.ssh/id_ed25519 ubuntu@203.0.113.10
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

deployment:
  server:
    host: 203.0.113.10
    user: ubuntu
    ssh_key: ~/.ssh/id_ed25519
    use_sudo: true
  healthcheck:
    path: /api/health
    timeout: 5s
    retries: 3
  resources:
    memory_max: 1024M
```

## 3. Bootstrap the Server (`prepare`)

Run once per VPS to install Caddy, `nextdeployd`, systemd units, cgroup v2 limits, and `/var/www/apps/<app-name>`:

```bash
nextdeploy prepare
```

## 4. Point DNS to Your VPS

Create an `A` record for `@` pointing to your VPS IPv4 address (and optionally a `CNAME` for `www`). Caddy automatically provisions Let's Encrypt TLS certificates on the first HTTPS visit.

## 5. Preview and Ship

```bash
nextdeploy plan
nextdeploy ship
```

NextDeploy builds `.next/standalone`, syncs content-hashed assets to the cumulative `shared_static` pool (eliminating `ChunkLoadError`), uploads the release, probes TCP + `/api/health`, and atomically flips the `current` symlink.

## 6. Verify & Tail Logs

```bash
nextdeploy status
nextdeploy logs -f
```

If anything looks wrong, revert in milliseconds without rebuilding:

```bash
nextdeploy rollback
```
