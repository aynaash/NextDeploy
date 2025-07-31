
# NextCore Runtime Architecture

## Overview

`nextruntime` is a dynamic container-based deployment engine tailored for Next.js apps. It orchestrates application builds, runtime bootstrapping, reverse proxy configuration (via Caddy), and lifecycle management.

This is part of the **NextCore** package — a runtime abstraction for deploying Next.js workloads with support for:

- Standalone builds
- Static export (CDN ready)
- Middleware support
- SSR/SSG/ISR routes
- Dynamic environment & port configuration

---

## Key Components

### 1. `NextCorePayload`
Carries all build metadata, app config, routes, static assets, middleware matchers, and environment settings.

### 2. Docker Runtime
- Container creation (`CreateContainer`)
- Networking config (`createNetworkingConfig`)
- Resource and security limits
- Mounts for static assets and image cache
- Environment configuration

### 3. Caddy Proxy Layer
Auto-generates a `Caddyfile` for each app:
- Handles dynamic route types (SSR, SSG, API, ISR)
- Supports middleware matchers
- Reloads Caddy via `SIGHUP`

### 4. Route Awareness
Routes are broken into:
- SSR
- ISR
- SSG
- API
- Middleware (with matching conditions)

---

## Future Work

- ✅ Add Prometheus support (`EnableMonitoring`)
- ✅ Enable container logging config (`ConfigureLogging`)
- ⚠️ Improve port binding handling (fix `string(port)`)
- 🔧 Add lifecycle hook support (`AddLifecycleHooks`)
- 📦 Add CDN optimization toggle
- 🔍 Improve error propagation and retry logic
- 💡 Web UI or CLI interface for managing deployments

---

## Deployment Workflow

1. `NewNextRuntime()` bootstraps a Docker runtime for a specific build.
2. `CreateContainer()` launches the container with isolated networking.
3. `ConfigureReverseProxy()` wires the app behind Caddy with route-level reverse proxy logic.
4. On any update, `reloadCaddy()` ensures proxy is synced.

---

## Example Output
A typical app container is deployed as:

```bash
docker run \
  --name myapp-<git-sha> \
  --network nextcore-network \
  -v ./public:/app/public \
  -e NODE_ENV=production \
  myapp:<git-sha>
