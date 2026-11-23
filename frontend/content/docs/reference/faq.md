---
title: Frequently Asked Questions (FAQ)
lede: "Comprehensive answers to architectural, operational, performance, billing, and AI agent integration questions about NextDeploy."
status: ready
sourceRef: docs/
related:
  - title: Limitations
    href: /docs/reference/limitations
  - title: Troubleshooting
    href: /docs/reference/troubleshooting
  - title: Agent-First Infrastructure
    href: /docs/agent-trace/agent-first-infra
---

## General & Architectural

### 1. How does NextDeploy compare to Vercel?
NextDeploy provides the developer ergonomics and zero-downtime cutover of Vercel, but **you own the infrastructure**:
- **Zero Bandwidth Markup:** You pay raw cloud infrastructure costs (a $4/month Hetzner/DigitalOcean VPS, or Cloudflare's $5/month Workers plan with 10M requests included) instead of Vercel's steep egress and edge compute markups.
- **No Vendor Lock-in:** Your code runs on standard Linux VPS, Cloudflare Workers, or AWS Lambda. You can switch targets anytime by modifying the `target` block in `nextdeploy.yml`.
- **Complete Infrastructure Ownership:** You retain full control over memory limits, systemd processes, database co-location, and Caddy reverse proxy rules.

---

### 2. Does NextDeploy support Next.js 15 and Next.js 16?
**Yes.** NextDeploy supports Next.js 13, 14, 15, and 16 out of the box:
- Full support for React 19, Server Actions, React Server Components (RSC), and Turbopack builds (`next build --turbo`).
- On Cloudflare Workers, NextDeploy's `nextcompile` compiler bundles the `nextserver` engine with `nodejs_compat_v2` runtime flags.
- On VPS targets, the standalone Node.js server executes directly under hardened systemd sandboxing.

---

### 3. How does Zero-Downtime Cutover work on a VPS without Docker?
NextDeploy achieves zero downtime using dual-stage health verification and atomic POSIX symlink swaps:
1. The new release is extracted into `/opt/nextdeploy/apps/<app>/releases/<timestamp>/`.
2. A new Node.js instance starts on an isolated loopback port (e.g. `3002`).
3. The `nextdeployd` daemon polls the TCP port listener and sends HTTP GET probes to `/api/health`.
4. Once verified, the daemon atomically swaps the `/opt/nextdeploy/apps/<app>/current` symlink pointer using `rename()`.
5. Caddy hot-reloads its upstream target via its internal admin API (`http://127.0.0.1:2019/load`) in under 2ms.
6. In-flight TCP requests on the previous version finish cleanly before the old process is retired.

---

### 4. What is Cumulative Static Chunk Pooling?
When users navigate your Next.js application while you deploy a new version, traditional tools wipe the old `.next/static` directory. When an active user clicks a link, their browser requests an old chunk and receives an **HTTP 404**, crashing the app with `ChunkLoadError`.

NextDeploy solves this by copying all hashed JavaScript chunks into `/opt/nextdeploy/apps/<app>/shared_static/` on every release. Old content-hashed chunks are never deleted during deployments. Active user sessions continue navigating smoothly without 404 errors.

---

### 5. What is Agent Trace & Handoff?
Agent Trace is NextDeploy's native observability layer designed for AI coding agents (Antigravity IDE, Claude Code, Cursor, Windsurf, Kiro). It records:
- Checkpoints, prompts, model reasoning trails, and modified files into `.nextdeploy/trace/`.
- Pre-flight build failures and cutover telemetry during `nextdeploy ship`.
- When a human developer needs to take over, running `nextdeploy takeover -o` prints a concise 20-line executive brief and opens all modified files directly in `$EDITOR`.

---

## Database & State Management

### 6. How do I run database migrations (Prisma / Drizzle / Kysely)?
You can define pre-deploy hooks in `nextdeploy.yml`:

```yaml
hooks:
  pre_ship:
    - npx prisma migrate deploy
    # or: npx drizzle-kit migrate
```
If migrations fail, `nextdeploy ship` immediately halts before cutting over traffic, protecting your production database.

---

### 7. Can I run PostgreSQL or Redis directly on the same VPS?
**Yes.** Because NextDeploy runs on bare-metal Linux without container encapsulation:
- You can install PostgreSQL or Redis directly on the server (`sudo apt install postgresql redis-server`).
- Connect your Next.js application to `postgresql://user:pass@127.0.0.1:5432/dbname`.
- Enjoy sub-millisecond local Unix socket or loopback database queries without network overhead.

---

## Monorepos & CI/CD

### 8. Does NextDeploy support Turborepo, Nx, and pnpm workspaces?
**Yes.** In monorepos, specify the application subpath:

```yaml
# nextdeploy.yml
monorepo:
  app_dir: apps/web
  package_manager: pnpm
```
NextDeploy runs builds from the workspace root or designated app directory and packages the output standalone bundle accurately.

---

### 9. Can I run multiple environments (Staging vs. Production)?
**Yes.** You can define environments in `nextdeploy.yml` or use environment-specific config files:

```bash
nextdeploy ship --env staging
nextdeploy ship --env production
```
Or pass a dedicated configuration file:
```bash
nextdeploy ship -c nextdeploy.staging.yml
```

---

### 10. Do I need Docker installed on my server?
**No.** NextDeploy intentionally avoids Docker daemon overhead, container networking bridges, and virtualized file layers. It runs directly on bare metal with Linux kernel `cgroup v2` memory sandboxing and systemd process isolation.
