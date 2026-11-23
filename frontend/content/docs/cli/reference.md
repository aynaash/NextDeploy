---
title: CLI Command Reference
lede: Complete reference for every NextDeploy CLI command, subcommand, global flag, alias, and exit code.
status: ready
sourceRef: cli/cmd/, cli/main.go
related:
  - title: Trace & Takeover Reference
    href: /docs/cli/trace
  - title: Self-Update
    href: /docs/cli/self-update
  - title: YAML Configuration Reference
    href: /docs/config/reference
---

## Global Flags

Global flags can be passed to any NextDeploy command:

| Flag | Shorthand | Type | Description |
| :--- | :--- | :--- | :--- |
| `--config` | `-c` | string | Explicit path to configuration file (default: `./nextdeploy.yml`). |
| `--verbose` | `-v` | boolean | Enable debug logging including HTTP payloads and SSH traces. |
| `--json` | | boolean | Output structured machine-readable JSON for agents and CI/CD. |
| `--help` | `-h` | boolean | Display inline help and flag documentation. |

Every command also features an interactive architecture explainer:
```bash
nextdeploy <command> explain
# Example:
nextdeploy ship explain
nextdeploy plan explain
```

---

## Exit Codes

| Code | Meaning | Context |
| :--- | :--- | :--- |
| `0` | **Success** | Operation completed normally. |
| `1` | **Fatal Error** | Configuration error, build failure, network timeout, or health check failure. |
| `2` | **Drift Detected** | Returned by `nextdeploy plan --detailed-exitcode` when pending changes exist. |

---

## Command Reference

### `nextdeploy init`
Scaffolds a new `nextdeploy.yml` configuration by introspecting the current Next.js project.

```bash
nextdeploy init [flags]
```

#### Flags
- `--target <vps|cloudflare|aws>`: Pre-select the deployment target and bypass the interactive prompt.
- `--force`: Overwrite existing `nextdeploy.yml` without prompting.
- `--template <minimal|full>`: Select configuration verbosity (default: `minimal`).

---

### `nextdeploy plan`
Calculates a Terraform-style dry-run execution plan showing infrastructure changes, route updates, and drift without applying changes.

```bash
nextdeploy plan [flags]
```

#### Flags
- `--detailed-exitcode`: Returns exit code `2` if infrastructure changes or drift are detected. Essential for CI/CD gates.
- `--only <modules>`: Limit plan evaluation to specific resource subsets (e.g. `d1,r2,dns`).
- `--refresh`: Force re-fetching remote cloud state before calculating drift.

---

### `nextdeploy build`
Builds the Next.js project for the active target and produces `.nextdeploy/metadata.json`.

```bash
nextdeploy build [flags]
```

#### Flags
- `--force` / `-f`: Force a full re-compilation even if inputs match `.nextdeploy/build.lock`.
- `--clean`: Remove existing `.next` and build caches before building.

---

### `nextdeploy ship` (Alias: `deploy`)
The primary deployment command: runs pre-flight validation, builds, syncs static chunks, provisions resources, executes atomic cutover, and verifies health.

```bash
nextdeploy ship [flags]
```

#### Flags
- `--environment` / `-e <name>`: Deploy to an isolated named environment (e.g. `staging`, `pr-42`).
- `--verify`: Enforce strict post-deploy HTTP smoke checks; rolls back automatically on failure.
- `--force` / `-f`: Rebuild and deploy even if git tree is dirty or cache matches.
- `--no-provision`: Skip cloud resource reconciliation; deploy only application code.
- `--skip-build`: Reuse existing `.nextdeploy/metadata.json` without running `next build`.
- `--timeout` / `-t <duration>`: Overall deployment timeout (default: `10m`).
- `--verbose` / `-v`: Print streaming progress for asset uploads and Lambda steps.

---

### `nextdeploy rollback`
Reverts production traffic to the immediately preceding active release in under 1 second without rebuilding.

```bash
# Revert to last working release
nextdeploy rollback

# Revert back multiple versions
nextdeploy rollback --steps 2

# Roll back to specific release identifier
nextdeploy rollback --release rel_20261123_081904
```

