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
nextdeploy init     # scaffold nextdeploy.yml in your Next.js repo
nextdeploy plan     # show what will happen, change nothing
nextdeploy ship     # build + push + deploy to your target
nextdeploy logs -f  # tail production logs
```

`nextdeploy.yml` holds everything — target type, domain, server, secrets
provider. See `sample.nextdeploy.yml` in this repo for the full schema.

## Targets

| Target          | Status | What it gives you                                  |
| --------------- | ------ | -------------------------------------------------- |
| **VPS**         | stable | Caddy + the `nextdeployd` daemon over SSH          |
| **AWS**         | stable | Lambda + S3 + CloudFront, full SSR/ISR/image opt   |
| **Cloudflare**  | beta   | Workers + R2 + KV via the `nextcompile` adapter    |

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
cli/internal/serverless/   AWS adapter (Lambda, S3, CloudFront, ACM)
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
