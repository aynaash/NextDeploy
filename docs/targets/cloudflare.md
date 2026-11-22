# Cloudflare Workers Target

Deploy your Next.js application globally to Cloudflare Workers with edge static asset serving from Cloudflare R2 and bindings for D1, KV, Queues, and Hyperdrive.

---

## How It Works (`nextcompile`)

NextDeploy compiles your Next.js `.next/standalone` build directly into a single, self-contained ESM Cloudflare Worker via its Go compiler **`nextcompile`** — with zero third-party adapter npm packages required:

1. **Engine Selection (`auto` | `nextserver` | `legacy`)**:
   - **`nextserver` (default for Turbopack & Next.js 16+)**: Runs Next.js's own `NextNodeServer` inside Cloudflare Workers (`workerd` + `nodejs_compat_v2`) using a build-time Virtual Filesystem (VFS), a static Turbopack/Webpack server chunk registry, a zero-buffering streaming Web↔Node HTTP bridge, and an R2 + KV `CacheHandler`.
   - **`legacy`**: Lightweight custom dispatcher using vendored React edge runtimes for classic Webpack builds.
   - Override via `serverless.cloudflare.engine` in `nextdeploy.yml` or `NEXTDEPLOY_CF_ENGINE=nextserver|legacy|auto`.
2. **In-Process esbuild Bundling**: Rewrites dynamic `require()` calls to static chunk lookups, replaces `vm`-based manifest evaluation and Node-only hooks with Workers-safe shims, and trims dev/experimental runtimes (~1.60 MB gzipped for a full Next.js 16 Turbopack app).
3. **Asset Fingerprinting & R2 Sync**: Content-hashed static assets (`/_next/static/*`), public files, and prerendered `.html`/`.rsc` artifacts are uploaded to Cloudflare R2 with release-versioned keys for atomic cutovers.
4. **Declarative Resource Reconciliation**: Provisions KV namespaces, D1 databases, R2 buckets, Hyperdrive configs, and Cloudflare Queues declared in `nextdeploy.yml`.

See [NextServer Cloudflare Engine Internals](/internals/nextserver-cloudflare-engine) for the full technical architecture.

---

## Configuration

Set `target_type: serverless` with `provider: cloudflare` in `nextdeploy.yml`:

```yaml
version: "1.0"
target_type: serverless

app:
  name: edge-app
  domain: edge.example.com
  environment: production

serverless:
  provider: cloudflare
  cloudflare:
    account_id: ${CLOUDFLARE_ACCOUNT_ID}
    api_token: ${CLOUDFLARE_API_TOKEN}
    zone_name: example.com
    engine: auto # auto | nextserver | legacy
    compatibility_date: "2025-04-01"
    compatibility_flags:
      - nodejs_compat_v2
    resources:
      kv:
        - name: SESSION_KV
          binding: SESSIONS
      d1:
        - name: prod_db
          binding: DB
      queues:
        - name: background-jobs
          binding: JOBS
      hyperdrive:
        - name: pg-hyperdrive
          binding: HYPERDRIVE
```

---

## Production Capabilities Matrix

| Capability | Status | Notes |
| :--- | :--- | :--- |
| **Next.js 14 / 15 / 16 & Native Turbopack** | ✅ **Production-ready** | Auto-selects the `nextserver` engine for Turbopack builds (no `--webpack` flag needed). |
| **Static & Prerendered Pages (`○`, `●`)** | ✅ **Production-ready** | Served directly from R2 (`env.ASSETS`) for HTML and client-side RSC Flight navigations. |
| **Dynamic Server Rendering (`ƒ`)** | ✅ **Production-ready** | Full App Router & Pages Router SSR executed by Next.js's own `NextNodeServer`. |
| **Progressive RSC Streaming (`Suspense`, `loading.tsx`)** | ✅ **Production-ready** | Zero-buffering Web `ReadableStream` ↔ Node `ServerResponse` HTTP bridge. |
| **ISR & On-Demand Revalidation** | ✅ **Production-ready** | R2 + KV `CloudflareCacheHandler` wired globally via `Symbol.for("@next/cache-handlers")`. |
| **API Routes (`/api/*`)** | ✅ **Production-ready** | Full Web Fetch and Node.js compatibility via `nodejs_compat_v2`. |
| **Server Actions** | ✅ **Production-ready** | Mutations, `revalidatePath`, `revalidateTag`, cookies, and redirects. |
| **Middleware & Proxy (`proxy.ts`)** | ✅ **Production-ready** | Runs ahead of route handlers for auth, headers, redirects, and rewrites. |
| **Cron Triggers (`scheduled`)** | ✅ **Production-ready** | Worker exports `scheduled()` handler and dispatches to `/api/cron`. |
| **Queue Consumers (`queue`)** | ✅ **Production-ready** | Worker exports `queue()` handler and dispatches batches to `/api/queue/[name]`. |
| **Custom Domains** | ✅ **Production-ready** | Automatic SSL and edge route binding with environment isolation safeguards. |
| **Edge Protection** | ✅ **Production-ready** | Built-in IP allow/deny rules, KV rate limiting, and HMAC session cookie verification ahead of Next.js. |
| **Custom `not-found.tsx` & `error.tsx`** | ✅ **Production-ready** | Full Next.js error boundaries with preserved HTTP status codes (e.g. true 404). |

---

## Preflight Planning (`nextdeploy plan`)

Before updating Cloudflare resources, run a Terraform-style dry run:

```bash
nextdeploy plan
```

NextDeploy inspects Cloudflare APIs and reports:
- Resources to create
- Bindings to update
- Configuration drift (exits with code 2 in CI if drift is detected)

---

## Deploying to Cloudflare

Deploy the Worker and static assets with:

```bash
nextdeploy ship
```

To tail live edge logs:

```bash
nextdeploy logs -f
```
