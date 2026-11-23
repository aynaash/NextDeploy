# NextDeploy

Deploy Next.js apps to a VPS, AWS Lambda, or Cloudflare Workers from one
config file and one command. Open source, self-hosted, no vendor lock-in.

```bash
curl -fsSL https://nextdeploy.org/install.sh | bash
```

Windows: download `install.bat` from <https://nextdeploy.org/install.bat>.

<p align="center">
  <img src="./assets/ship-demo.gif" alt="nextdeploy ship — edit a Next.js component and deploy to Cloudflare in seconds (2× speed, no edits)" width="820" />
</p>

Questions, bug reports, or just want to follow along? Join the community
on [Discord](https://discord.gg/xd9Cub9fm).

## Quick start

```bash
nextdeploy init         # scaffold nextdeploy.yml in your Next.js repo
nextdeploy plan         # show what will happen, change nothing
nextdeploy ship         # build + push + deploy to your target
nextdeploy takeover -o  # human takeover from AI coding agents in your $EDITOR
nextdeploy trace ui     # launch local trace dashboard at http://localhost:4044
nextdeploy logs -f      # tail production logs
```

`nextdeploy.yml` holds everything — target type, domain, server, secrets
provider. See `sample.nextdeploy.yml` in this repo for the full schema.

## Key Architecture & Features

- **True Zero-Downtime Atomic Cutover**: Dual-stage TCP + HTTP health probing with atomic POSIX symlink renames (`current.tmp` ➔ `current`).
- **Cumulative Static Chunk Pooling (`shared_static`)**: Eliminates the Next.js `ChunkLoadError` 404s for active user sessions across updates.
- **Bare-Metal Speed (Go + Caddy + Systemd)**: No Docker daemon overhead. Uses Linux kernel `cgroup v2` memory sandboxing and offloads all static bytes directly to NVMe disk via Caddy.
- **Multi-Target Portability**: Deploy to a $4 VPS, AWS Lambda, or Cloudflare Workers from a single config file.
- **Native Agent Trace & Human Takeover**: Track the mental model, prompts, and diffs of AI coding agents (Antigravity, Claude Code, Cursor) with an embedded local dashboard (`nextdeploy trace ui` on `:4044`) and 1-command developer takeover (`nextdeploy takeover -o`).
- **Zero-Touch CI/CD Automation**: Production-ready GitHub Actions workflows for VPS (hardened SSH, dual-key paths, host-key pinning) and Cloudflare (ephemeral PR previews, remote state encryption, and dry-run IaC planning via `nextdeploy plan --detailed-exitcode`).

## Targets

| Target          | Status | What it gives you                                  |
| --------------- | ------ | -------------------------------------------------- |
| **VPS**         | stable | Caddy + the `nextdeployd` daemon over SSH          |
| **AWS**         | stable | Lambda + S3 + CloudFront, full SSR/ISR/image opt   |
| **Cloudflare**  | stable | Workers + R2 + KV + D1 + Hyperdrive + Queues via `nextcompile` (`nextserver` engine, Next.js 14/15/16 + Turbopack) |

## Build from source

```bash
git clone https://github.com/aynaash/NextDeploy
cd NextDeploy
go build -o nextdeploy ./cli
```

### Live reloading with Air
For rapid local iteration, run `mage dev` (or `air -c .air.cli.toml`) to automatically recompile `nextdeploy` to `~/.nextdeploy/bin/nextdeploy` on every code change. See [`docs/local-development-air.md`](./docs/local-development-air.md).

Requires Go 1.26+. To embed the auxiliary AWS Lambdas (`imgopt`, `revalidator`)
in a local build, run `./build-lambdas.sh` (or `mage buildLambdas`) before
`go build`. Official release binaries on GitHub compile them automatically via
GoReleaser; see `.goreleaser.yml`.

## Repository layout

```
cli/                       Cobra CLI entry, all top-level commands
daemon/                    nextdeployd — the agent that runs on each VPS
shared/nextcompile/        Build-time compiler + JS runtime for CF Workers
shared/nextcore/           Next.js project introspection (config, routes, deps)
cli/internal/serverless/   AWS adapter (Lambda, S3, CloudFront, ACM)
sample.nextdeploy.yml      Annotated reference config
```

## Documentation

- **Full Documentation Site**: <https://nextdeploy.org/docs>
- **Docs Source**: Maintained in [`docs/`](./docs/). Run `mage docs` (or `cd docs && npm run docs:dev`) to run locally with hot-reloading.
- **VPS Deployment & Go Sidecar**: [`docs/targets/vps.md`](./docs/targets/vps.md)
- **Cloudflare Workers Target**: [`docs/targets/cloudflare.md`](./docs/targets/cloudflare.md)
- **AWS Serverless Target**: [`docs/targets/aws.md`](./docs/targets/aws.md)
- **Multi-Server Stress Testing**: [`docs/guide/multiserver-stress-testing.md`](./docs/guide/multiserver-stress-testing.md) — Local Linux cluster emulation with NGINX load balancing, heavy SSR, and zero-downtime rolling deploys.
- **Sample Configuration**: [`sample.nextdeploy.yml`](./sample.nextdeploy.yml)
- **Inline Help**: Each CLI command includes `nextdeploy <cmd> explain` for interactive architecture walk-throughs.

## Contributing

Issues and PRs welcome. Run `go test ./...` before pushing — the release
pipeline is gated on a green test run. For larger changes, open an issue
first so we can align on direction. The
[Discord](https://discord.gg/xd9Cub9fm) is the fastest place to get a
yes/no on direction before you spend time on a PR.

## License

MIT — see [LICENSE](./LICENSE).
