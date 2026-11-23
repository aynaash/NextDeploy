---
layout: home

hero:
  name: "NextDeploy"
  text: "Zero-Downtime Next.js Deployments"
  tagline: "Deploy Next.js apps to a $4 VPS, AWS Lambda, or Cloudflare Workers from one config file and one command. Open source, self-hosted, no vendor lock-in."
  image:
    src: /logo.svg
    alt: NextDeploy Logo
  actions:
    - theme: brand
      text: Get Started
      link: /guide/getting-started
    - theme: alt
      text: Agent Trace (AI Agents)
      link: /guide/agent-first-infra
    - theme: alt
      text: Why NextDeploy?
      link: /guide/architecture
    - theme: alt
      text: View on GitHub
      link: https://github.com/aynaash/NextDeploy

features:
  - title: Native Agent Trace & Takeover
    details: Provider-independent continuity for AI coding agents (Antigravity, Claude Code, Cursor). Local web dashboard (:4044) and instant developer takeover (`nextdeploy takeover -o`).
    icon: 🤖
  - title: True Zero-Downtime Cutover
    details: Dual-stage TCP + HTTP health probing with atomic POSIX symlink renames (current.tmp ➔ current). Zero dropped connections during deploys.
    icon: ⚡
  - title: Cumulative Static Chunk Pooling
    details: Eliminates Next.js ChunkLoadError 404s permanently by preserving older content-hashed JS bundles across deployments in shared_static.
    icon: 📦
  - title: Bare-Metal Speed
    details: No Docker daemon overhead on VPS. Uses Linux kernel cgroup v2 memory sandboxing, systemd supervision, and Caddy direct NVMe byte offloading.
    icon: 🚀
  - title: Multi-Target Portability
    details: One configuration schema for Linux VPS, Cloudflare Workers (via nextcompile), or AWS Lambda + S3 + CloudFront. Switch targets seamlessly.
    icon: 🌐
  - title: Built-in Go Sidecar
    details: Optional lightweight Go sidecar serving prerendered HTML and performing on-the-fly image optimization without touching Node.js.
    icon: 🏎️
  - title: Terraform-Style Planning & Rollbacks
    details: Preflight drift detection with `nextdeploy plan` and instant one-command rollbacks with `nextdeploy rollback`.
    icon: 🛡️
---

## Quick Install

::: code-group

```bash [macOS / Linux]
curl -fsSL https://nextdeploy.org/install.sh | bash
```

```powershell [Windows]
# Download and execute install.bat
irm https://nextdeploy.org/install.bat | iex
```

```bash [Build from Source]
git clone https://github.com/aynaash/NextDeploy.git
cd NextDeploy
go build -o nextdeploy ./cli
```

:::

## Deploy in 4 Simple Steps

```bash
# 1. Initialize configuration in your Next.js project
nextdeploy init

# 2. Dry-run preflight check (changes nothing)
nextdeploy plan

# 3. Build, push, and cutover with zero downtime
nextdeploy ship

# 4. Stream real-time production logs
nextdeploy logs
```

## The Era of AI Coding Agents: Agent Trace & Takeover

Your coding agent should be replaceable. Your engineering context and production deployments shouldn't be. NextDeploy includes **Agent Trace**:

```bash
# Human developer takeover: scan 20-line brief and open touched files in $EDITOR
nextdeploy takeover -o

# Launch the interactive local web dashboard at http://localhost:4044
nextdeploy trace ui

# Transfer unfinished work seamlessly to another agent
nextdeploy trace resume --agent claude
```

[Read the Agent Trace Guide →](/guide/agent-first-infra)

## Demo

<p align="center">
  <img src="https://raw.githubusercontent.com/aynaash/NextDeploy/main/assets/ship-demo.gif" alt="nextdeploy ship demo" style="border-radius: 8px; box-shadow: 0 4px 20px rgba(0,0,0,0.15); max-width: 100%;" />
</p>

## Supported Targets

| Target | Status | Engine | Key Capabilities |
| :--- | :--- | :--- | :--- |
| **VPS (Self-Hosted)** | **Stable** | Caddy + `nextdeployd` + Systemd | Atomic POSIX cutover, cgroup v2 limits, Cumulative chunk pooling, Go sidecar |
| **Cloudflare Workers** | **Beta** | `nextcompile` + R2 + KV | Single-bundle compilation, Server Actions, SSR, R2 static assets, D1/Queues/Hyperdrive |
| **AWS Serverless** | **Stable** | Lambda + S3 + CloudFront | Automated ACM certificates, S3 static assets, CloudFront distribution, SQS integration |

::: tip Join the Community
Questions, bug reports, or feature requests? Join developers building with NextDeploy on [Discord](https://discord.gg/xd9Cub9fm).
:::
