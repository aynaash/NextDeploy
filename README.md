# NextDeploy — Self-Hosted Next.js Deployment Engine

[![GitHub Release](https://img.shields.io/github/v/release/aynaash/NextDeploy?style=flat-square)](https://github.com/aynaash/NextDeploy/releases)
[![License: MIT](https://img.shields.io/badge/License-MIT-emerald.svg?style=flat-square)](./LICENSE)
[![Go Version](https://img.shields.io/badge/Go-1.26%2B-00ADD8?style=flat-square&logo=go)](./go.mod)
[![Documentation](https://img.shields.io/badge/Docs-nextdeploy.org-10b981?style=flat-square)](https://nextdeploy.org/docs)
[![Discord](https://img.shields.io/badge/Discord-Join%20Community-5865F2?style=flat-square&logo=discord&logoColor=white)](https://discord.gg/xd9Cub9fm)

**Deploy Next.js on your infrastructure. Not someone else's platform.**

NextDeploy is an open-source, self-hosted Next.js deployment engine and Vercel  and Opensst alternative built in Go. It builds, analyses, and deploys modern Next.js applications (Next.js 14, 15, and 16  App Router, Pages Router, and Turbopack) to your own **Linux VPS**, **AWS infrastructure** (Lambda, S3, CloudFront, ACM, Secrets Manager), or **Cloudflare Workers** from a single `nextdeploy.yml` configuration file and one CLI.

No managed platform subscription. No infrastructure lock-in. Your servers, your cloud accounts, your deployment pipeline.

```bash
curl -sSf https://nextdeploy.org/install.sh | sh
```

Windows: download `install.bat` from <https://nextdeploy.org/install.bat> (`curl.exe -sSfO https://nextdeploy.org/install.bat && install.bat`).

<p align="center">
  <img src="./assets/ship-demo.gif" alt="nextdeploy ship — edit a Next.js component and deploy to Cloudflare in seconds (2× speed, no edits)" width="820" />
</p>

Questions, bug reports, or just want to follow along? Join the community on [Discord](https://discord.gg/xd9Cub9fm) or explore the full docs at [nextdeploy.org/docs](https://nextdeploy.org/docs).

---

## Deployment Targets

One deployment engine. Multiple infrastructure targets — switch targets in `nextdeploy.yml` without rewriting your application.

| Target | Status | Architecture & Capabilities | Guides |
| :--- | :--- | :--- | :--- |
| **Linux VPS** | stable | Bare-metal Ubuntu/Debian with `nextdeployd`, Caddy reverse proxy, automatic HTTPS, atomic port-switching, systemd isolation, Coraza WAF, and instant rollback | [VPS Quickstart](https://nextdeploy.org/docs/getting-started/vps-quickstart) · [`docs/targets/vps.md`](./docs/targets/vps.md) |
| **AWS** | stable | Automated provisioning of AWS Lambda, S3, CloudFront, ACM SSL certificates, SQS ISR revalidation, Sharp image optimization, and AWS Secrets Manager | [AWS Quickstart](https://nextdeploy.org/docs/getting-started/aws-quickstart) · [`docs/targets/aws.md`](./docs/targets/aws.md) |
| **Cloudflare Workers** | stable | Compiled via `nextcompile` (`nextserver` engine) for Cloudflare Workers, R2 static assets, KV, D1, Hyperdrive, Queues, and Durable Objects (Next.js 14/15/16 + Turbopack) | [Cloudflare Quickstart](https://nextdeploy.org/docs/getting-started/cloudflare-quickstart) · [`docs/targets/cloudflare.md`](./docs/targets/cloudflare.md) |

---

## Build Intelligence (`metadata.json`)

Current Nextjs deployment tools only move files from one environment to another. NextDeploy inspects your Next.js build output on every `nextdeploy build` and generates a structured `metadata.json` so the engine understands your application before shipping:

- **Route classification**: Automatically identifies static, SSR, ISR, API, and middleware routes from Next.js build manifests.
- **Integration detection**: Inspects generated build output to identify supported third-party integrations and external origins.
- **Secret validation**: Cross-references detected integrations with configured secrets and warns about missing configuration before deployment.
- **ISR awareness**: Identifies ISR routes and their revalidation intervals for targeted cache management.
- **Deployment provenance**: Records Git commit information, dirty working-tree state, Next.js version, and build timestamp.
- **Build fingerprinting**: Records package manager, application entrypoint, output mode, and asset inventory to determine how the application should be deployed.

---

## Production Features Without Platform Lock-In

- **Atomic zero-downtime deployments**: Dual-stage TCP + HTTP health probing with atomic port-switching and POSIX symlink renames (`current.tmp` ➔ `current`) before traffic moves to the new release.
- **Instant rollback**: Keep previous releases available and restore a working deployment in seconds via `nextdeploy rollback`.
- **Infrastructure security**: Hardened `systemd` services (`DynamicUser=yes`, `ProtectSystem=strict`, `PrivateTmp=yes`), ChaCha20-Poly1305 encrypted secrets, HMAC-SHA256 command signing, and integrated Coraza WAF (OWASP CRS) protection.
- **Automated provisioning**: Prepare a Linux VPS (`nextdeploy prepare`) or provision supported AWS and Cloudflare resources through the deployment workflow.
- **Deployment auditability**: Structured audit logs tracking deployments, system changes, Git provenance, and application releases.
- **Incremental builds**: Avoid unnecessary rebuild work when your application has not changed.
- **Cumulative static chunk pooling (`shared_static`)**: Eliminates Next.js `ChunkLoadError` 404s for active user sessions across zero-downtime updates.
- **Native Agent Trace & MCP server**: Track AI coding agent mental models, prompts, and diffs (`nextdeploy trace ui` on `http://localhost:4044`), run `nextdeploy takeover -o`, or connect Cursor, Claude Code, and Antigravity via `nextdeploy mcp`.
- **Zero-touch CI/CD generation**: Generate GitHub Actions or GitLab CI workflows with `nextdeploy generate-ci` including PR drift detection (`nextdeploy plan --detailed-exitcode`) and ephemeral Cloudflare preview environments.

---

## How NextDeploy Works

1. **01. Analyse** — Run `nextdeploy build` to inspect your Next.js output, classify routes, identify integrations, fingerprint assets, and generate deployment metadata.
2. **02. Validate** — Run `nextdeploy plan` to check deployment requirements, configuration, secrets, build constraints, and application metadata before shipping.
3. **03. Deploy** — Run `nextdeploy ship` to build when necessary, package your application, and deploy it to your selected infrastructure target.
4. **04. Verify** — Health checks, deployment status (`nextdeploy status`), release tracking, and instant rollback (`nextdeploy rollback`) keep production deployments reliable.

### CLI Quick Start

```bash
# Install NextDeploy
curl -sSf https://nextdeploy.org/install.sh | sh

# Initialize your project (creates nextdeploy.yml)
nextdeploy init

# Prepare your VPS (VPS deployments only)
nextdeploy prepare

# Preview planned infrastructure and release changes
nextdeploy plan

# Deploy your application
nextdeploy ship

# Check deployment status & stream logs
nextdeploy status
nextdeploy logs -f

# Agent Trace & Human Takeover
nextdeploy trace ui     # launch local trace dashboard at http://localhost:4044
nextdeploy takeover -o  # open agent handoff brief in your $EDITOR
```

`nextdeploy.yml` holds everything — target type, domain, server, secrets provider, and bindings. See [`sample.nextdeploy.yml`](./sample.nextdeploy.yml) for the full annotated schema.

---

## Who NextDeploy Is For

- Developers self-hosting Next.js applications on their own infrastructure.
- Teams deploying Next.js to bare-metal Linux VPS instances (Hetzner, DigitalOcean, Linode, bare metal).
- Engineers building serverless and edge Next.js apps on AWS and Cloudflare Workers.
- Businesses seeking predictable cloud costs and full infrastructure ownership.
- Developers looking for an open-source, self-hosted alternative to managed deployment platforms.

---

## Build from source

```bash
git clone https://github.com/aynaash/NextDeploy
cd NextDeploy
go build -o nextdeploy ./cli
```

### Live reloading with Air

For rapid local iteration, run `mage dev` (or `air -c .air.cli.toml`) to automatically recompile `nextdeploy` to `~/.nextdeploy/bin/nextdeploy` on every code change. See [`docs/local-development-air.md`](./docs/local-development-air.md).

Requires Go 1.26+. To embed the auxiliary AWS Lambdas (`imgopt`, `revalidator`) in a local build, run `./build-lambdas.sh` (or `mage buildLambdas`) before `go build`. Official release binaries on GitHub compile them automatically via GoReleaser; see [`.goreleaser.yml`](./.goreleaser.yml).

## Repository layout

```
cli/                       Cobra CLI entry, all top-level commands
daemon/                    nextdeployd — the agent that runs on each VPS
shared/nextcompile/        Build-time compiler + JS runtime for CF Workers
shared/nextcore/           Next.js project introspection (config, routes, deps)
cli/internal/serverless/   AWS & Cloudflare deployment adapters
frontend/                  Official NextDeploy website & documentation (nextdeploy.org)
sample.nextdeploy.yml      Annotated reference config
```

## Documentation

- **Full Documentation Site**: <https://nextdeploy.org/docs>
- **Getting Started**: <https://nextdeploy.org/docs/getting-started/installation>
- **Build Metadata & Intelligence**: <https://nextdeploy.org/docs/concepts/metadata>
- **Security Model**: <https://nextdeploy.org/docs/concepts/security>
- **Docs Source**: Maintained in [`docs/`](./docs/) and [`frontend/content/docs/`](./frontend/content/docs/). Run `mage docs` (or `cd docs && npm run docs:dev`) to preview VitePress docs locally.
- **VPS Deployment & Go Sidecar**: [`docs/targets/vps.md`](./docs/targets/vps.md)
- **Cloudflare Workers Target**: [`docs/targets/cloudflare.md`](./docs/targets/cloudflare.md)
- **AWS Serverless Target**: [`docs/targets/aws.md`](./docs/targets/aws.md)
- **Multi-Server Stress Testing**: [`docs/guide/multiserver-stress-testing.md`](./docs/guide/multiserver-stress-testing.md)  Local Linux cluster emulation with NGINX load balancing, heavy
SSR, and zero-downtime rolling deploys in progress
- **Sample Configuration**: [`sample.nextdeploy.yml`](./sample.nextdeploy.yml)
- **Inline Help**: Each CLI command includes `nextdeploy <cmd> explain` for interactive architecture walk-throughs.

## Contributing

Issues and PRs always welcomed. Run `go test ./...` before pushing , the release pipeline is gated on a green test run. For larger changes, open an issue first so we can align on direction. The [Discord](https://discord.gg/xd9Cub9fm) is the fastest place to get a yes/no on direction before you spend time on a PR.

## License

MIT — see [LICENSE](./LICENSE).
