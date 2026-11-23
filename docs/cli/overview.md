# CLI Reference Overview

NextDeploy provides a developer-friendly command-line interface built with Cobra.

Every command includes built-in deep explanations accessible via:
```bash
nextdeploy <command> explain
```

---

## Configuration & Help

NextDeploy automatically reads its configuration from `nextdeploy.yml` in the project root.

To view help for any command:
```bash
nextdeploy <command> --help
```

To view built-in deep architectural explanations:
```bash
nextdeploy <command> explain
```

---

## Command Summary

| Command | Alias | Description |
| :--- | :--- | :--- |
| [`nextdeploy init`](/cli/init) | | Initialize `nextdeploy.yml` or scaffold a fullstack starter. |
| [`nextdeploy plan`](/cli/plan) | | Dry-run preflight: detect drift and preview changes without modifying resources. |
| [`nextdeploy build`](/cli/build) | | Build the Next.js app and prepare deployable standalone artifacts. |
| [`nextdeploy ship`](/cli/ship) | `deploy` | Build, package, upload, health-check, and cutover with zero downtime. |
| [`nextdeploy rollback`](/cli/rollback) | | Instantly revert to a previous deployment without rebuilding. |
| [`nextdeploy status`](/cli/status) | | Inspect deployment health, uptime, and target service state. |
| [`nextdeploy logs`](/cli/logs) | | Stream live server logs or daemon audit logs. |
| [`nextdeploy secrets`](/cli/secrets) | | Manage encrypted environment variables and secret stores. |
| [`nextdeploy prepare`](/cli/prepare) | | Bootstrap server dependencies (Caddy, daemon, systemd, security hardening). |
| [`nextdeploy inspect`](/cli/inspect) | | Introspect standalone bundle size and top dependency weights. |
| [`nextdeploy generate-ci`](/cli/ci-cd) | | Scaffold automated GitHub Actions or GitLab CI pipelines. |
| [`nextdeploy trace`](/cli/trace) | `takeover` | AI agent continuity layer and developer takeover briefs. |
| `nextdeploy target` | `target-type` | Print resolved deployment target (e.g. `cloudflare`, `vps`, `aws`). |
| `nextdeploy update` | | Self-update the `nextdeploy` CLI binary to the latest release. |
| `nextdeploy version` | | Print current CLI version, commit hash, and build timestamp. |
