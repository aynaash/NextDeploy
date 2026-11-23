# Contributing to NextDeploy

Thanks for considering contributing to NextDeploy! This guide covers local
setup, commit and PR conventions, and the checks CI runs on every pull request.

For larger features or architectural changes, open an issue or ask on
[Discord](https://discord.gg/xd9Cub9fm) first so we can align on direction
before you invest time in a big PR.

## Prerequisites

- **Go 1.26+** (matches `go.mod`)
- **Mage:** `go install github.com/magefile/mage@latest`
- **Node.js 20+** (only needed if you edit `docs/` or JS runtime code under
  `shared/nextcompile/runtime_src/` or `tests/canary/`)

## First-Time Setup

```bash
git clone https://github.com/aynaash/NextDeploy.git
cd NextDeploy

# Install the repo's pre-commit hook (blocks binaries, secrets, and unformatted Go)
mage preCommitInstall

# Run the unit test suite
go test ./...
```

### Useful Local Commands

```bash
mage buildCLI       # Build ./bin/nextdeploy
mage buildDaemon    # Build ./bin/nextdeployd (Linux)
mage buildLambdas   # Compile embedded AWS Lambdas (imgopt, revalidator)
mage dev            # Live-reload CLI via Air
mage fmt            # Run gofmt + goimports
mage testUnit       # Run unit tests with race detector
mage docs           # Preview the VitePress docs locally
```

## Pull Request Workflow

1. **Always branch from `main`** — keep one logical change per PR. Avoid mixing
   unrelated refactors, docs rewrites, and feature code in the same PR.
2. **Use Conventional Commits** for your PR title (squash-merged PR titles feed
   directly into `release-please` for semantic versioning and `CHANGELOG.md`):
   - `feat(scope): ...` — user-facing feature (bumps minor)
   - `fix(scope): ...` — bug fix (bumps patch)
   - `perf(scope): ...` — performance improvement
   - `refactor(scope): ...` — internal restructuring without behavior change
   - `docs(scope): ...` — documentation only
   - `test(scope): ...` — tests only
   - `ci(scope): ...` / `chore(scope): ...` — CI, dependencies, maintenance
   - Add `!` after the type/scope (e.g., `feat(config)!: ...`) for breaking changes.
3. **Write tests:** New business logic should ship with unit tests; bug fixes
   should include a regression test.
4. **Follow [CODE_QUALITY.md](./CODE_QUALITY.md):** Especially the boy-scout
   rule for large files and avoiding `strings.Contains(err.Error(), ...)` or
   silent `_ =` error ignores.
5. **Never commit secrets, binaries, or local paths:** Use RFC 2606 domains
   (`example.com`) and RFC 5737 IPs (`192.0.2.0/24`, `203.0.113.0/24`) in
   examples and tests.

## What CI Checks on Every PR

- `go mod verify` and `go mod tidy` cleanliness
- `gofmt` formatting (`mage fmtCheck`)
- Cross-platform CLI builds (Linux & macOS), auxiliary Lambda builds, and Linux
  daemon build
- Unit tests (`mage testUnit`) and coverage ratchet (`mage coverRatchet`)
- `golangci-lint` on newly introduced issues (`.golangci.yml`)
- `govulncheck` vulnerability scan
- Pre-commit hook smoke test (`.githooks/pre-commit`)
- VitePress docs build (when `docs/**` is modified)

## Reporting Security Issues

Do not open a public issue for security vulnerabilities. Follow
[SECURITY.md](./SECURITY.md) instead.
