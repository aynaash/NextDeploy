# Contributing to NextDeploy

Thank you for your interest in contributing to NextDeploy! This guide explains how to set up your local development environment, build the CLI, test the daemon, and contribute to both the Go backend and Next.js frontend.

---

## 1. Prerequisites

- **Go**: 1.25+ installed with `$GOPATH/bin` in your `$PATH`.
- **Node.js & Package Manager**: Node 20+ and [Bun](https://bun.sh/) (preferred for `frontend/`) or `npm`.
- **Git**: For version control and commit verification.
- **Mage** (optional, recommended): Run `go install github.com/magefile/mage@latest`.
- **Air** (optional, for live reload): Run `go install github.com/air-verse/air@latest`.

---

## 2. Repository Layout

NextDeploy is structured as a monorepo containing:
- `cli/`: The `nextdeploy` command-line application (Go + Cobra).
- `daemon/`: The `nextdeployd` systemd daemon for VPS targets (Go).
- `shared/`: Shared domain logic (`nextcore`, `nextcompile`, `agenttrace`, `secrets`, `config`).
- `frontend/`: Official Next.js 16 (Turbopack) web application and documentation (`nextdeploy.org`).
- `docs/`: VitePress documentation source.

---

## 3. Local Development Workflows

### A. Working on the Go CLI

#### Option 1: Standard Go Build
```bash
# Build nextdeploy binary locally
go build -o nextdeploy ./cli

# Verify version and help
./nextdeploy version
./nextdeploy --help
```

#### Option 2: Live Reloading with Air & Mage
Run Air to automatically recompile `nextdeploy` to `~/.nextdeploy/bin/nextdeploy` whenever Go files change:
```bash
# Using Mage
mage dev

# Or directly with Air
air -c .air.cli.toml
```

---

### B. Working on the Frontend Web App (`frontend/`)

```bash
cd frontend

# Install dependencies with Bun
bun install

# Start local Next.js dev server on port 3000
bun run dev

# Run production build and type checking
bun run build
```

---

### C. Working on the Embedded Agent Trace Dashboard

Agent Trace serves a local dashboard on port `4044`:
```bash
# Test the web dashboard locally
go run ./cli trace ui --port 4044
# Open http://localhost:4044 in your browser
```

---

## 4. Running Tests & Quality Verification

Before submitting a Pull Request, make sure all test suites pass:

### 1. Go Unit Tests
```bash
# Run all Go package tests
go test -v ./shared/agenttrace/...
go test -v ./shared/config/...
go test -v ./shared/nextcompile/...
go test -v ./cli/cmd/...
```

### 2. Frontend Production Build
```bash
cd frontend
bun run build
```

### 3. VitePress Documentation Build
```bash
cd docs
npm run docs:build
```

---

## 5. Code Quality Guidelines

We follow strict engineering principles documented in [CODE_QUALITY.md](./CODE_QUALITY.md):
- **Small, cohesive packages**: Avoid bloated files; aim for ≤ 300 LOC per file.
- **Errors are values**: Wrap errors with context (`fmt.Errorf("doing X: %w", err)`).
- **Interface at the consumer**: Define minimal consumer-facing interfaces.
- **Boy-scout rule**: Leave any file or package cleaner than you found it.

---

## 6. Pull Request Checklist

- [ ] Code follows formatting standards (`gofmt -s -w .` and `bun run lint`).
- [ ] New features include corresponding unit tests.
- [ ] Relevant documentation has been updated in `frontend/content/docs/` or `docs/`.
- [ ] Commit messages follow [Conventional Commits](https://www.conventionalcommits.org/) (e.g. `feat: add agent trace resume command`, `fix: handle missing secret warning`).
