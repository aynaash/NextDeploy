---
title: Zero-Downtime Cutover
lede: "How NextDeploy achieves true zero-downtime releases on Linux VPS hosts using ephemeral port allocation, dual-stage TCP/HTTP health probes, atomic POSIX renameat() symlink swaps, and graceful connection draining."
status: ready
sourceRef: docs/internals/atomic-cutover.md, daemon/
related:
  - title: Chunk Pooling
    href: /docs/vps/chunk-pooling
  - title: Rollback
    href: /docs/vps/rollback
  - title: Daemon (nextdeployd)
    href: /docs/vps/daemon
---

## Release Isolation & Ephemeral Port Allocation

Every `nextdeploy ship` builds into a brand-new immutable release directory:

```text
/opt/nextdeploy/apps/my-app/releases/2026-11-23_14-20-00/
```

1. `nextdeployd` inspects active listening ports and assigns an idle internal loopback port (e.g. `127.0.0.1:3002` while the active release runs on `:3001`).
2. The incoming release boots inside its own systemd scope under its configured `cgroup v2` memory limit.
3. A temporary symlink `current.tmp` is created pointing at the new release directory.

---

## Dual-Stage Health Probing

No live traffic shifts until both stages pass:

### Stage 1: TCP Port Probe
Verifies the Node.js HTTP listener is bound and accepting connections — catching startup crashes, missing env vars, or syntax errors immediately.

### Stage 2: HTTP Semantic Probe
Issues HTTP `GET` requests to `deployment.healthcheck.path` (default `/api/health`):

```http
GET /api/health HTTP/1.1
Host: 127.0.0.1:3002
```

The deploy advances only when the endpoint returns `200 OK` within `timeout` (default `5s`) across `retries` attempts.

---

## Atomic Symlink Swap & Connection Draining

```mermaid
flowchart TD
    Client[Incoming Client Requests: 443] --> Caddy[Caddy Reverse Proxy]
    Caddy -.->|1. In-flight requests drain| OldNode[Old Release on 127.0.0.1:3001]
    Caddy ==>|2. New requests cut over| NewNode[New Release on 127.0.0.1:3002]
    Daemon[nextdeployd] -->|renameat() symlink swap| Link[current -> releases/2026-11-23_14-20-00]
    Daemon -->|Reload upstream config| Caddy
    Daemon -->|SIGTERM after 30s grace period| OldNode
```

### The 4-Step Cutover Protocol:
1. `nextdeployd` updates Caddy's upstream via its local admin API `localhost:2019/load`.
2. Executes an atomic Linux `renameat()` swap replacing `current` with `current.tmp`.
3. Caddy immediately routes all new incoming requests to the new release on `:3002`.
4. Sends `SIGTERM` to the previous release on `:3001`, allowing in-flight requests up to 30 seconds to finish cleanly before terminating the process.

If health checks fail at any point, the active release stays untouched and zero users see an error.
