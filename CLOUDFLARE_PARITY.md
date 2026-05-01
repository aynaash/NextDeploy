This is the project I built as the Go to dogfooding app but I have
Current state (2026-04-18): Nextdeploy can deploy simple SSR apps to Cloudflare via a worker shim, but cannot ship production apps with the full feature set.

What "Pesastream-shiaped" means (reference app)
Next.js 16.1.6 App Router, 428 routes (209 static + 219 dynamic, 207 API routes)

Middleware for subdomain routing

54 files with Server Actions

6 Durable Object classes

Cron job (\* \* \* \* \* payment fanout)

Hyperdrive (Postgres TCP proxy)

45+ secrets, R2 for assets, streaming SSR, PWA + Service Worker

Critical Gaps (Tier A — app won't function)
Gap Issue
scheduled() handler Cron jobs can't run — shim only exports fetch
Durable Object exports 6 DO classes missing → deploy validation fails
queue() handler Queue consumers can't receive messages
workerd condition Missing in esbuild flags → pg-cloudflare resolves to empty stub
cloudflare:sockets external Required for Hyperdrive/pg; must preserve as ESM
Hyperdrive binding injection Shim must read env.HYPERDRIVE.connectionString before handlers run
Middleware invocation Detector misses src/middleware.ts → subdomain routing dead
Streaming SSR Current buffered response breaks POS UI progressive render
Production Gaps (Tier B)
Server Actions detection + routing (54 files with "use server" not detected)

R2 asset upload with per-type Cache-Control

Secret sync — current wrangler secret put hits rate limit (13 of 45+ fails)

API route inventory (207 routes not emitted in metadata)

Build-ID injection for SW headers

WebSocket upgrade support

Image optimization (/\_next/image returns 501 today)

Incremental cache / tag cache (no-op default needed)

Metadata.json Extensions Required
New fields needed: middleware path/matcher, server actions file list/count, API route inventory, Durable Object class mappings, queue bindings, cron schedules, all Cloudflare bindings (hyperdrive/r2/kv/d1), compatibility flags, and required secrets scanned from code.

Worker Shim Extensions
Current shim exports only { fetch }. Target needs:

Re-export DO classes for wrangler.toml

scheduled() handler (cron dispatch to internal routes)

queue() handler

Binding overrides (Hyperdrive → globalThis + process.env)

Middleware proxy invocation

Streaming response via TransformStream

Deploy Pipeline Additions
One-shot flow: discovery → build (with --conditions=workerd) → asset upload (R2 with cache headers) → secret sync (using bulk API) → deploy → post-deploy smoke tests → auto-rollback on failure

Each step independently runnable for debugging.

Suggested Phasing
Phase 1 (1 week): Pesastream deploys without crashing

workerd conditions + cloudflare:sockets ESM preservation

DO + cron + queue exports (via user config)

Binding overrides, middleware detection, streaming response

R2 asset upload, secret bulk sync

Phase 2 (1 week): Production-grade

Server actions audit, API route inventory, build-ID wiring, WebSocket

Smoke test runner + auto-rollback

Phase 3 (1 week): Framework parity

Image optimization, incremental cache, skew protection, preloading

Non-goals
No runtime bundle patching (single-pass esbuild + user-declared config)

No implicit cache (user declares or none)

No AWS fallback or shims

No hidden magic — everything in config or metadata.json
