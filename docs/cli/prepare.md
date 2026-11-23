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
| `--server` | string | Prepare a specific server by name; defaults to the first configured server. |
| `--all` | boolean | Prepare every configured server in `nextdeploy.yml`. |
| `--allow-root` | boolean | Allow provisioning over a root SSH login (default false, non-root sudo user recommended). |
| `--skip-runtimes` | boolean | Do not install Node/Corepack (for hosts managing runtimes independently). |
| `--skip-hardening` | boolean | Do not install logrotate or fail2ban jails. |
| `--timeout` | duration | Overall timeout for the preparation (default: `10m`). |
| `--stream` | boolean | Stream raw output without color filtering. |
| `--verbose` | boolean | Enable verbose Ansible output (`-v`). |

---

## What `prepare` Executes

1. **Host Verification**: Validates remote OS distribution (Ubuntu 22.04+, Debian 12+, Rocky Linux 9+).
2. **Package Setup**: Ensures `curl`, `rsync`, and modern systemd utilities are installed.
3. **Caddy Installation**: Sets up official Caddy apt repository, installs Caddy binary, and creates `/etc/caddy/Caddyfile.d/` includes.
4. **Daemon Installation**: Downloads and verifies matching `nextdeployd` architecture binary (`amd64` or `arm64`) to `/usr/local/bin/nextdeployd`.
5. **Systemd & Cgroups**: Configures `cgroup v2` memory controllers and sets up `nextdeployd.service` under systemd.
