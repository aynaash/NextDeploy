---
title: Workers Runtime
lede: How NextDeploy compiles and runs Next.js 14, 15, and 16 (including native Turbopack) inside Cloudflare Workers (workerd) with nodejs_compat_v2, R2 asset serving, and a 1.60 MB gzipped Worker bundle.
status: ready
sourceRef: shared/nextcompile/, shared/nextcompile/runtime_src/nextserver/, cli/internal/serverless/cloudflare_adapter.go, cli/internal/serverless/cloudflare_bindings.go
related:
  - title: NextCompile (full reference)
    href: /docs/advanced/nextcompile
  - title: Running Next.js 16 Turbopack on Cloudflare (Deep Dive)
    href: /docs/advanced/nextjs-16-turbopack-on-cloudflare
  - title: Bindings
    href: /docs/cloudflare/bindings
  - title: Lambda Bridge (AWS counterpart)
    href: /docs/aws/lambda-bridge
---

## Why a Compiler Instead of a Node Process Bridge

On AWS Lambda or a VPS, NextDeploy can run Next.js inside a Node.js process. Cloudflare Workers run on **V8 isolates (`workerd`)** with no OS processes and no local filesystem.

Instead of shipping third-party adapter npm packages or forcing `--webpack`, NextDeploy compiles `.next/standalone` directly in Go (`nextcompile`) + in-process `esbuild` into a single self-contained `worker.mjs`:

- **Zero external npm dependencies** in your app's `package.json`.
- **Auto-detects Turbopack vs Webpack** (`engine: auto` chooses `nextserver` for Turbopack builds and `legacy` for classic Webpack builds).
- **Runs real `NextNodeServer`** via a build-time Virtual Filesystem, static server chunk registry, streaming Web↔Node HTTP bridge, and R2+KV `CacheHandler`.

## Compatibility Date & Flags

NextDeploy defaults to:

```yaml
serverless:
  provider: cloudflare
  cloudflare:
    engine: auto # auto | nextserver | legacy
    compatibility_date: "2025-04-01"
    compatibility_flags:
      - nodejs_compat_v2
```

With `nodejs_compat_v2` on `2025-04-01`, `workerd` natively provides `node:async_hooks` (`AsyncLocalStorage`), `node:buffer`, `node:crypto`, `node:events`, `node:stream`, `node:path`, `node:url`, `node:util`, `node:zlib`, `node:assert`, `node:net`, `node:tls`, `node:dns`, and `node:module`.

For Node APIs that `workerd` does not implement (or leaves as throwing `[unenv]` stubs):

- `fs` / `node:fs` / `node:fs/promises` → backed by NextDeploy's build-time Virtual Filesystem (`vfs_fs_shim.mjs`).
- `process.listeners`, `process.on`/`off`/`emit`, `process.cwd`, `process.uptime`, `process.hrtime.bigint`, `process.stdout`/`stderr` → polyfilled before boot by `ensureProcessCompat()`.
- `vm.runInNewContext` (`evalManifest`) → replaced with static chunk-registry execution (100% CSP-compliant).
- `tty`, `os`, `perf_hooks`, `child_process`, `worker_threads` → safe CJS stubs (`isatty: () => false`, real `globalThis.performance`).

## Bundle Size & Tier Limits

Cloudflare enforces gzipped Worker script limits of **3 MB on the Free plan** and **10 MB on the Paid plan**.

NextDeploy automatically trims Next.js dev/experimental/edge-runtime bundles (`*.runtime.dev.js`, `*-experimental.runtime.*`, `react-dom/static`, `caniuse-lite`, `@swc/helpers`) and prints the top bundle contributors on every build:

- Full Next.js 16.0.10 + Turbopack + React 19.2.0 app (52 routes): **6,432.6 KB raw → 1.60 MB gzipped** (fits even the Free tier).

## Worker Exports & Bindings

Every compiled Worker exports:

- `fetch(request, env, ctx)` — runs edge protection guard, `/_next/image`, R2 static/SSG fast paths, and `NextNodeServer`.
- `scheduled(event, env, ctx)` — dispatches Cron Triggers to `POST /api/cron`.
- `queue(batch, env, ctx)` — dispatches Cloudflare Queue batches to `POST /api/queue/[name]`.

String secrets and env vars folded into the Worker upload are automatically mirrored into `process.env` at request time, while Cloudflare bindings (`env.DB`, `env.ASSETS`, `env.KV`, `env.HYPERDRIVE`, `env.AI`) are available via `globalThis.__nextdeployEnv`.

## Observability & Logs

Stream live edge logs from your deployed Worker:

```bash
nextdeploy logs -f
```

Post-deploy `SmokeVerify` probes your custom domain root and up to 3 public static routes (automatically skipping Next.js internal `/_not-found` and `/_global-error` routes) and reports exact status codes and latencies.
