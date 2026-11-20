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
| `set <KEY> <VALUE>` | Set an encrypted secret or environment variable. |
| `get <KEY>` | View the value of a secret (requires decryption key). |
| `list` | List all defined secret names without revealing values. |
| `push` | Push local secret state to the remote server or Cloudflare secret store. |
| `pull` | Pull remote secrets into the local encrypted secret manifest. |

---

## Secrets Safety

- Secrets are never committed in plaintext to git.
- For Cloudflare Workers, secrets are passed via authenticated Cloudflare Secret Store or encrypted Worker bindings.
- For VPS, secrets are mounted into protected `/etc/nextdeploy/secrets/<app>.env` with `0600` permissions.
