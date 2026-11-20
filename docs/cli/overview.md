# CLI Reference Overview

NextDeploy provides a developer-friendly command-line interface built with Cobra.

Every command includes built-in deep explanations accessible via:
```bash
nextdeploy <command> explain
```

---

## Global Flags

The following flags can be passed to any `nextdeploy` command:

| Flag | Shorthand | Type | Description |
| :--- | :--- | :--- | :--- |
| `--config` | `-c` | string | Path to configuration file (default: `./nextdeploy.yml`). |
| `--verbose` | `-v` | boolean | Enable verbose debug output. |
| `--json` | | boolean | Output results in machine-readable JSON format. |
| `--help` | `-h` | boolean | Display help information for the command. |

---

## Command Summary

| Command | Alias | Description |
| :--- | :--- | :--- |
| [`nextdeploy init`](/cli/init) | | Interactively initialize `nextdeploy.yml` in a project. |
| [`nextdeploy plan`](/cli/plan) | | Dry-run preflight: detect drift and preview changes without modifying resources. |
| [`nextdeploy build`](/cli/build) | | Build the Next.js app and run NextCore route introspection. |
| [`nextdeploy ship`](/cli/ship) | `deploy` | Build, package, upload, health-check, and cutover with zero downtime. |
| [`nextdeploy rollback`](/cli/rollback) | | Instantly revert to a previous deployment without rebuilding. |
| [`nextdeploy status`](/cli/status) | | Inspect deployment health, uptime, and server/container resource usage. |
| [`nextdeploy logs`](/cli/logs) | | Stream live server logs or query historical deployment logs. |
| [`nextdeploy secrets`](/cli/secrets) | | Manage encrypted environment variables and secret stores. |
| [`nextdeploy prepare`](/cli/prepare) | | Bootstrap server dependencies (Caddy, daemon, systemd, cgroups). |
| [`nextdeploy inspect`](/cli/inspect) | | Introspect project routes, ISR tags, and build manifests. |
| [`nextdeploy generate-ci`](/cli/ci-cd) | | Scaffold automated GitHub Actions deployment pipelines. |
| `nextdeploy update` | | Self-update the `nextdeploy` CLI binary to the latest release. |
| `nextdeploy version` | | Print current CLI version, commit hash, and build timestamp. |
