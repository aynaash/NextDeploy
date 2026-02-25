# NextDeploy Architecture Updates: Zero-Downtime & CI/CD Pipeline

This document outlines the recent architectural improvements made to NextDeploy's continuous deployment strategy and daemon execution mechanics to support true "Zero-Touch" and "Zero-Downtime" shipping.

## 1. Native CI/CD Integration (`nextdeploy generate-ci`)

NextDeploy fundamentally removes the requirement for developers to manually execute deployments locally. Since the `nextdeploy` CLI is a single stateless binary, it executes perfectly within headless environments.

**What was implemented:**
*   A new CLI command `nextdeploy generate-ci` (or `nextdeploy ci`) was introduced.
*   This command natively scaffolds a highly-optimized `.github/workflows/nextdeploy.yml` pipeline directly into the developer's project directory.
*   The generated pipeline handles configuring Node/Bun environments, fetching the `nextdeploy` binary, and triggering `nextdeploy deploy` dynamically against their target VPS on every push to the `main` branch.

**Dependencies Configured:**
*   `SSH_PRIVATE_KEY` (Required for VPS authentication)
*   `DOPPLER_TOKEN` (Optional for environment secret injection)

---

## 2. Zero-Downtime Daemon Architecture (The Symlink Upgrade)

Previously, `nextdeployd` handled updates destructively: it would stop the active process, delete the entire application directory, expand the new `.tar.gz` payload in its place, and restart the process. This caused inevitable downtime (502 errors) during the file replacement and process boot stage.

We have fundamentally re-architected the `handleShip` extraction flow in `daemon/internal/daemon/command_handler.go` to adopt a **Symlink / Release-based Architecture** (similar to Capistrano or Vercel's internal mechanisms).

### The New Architecture Flow

1.  **Isolated Release Extraction:**
    *   Incoming payloads (`app.tar.gz`) are no longer extracted directly into `/opt/nextdeploy/apps/{appName}`.
    *   Instead, they are extracted into an isolated timestamped directory: `/opt/nextdeploy/apps/{appName}/releases/{timestamp}`.
2.  **Current Pointer Linking:**
    *   The daemon now creates and manages an `os.Symlink` at `/opt/nextdeploy/apps/{appName}/current` that automatically points to the latest successful release directory.
3.  **Process Integration:**
    *   The Next.js `systemd` configuration block now specifically targets the `/current` symlink path, rather than a hardcoded static directory.

### Pending Backend Mechanisms (TODO: Yusuf)

While the foundational directory architecture is implemented, true 100% Zero-Downtime routing requires advanced state orchestration. I have left specific `TODO: Yusuf` markers in the backend code where these mechanics must be injected:

*   **Dynamic Port Allocation:** 
    Currently, all deployments default to assuming port `3000`. The daemon needs logic to dynamically scan and reserve an open sequential port (e.g., `3001`, `3002`) for the incoming release bundle so it can spin up asynchronously without Port Collision panics.
*   **Zero-Downtime Orchestration Sequence:** 
    Instead of forcefully restarting the existing `systemd` process, the backend sequence must be updated to:
    1.  Start the NEW systemd service representing the new release on the dynamically acquired port.
    2.  Execute a Health Check query (HTTP GET) against the new port.
    3.  If healthy, dynamically re-write the Caddy configuration block referencing the new active port.
    4.  Run `systemctl reload caddy` to instantly shift reverse-proxy traffic.
    5.  Gracefully stop the old legacy `systemd` process to release resources.
