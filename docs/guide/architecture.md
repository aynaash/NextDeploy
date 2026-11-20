# Core Architecture

NextDeploy is designed from the ground up to solve the real-world operational challenges of hosting production Next.js apps outside proprietary cloud platforms.

```
                  ┌──────────────────────────────┐
                  │      Internet Traffic        │
                  └──────────────┬───────────────┘
                                 │
                   HTTPS :443    ▼
                  ┌──────────────────────────────┐
                  │     Caddy Web Server         │
                  │ (Auto-HTTPS, NVMe sendfile)  │
                  └──────┬───────────────┬───────┘
                         │               │
        /_next/static/*  │               │ /api/*, dynamic SSR
        /public/*        │               │
                         ▼               ▼
                 ┌───────────────┐  ┌────────────────────────────────────┐
                 │ shared_static │  │       Active Release (current)     │
                 │ Cumulative    │  │ ┌────────────────────────────────┐ │
                 │ Chunk Pool    │  │ │      Optional Go Sidecar       │ │
                 └───────────────┘  │ │  (Prerendered RSC & Image Opt) │ │
                                    │ └──────────────┬─────────────────┘ │
                                    │                │ fallback          │
                                    │                ▼                   │
                                    │ ┌────────────────────────────────┐ │
                                    │ │      Next.js Node Server       │ │
                                    │ │     (cgroup v2 sandboxed)      │ │
                                    │ └────────────────────────────────┘ │
                                    └────────────────────────────────────┘
```

---

## 1. True Zero-Downtime Atomic Cutover

Traditional deployment tools often drop packets or result in 502 Bad Gateway errors while switching Node.js processes. NextDeploy implements a **dual-stage preflight health probe with atomic POSIX cutover**:

### Stage 1: TCP Port Probing
When a new release is spawned on an isolated internal port (e.g. `:3001` while active release is `:3000`), `nextdeployd` polls the TCP socket to verify the process is actively accepting connections.

### Stage 2: HTTP Semantic Health Probe
`nextdeployd` issues HTTP `GET` requests to the configured health endpoint (e.g. `/api/health`). It asserts:
- HTTP status code `200 OK`
- Response time within SLA limits (default `< 5000ms`)
- Process memory stability under initial boot

### Stage 3: Atomic Symlink Switch
Only after both probes succeed does `nextdeployd` execute a POSIX `renameat()` operation:

```
releases/
├── 2026-11-20_v1/       (old version)
├── 2026-11-20_v2/       (new version)
current.tmp ─────────────► releases/2026-11-20_v2
# Atomic rename: mv -T current.tmp current
current ─────────────────► releases/2026-11-20_v2
```

Because directory symlink replacement via `rename` is guaranteed to be atomic by the Linux kernel, not a single incoming HTTP request is dropped. Caddy immediately routes subsequent requests to the new socket, while in-flight requests on the old process drain and terminate gracefully.

---

## 2. Cumulative Static Chunk Pooling (`shared_static`)

### The `ChunkLoadError` Problem
In modern client-side applications, Next.js generates content-hashed JavaScript chunks:
```
/_next/static/chunks/app/dashboard-a82fbc.js
```
If User A has your app open in their browser on version 1, and you deploy version 2, version 2 introduces new chunk hashes. In standard Docker deployments where old containers are replaced, User A clicking on a new page immediately encounters:
```
Unhandled Runtime Error: ChunkLoadError: Loading chunk 842 failed.
```

### NextDeploy's Solution
NextDeploy copies all built static chunks into a cumulative `shared_static` pool on the host filesystem:
- **Immutable Hashed Chunks**: Content-hashed assets (`/_next/static/*`) never conflict and are accumulated across releases.
- **Immediate Cache Headers**: Caddy serves these chunks directly off NVMe disk with `Cache-Control: public, max-age=31536000, immutable`.
- **Retention Management**: Historical chunks are retained based on your configured retention policy (default: 10 releases), guaranteeing zero `ChunkLoadError` crashes for active browser tabs.

---

## 3. Bare-Metal Performance (Go + Caddy + Systemd)

Rather than running heavy Docker virtualization that wastes CPU cycles and memory:

- **Direct NVMe I/O**: Static assets never pass through the Node.js event loop. Caddy handles TLS termination (with automatic Let's Encrypt certificates) and streams static files directly from disk via the Linux kernel `sendfile` syscall.
- **Linux Cgroups v2 Sandboxing**: Memory limits specified in `nextdeploy.yml` (e.g. `memory: 1024M`) are enforced using Linux kernel `cgroup v2` (`memory.max` and `memory.high`). If Node.js leaks memory, it is constrained without bringing down the host.
- **Systemd Supervision**: Services are managed as native systemd units with auto-restart, log forwarding to journald, and unprivileged user sandboxing.

---

## 4. High-Performance Go Sidecar

For VPS deployments, NextDeploy includes an optional Go sidecar binary (`nextdeploy-sidecar`):

- Sits between Caddy and the Next.js Node.js server.
- **Prerendered RSC Cache**: Detects and serves static pages and React Server Component (`.rsc`) payloads directly in microsecond Go latency.
- **Image Optimization**: Optimizes incoming image requests (resizing, WebP/AVIF transcoding) in multi-threaded Go using libvips/webp bindings, offloading intensive CPU work from Node's single-threaded event loop.
- **Transparent Fallback**: Any dynamic routes (`ƒ`), Server Actions, or middleware requests pass straight through to Node.js.

---

## 5. NextCore Project Introspection

Before deployment, NextDeploy's internal engine `nextcore` analyzes the Next.js compilation artifact:

- **Route Classification**: Classifies every route as Static (`○`), SSG (`●`), Dynamic SSR (`ƒ`), or API route (`λ`).
- **Server Action Registry**: Extracts action identifiers to ensure flight payloads are correctly parsed and validated.
- **ISR & Middleware Extraction**: Identifies revalidation intervals and route matcher expressions to configure reverse proxy cache policies automatically.
