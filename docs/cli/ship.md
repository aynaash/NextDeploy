# `nextdeploy ship` (alias: `deploy`)

Build, package, synchronize static chunks, perform health checking, and atomically cut over traffic.

---

## Usage

```bash
nextdeploy ship [flags]
# or
nextdeploy deploy [flags]
```

### Flags

| Flag | Shorthand | Type | Description |
| :--- | :--- | :--- | :--- |
| `--force` | `-f` | boolean | Force a full rebuild even if incremental build state matches. |
| `--verify` | | boolean | Fail the deployment if post-deploy smoke checks fail (ideal for CI pipelines). |
| `--environment` | `-e` | string | Deploy to a named environment (e.g. `staging`, `pr-42`), isolating resources. |
| `--verbose` | `-v` | boolean | Print detailed deployment logs (S3/R2 uploads, Lambda/Worker steps, CDN status). |

---

## Deployment Lifecycle

```
[Phase 1] Config & Preflight
   ├── Read nextdeploy.yml
   ├── Git sanity check
   └── Verify remote credentials

[Phase 2] Build & Introspection
   ├── Run nextdeploy build
   └── Introspect routes & Server Actions

[Phase 3] Static Chunk Sync
   ├── Accumulate chunks in shared_static (VPS)
   └── Upload hashed assets to R2 / S3 with immutable cache headers

[Phase 4] Compute Provisioning
   ├── VPS: Spawn new release on isolated port
   ├── Cloudflare: Upload compiled Worker bundle
   └── AWS: Update Lambda function code & publish alias

[Phase 5] Dual-Stage Health Probing
   ├── Stage 1: TCP port listener verification
   └── Stage 2: HTTP GET /api/health returns 200 OK

[Phase 6] Atomic Cutover
   ├── VPS: Atomic symlink switch (current.tmp ➔ current)
   ├── Cloudflare: Point custom domain routing to new deployment
   └── AWS: Shift CloudFront alias traffic
```

---

## Deep Explanation

To inspect internal phase functions and source references:

```bash
nextdeploy ship explain
```
