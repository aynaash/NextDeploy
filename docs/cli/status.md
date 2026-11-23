# `nextdeploy status`

Query the active deployment state, remote daemon health, process uptime, and target connectivity.

---

## Usage

```bash
nextdeploy status
```

---

## Output Example (VPS)

```text
NextDeploy Status:
────────────────────────────────────────────────────
Application:      my-app (production)
Domain:           app.example.com
Target:           VPS (203.0.113.10)
Active Release:   2026-11-20_18-42-01 (commit: e4c9a81)
Service Status:   healthy (systemd: active (running))
Uptime:           4 days, 6 hours, 12 minutes
HTTP Probe:       200 OK (38ms)
```

## Deep Explanation

To inspect the status check logic and source functions:

```bash
nextdeploy status explain
```
