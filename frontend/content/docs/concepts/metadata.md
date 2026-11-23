---
title: Build Metadata Architecture
lede: "How NextCore introspects your .next build output and emits .nextdeploy/metadata.json to drive packaging, routing, caching, and atomic cutover across all targets."
status: ready
sourceRef: shared/nextcore/nextcore.go, shared/nextcore/routes.go, shared/nextcore/types.go
related:
  - title: How NextDeploy Works
    href: /docs/concepts/how-it-works
  - title: CLI Reference
    href: /docs/cli/reference
  - title: NextCompile (Cloudflare Runtime)
    href: /docs/advanced/nextcompile
---

## What is `metadata.json`?

Rather than treating deployment as a blind file transfer, NextDeploy generates an explicit architectural manifest: `.nextdeploy/metadata.json`.

This file is emitted by the **NextCore** static analysis engine (`shared/nextcore`) during `nextdeploy build` or `nextdeploy ship`. It acts as the single source of truth passed to:
- The **VPS daemon (`nextdeployd`)** to configure systemd units, cgroups, and Caddyfiles.
- The **Cloudflare compiler (`nextcompile`)** to configure VFS shims and R2 asset namespaces.
- The **AWS serverless adapter** to configure Lambda environment caches and CloudFront routing behaviors.

---

## Complete Metadata Schema

Below is an annotated example of `.nextdeploy/metadata.json`:

```json
{
  "app_name": "my-nextjs-app",
  "domain": "app.example.com",
  "dist_dir": ".next",
  "output_mode": "standalone",
  "package_manager": "bun",
  "git_commit": "8f3b21c9a4e",
  "git_dirty": false,
  "asset_version": "8f3b21c",
  "generated_at": "2026-11-23T15:37:00Z",
  "health_path": "/api/health",

  "route_info": {
    "static_routes": ["/", "/pricing", "/about"],
    "dynamic_routes": ["/blog/[slug]", "/users/[id]"],
    "ssg_routes": ["/terms", "/privacy"],
    "isr_routes": ["/blog/post-1", "/products/widget-42"],
    "isr_detail": {
      "/products/widget-42": {
        "revalidate": 60,
        "tags": ["product", "widget"]
      }
    },
    "api_routes": ["/api/health", "/api/webhooks/stripe"]
  },

  "detected_features": {
    "has_server_actions": true,
    "has_middleware": true,
    "has_instrumentation": true,
    "has_turbopack": true,
    "detected_integrations": ["stripe", "prisma", "cloudinary"]
  },

  "image_config": {
    "domains": [],
    "remote_patterns": [
      {
        "protocol": "https",
        "hostname": "images.unsplash.com"
      }
    ],
    "formats": ["image/avif", "image/webp"],
    "device_sizes": [640, 750, 1080, 1200, 1920]
  },

  "static_assets": {
    "chunk_count": 48,
    "total_bytes": 1845209,
    "hashed_chunks": [
      "_next/static/chunks/app/page-8f92a1.js",
      "_next/static/css/app/layout-3c4b5e.css"
    ]
  },

  "resources": {
    "memory_max": "1G",
    "cpu_quota": "200%"
  }
}
```

---

## Fields Breakdown & Subsystem Usage

| Field Group | Source in `.next/` | Used By Subsystems |
| :--- | :--- | :--- |
| **`route_info`** | `routes-manifest.json`, `app-paths-manifest.json`, `prerender-manifest.json` | Used for smoke verification probes, ISR cache invalidation, and CloudFront path behaviors. |
| **`detected_features`** | AST inspection of compiled server code | Detects external SDKs (Stripe, Prisma) and warns before deployment if secrets are missing. |
| **`static_assets`** | `.next/static/` directory | Drives cumulative chunk pooling (`shared_static`) on VPS and R2 uploads on Cloudflare. |
| **`image_config`** | Evaluated `next.config.{js,mjs,ts}` | Configures CloudFront `/_next/image` behavior and SSRF security allowlists. |
| **`resources`** | `nextdeploy.yml` | Injected into the VPS daemon's systemd unit file for Linux kernel `cgroup v2` memory capping. |
| **`health_path`** | `nextdeploy.yml` | Probed by `nextdeployd` during Stage 2 health checks before traffic cutover. |

---

## Inspecting Build Metadata Locally

You can inspect the generated metadata without deploying using the CLI:

```bash
# Build and introspect
nextdeploy build

# Inspect the resulting metadata and route inventory
nextdeploy inspect

# Output raw JSON metadata
cat .nextdeploy/metadata.json
```
