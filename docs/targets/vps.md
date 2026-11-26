# VPS Deployment & Go Sidecar

Deploying to a Linux VPS gives you bare-metal speed, complete ownership of your infrastructure, and predictable $4–$20/month hosting costs without compute markup.

---

## Architecture on VPS

When targeting a VPS, NextDeploy orchestrates three core components:

1. **Caddy Reverse Proxy**: Handles automatic Let's Encrypt TLS certificate generation, HTTP/2 and HTTP/3 termination, and direct streaming of static files (`/_next/static/*` and `public/*`) from NVMe disk.
2. **`nextdeployd` Daemon**: A lightweight Go agent installed on your VPS that manages atomic releases, process supervision via systemd, and dual-stage health probing.
3. **Optional Go Sidecar (`gosidecar`)**: An in-memory Go reverse proxy that sits between Caddy and Node.js to serve prerendered RSC payloads and execute image transformations at native machine speed.

```
Incoming Request (:443)
       │
       ▼
 ┌───────────┐       Static File
 │   Caddy   ├──────────────────────────► Disk: /opt/nextdeploy/apps/.../shared_static
 └─────┬─────┘
       │ Dynamic / SSR
       ▼
 ┌──────────────────────────────────────┐
 │          Go Sidecar (:3001)          │
 │  ┌────────────────────────────────┐  │
 │  │ Prerendered HTML / RSC Cache   │  │
 │  └────────────────┬───────────────┘  │
 │                   │ Cache Miss       │
 │                   ▼                  │
 │  ┌────────────────────────────────┐  │
 │  │ Image Transcoding Engine       │  │
 │  └────────────────┬───────────────┘  │
 └───────────────────┼──────────────────┘
                     │ Dynamic SSR / API / Actions
                     ▼
          ┌─────────────────────┐
          │ Next.js Node Server │
          │       (:3000)       │
          └─────────────────────┘
```

---

## Prerequisites

Before bootstrapping or shipping a full-stack Next.js app to a VPS, make sure you have:

1. **Linux VPS:** Ubuntu or Debian server with a public IPv4 address and inbound ports `22` (SSH), `80` (HTTP/ACME), and `443` (HTTPS) open.
2. **Non-root `sudo` user:** A non-root SSH user (e.g., `deploy` or `ubuntu`) with `sudo` privileges. `nextdeploy prepare` refuses `root` SSH logins by default for security (pass `--allow-root` only if unavoidable).
3. **SSH private key path:** A local private SSH key (`~/.ssh/id_ed25519` with `chmod 600`) configured as `key_path` in `nextdeploy.yml`, with its public key installed in the remote user's `~/.ssh/authorized_keys`.
4. **Domain name pointing to your VPS IP:** An `A` record (`@` or subdomain -> VPS IPv4) at your DNS registrar so Caddy can issue Let's Encrypt TLS certificates automatically.
5. **Secrets synchronization:** Sync full-stack environment variables (`DATABASE_URL`, `AUTH_SECRET`, API keys) via **Doppler (recommended)** (`doppler run -- nextdeploy secrets sync`) or **`.env`** (`nextdeploy secrets load .env.production && nextdeploy secrets sync`).

---

## Step-by-Step Setup

### 1. Configure SSH Access

Ensure your local machine can connect to your VPS with SSH key authentication:

```bash
ssh -i ~/.ssh/id_ed25519 ubuntu@203.0.113.10
```

Verify that the remote user has `sudo` access if automatic dependency provisioning is required.

### 2. Prepare the Server (`prepare`)

Run the automated preparation command to bootstrap your server:

```bash
nextdeploy prepare
```

This command connects via SSH and:
- Installs and configures **Caddy** with systemd integration.
- Configures Linux kernel **cgroups v2** memory sandboxing.
- Installs the matching version of **`nextdeployd`**.
- Sets up deployment directory structures (`/opt/nextdeploy/apps/<app-name>`).

### 3. Configure `nextdeploy.yml`

In your Next.js project root, ensure `target_type` is set to `vps`:

```yaml
version: "1.0"
target_type: vps

app:
  name: my-store
  domain: store.example.com
  port: 3000
  environment: production
  gosidecar:
    enabled: true
    image_optimization: true
    isr_cache: true

servers:
  - name: prod-01
    host: 203.0.113.10
    username: ubuntu
    key_path: ~/.ssh/id_ed25519
    port: 22
```

### 4. Deploy (`ship`)

Ship your app:

```bash
nextdeploy ship
```

NextDeploy will:
1. Build your Next.js standalone application locally.
2. Synchronize static assets to the cumulative `shared_static` pool.
3. Rsync server runtime files to `/opt/nextdeploy/apps/<app>/releases/<timestamp>`.
4. Spawn the new Node.js instance on an isolated port.
5. Perform dual-stage TCP + HTTP health probing.
6. Atomically flip the `current` symlink.
7. Gracefully drain and terminate the previous release.

---

## Enabling the Go Sidecar

The Go sidecar is designed specifically for high-traffic Next.js apps. It intercepts requests before they hit Node.js:

- **Static Pages & RSC**: Serves `.rsc` and `.html` directly from memory or disk cache in sub-millisecond latency.
- **Image Optimization**: Bypasses Node.js image processing completely. Uses Go concurrency to transcode WebP/AVIF images.
- **Zero Configuration**: Enable it in `nextdeploy.yml`:

```yaml
app:
  gosidecar:
    enabled: true
    image_optimization: true
    isr_cache: true
    max_image_workers: 4
```

---

## Automated CI/CD for VPS

NextDeploy includes first-class CI/CD generation for VPS deployments with zero vendor lock-in.

Generate the GitHub Actions workflow with:

```bash
nextdeploy generate-ci
```

### Production CI/CD Features for VPS:
- **Target-Aware at Runtime**: The generated workflow detects VPS deployment from `nextdeploy.yml` without needing to re-scaffold CI when retargeting.
- **SSH Key & Path Hardening**: Securely writes `$SSH_PRIVATE_KEY` (`0600`) to both standard paths (`~/.ssh/id_ed25519` and `~/.ssh/id_rsa`) and enforces directory permissions (`chmod 700 ~/.ssh`).
- **Host Key Pinning**: Supports `SSH_KNOWN_HOSTS` to verify the VPS public key on ephemeral CI runners and avoid MITM vulnerabilities.
- **Strict Host Checking**: Pass `NEXTDEPLOY_STRICT_HOST_KEY=1` in GitHub Secrets to fail closed if an unpinned host connects.
- **Atomic Cutover in CI**: Automatically invokes `nextdeploy ship --verify` to guarantee dual-stage health probing passes before marking the CI job as successful.

---

## Upgrading the VPS Daemon

When updating your local `nextdeploy` CLI, upgrade the remote server agent with:

```bash
nextdeploy upgrade-daemon
```
