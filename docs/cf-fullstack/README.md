# NextDeploy — remaining work (the backlog)

Single source of truth for everything left to make NextDeploy a real
"mini-Vercel for Cloudflare": deploy + operate a full-stack App Router app
(SSR + RSC + AI/D1/R2/Vectorize/KV) with production CI/CD. Keep the status
columns current as you land each item.

**Delivery convention (Hersi's rule).** Runtime/feature work on NextDeploy source
(`.go`/`.mjs`) is delivered as **guide `.md` files** with the exact code + the
lesson; you implement by hand to stay 100% responsible for the code. The
`nextdeploy-fullstack-cloudflare` **template** is the exception — it's a consumer
app, edited directly, and serves as the live end-to-end target + reference impl.

Status: ✅ done · ✍️ guide written (ready to implement) · 🔬 written + validated
against source/fixture · 📝 to write · ❌ not started · 🚧 partial

---

## 0. The one thing that makes the app *work*

Deploying the template today provisions everything correctly but renders a
**dead shell** — `rsc.mjs` emits a blank `<div id="__next">`, so no client island
(the RAG `AskBox`) hydrates. **Closing the RSC hydration gap (§1, guides 01→06) is
the headline.** Everything else is deploy/operate polish on top.

---

## 1. CF full-stack runtime (RSC hydration) — the headline

| # | Guide | What | Status | Source touchpoints |
|---|---|---|---|---|
| 01 | `01-nextconfig-forwarding.md` | Forward `basePath`/`assetPrefix`/`i18n`/`imageConfig` through the bridge | 🔬 verified accurate, **not implemented** | `nextcompile_bridge.go`, `nextcore/types.go`, `nextcompile/{types,manifest}.go` |
| 02 | `02-middleware-conditions.md` | Carry middleware `has`/`missing` conditions + evaluate them | 🔬 verified accurate, **not implemented** | `nextcompile_bridge.go`, `manifest.go`, new `middleware_match.mjs`, `dispatcher.mjs` |
| 03 | `03-ssr-m0-bootstrap-chunks.md` | Extract ordered client bootstrap chunk list per route | 🔬 written + **fixture built + algorithm validated**, not implemented | `scanner.go`, `types.go`, fixture `testdata/fixtures/next15-appdir/` |
| 04 | `04-ssr-vendoring.md` | Vendor `react-dom/server.edge` + `react-server-dom-webpack/client.edge` | ✍️ written (not re-verified this session) | `nextcompile` vendoring + `runtime_src/vendor/` |
| 05 | `05-ssr-m1-flight-to-html.md` | **Flight → HTML on the worker** + inline `self.__next_f` + bootstrap scripts | 📝 **to write** (fixture ready; needs SSR spike) | `rsc.mjs`, dispatch entry, `EmitDispatchTable` |
| 06 | `06-ssr-m2-hydration.md` | **Hydration**: script order/preloads, consume 03's `BootstrapChunks`, honor `basePath`/`assetPrefix` | 📝 **to write** | `rsc.mjs`, dispatch entry |
| 07 | `07-ppr.md` | Partial Prerendering (static shell + dynamic holes); currently 501 | 📝 to write (depends 05/06) | `rsc.mjs:59` |
| 08 | `08-polish-notes.md` | Grab-bag: i18n routing (N/A App Router), `deriveBindings`, `elideDeadRoutes`; **DO migrations + zone TTL already done** | ✍️ written (notes, fidelity-tagged) | `compiler.go`, `cloudflare_*.go` |

**Definition of done for §1:** the template's `AskBox` counter/query hydrates and
the AI answer renders in a browser with no hydration mismatch.

**Sequence:** 01 → 03 → 04 → 05 → 06 (the SSR engine, ship together) → 07. 02 is
an independent security fix; 08 is optional polish.

### Still to author: guides 05, 06, 07 (specs)
- **05 (M1)** — In `rsc.mjs`, replace `wrapFlightInHtmlShell` with a real SSR pass:
  feed the Flight stream to `react-server-dom-webpack/client.edge` → React element
  tree → `react-dom/server.edge` `renderToReadableStream` → HTML stream. Inline the
  Flight chunks as `self.__next_f.push(...)` scripts so the client can resume.
  Needs the vendored client libs from guide 04. Assert against
  `testdata/fixtures/next15-appdir`.
- **06 (M2)** — Emit `<script src="/_next/<chunk>">` for each entry in
  `ModuleRef.BootstrapChunks` (guide 03), in order, honoring `assetPrefix`/`basePath`
  (guide 01). Add the React hydration bootstrap. DoD = interactive counter.
- **07 (PPR)** — Detect `experimental.ppr`; stream the static shell, then resolve
  dynamic holes via the Flight protocol. Depends on 05/06.

---

## 2. CI/CD — "push to main, it deploys per type"

| # | Guide | What | Status |
|---|---|---|---|
| 09 | `09-cicd-github-actions.md` | Unify `generate-ci` + scaffolder into one shared renderer emitting a **target-aware** workflow (detect cloudflare/aws/vps at runtime), pinned CLI, `--verify`, caching, symlink fix | 🔬 rewritten + **rendered YAML validated**, not implemented |
| 10 | `10-preview-deployments.md` | Per-PR **preview deployments** (`<app>-pr-N` stack, preview URL on PR, teardown on close) | ✍️ written; **needs the `nextdeploy ship --environment` feature, which does not exist yet** |

- The template already carries the working target-aware `deploy.yml` + PR `ci.yml`
  (the reference impl for guide 09).
- **Guide 10 is the one genuinely-unbuilt CI/CD *feature*:** it requires an
  `--environment` flag on `ship`/`destroy` for isolated stacks. Guide 10 is written
  but not re-verified this session, and the underlying flag is absent.

---

## 3. Operate / manage at scale — not yet guides (new work)

Gaps found while assessing "run a project of this scale." Each needs a guide.

| Item | Why it matters | State |
|---|---|---|
| **Multi-environment** (staging → prod) | One `nextdeploy.yml` = one target today; no promotion flow | ❌ ties to guide 10's `--environment` |
| **Monitoring / alerting** | Workers Logs config exists; no metrics/alerts | 🚧 logs only |
| **Secret rotation** | `secrets_store` refs supported; no rotation workflow | ❌ |
| **Drift detection** | Only immutable-resource errors surface (e.g. Vectorize dims) | 🚧 |
| **D1 / Vectorize backup + seed** | No backup/seed story for stateful resources | ❌ |

### 3a. Provisioning safety — avoid re-provisioning footguns

Provisioning is idempotent (**name = identity**; list-and-match, no state file),
but **silent** about the config edits that strand data or duplicate resources.
Behavior + footguns are documented in
[`../resource-provisioning.md`](../resource-provisioning.md). Each fix is small
and high-value:

| Item | What to build | Why | Status | Source touchpoints |
|---|---|---|---|---|
| **Orphan/rename detection in `plan`** | Warn when an account resource matching the app prefix is no longer declared (looks like a rename → new empty resource + orphaned data) | The #1 footgun: renaming a `resources.*.name` silently creates a new empty resource and abandons the old (still billed) | 📝 **to write — recommended next (small)** | `cloudflare_apply.go` (`Plan`), `ensure*` list funcs |
| **`--allow-rename` guard** | Fail-closed: a resource `name` change aborts `apply`/`ship` unless `--allow-rename` is passed | Make the orphan footgun opt-in, mirroring the existing destroy-protection pattern | 📝 | `cloudflare_apply.go`, `cmd/apply.go`, `cmd/ship.go` |
| **Orphan report** | `plan` lists account resources matching `<app>-*` that aren't in the config | Surfaces billed-but-unmanaged resources (removing from config ≠ delete) | 📝 | `cloudflare_apply.go` |
| **Env-namespacing helper** | Auto-derive `<app>-<env>-<resource>` names for preview/staging stacks | Prerequisite for per-PR previews — every env needs distinct resource names or they collide/share | 📝 ties to §2 guide 10 / §3 multi-env | config resolution, `--environment` |

**Recommended first:** orphan/rename detection in `plan` — the biggest safety net
for the least code, and it makes the first real deploys trustworthy.

**Already verified done (don't re-open):** provisioning for D1/KV/R2/**Vectorize**/
Hyperdrive/Queues (`ensure*` funcs), bindings into the Worker upload, D1 migrations,
`ship --verify` smoke gate, `rollback`, `destroy`, edge protection guard, Server
Actions runtime, DO migrations, zone-TTL graceful skip.

---

## 4. Test-infra hardening — separate track (partial)

From the earlier test plan; coverage floor bumped to **23.1%** this session.

| Package | Coverage | Remaining |
|---|---|---|
| `cli/internal/serverless` | ~18% | Deepen binding metadata + plan-diff (create/no-op/immutable-drift), ref-resolver failures. Most deploy-critical. |
| `shared/nextcompile` | ~83% | Negative paths: missing/malformed `.next`, no-actions, bad RSC vendor. |
| `daemon/internal/daemon` | ~14.5% | `NewNextDeployDaemon` init failures, `IsIPAllowed` CIDR/IPv6 edges, `AuditLogger` write failures. |

Done this session: `shared/secrets` (SecretManager+options 100%), `shared/updater`
(39→45%), `shared/nextbuild` (already 100%), `shared/config` CF round-trip,
`cmd/revalidator` handler paths. (Some directly-written tests were reverted to be
re-authored by hand — re-add per the guides convention if desired.)

---

## 5. The template as the live target

`../../../nextdeploy-fullstack-cloudflare/` — a **working RAG app** (D1 + R2 +
Workers AI + Vectorize + KV) with a target-aware deploy pipeline. It builds +
type-checks clean. It is the end-to-end proof for §1: once hydration lands, its
`AskBox` works in a browser; until then it deploys as a dead shell. Use it to
validate every §1 guide against real Next output.

---

## Priority

1. **§1 guides 05 → 06** — the hydration engine (makes the app functional). ← start here
2. §1 guides 01, 03, 04 implementation (prerequisites/parallel).
3. §2 guide 09 implementation, then guide 10 + §3 multi-env (the `--environment` feature).
4. §1 guide 07 (PPR), §4 test-infra, §3 operate polish.

Each guide is independently compilable + testable. After each:
`go build ./cli/... ./shared/...` + the package's `go test` + any `node --test`
for touched `.mjs`.
