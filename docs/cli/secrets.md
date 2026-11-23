# `nextdeploy secrets`

Manage encrypted environment variables and secret stores for NextDeploy projects.

---

## Usage

```bash
nextdeploy secrets <subcommand> [flags]
```

### Subcommands

| Subcommand | Description |
| :--- | :--- |
| `set <KEY=VALUE...>` | Set one or more secrets. Accepts `KEY=VALUE` or `KEY VALUE`. |
| `get <KEY>` | View the decrypted value of a secret. |
| `list` | List all defined secret names without revealing values. |
| `unset <KEY...>` | Remove one or more secrets permanently. |
| `load [FILENAME]` | Load secrets from a local `.env` file into the target store (defaults to `.env`). |
| `sync` | Synchronize secrets to target providers or the remote daemon. |
| `prune [--apply]` | Find (and optionally remove) orphaned remote secrets not in local state. |

---

## Secrets Safety

- Secrets are never committed in plaintext to git.
- For Cloudflare Workers, secrets are passed via authenticated secret text bindings.
- For AWS, secrets are pushed to AWS Secrets Manager (`nextdeploy/apps/<name>/<environment>`) and fetched via Lambda Secrets Extension.
- For VPS, secrets are securely stored encrypted on disk at `/opt/nextdeploy/secrets/` with `0600` permissions.
