# Changelog

## [0.12.1](https://github.com/aynaash/NextDeploy/compare/v0.12.0...v0.12.1) (2026-06-15)


### Bug Fixes

* **security:** resolve both open Dependabot alerts ([abed562](https://github.com/aynaash/NextDeploy/commit/abed562e8353dec4cf4d37594b00f1de5ef7c5b2))
* **security:** resolve both open Dependabot alerts ([a5a8f0f](https://github.com/aynaash/NextDeploy/commit/a5a8f0fe0c4861fe921e94029d0c26857dd3e5cb))

## v0.9.0 — Cloudflare deployment pipeline

A complete, turnkey Cloudflare Workers deployment pipeline: resource
provisioning, an edge protection guard, secure secrets handling, reconcile/apply,
an infra sniffer, and a deployment-infra scaffolder. NextDeploy stays a
**deployment pipeline** — it provisions and protects; it does not build your app.

### New capabilities (line by line)

#### D1 — provisioning + idempotent migrations
- `cli/internal/serverless/cloudflare_d1.go`
  - `ensureD1Database` — creates a D1 DB by name if absent; returns the UUID.
  - `findD1ID` — resolves a DB name → UUID via the list endpoint (exact match).
  - `applyD1Migrations` — applies `*.sql` from `migrations_dir` once each, in
    filename order, tracked in a `_nextdeploy_migrations` table (idempotent;
    survives DB recreation).
  - `d1Exec` / `d1Record` / `queryAppliedMigrations` — D1 `/query` helpers.
  - Pure helpers: `sortedMigrationFiles`, `pendingMigrations`,
    `parseAppliedMigrations`.
- `shared/config/types.go` — `CFD1Binding` (`id` | `ref`) + `CFD1Resource`
  (`name`, `migrations_dir`, `location_hint`).

#### KV — provisioning (backs the rate limiter)
- `cli/internal/serverless/cloudflare_kv.go`
  - `ensureKVNamespace` / `findKVNamespaceID` — create-by-title, idempotent.
- `shared/config/types.go` — `CFKVBinding.Ref` + `CFKVResource`. The protection
  rate-limiter's KV namespace now auto-provisions (no manual id).

#### Edge protection guard (proxy layer)
- `shared/protection/protection.go` — decoupled normalizer. `BuildRuntime`
  validates `CFProtection` and resolves defaults (login path auto-public,
  always-public `/_next/*`, dedup/sort, secure validation) → emits the JSON the
  runtime reads.
- `shared/nextcompile/runtime_src/guard.mjs` — the edge guard (runs before the
  app): IP allow/deny, **KV-backed per-IP rate limit**, **stateless HMAC
  session-cookie auth** (no DB round-trip → works with D1 *or* BYO Postgres).
  `runGuard` returns a `Response` to short-circuit (403/429/401/302) or null.
- `shared/nextcompile/protection.go` — `EmitProtectionConfig` writes
  `_nextdeploy/protection.json` (`null` when no protection).
- Wiring: `CompileOpts.Protection` (types.go), emitted in `compiler.go`,
  imported by `entry.go` (`worker_entry.mjs`), invoked first in `dispatcher.mjs`;
  `nextcompile_bridge.go` maps YAML → runtime. `globalThis.__nextdeployEnv`
  exposes bindings to user `proxy.ts`.
- `shared/config/types.go` — `CFProtection` / `CFAuth` / `CFRateLimit`.

#### Workers Logs (observability)
- `cli/internal/serverless/cloudflare_bindings.go` — `buildObservability`
  enables Workers Logs by default in the script metadata (invocation logs + head
  sampling, clamped 0..1). Config: `CFObservability`.

#### Secure vars & secrets
- `shared/secenv/secenv.go`
  - `Classify` — secure-by-default split: only `NEXT_PUBLIC_*` + an explicit
    allowlist are public vars; everything else is a secret.
  - `RegisterSecrets` — feeds `shared/sensitive` so secret values are scrubbed
    from logs.
  - `Preflight` / `PlaintextSecretWarnings` / `GitignoreCovered` — commit-safety
    (uncommitted `.env`, plaintext secrets) + Doppler / CF Secret Store nudge.
- `cli/internal/serverless/secrets_preflight.go` — gathers signals from cwd +
  config; wired into `deploy.go` (register + warn before any logging).
- `cli/internal/serverless/cloudflare_bindings.go` — **CF Secret Store** binding
  (`bindings.secrets_store`): reference a secret by `store_id`/`secret_name`; the
  value never enters yaml or the upload metadata.

#### Reconcile / recreate
- `cli/internal/serverless/cloudflare_apply.go` — `Apply` = Plan → provision all
  declared resources; recreates anything deleted out-of-band; stops on immutable
  drift.
- `cli/internal/serverless/cloudflare_plan.go` — `planD1`, `planKV` added.
- `cli/internal/serverless/cloudflare_resources.go` — provisions D1 + KV.
- `cli/cmd/apply.go` — `nextdeploy apply` (plan → confirm → reconcile; `--yes`).

#### Infra sniffer (use-my-existing-app path)
- `cli/internal/infrasniff/` — `Sniff` scans source heuristics **and** parses
  `wrangler.{jsonc,json,toml}` (authoritative bindings; JSONC comment/trailing-
  comma stripper) → detects D1/R2/KV/AI/Hyperdrive/Vectorize/Queue + secrets +
  auth, then renders a prefilled `nextdeploy.yml`.

#### Scaffolder (deployment infra, NOT an app)
- `cli/internal/scaffold/` — embeds a deployment-infra template: `nextdeploy.yml`
  (bindings/resources/protection/observability/secrets_store), `proxy.ts`
  placeholder, `lib/env.ts` binding accessor, `migrations/`, CI workflow, minimal
  `package.json` (no imposed auth/ORM). Pluggable DB: `d1` | `byo` (Hyperdrive).
  Never overwrites. Auth UI, password hashing, schema, and business logic are the
  user's — not generated.
- `cli/internal/initialcommand/init.go` — Cloudflare init offers
  **[scaffold infra | use my cwd app]**; the cwd path runs the sniffer.

### Tests
~110 Go tests + 26 `node --test` guard cases, all passing. SDK calls are
httptest-mocked; the runtime guard is node-tested. Test files (`*.test.mjs`) are
excluded from the shipped Worker runtime.

### Notes
- Cloudflare-only; the AWS subpackage refactor remains WIP and is unaffected.
- Scratch design notes live in `cf.md` / `proxy-fullstack-plan.md` (gitignored).
