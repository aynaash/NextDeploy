# `nextdeploy plan`

Perform a dry-run preflight check. Inspect target infrastructure and preview what changes would occur without modifying any live resources.

---

## Usage

```bash
nextdeploy plan [flags]
```

### Flags

| Flag | Type | Description |
| :--- | :--- | :--- |
| `--detailed-exitcode` | boolean | Exit with status code `2` if infrastructure drift or changes are detected (ideal for CI pipelines). |
| `--only` | string | Scope plan to module(s): comma-separated list of `{dataplane,edge}`. |

---

## What `plan` Verifies

1. **Configuration Validation**: Asserts all fields in `nextdeploy.yml` conform to the schema and environment variables are present.
2. **Provider Authentication**: Validates Cloudflare or AWS credentials and permissions.
3. **Resource Drift Detection**: Compares declared KV namespaces, D1 databases, and Queue bindings against live cloud resources.
4. **Build Introspection**: Checks route manifests and ensures output mode is compatible.

---

## Output Example

```text
NextDeploy Plan Summary:
────────────────────────────────────────────────────
Target:        Cloudflare Workers
Environment:   production
Domain:        app.example.com

Planned Actions:
  + Create KV Namespace: CACHE_KV
  ~ Update Worker Script: edge-app
  # No-op: D1 Database "primary_db" (in sync)

Plan: 1 to add, 1 to change, 0 to destroy.
```

If run with `--detailed-exitcode` in a CI/CD pipeline, an exit code of `0` denotes no changes needed, `2` indicates drift detected, and `1` indicates an error.
