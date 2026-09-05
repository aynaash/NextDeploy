# NextDeploy

Point it at a Next.js app. It works out what Cloudflare needs to run it,
provisions it, and ships it.

`wrangler` ships the Worker. Terraform manages the resources. Neither knows
it's a Next.js app — NextDeploy does all three.

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
nextdeploy init     # scaffold nextdeploy.yml in your Next.js repo
nextdeploy plan     # show what will happen, change nothing
nextdeploy ship     # build + push + deploy to your target
nextdeploy logs -f  # tail production logs
```

`nextdeploy.yml` holds everything — target type, domain, server, secrets
provider. See `sample.nextdeploy.yml` in this repo for the full schema.

## What ships today

Workers + R2 for the app, and desired-state reconciliation for the resources
around it — KV, D1, Hyperdrive, Queues, Vectorize, AI Gateway, DNS and zone
settings — with `plan` before `apply`, orphan detection, and a teardown that
reports what survived.

Read [`CLOUDFLARE_PARITY.md`](./CLOUDFLARE_PARITY.md) before you ship a
full-stack App Router app. It is the contract for what the Worker runtime
does and does not cover, and it is kept honest on purpose.

**VPS** (Caddy + the `nextdeployd` daemon over SSH) still works and is still
supported for bugs — see [`docs/VPS_DEPLOY_FLOW.md`](./docs/VPS_DEPLOY_FLOW.md).
It is the escape hatch for the quarter after a Next.js release that Workers
can't run yet. It is not where new work goes.

**AWS** was removed in favour of depth on one target. The last release with
Lambda/CloudFront support is v0.15.1.

## Build from source

```bash
git clone https://github.com/aynaash/NextDeploy
cd NextDeploy
go build -o nextdeploy ./cli
```

Requires Go 1.25+. The release binaries on GitHub are built with
GoReleaser; see `.goreleaser.yml`.

## Repository layout

```
cli/                       Cobra CLI entry, all top-level commands
daemon/                    nextdeployd — the agent that runs on each VPS
shared/nextcompile/        Build-time compiler + JS runtime for CF Workers
shared/nextcore/           Next.js project introspection (config, routes, deps)
cli/internal/serverless/   Cloudflare adapter + the plan/apply resource layer
sample.nextdeploy.yml      Annotated reference config
```

## Documentation

- Full docs: <https://nextdeploy.org/docs>
- Sample config: [`sample.nextdeploy.yml`](./sample.nextdeploy.yml)
- Each command also has `nextdeploy <cmd> explain` for inline help

## Contributing

Issues and PRs welcome. Run `go test ./...` before pushing — the release
pipeline is gated on a green test run. For larger changes, open an issue
first so we can align on direction. The
[Discord](https://discord.gg/xd9Cub9fm) is the fastest place to get a
yes/no on direction before you spend time on a PR.

## License

MIT — see [LICENSE](./LICENSE).
