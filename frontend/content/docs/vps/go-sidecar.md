---
title: Go Sidecar
lede: Optional high-performance Go reverse proxy sitting between Caddy and Next.js on VPS deployments — serves prerendered HTML/RSC payloads in sub-millisecond latency and offloads image optimization from Node.js.
status: ready
sourceRef: docs/internals/go-sidecar.md, docs/deploy-nextjs-vps-go-sidecar.md
related:
  - title: VPS Overview
    href: /docs/vps/overview
  - title: VPS Quickstart
    href: /docs/getting-started/vps-quickstart
  - title: Zero-Downtime Cutover
    href: /docs/vps/atomic-cutover
---

## Why Use the Go Sidecar

In a standard Node.js VPS deployment, Node's single-threaded event loop handles dynamic SSR, prerendered HTML/RSC serving, and CPU-heavy `/_next/image` transcoding all at once.

NextDeploy's optional Go sidecar sits between Caddy and your Next.js Node server:

```
Request ──► Caddy (:443) ──► Go Sidecar (:3001)
                               │
                               ├─► Prerendered HTML / RSC? ──► Serve directly (sub-ms)
                               ├─► /_next/image request?   ──► Multi-threaded Go transcoding
                               │
                               └─► Dynamic SSR / API /     ──► Reverse proxy to Node (:3000)
                                   Middleware / Actions
```

The sidecar is an acceleration layer, not a replacement for Next.js. Any route that needs runtime rendering, middleware, or dynamic execution passes straight through to Node.js over HTTP/1.1 keep-alive connections.

## Capabilities

- **In-Memory Prerendered HTML & RSC Cache:** Indexes static build outputs into a radix tree and serves eligible HTML and `.rsc` payloads without waking Node.js.
- **Multi-Threaded Image Optimization:** Handles `/_next/image?url=...&w=...&q=...` on a dedicated Go worker pool (resizing and WebP/AVIF/JPEG transcoding with local caching) so image processing never blocks Node's event loop.
- **Transparent Fallback:** Unsupported routes, dynamic queries, Server Actions, and middleware pass directly through to Node.js.

## Enabling in `nextdeploy.yml`

The Go sidecar is available for `target_type: vps` deployments:

```yaml
target_type: vps

app:
  name: my-nextjs-app
  environment: production
  domain: app.example.com
  port: 3000
  gosidecar:
    enabled: true
    image_optimization: true
    isr_cache: true
    max_image_workers: 4
```

Then deploy as usual:

```bash
nextdeploy prepare   # first-time VPS setup
nextdeploy ship      # deploys Node + Go sidecar + Caddy routing
```