---

### `nextdeploy status`
Displays live deployment health, active release ID, git provenance, custom domains, and compute status.

```bash
nextdeploy status [flags]
```

#### Flags
- `--probe`: Actively execute live HTTP GET probes against `/` and `/api/health`.
- `--json`: Output machine-readable JSON status report.

---

### `nextdeploy logs`
Streams real-time production logs from the active deployment target.

```bash
# Stream live logs (follow mode)
nextdeploy logs -f

# Print last 100 log lines
nextdeploy logs --tail 100
```

---

### `nextdeploy secrets`
Manages ChaCha20-Poly1305 encrypted environment variables and cloud secret providers.

```bash
# Set a secret
nextdeploy secrets set STRIPE_SECRET=sk_live_123

# Get secret value
nextdeploy secrets get STRIPE_SECRET

# List all declared secret keys (values masked)
nextdeploy secrets list

# Delete a secret
nextdeploy secrets delete STRIPE_SECRET

# Import from a .env file
nextdeploy secrets import --file .env.production

# Rotate local encryption key
nextdeploy secrets rotate
```

---

### `nextdeploy trace` & `nextdeploy takeover`
The native Agent Trace continuity subsystem for AI coding agents and human developers.

```bash
# Print Developer Takeover Brief
nextdeploy takeover

# Print brief AND open all touched files in $EDITOR
nextdeploy takeover -o

# Launch embedded web dashboard at http://localhost:4044
nextdeploy trace ui --port 4044

# Snapshot agent thought process and git diffs
nextdeploy trace checkpoint -m "Implement Stripe Server Actions"

# Resume session in another AI agent without losing context
nextdeploy trace resume --agent claude
```

---

### `nextdeploy target`
Inspects or switches the active deployment target in `nextdeploy.yml`.

```bash
# Display current active target
nextdeploy target

# Switch target to Cloudflare Workers
nextdeploy target switch cloudflare

# Switch target to Linux VPS
nextdeploy target switch vps

# Switch target to AWS Serverless
nextdeploy target switch aws
```

---

### `nextdeploy generate-ci`
Generates zero-touch CI/CD pipelines with runtime target autodetection.

```bash
# Generate GitHub Actions workflow (.github/workflows/nextdeploy.yml)
nextdeploy generate-ci --provider github

# Generate GitLab CI pipeline (.gitlab-ci.yml)
nextdeploy generate-ci --provider gitlab

# Include ephemeral pull request preview worker deployments
nextdeploy generate-ci --provider github --preview
```

---

### `nextdeploy prepare`
One-time automated bootstrapping for Linux VPS servers over SSH: installs Caddy, configures systemd sandbox services, enables cgroups v2, and prepares `/opt/nextdeploy/`.

```bash
nextdeploy prepare
```

---

### `nextdeploy inspect`
Inspects standalone bundle sizes, classified Next.js routes, Server Actions, and dependency weight.

```bash
nextdeploy inspect
```

---

### `nextdeploy destroy`
Tears down provisioned cloud infrastructure cleanly (workers, Lambda functions, S3 buckets, Caddy reverse proxies).

```bash
# Destroy current environment
nextdeploy destroy

# Destroy named preview stack with confirmation bypass
nextdeploy destroy --environment pr-42 --force
```

---

### `nextdeploy update`
Checks for and updates the local NextDeploy CLI binary directly from GitHub Releases.

```bash
# Check and upgrade binary
nextdeploy update

# Check for updates without installing
nextdeploy update --check
```

---

### `nextdeploy mcp`
Runs the native Model Context Protocol (MCP) server over stdio JSON-RPC 2.0 or scaffolds MCP configuration files for AI coding agents.

```bash
# Run stdio JSON-RPC server (used by Cursor, Claude Desktop, Antigravity IDE)
nextdeploy mcp

# Automatically generate configuration files
nextdeploy mcp init --client cursor
nextdeploy mcp init --client antigravity
nextdeploy mcp init --client all
```
