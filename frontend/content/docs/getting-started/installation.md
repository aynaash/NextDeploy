---
title: Installation
lede: Install the NextDeploy CLI on macOS, Linux, or Windows — or build directly from Go source.
status: ready
sourceRef: cli/main.go, docs/guide/getting-started.md
related:
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

- Credentials for your chosen target:
  - **VPS:** Linux server (Ubuntu/Debian) with SSH key access and `sudo`.
  - **Cloudflare:** `CLOUDFLARE_API_TOKEN` and `CLOUDFLARE_ACCOUNT_ID`.
  - **AWS:** `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, and `AWS_REGION`.

## Keeping Up to Date

Upgrade in place at any time:

```bash
nextdeploy update
```
