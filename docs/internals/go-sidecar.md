# Go High-Performance Sidecar

The Go sidecar is an optional acceleration engine designed to eliminate Node.js bottlenecks in high-throughput Next.js workloads.

---

## The Node.js Bottleneck

In a standard Next.js deployment, Node.js handles:
- Dynamic SSR rendering
- Prerendered HTML / RSC payload serving
- Server Actions
- Image resizing, format conversion (Sharp / libvips bindings)

While Node.js is great for business logic and React rendering, CPU-intensive operations (such as image transcoding) and high-volume static payload delivery can block the single-threaded event loop, degrading latency for dynamic users.

---

## Sidecar Architecture

The NextDeploy Go sidecar (`cmd/sidecar`) is a lightweight proxy sitting between Caddy and Node.js:

```
Request ──► Caddy ──► Go Sidecar (:3001)
                         │
                         ├─► Prerendered HTML / RSC? ──► Serve directly (sub-ms)
                         ├─► Image Optimization?    ──► Transcode in Go threadpool
                         │
                         └─► Dynamic SSR / Action?  ──► Reverse Proxy to Node (:3000)
```

### 1. In-Memory Prerendered Cache
The sidecar loads static route manifests into a radix tree. When a request matches an SSG or prerendered RSC page, the sidecar serves the cached bytes directly without waking up Node.js.

### 2. Multi-Threaded Image Optimization
When a request arrives at `/_next/image?url=...&w=...&q=...`:
- The sidecar checks its local image cache.
- On a cache miss, it fetches the source image and processes it using multi-threaded Go image decoders/encoders (WebP, AVIF, JPEG).
- The operation runs on a dedicated worker pool without impacting Node's event loop.

### 3. Transparent Fallback
If the sidecar encounters any unsupported request, dynamic query, or custom middleware header, it immediately forwards the raw request over HTTP/1.1 keep-alive connections to Node.js.
