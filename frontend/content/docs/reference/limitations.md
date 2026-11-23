---
title: Limitations & Architectural Boundaries
lede: "Architectural boundaries, runtime execution ceilings, payload limits, and target-specific constraints across VPS, Cloudflare Workers, and AWS Lambda."
status: ready
sourceRef: docs/reference/
related:
  - title: FAQ
    href: /docs/reference/faq
  - title: Troubleshooting
    href: /docs/reference/troubleshooting
  - title: Architecture Concepts
    href: /docs/concepts/architecture
---

## Target Architecture Comparison Matrix

Before choosing a deployment target, review the architectural boundaries and constraints of each runtime environment:

| Feature / Limit | Linux VPS Bare-Metal | Cloudflare Workers | AWS Lambda Serverless |
| :--- | :--- | :--- | :--- |
| **Max Payload Size (Request / Response)** | Unlimited (Configurable in Caddy) | 100MB (Request) / Unlimited streaming | 6MB (Sync Invoke) / Unlimited streaming |
| **Execution Timeout** | Unlimited (Long-running processes allowed) | 30s CPU time (Unlimited I/O wait on Paid) | 15 minutes (Default: 30s) |
| **Bundle Size Ceiling** | Unlimited (Constrained only by disk) | 10MB compressed (3MB on Free plan) | 250MB uncompressed / 50MB zipped |
| **WebSockets Support** | Full bidirectional persistent WebSockets | Supported via WebSocket Hibernation API | Supported via API Gateway WebSockets |
| **Node.js Native C++ Addons (`.node`)** | Full support (compiled on host Linux) | Not supported (`workerd` / Wasm only) | Supported (compiled for `amzn2` / `al2023`) |
| **Persistent Disk Storage** | Local NVMe SSD / `/var/www` | None (Ephemeral memory; use R2/KV) | 512MB–10GB ephemeral `/tmp` or EFS mount |
| **Zero-Downtime Rollbacks** | Instant (POSIX symlink swap in <10ms) | Instant (Worker script alias flip) | Instant (Lambda function alias routing) |
| **Cold Starts** | 0ms (Always-on hot process) | <5ms global edge cold start | 150ms–400ms (Reduced with ARM64 + provisioned) |

---

## Target-Specific Constraints & Details

### 1. Cloudflare Workers Target

NextDeploy compiles your Next.js application into a standard Cloudflare Worker utilizing the `nextserver` runtime engine and Cloudflare's `nodejs_compat_v2` compatibility flag:

- **Script Size Limit (10MB):** Cloudflare enforces a strict 10MB compressed bundle limit.
  - *NextDeploy Optimization:* NextDeploy automatically extracts all static assets (`.next/static/*` and `public/*`) and uploads them directly to Cloudflare R2 before compiling the Worker bundle. This ensures only server logic is bundled into the script.
  - *Mitigation for Monoliths:* If heavy server dependencies (e.g. headless browsers, PDF generators) exceed 10MB, externalize them or deploy that specific microservice to VPS or AWS Lambda.
- **Node.js Native C++ Binaries:** Cloudflare's `workerd` edge runtime executes V8 JavaScript and WebAssembly. It does not load `.node` compiled C++ modules (such as non-Wasm `sharp` or native `bcrypt`). NextDeploy shims cryptographic and standard APIs using Web Crypto and Pure-JS implementations.
- **Dynamic Code Evaluation:** `eval()` and `new Function()` with dynamic strings are restricted in standard worker mode for security.

---

## 2. AWS Serverless Target

NextDeploy packages the Next.js standalone server into an AWS Lambda function with Amazon CloudFront distribution:

- **250MB Uncompressed Package Limit:** AWS Lambda limits the uncompressed deployment artifact to 250MB. NextDeploy separates static chunks into S3 with CloudFront Origin Access Control (OAC), keeping the core Lambda function package between 18MB and 35MB.
- **6MB Synchronous Payload Ceiling:** AWS API Gateway and Lambda synchronous invocations return `413 Request Entity Too Large` if request or response bodies exceed 6MB.
  - *Workaround:* For file uploads, use S3 Presigned URLs so user browsers upload directly to S3.
  - *For Large Downloads:* NextDeploy enables Lambda Response Streaming, bypassing the 6MB response limit.
- **Lambda Ephemeral File Storage:** The root filesystem is read-only. Writable scratch space is limited to `/tmp` (configurable from 512MB up to 10GB). Do not store persistent user uploads in `/tmp`.

---

## 3. VPS Bare-Metal Target

Running on Linux VPS bare metal provides complete freedom without serverless restrictions, with the following operational considerations:

- **Single Host by Default:** Standard `nextdeploy.yml` configurations target a single server IP. While single NVMe VPS instances easily handle thousands of requests per second, multi-region high availability requires deploying to multiple server targets behind an external DNS load balancer (such as Cloudflare Load Balancing).
- **Static Chunk Disk Accumulation:** NextDeploy's Cumulative Static Chunk Pooling (`shared_static`) intentionally retains old hashed JavaScript chunks across releases to prevent client-side `ChunkLoadError` 404 crashes. Over hundreds of deployments, old chunks can consume several gigabytes of disk space. Run `nextdeploy cleanup` periodically to prune chunks older than 90 days.
- **Server Maintenance:** Unlike serverless platforms where underlying OS security patches are managed by the cloud provider, you are responsible for Linux kernel updates and security patches (`apt update && apt upgrade`).
