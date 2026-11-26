---
title: Installation
lede: Install the NextDeploy CLI on macOS, Linux, or Windows — or build directly from Go source.
status: ready
sourceRef: cli/main.go, docs/guide/getting-started.md
related:
  - title: Requirements
    href: /docs/getting-started/requirements
  - title: VPS Quickstart
    href: /docs/getting-started/vps-quickstart
  - title: Cloudflare Quickstart
    href: /docs/getting-started/cloudflare-quickstart
  - title: AWS Quickstart
    href: /docs/getting-started/aws-quickstart
---

## macOS & Linux

Install the pre-compiled binary with the official install script:

```bash
curl -fsSL https://nextdeploy.org/install.sh | bash
```

This downloads the release binary for your OS/architecture, installs `nextdeploy` into `/usr/local/bin` (or `~/.local/bin`), and verifies permissions.

## Windows

Run the PowerShell installer:

```powershell
irm https://nextdeploy.org/install.bat | iex
```

Or download `install.bat` directly from [https://nextdeploy.org/install.bat](https://nextdeploy.org/install.bat).

## Build from Source

NextDeploy is written in Go (requires Go 1.25+):

```bash
git clone https://github.com/aynaash/NextDeploy.git
cd NextDeploy
go build -o nextdeploy ./cli
sudo mv nextdeploy /usr/local/bin/
```

## Verify Installation

```bash
nextdeploy version
```

## Prerequisites for Your Next.js App

- A Next.js 14, 15, or 16 app that succeeds on `next build` (Turbopack or Webpack).
- Enable standalone output in `next.config.mjs` (NextDeploy also warns or patches if missing):

```js
/** @type {import('next').NextConfig} */
const nextConfig = {
  output: 'standalone',
}

export default nextConfig
```

- Target infrastructure & credentials (see the full [Requirements Guide](/docs/getting-started/requirements)):
  - **VPS (Full-Stack Apps):** An Ubuntu/Debian Linux VPS (ports `22`, `80`, `443` open), a **non-root user with `sudo`** (`nextdeploy prepare` refuses `root` by default), your **SSH private key path** (`key_path: ~/.ssh/id_ed25519`), a **domain name** with an `A` record pointing to the VPS IP, and **secrets sync** via **Doppler (recommended)** or `.env` (`nextdeploy secrets load`).
  - **Cloudflare:** `CLOUDFLARE_API_TOKEN`, `CLOUDFLARE_ACCOUNT_ID`, a domain on Cloudflare DNS, and secrets sync via Doppler or `.env`.
  - **AWS:** `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_REGION`, registrar DNS access for ACM + CloudFront, and secrets sync via Doppler or `.env`.

## Keeping Up to Date

Upgrade in place at any time:

```bash
nextdeploy update
```
