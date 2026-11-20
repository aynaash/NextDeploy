# Cloudflare Workers Target

Deploy your Next.js application globally to Cloudflare Workers with edge static asset serving from Cloudflare R2 and bindings for D1, KV, Queues, and Hyperdrive.

---

## How It Works (`nextcompile`)

Unlike naive node-emulation wrappers, NextDeploy uses an internal specialized compiler called **`nextcompile`**:

1. **Introspection**: Analyzes `.next/standalone` artifacts, routes, and Server Action manifests.
2. **Bundling**: Compiles your Next.js application into a single, high-performance Cloudflare Worker bundle using vendored `react-server-dom-webpack` and `react-dom/server.edge`.
3. **Asset Fingerprinting & R2 Sync**: Content-hashed static assets (`/_next/static/*`) and public assets are uploaded to Cloudflare R2 with deterministic fingerprinting.
4. **Declarative Resource Reconciliation**: Declaratively provisions KV namespaces, D1 databases, and Cloudflare Queues declared in `nextdeploy.yml`.

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
    compatibility_date: "2024-09-23"
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
| **Static & Prerendered Pages (`○`, `●`)** | ✅ **Production-ready** | Served directly via Worker + R2 asset caching. |
| **Dynamic Server Rendering (`ƒ`)** | ✅ **Production-ready** | App Router dynamic SSR using vendored `react-server-dom-webpack` edge runtimes. |
| **API Routes (`/api/*`)** | ✅ **Production-ready** | Full Node.js Edge runtime compatibility. |
| **Server Actions** | ✅ **Production-ready** | Full support for mutations, `revalidatePath`, `revalidateTag`, cookies, and redirects. |
| **Middleware** | ✅ **Production-ready** | Runs ahead of request dispatcher for auth, headers, and rewriting. |
| **Cron Triggers (`scheduled`)** | ✅ **Production-ready** | Worker exports `scheduled()` handler, cron expressions configured automatically. |
| **Queue Consumers (`queue`)** | ✅ **Production-ready** | Worker exports `queue()` handler and dispatches batches to `/api/queue/[name]`. |
| **Custom Domains** | ✅ **Production-ready** | Automatic SSL and edge route binding with environment isolation safeguards. |
| **Edge Protection** | ✅ **Production-ready** | Built-in IP allow/deny rules, KV rate limiting, and HMAC session cookie verification. |
| **Progressive RSC Streaming (Suspense)** | ⚠️ *Roadmap* | Responses are delivered complete once Flight streams resolve; progressive chunk streaming is in development. |
| **Partial Prerendering (PPR)** | ⚠️ *Roadmap* | Returns 501 Not Implemented if `experimental.ppr` is enabled. |

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
