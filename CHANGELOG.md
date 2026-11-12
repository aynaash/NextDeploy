# Changelog

## [0.20.0](https://github.com/aynaash/NextDeploy/compare/v0.19.1...v0.20.0) (2026-11-11)


### Features

* enhance serverless deployment packaging and improve error handling ([377d6ca](https://github.com/aynaash/NextDeploy/commit/377d6ca50650439ceb58baf40b9c6381fd7edcb7))
* improve serverless support and CLI commands ([cdf2e1c](https://github.com/aynaash/NextDeploy/commit/cdf2e1c0aac644b4bcea5ec07d32dabd38ab3872))
* improve serverless support and CLI commands ([8be42d2](https://github.com/aynaash/NextDeploy/commit/8be42d2e60b33d39bbc3e2cfe9404650c33f15e0))

## [0.19.1](https://github.com/aynaash/NextDeploy/compare/v0.19.0...v0.19.1) (2026-11-07)


### Bug Fixes

* **cloudflare:** preserve ASSETS binding when user declares non-colliding R2 buckets ([60b82b4](https://github.com/aynaash/NextDeploy/commit/60b82b4468a035ed16bf3d927bd107dfec4b7d36))

## [0.19.0](https://github.com/aynaash/NextDeploy/compare/v0.18.0...v0.19.0) (2026-11-07)


### Features

* **scaffold:** fullstack Next.js template scaffolding on init for Cloudflare, VPS, and AWS ([56c0fee](https://github.com/aynaash/NextDeploy/commit/56c0feeeb532de098da1724aa8fbe14313cb897f))

## [0.18.0](https://github.com/aynaash/NextDeploy/compare/v0.17.0...v0.18.0) (2026-11-07)


### Features

* **dev:** improve Air live-reloading, strip macOS metadata files, and document local workflow ([5f547dd](https://github.com/aynaash/NextDeploy/commit/5f547dd19b4eae29572ad3767c43ce521783dc34))


### Bug Fixes

* **nextcompile:** fallback to project root for RSC vendoring and emit stubs when skipped ([9a5734e](https://github.com/aynaash/NextDeploy/commit/9a5734e099c4f49bba221ff005000fb3f8f91087))

## [0.17.0](https://github.com/aynaash/NextDeploy/compare/v0.16.3...v0.17.0) (2026-11-07)


### Features

* add debug.log to Caddy logs, assetVersion to manifest, and Cloudflare remote state sync ([029427d](https://github.com/aynaash/NextDeploy/commit/029427df7e6a57cbe638cc273c043eb95fee961d))


### Bug Fixes

* **ci:** bump golangci-lint to v2.15.0 for Go 1.26 compatibility ([c1961ee](https://github.com/aynaash/NextDeploy/commit/c1961ee2d8a589bc0c23659ab895cdfd48a2f3f8))
* **ci:** use stable Go for govulncheck to avoid stdlib CVE failures ([190ae8a](https://github.com/aynaash/NextDeploy/commit/190ae8ad77190e271df22a65c26b24819ace2ce1))
* **serverless:** adopt Wrangler D1 migrations and handle token rotation in local state ([16484d8](https://github.com/aynaash/NextDeploy/commit/16484d87eb9d53ffb6bd5cbd00e692ef902e5705))

## [0.16.0](https://github.com/aynaash/NextDeploy/compare/v0.15.1...v0.16.0) (2026-11-02)

### ⚠ BREAKING CHANGES

* **prepare:** provision over static Go agent; fix deploy correctness (`prepare` uses Go agent by default; `app.name` restricted to `^[a-z0-9-]+$`).

### Features

* **cloudflare:** SSR runtime, resource adoption, named environments, and shared CI.
* **prepare:** provision Caddy, the daemon service, and Doppler in agent mode.

### Security & Bug Fixes

* **security:** resolve CodeQL SSRF vulnerability in `cmd/imgopt`.
* **security:** bump `golang.org/x/crypto` to v0.56.0, `google.golang.org/grpc` to v1.84.0, and `vitest` to v4.1.11.
* **cloudflare:** rollback now picks active version and purges cache.
* **prepare:** add caddy to nextdeploy group for static asset access.
* **ship:** point DNS guide at server IP address rather than config nickname.

### Performance Improvements

* **caddy:** serve static assets (`public/`) directly from Caddy instead of proxying to Node.

## [0.15.1](https://github.com/aynaash/NextDeploy/compare/v0.15.0...v0.15.1) (2026-07-03)


### Bug Fixes

* **cloudflare:** destroy actually tears everything down ([#43](https://github.com/aynaash/NextDeploy/issues/43)) ([84e0a73](https://github.com/aynaash/NextDeploy/commit/84e0a732e4cdc824de4b9e669bd3d0bbd0d08b78))

## [0.15.0](https://github.com/aynaash/NextDeploy/compare/v0.14.4...v0.15.0) (2026-07-03)


### Features

* **runtime:** execute Server Actions on Cloudflare ([#41](https://github.com/aynaash/NextDeploy/issues/41)) ([27d72db](https://github.com/aynaash/NextDeploy/commit/27d72db33444d64b3f8eb48483cea1de0a058af9))

## [0.14.4](https://github.com/aynaash/NextDeploy/compare/v0.14.3...v0.14.4) (2026-07-03)


### Bug Fixes

* **nextcompile:** parse real Next 14/15 server-action manifest shape ([#40](https://github.com/aynaash/NextDeploy/issues/40)) ([74013c5](https://github.com/aynaash/NextDeploy/commit/74013c595102ea2123837a4d659c66986d41905c))


### Documentation

* **cloudflare:** correct over-claim on client hydration ([#38](https://github.com/aynaash/NextDeploy/issues/38)) ([6d3fdaa](https://github.com/aynaash/NextDeploy/commit/6d3fdaa4e5a50b4549cc4ddcf2cedacc58fff36c))

## [0.14.3](https://github.com/aynaash/NextDeploy/compare/v0.14.2...v0.14.3) (2026-07-03)


### Bug Fixes

* harden Cloudflare deploy pipeline (+ Go modernization & test coverage) ([#36](https://github.com/aynaash/NextDeploy/issues/36)) ([5904aa8](https://github.com/aynaash/NextDeploy/commit/5904aa85cec7b088ff23189f5cdc52107c7d7f31))

## [0.14.2](https://github.com/aynaash/NextDeploy/compare/v0.14.1...v0.14.2) (2026-06-15)


### Bug Fixes

* **nextcompile:** URL-decode R2 asset keys (fixes ChunkLoadError) ([#34](https://github.com/aynaash/NextDeploy/issues/34)) ([35d7c3b](https://github.com/aynaash/NextDeploy/commit/35d7c3b67086409125a93aaaed0188e8c8d4dcaa))

## [0.14.1](https://github.com/aynaash/NextDeploy/compare/v0.14.0...v0.14.1) (2026-06-15)


### Bug Fixes

* **nextcompile:** wire Next 15 .js client-reference-manifests ([#30](https://github.com/aynaash/NextDeploy/issues/30)) ([251a86b](https://github.com/aynaash/NextDeploy/commit/251a86bdafb6f911750a7558562bf23f43e19837))


### Documentation

* **cloudflare:** mark client-component hydration resolved ([#32](https://github.com/aynaash/NextDeploy/issues/32)) ([aac13fb](https://github.com/aynaash/NextDeploy/commit/aac13fbbe0ff8dafd65db21035e6cdcc5f9f3b70))

## [0.14.0](https://github.com/aynaash/NextDeploy/compare/v0.13.1...v0.14.0) (2026-06-15)


### Features

* **mage:** add dogfood target to ship a real app with the local build ([#24](https://github.com/aynaash/NextDeploy/issues/24)) ([ef01eed](https://github.com/aynaash/NextDeploy/commit/ef01eed76ecd53d1576ed0f0bbaf296f37105fef))
* **ship:** auto-provision declared CF resources before deploying ([#27](https://github.com/aynaash/NextDeploy/issues/27)) ([807f3cc](https://github.com/aynaash/NextDeploy/commit/807f3cc58f0f43d6c0988538e687be2fd59e2cca))


### Bug Fixes

* **archive:** stop rejecting Next.js catch-all routes on extract ([#23](https://github.com/aynaash/NextDeploy/issues/23)) ([3c279fe](https://github.com/aynaash/NextDeploy/commit/3c279fe926b1039814fc5a85c53a8c6e0df7e587))
* **cloudflare:** externalize critters / AMP / react-dom server.edge ([#26](https://github.com/aynaash/NextDeploy/issues/26)) ([6ac6433](https://github.com/aynaash/NextDeploy/commit/6ac6433e243672af10ff74fbaaca0e9361d70153))
* **cloudflare:** override existing origin when attaching custom domain ([#28](https://github.com/aynaash/NextDeploy/issues/28)) ([ee88aea](https://github.com/aynaash/NextDeploy/commit/ee88aea57cbdbe168f2f23b43551ff51ef88b609))


### Documentation

* **cloudflare:** document App Router runtime limitations ([#29](https://github.com/aynaash/NextDeploy/issues/29)) ([4d87f85](https://github.com/aynaash/NextDeploy/commit/4d87f85943adbaed959b4cfe0024735cf3d5f40e))

## [0.13.1](https://github.com/aynaash/NextDeploy/compare/v0.13.0...v0.13.1) (2026-06-15)


### Bug Fixes

* **updater:** let download retries recover from a partial file ([#21](https://github.com/aynaash/NextDeploy/issues/21)) ([85950fc](https://github.com/aynaash/NextDeploy/commit/85950fcfb29f4476d8e4e34f35b11cdaf5a8fef1))

## [0.13.0](https://github.com/aynaash/NextDeploy/compare/v0.12.2...v0.13.0) (2026-06-15)


### Features

* **telemetry:** anonymous, signed "apps shipped" counter ([#18](https://github.com/aynaash/NextDeploy/issues/18)) ([4622e63](https://github.com/aynaash/NextDeploy/commit/4622e630bfde1168ea5a94d6147ce4065db234aa))

## [0.12.2](https://github.com/aynaash/NextDeploy/compare/v0.12.1...v0.12.2) (2026-06-15)


### Bug Fixes

* **cloudflare:** sanitize R2 bucket and Worker names to lowercase ([#17](https://github.com/aynaash/NextDeploy/issues/17)) ([b296c78](https://github.com/aynaash/NextDeploy/commit/b296c7855c9fb3348a795751f5244b68808fd9a8))
* **updater:** match GoReleaser's lowercase asset names ([#16](https://github.com/aynaash/NextDeploy/issues/16)) ([a80e3dc](https://github.com/aynaash/NextDeploy/commit/a80e3dca0c71c4fe1625563cec8ec3bea3e6d9eb))

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
