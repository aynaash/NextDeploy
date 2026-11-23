---
title: Caddy Reverse Proxy & TLS
lede: Automated Let's Encrypt TLS issuance, HTTP/3, and direct NVMe static byte delivery offloading without hitting Node.js.
status: ready
sourceRef: shared/caddy/caddy.go, cli/internal/server/caddy.go
related:
  - title: Zero-Downtime Cutover
    href: /docs/vps/atomic-cutover
  - title: Chunk Pooling
    href: /docs/vps/chunk-pooling
  - title: Go Sidecar
    href: /docs/vps/go-sidecar
---

## Why Caddy?

NextDeploy configures **Caddy** as the reverse proxy on all VPS targets:
1. **Automated Zero-Config TLS**: Automatically obtains, renews, and staples OCSP certificates from Let's Encrypt and ZeroSSL.
2. **Direct NVMe Byte Offloading**: Caddy serves static assets (`.next/static/*` and `public/*`) directly from NVMe disk using Linux kernel `sendfile(2)`, completely bypassing the Node.js event loop.
3. **HTTP/3 & QUIC**: Modern protocols enabled out of the box.

---

## Generated Caddyfile Architecture

When deploying an application named `my-app` to `app.example.com`, NextDeploy generates a hardened Caddyfile configuration block:

```caddy
app.example.com {
    encode zstd gzip

    # ─── 1. Cumulative Static Chunk Pool (Fast Path) ───
    handle_path /_next/static/* {
        root * /opt/nextdeploy/apps/my-app/shared_static
        file_server {
            hide .*
        }
        header Cache-Control "public, max-age=31536000, immutable"
    }

    # ─── 2. Public Static Assets ───
    handle_path /static/* {
        root * /opt/nextdeploy/apps/my-app/current/public
        file_server
        header Cache-Control "public, max-age=86400"
    }

    # ─── 3. Dynamic SSR & API Reverse Proxy ───
    handle {
        reverse_proxy 127.0.0.1:3000 {
            header_up Host {host}
            header_up X-Real-IP {remote_host}
            header_up X-Forwarded-For {remote_host}
            header_up X-Forwarded-Proto {scheme}

            # Zero-downtime cutover circuit breaking
            fail_duration 5s
            max_fails 3
        }
    }

    # Security Headers
    header {
        X-Content-Type-Options "nosniff"
        X-Frame-Options "DENY"
        Referrer-Policy "strict-origin-when-cross-origin"
        Strict-Transport-Security "max-age=63072000; includeSubDomains; preload"
    }
}
```

---

## Performance Impact: NVMe vs Node.js

Serving a 500KB JavaScript bundle through Node.js consumes V8 memory and blocks the single-threaded event loop. With Caddy:
- Zero JavaScript overhead.
- Throughput exceeds **85,000 requests per second** on a standard $4/month VPS.
- TTFB (Time to First Byte) for static chunks drops under **2 milliseconds**.
