---
title: Local Development & Air
lede: "Rapid local iteration when contributing to the NextDeploy Go CLI, daemon, and JS runtimes using Mage tasks, Air live-reloading, and standalone test harness mocks."
status: ready
sourceRef: magefile.go, .air.cli.toml, .air.daemon.toml, docs/local-development-air.md
related:
  - title: NextCompile (Cloudflare Runtime)
    href: /docs/advanced/nextcompile
  - title: CLI Reference
    href: /docs/cli/reference
  - title: Contributing
    href: /docs/contributing
---

## Monorepo Architecture Overview

The NextDeploy repository is organized into distinct Go systems packages, JavaScript runtime shims, and Next.js frontend documentation:

```text
NextDeploy/
├── cli/                   # NextDeploy CLI entrypoint & subcommands (Cobra)
│   ├── cmd/               # plan, ship, trace, takeover, status, rollback
│   └── internal/          # Serverless, VPS, and DNS orchestrators
├── daemon/                # nextdeployd Linux systems daemon
│   └── cmd/nextdeployd/   # Standalone Go daemon binary for Linux VPS
├── shared/                # Core shared Go packages
│   ├── agenttrace/        # Native Agent Trace engine & Web UI (:4044)
│   ├── config/            # nextdeploy.yml parser & validator
│   ├── nextcompile/       # Cloudflare NextServer bundler & runtime JS shims
│   └── secrets/           # AES-256-GCM encryption & Doppler bridge
├── frontend/              # Next.js 16 (Turbopack) documentation & web app
└── magefile.go            # Cross-platform Mage build script
```

---

## Prerequisites

- **Go 1.25+**
- **Node.js 20+** or **Bun 1.2+**
- **Mage:** `brew install mage` or `go install github.com/magefile/mage@latest`
- **Air:** `go install github.com/air-verse/air@latest` (auto-installed on first `mage dev`)

---

## Live-Reloading the CLI (`.air.cli.toml`)

When developing new CLI subcommands or fixing bugs, Air watches all Go files and recompiles the binary instantly on every file save:

```bash
mage dev
# or directly:
air -c .air.cli.toml
```

### Path Configuration
Air outputs the compiled development binary to `~/.nextdeploy/bin/nextdeploy`. Add this directory to your shell configuration (`~/.zshrc` or `~/.bashrc`):

```bash
export PATH="$HOME/.nextdeploy/bin:$PATH"
```

Now, every time you save a Go source file in `cli/` or `shared/`, running `nextdeploy` in any other terminal tab will immediately execute your freshly compiled code.

### Sample `.air.cli.toml` Configuration

```toml
root = "."
tmp_dir = "tmp"

[build]
  bin = "./tmp/nextdeploy"
  cmd = "go build -o ~/.nextdeploy/bin/nextdeploy ./cli"
  delay = 500
  exclude_dir = ["assets", "tmp", "vendor", "testdata", "frontend", ".nextdeploy", "docs"]
  include_ext = ["go", "tpl", "tmpl", "html"]
  exclude_regex = ["_test.go"]
  stop_on_error = true

[log]
  time = true
```

---

## Live-Reloading the VPS Daemon (`.air.daemon.toml`)

To develop and test the Linux systems daemon (`nextdeployd`):

```bash
mage devDaemon
# or:
air -c .air.daemon.toml
```

Air compiles `nextdeployd` to `~/.nextdeploy/bin/nextdeployd` and launches it locally, allowing you to test socket communication, port arbitration, and release directory management on your development machine.

---

## Running Test Suites

NextDeploy includes comprehensive Go unit tests, JS runtime conformance tests, and real standalone build smoke tests:

### 1. Go Unit Tests
```bash
# Run all unit tests across the entire monorepo
mage test

# Run tests in a specific package
go test -v ./shared/agenttrace/...
go test -v ./shared/config/...
go test -v ./cli/cmd/...
```

### 2. NextCompile & NextServer Runtime Tests
Verify that NextDeploy's Cloudflare runtime shims accurately simulate Node.js standard libraries (`http`, `crypto`, `async_hooks`):

```bash
node --test shared/nextcompile/runtime_src/*.test.mjs shared/nextcompile/runtime_src/nextserver/*.test.mjs
```

### 3. Real Standalone Build Smoke Test
Compile and execute an actual Next.js 15 or 16 standalone build through NextDeploy's `nextserver` engine locally without deploying to Cloudflare:

```bash
NEXTDEPLOY_TEST_STANDALONE=/path/to/my-nextjs-app/.next/standalone \
  go test -v ./cli/internal/serverless -run TestBuildWorkerBundle_RealStandaloneSmoke
```

---

## Developing the Documentation Frontend (`frontend/`)

The documentation web application in `./frontend` is built with Next.js 16 and Turbopack:

```bash
cd frontend
bun install
bun run dev
```

Open `http://localhost:3000` in your browser. All Markdown and MDX documents in `frontend/content/docs/` hot-reload instantly.

To verify a production build before submitting changes:
```bash
bun run build
```

---

## Common Mage Tasks Summary

| Command | Action |
| :--- | :--- |
| `mage build` | Compiles both `nextdeploy` and `nextdeployd` into `./bin/` |
| `mage check` | Runs formatting, linting (`golangci-lint`), and tests |
| `mage dev` | Launches Air live reloader for the CLI |
| `mage devDaemon` | Launches Air live reloader for the VPS daemon |
| `mage lint` | Executes `golangci-lint run ./...` |
| `mage clean` | Cleans temporary build artifacts and test caches |
