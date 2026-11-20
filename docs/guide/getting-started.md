# Getting Started

NextDeploy is an open-source, self-hosted deployment engine built specifically for Next.js applications. It allows you to package and deploy modern Next.js apps (versions 13, 14, and 15) to a bare-metal Linux VPS, Cloudflare Workers, or AWS Lambda from a single configuration file (`nextdeploy.yml`).

---

## Prerequisites

Before deploying with NextDeploy, make sure you have:

- A Next.js application that successfully runs `next build`.
- Recommended: Output standalone enabled in `next.config.js`:
  ```js
  // next.config.js / next.config.mjs
  module.exports = {
    output: 'standalone',
  }
  ```
- Target credentials:
  - **VPS target**: A Linux VPS (Ubuntu, Debian) with SSH access and sudo privileges.
  - **Cloudflare target**: `CLOUDFLARE_API_TOKEN` and `CLOUDFLARE_ACCOUNT_ID`.
  - **AWS target**: AWS credentials (`AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_REGION`).

---

## Installation

### macOS & Linux

Install the pre-compiled binary via the official installation script:

```bash
curl -fsSL https://hersi.dev/nextdeploy/install.sh | bash
# Mirror:
curl -fsSL https://nextdeploy.org/install.sh | bash
```

This installs `nextdeploy` to `/usr/local/bin` (or `~/.local/bin`) and verifies execution permissions.

### Windows

Run the PowerShell installer:

```powershell
irm https://hersi.dev/nextdeploy/install.bat | iex
```

### Build from Source

NextDeploy is written in Go. You can build it locally with Go 1.25+:

```bash
git clone https://github.com/aynaash/NextDeploy.git
cd NextDeploy
go build -o nextdeploy ./cli
sudo mv nextdeploy /usr/local/bin/
```

Verify your installation:

```bash
nextdeploy version
```

---

## Quickstart Workflow

Deploying your Next.js application takes just four commands:

### 1. Initialize Configuration

Inside the root directory of your Next.js project, run:

```bash
nextdeploy init
```

NextDeploy will inspect your project and guide you through an interactive setup:
- Choose your target (`vps`, `serverless` with Cloudflare or AWS).
- Specify application name, domain, and environment.
- Configure server connection or cloud credentials.

This generates a validated `nextdeploy.yml` in your project root.

### 2. Dry-Run Preflight (`plan`)

Before making any changes to production, preview the exact deployment actions:

```bash
nextdeploy plan
```

The `plan` command:
- Validates your `nextdeploy.yml` syntax and credentials.
- Inspects your Next.js routes, server actions, and middleware.
- Performs drift detection against target infrastructure.
- Exits with a clear diff summary without modifying target resources.

### 3. Build & Deploy (`ship`)

Ship your application with zero downtime:

```bash
nextdeploy ship
```

> **Tip:** `nextdeploy deploy` is an alias for `ship`.

What happens during `ship`:
1. **Introspection**: Analyzes your Next.js build output (dynamic vs static routes, Server Actions, ISR tags).
2. **Compilation / Packaging**:
   - For **VPS**: Bundles app files and syncs static assets to cumulative shared pool.
   - For **Cloudflare**: Compiles app into a unified Worker via `nextcompile` and uploads static assets to R2.
   - For **AWS**: Packages compute into Lambda zip, deploys CloudFront and S3 assets.
3. **Dual-Stage Health Verification**: Verifies TCP listener and performs HTTP health check against `/api/health` on the new version.
4. **Atomic Cutover**: Atomically flips the traffic pointer (`current.tmp` ➔ `current` symlink or Cloudflare Worker script routing).

### 4. Monitor & Tail Logs

Verify your live deployment and stream runtime logs:

```bash
# Check deployment status and container/server health
nextdeploy status

# Tail real-time production logs
nextdeploy logs -f
```

---

## Instant Rollback

If an unexpected bug reaches production, instantly roll back without rebuilding:

```bash
# Revert to the immediately preceding deployment
nextdeploy rollback

# Or roll back 2 versions
nextdeploy rollback --steps 2
```

Rollbacks execute in milliseconds by flipping the active deployment symlink or Worker script version pointer.

---

## Next Steps

- Explore [Core Architecture](/guide/architecture) to see how zero-downtime cutover works under the hood.
- Review the [Configuration Reference](/guide/configuration) for every available `nextdeploy.yml` setting.
- Read target-specific deployment guides:
  - [VPS Deployment & Go Sidecar](/targets/vps)
  - [Cloudflare Workers Target](/targets/cloudflare)
  - [AWS Serverless Target](/targets/aws)
