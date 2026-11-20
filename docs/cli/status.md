# `nextdeploy status`

Query the active deployment state, container health, CPU/memory usage, and recent release history.

---

## Usage

```bash
nextdeploy status [flags]
```

### Flags

| Flag | Type | Description |
| :--- | :--- | :--- |
| `--watch` | boolean | Continuously poll and display live status updates every 2 seconds. |
| `--json` | boolean | Emit status and telemetry metrics as structured JSON. |

---

## Output Example

```text
NextDeploy Status:
────────────────────────────────────────────────────
Application:      my-app (production)
Domain:           app.example.com
Target:           VPS (203.0.113.10)
Active Release:   2026-11-20_18-42-01 (commit: e4c9a81)
Service Status:   healthy (systemd: active (running))
Uptime:           4 days, 6 hours, 12 minutes

Resource Usage:
  Memory:         342 MB / 1024 MB (33.4%)
  CPU:            1.8%
  Caddy Status:   Active (TLS auto-renewed, HTTP/3 enabled)

Recent Releases:
  * 2026-11-20_18-42-01 (ACTIVE)
    2026-11-19_14-10-22 (healthy)
    2026-11-18_09-05-18 (healthy)
```
