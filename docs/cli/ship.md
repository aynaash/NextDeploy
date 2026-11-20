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
| `--skip-build` | | boolean | Reuse existing build metadata from `.nextdeploy/metadata.json` instead of rebuilding. |
| `--no-provision` | | boolean | Skip Cloudflare resource provisioning and use existing bindings. |
| `--timeout` | `-t` | duration | Overall deployment timeout (default: `10m`). |
| `--force` | `-f` | boolean | Proceed even if uncommitted git changes are detected. |

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
