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
- `--target` / `-t <vps|cloudflare|aws>`: Pre-select the deployment target (`vps`, `cloudflare`, `aws`).
- `--platform` / `-p <name>`: Framework type (`nextjs`, `remix`, `astro`, `sveltekit`).
- `--mode` / `-m <name>`: Runtime mode (`serverless`, `container`, `standalone`).
- `--db` / `-d <name>`: Database template (`postgres`, `mysql`, `sqlite`, `mongodb`, `none`).
- `--force` / `-f`: Overwrite existing configuration file without confirmation.

---

### `nextdeploy plan`
Calculates a Terraform-style dry-run execution plan showing infrastructure changes, route updates, and drift without applying changes.

```bash
nextdeploy plan [flags]
```

#### Flags
- `--detailed-exitcode`: Returns exit code `2` if infrastructure changes or drift are detected. Essential for CI/CD gates.
- `--only <modules>`: Limit plan evaluation to specific resource subsets (e.g. `d1,r2,dns`).

---

### `nextdeploy build`
Builds the Next.js project for the active target and produces `.nextdeploy/metadata.json`.

```bash
nextdeploy build [flags]
```

#### Flags
- `--force` / `-f`: Force a full re-compilation even if inputs match `.nextdeploy/build.lock`.

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
- `--verbose` / `-v`: Print streaming progress for asset uploads and deployment steps.

---

### `nextdeploy rollback`
Reverts production traffic to the immediately preceding active release in under 1 second without rebuilding.

```bash
# Revert to last working release
nextdeploy rollback

# Revert back multiple versions
nextdeploy rollback --steps 2

# Roll back to a specific commit or release hash
nextdeploy rollback --to a1b2c3d
```

#### Flags
- `--steps` / `-s <int>`: Number of releases to roll back (default: 1).
- `--to <string>`: Specific release commit or version hash to target.

---

### `nextdeploy status`
Displays live deployment health, active release ID, git provenance, custom domains, and compute status.

```bash
nextdeploy status
```

---

### `nextdeploy logs`
Streams real-time production logs from the active deployment target (live streaming is default).

```bash
# Stream live logs
nextdeploy logs

# Filter by route path
nextdeploy logs --route /api/webhooks

# Stream security and daemon audit events
nextdeploy logs --audit

# Stream nextdeployd daemon internal logs
nextdeploy logs --daemon

# Stream all channels simultaneously
nextdeploy logs --all
```

#### Flags
- `--route <path>`: Filter logs to requests matching a specific route pattern.
- `--audit`: Stream security and deployment audit events.
- `--daemon`: Stream `nextdeployd` system service logs.
- `--all`: Stream all log channels simultaneously.

---

### `nextdeploy secrets`
Manages ChaCha20-Poly1305 encrypted environment variables and remote secret synchronization.

```bash
# Set a secret
nextdeploy secrets set STRIPE_SECRET=sk_live_123

# Get secret value
nextdeploy secrets get STRIPE_SECRET

# List all declared secret keys (values masked)
nextdeploy secrets list

# Remove a secret key
nextdeploy secrets unset STRIPE_SECRET

# Load secrets from a local .env file
nextdeploy secrets load .env.production

# Re-encrypt and sync declared secrets with remote daemon/target
nextdeploy secrets sync

# Prune undeclared remote secrets
nextdeploy secrets prune
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
Displays the active deployment target configured in `nextdeploy.yml`.

```bash
# Display current active target
nextdeploy target

# Output active target as JSON
nextdeploy target --json
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
nextdeploy prepare [flags]
```

#### Flags
- `--server` / `-s <name>`: Target specific server defined in `nextdeploy.yml`.
- `--all`: Prepare all configured servers sequentially.
- `--allow-root`: Allow running setup under root account.
- `--skip-runtimes`: Skip Node.js/Bun runtime installation.
- `--skip-hardening`: Skip UFW firewall and SSH hardening rules.
- `--stream`: Stream raw remote SSH output directly to terminal.
- `--verbose` / `-v`: Enable detailed diagnostic output.

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
