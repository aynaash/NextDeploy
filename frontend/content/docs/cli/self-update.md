---
title: Self-Update
lede: "Upgrade the NextDeploy CLI binary in place with cryptographic integrity validation, version pinning, and fleet daemon synchronization."
status: ready
sourceRef: cli/cmd/update.go, cli/cmd/upgrade_daemon.go
related:
  - title: Installation
    href: /docs/getting-started/installation
  - title: CLI Reference
    href: /docs/cli/reference
  - title: VPS Daemon
    href: /docs/vps/daemon
---

## In-Place CLI Self-Upgrade (`nextdeploy update`)

NextDeploy includes a zero-dependency self-update mechanism that inspects GitHub Releases, verifies cryptographic checksums, downloads the binary for your specific OS/architecture, and replaces the running binary using atomic POSIX rename operations:

```bash
# Upgrade CLI to the latest stable release
nextdeploy update

# Verify the newly active version and build commit
nextdeploy version
```

### How `nextdeploy update` Operates:
1. **GitHub Release Check:** Queries `https://api.github.com/repos/aynaash/NextDeploy/releases/latest`.
2. **Platform Resolution:** Detects host operating system (`darwin`, `linux`, `windows`) and architecture (`arm64`, `amd64`).
3. **Checksum Verification:** Downloads `checksums.txt` and calculates SHA-256 hash of the downloaded archive. If hashes do not match, the update immediately aborts without modifying the disk.
4. **Atomic Binary Swap:** Writes the new executable to a temporary file (`nextdeploy.tmp`) in the same directory as the current binary and executes `os.Rename()`. On POSIX systems, this swap is instantaneous and safe even while the CLI is running.

---

## Pinning Specific Versions

To install a pinned release tag (for example `v1.0.0`) in CI/CD or on a workstation, download the release archive directly from GitHub Releases or pass a versioned tag to the installer:

```bash
# Install a pinned release via GitHub Releases + checksum verification
curl -fsSL https://github.com/aynaash/NextDeploy/releases/download/v1.0.0/checksums.txt -o checksums.txt
```

---

## Synchronizing the VPS Daemon Fleet (`nextdeploy upgrade-daemon`)

When running on Linux VPS targets, the remote `nextdeployd` systems daemon should stay in sync with your local CLI version.

Run `upgrade-daemon` from your development machine:

```bash
# Upgrade daemon on the server configured in nextdeploy.yml
nextdeploy upgrade-daemon

# Upgrade across multiple configured server environments
nextdeploy upgrade-daemon --env production
```

### The Daemon Upgrade Protocol:
1. The CLI builds or downloads the static `linux/amd64` or `linux/arm64` `nextdeployd` binary.
2. The binary is uploaded over secure SSH / SFTP to `/usr/local/bin/nextdeployd.new`.
3. The CLI invokes the running daemon's hot-upgrade endpoint.
4. The daemon executes an atomic rename over `/usr/local/bin/nextdeployd` and restarts its systemd service unit via `systemctl restart nextdeployd`.
5. Active Next.js processes and Caddy traffic routing are **not restarted**, resulting in zero user-facing downtime.

---

## Fresh Installation via Curl Script

If NextDeploy is not yet installed or you are configuring a new developer machine or CI runner:

```bash
# Standard install (installs to /usr/local/bin or ~/.nextdeploy/bin)
curl -fsSL https://nextdeploy.org/install.sh | bash

# Non-root installation
curl -fsSL https://nextdeploy.org/install.sh | INSTALL_DIR=$HOME/.local/bin bash
```

---

## Package Manager Updates

If you originally installed NextDeploy via Homebrew:

```bash
brew update
brew upgrade nextdeploy
```

Verify your installation:
```bash
nextdeploy --version
```
