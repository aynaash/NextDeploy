# `nextdeploy logs`

Stream real-time production logs or inspect historical logs from your VPS host or Cloudflare Workers.

---

## Usage

```bash
nextdeploy logs [flags]
```

### Flags

| Flag | Shorthand | Type | Description |
| :--- | :--- | :--- | :--- |
| `--follow` | `-f` | boolean | Continuously tail and stream log output in real time. |
| `--lines` | `-n` | integer | Number of recent lines to display (default: `100`). |
| `--filter` | | string | Case-insensitive regex pattern or string filter for log entries. |
| `--timestamps` | | boolean | Prepend ISO 8601 timestamps to each log line. |

---

## Examples

```bash
# Stream live logs
nextdeploy logs -f

# Show last 250 log entries
nextdeploy logs -n 250

# Filter for error logs
nextdeploy logs --filter "ERROR"
```
