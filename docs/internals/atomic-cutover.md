# Zero-Downtime Atomic Cutover

Zero downtime is often promised, but rarely delivered in simple self-hosted environments. When running multiple services on a VPS, rolling updates frequently run into port collisions, partial request drops, or memory exhaustion.

Here is how NextDeploy achieves true zero-downtime cutover without container virtualization overhead.

---

## The Linux Inode Symlink Swap

In Linux filesystems (ext4, XFS, Btrfs), the `renameat` syscall provides an atomic directory swap guarantee:

```c
rename("/var/www/apps/my-app/current.tmp", "/var/www/apps/my-app/current");
```

1. **Release Isolation**: Every deployment builds into a brand new directory:
   ```
   /var/www/apps/my-app/releases/2026-11-20_20-15-00/
   ```
2. **Ephemeral Port Allocation**: `nextdeployd` assigns an idle internal loopback port (e.g. `127.0.0.1:3002`) to the incoming release.
3. **Cgroup Isolation**: The new release is launched inside its own systemd scope under a strict `memory.max` cgroup slice.
4. **Symlink Preparation**: A temporary symlink `current.tmp` points to the new directory.

---

## Dual-Stage Health Probing

Before traffic shifts, `nextdeployd` executes two verification stages:

### Stage 1: TCP Port Probing
Checks if the Node.js HTTP server socket is open and listening. This guards against instant crashes due to missing syntax or missing binary dependencies.

### Stage 2: Semantic HTTP Probing
Sends HTTP `GET` requests to the configured healthcheck path:
```http
GET /api/health HTTP/1.1
Host: 127.0.0.1:3002
```
The deploy only advances if:
- Returns HTTP `200 OK`
- Completes within `timeout` (default `5s`)
- Responds consistently across `retries` attempts

---

## Traffic Shift & Connection Draining

```
            [ Caddy Reverse Proxy ]
                     │
           ┌─────────┴─────────┐
           ▼                   ▼
    Old Release :3001    New Release :3002
    (Drain In-Flight)    (Accept New Traffic)
```

1. **Caddy Reconfiguration**: `nextdeployd` signals Caddy via its admin API (`localhost:2019/load`) to update upstream backends.
2. **Atomic Symlink Flip**: `current.tmp` replaces `current`.
3. **Graceful Draining**: The old release receives a `SIGTERM` signal. Existing connections are given a configurable drain window (default: 30 seconds) to complete.
4. **Termination**: Once connections reach zero or the drain window expires, `SIGKILL` cleanly terminates the old process.
