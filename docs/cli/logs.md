# `nextdeploy logs`

Stream real-time production logs or inspect historical logs from your VPS host or Cloudflare Workers.

---

## Usage

```bash
nextdeploy logs [flags]
```

### Flags

| Flag | Type | Description |
| :--- | :--- | :--- |
| `--route` | string | Filter logs by a specific route (e.g. `/api/upload`). |
| `--audit` | boolean | Stream the daemon audit log. |
| `--daemon` | boolean | Stream the daemon process log. |
| `--all` | boolean | Stream everything simultaneously (App + Audit + Daemon). |

---

## Examples

```bash
# Stream live application logs
nextdeploy logs

# Filter application logs for a specific route
nextdeploy logs --route /api/webhooks

# Stream the daemon audit log
nextdeploy logs --audit

# Stream all logs
nextdeploy logs --all
```
