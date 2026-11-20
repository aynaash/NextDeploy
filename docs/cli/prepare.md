# `nextdeploy prepare`

Bootstrap dependencies on a target Linux VPS host, configuring Caddy, systemd, cgroup limits, and `nextdeployd`.

---

## Usage

```bash
nextdeploy prepare [flags]
```

### Flags

| Flag | Type | Description |
| :--- | :--- | :--- |
| `--agent-only` | boolean | Install or upgrade only `nextdeployd` without altering host packages or Caddy configuration. |
| `--caddy-only` | boolean | Reconfigure Caddy reverse proxy without touching system services. |

---

## What `prepare` Executes

1. **Host Verification**: Validates remote OS distribution (Ubuntu 22.04+, Debian 12+, Rocky Linux 9+).
2. **Package Setup**: Ensures `curl`, `rsync`, and modern systemd utilities are installed.
3. **Caddy Installation**: Sets up official Caddy apt repository, installs Caddy binary, and creates `/etc/caddy/Caddyfile.d/` includes.
4. **Daemon Installation**: Downloads and verifies matching `nextdeployd` architecture binary (`amd64` or `arm64`) to `/usr/local/bin/nextdeployd`.
5. **Systemd & Cgroups**: Configures `cgroup v2` memory controllers and sets up `nextdeployd.service` under systemd.
