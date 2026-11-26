---
title: Troubleshooting
lede: "Fast fixes for the most common build, Cloudflare API, DNS, SSH, runtime, and Agent Trace issues encountered when deploying Next.js apps with NextDeploy."
status: ready
sourceRef: cli/internal/serverless/, shared/nextcompile/, cli/internal/dns/guide.go, shared/agenttrace/
related:
  - title: Limitations
    href: /docs/reference/limitations
  - title: FAQ
    href: /docs/reference/faq
  - title: Agent Trace Architecture
    href: /docs/agent-trace/architecture
---

## Cloudflare Deploy Issues

### `Error: KV PUT failed: 429 Too Many Requests` (`checkRateLimit`)
If your deployed Cloudflare Worker throws a `500 Internal Server Error` with:

```text
Error: KV PUT failed: 429 Too Many Requests
    at async checkRateLimit (worker.mjs:...)
    at async runGuard (worker.mjs:...)
```

- **Cause:** Cloudflare Workers KV enforces a hard limit of **1 `PUT` per second to the same key** (plus 1,000 `PUT`s/day on the Free plan). Navigating Next.js pages fires both the document request and concurrent `<Link>` RSC prefetches within the same second, hitting the same `rl:<ip>:<windowId>` rate-limit key.
- **Fix 1 — Scope rate limiting away from static/docs routes:** Exclude marketing and documentation routes in `nextdeploy.yml` and target `rate_limit.paths` to sensitive routes:
  ```yaml
  serverless:
    cloudflare:
      protection:
        enabled: true
        public_paths: ["/", "/login", "/docs", "/docs/*"]
        rate_limit:
          kv_binding: RATE_LIMIT
          requests_per_minute: 60
          paths: ["/api/*", "/app/*"]
  ```
- **Fix 2 — Upgrade/rebuild the NextDeploy binary:** Current builds of `shared/nextcompile/runtime_src/guard.mjs` coalesce `kv.put` writes to at most once every `1,100ms` per key per isolate, run writes through `ctx.waitUntil(...)`, and catch KV `429` errors gracefully so traffic never 500s. Rebuild your `nextdeploy` CLI binary (`nextdeploy update` or `go build`) and re-run `nextdeploy ship`.

### Next.js 16.4+ Worker 500: `TypeError: l2 is not a function` / `Cannot set property setImmediate`
If every route (`/`, `/docs`, `/api/health`) returns `HTTP 500 Internal Server Error` after upgrading to Next.js `16.4+` and live Cloudflare logs (`nextdeploy logs`) or local smoke tests show either:

```text
TypeError: l2 is not a function
    at r0.loadManifests (...)
```

or:

```text
Error: Failed to load external module next/dist/compiled/next-server/app-page-turbo.runtime.prod.js:
TypeError: Cannot set property setImmediate of #<Object> which has only a getter
```

- **Cause:**
  1. Next.js `16.4+` imports `evalManifestFromRelativePath` (and relies on `handleMissing` returning `undefined`) from `next/dist/server/load-manifest.external.js`. Older Worker shims only exported `evalManifest` and `loadManifestFromRelativePath`.
  2. Next.js `16.4+` executes `install()` at module load inside `next/dist/server/node-environment-extensions/fast-set-immediate.external.js`, assigning `require("node:timers").setImmediate = patchedSetImmediate`, which throws on esbuild's read-only ESM namespace wrapper in Cloudflare Workers.
- **Fix:** Update/rebuild the `nextdeploy` CLI binary (`nextdeploy update` or `go build -o ./bin/nextdeploy ./cli`) so `cli/internal/serverless/cloudflare/adapter/adapter.go` and `shared/nextcompile/runtime_src/next_shims/fast_set_immediate.mjs` include the updated `load-manifest.external.js` and `fast-set-immediate.external.js` shims, then re-run `nextdeploy ship`.

### `purge_cache: 401 Unauthorized` (non-fatal)
Your deployment succeeded, but your Cloudflare API token is missing the **Zone → Cache Purge: Purge** permission. Add that permission in the Cloudflare dashboard to enable automatic edge cache purges.

### `duplicate Worker binding name`
An `.env` or Doppler secret key has the same name as a declared Cloudflare binding (e.g. `ASSETS`, `DB`, `RATE_LIMIT`). Rename the binding or remove the conflicting key from `.env`.

### `script size exceeds 10MB limit` (Worker Script Limit Exceeded)
Cloudflare enforces a **10MB compressed script size limit** on the Paid Plan (3MB on Free). While NextDeploy automatically strips `.next/static` and `public/` into R2, extremely large Node.js server dependencies can inflate the worker bundle.
- **Fix 1: Externalize heavy server dependencies:** In `nextdeploy.yml`:
  ```yaml
  serverless:
    cloudflare:
      bundle:
        external:
          - "@aws-sdk/*"
          - "canvas"
  ```
- **Fix 2: Inspect standalone build output:** Check which node modules are bloated inside `.next/standalone/node_modules`. Ensure heavy development tools are in `devDependencies` rather than `dependencies`.
- **Fix 3: Verify asset offload:** Ensure `serverless.cloudflare.resources.r2` has an `assets_bucket` configured so static media isn't bundled into code.

### Forcing or Comparing Worker Engines
If you want to explicitly force the `nextserver` or `legacy` engine for debugging:

```bash
NEXTDEPLOY_CF_ENGINE=nextserver nextdeploy ship
NEXTDEPLOY_CF_ENGINE=legacy nextdeploy ship
```

### Local Reproduction of a Cloudflare Worker Bundle
Test your compiled Worker locally against Node before uploading:

```bash
NEXTDEPLOY_CF_ENGINE=nextserver NEXTDEPLOY_TEST_STANDALONE=/path/to/your-app/.next/standalone \
  go test -v ./cli/internal/serverless/cloudflare/adapter -run TestBuildWorkerBundle_RealStandaloneSmoke
```

---

## Next.js Runtime & Build Errors

### `DYNAMIC_SERVER_USAGE` Error during prerendering
This occurs when a page or component uses dynamic functions (such as `cookies()`, `headers()`, or `searchParams`) without marking the route segment dynamic.
- **Fix:** Add route segment config at the top of the route file:
  ```typescript
  export const dynamic = 'force-dynamic';
  ```
  Or ensure dynamic functions are wrapped inside a `<Suspense>` boundary if static shell rendering is desired.

### `NEXT_NOT_FOUND` or `NEXT_REDIRECT` uncaught exception in edge middleware
Next.js handles `notFound()` and `redirect()` by throwing internal errors that must be caught by the Next.js runtime.
- **Fix:** If your custom middleware or route handler wraps downstream calls in `try/catch`, ensure you re-throw Next.js internal errors:
  ```typescript
  try {
    // business logic
  } catch (error) {
    if (error && typeof error === 'object' && 'digest' in error) {
      throw error; // Let Next.js handle NEXT_REDIRECT or NEXT_NOT_FOUND
    }
    // log application error
  }
  ```

### `Cannot find name 'R2Bucket'` or `'KVNamespace'` during `next build`
Install `@cloudflare/workers-types` as a dev dependency (or type bindings as `any` / minimal interfaces in `lib/env.ts`) so `next build`'s TypeScript check resolves Cloudflare binding types cleanly:
```bash
npm install -D @cloudflare/workers-types
```
Add to `tsconfig.json`:
```json
{
  "compilerOptions": {
    "types": ["@cloudflare/workers-types/2023-07-01"]
  }
}
```

### Doppler CLI prints help text instead of running `nextdeploy`
Include `--` before the command so Doppler doesn't parse `nextdeploy` flags:

```bash
doppler run -- nextdeploy ship
```

---

## AWS Serverless Deploy Issues

### `504 Gateway Timeout` on Lambda Invocations
- **Cause:** Lambda cold start exceeded CloudFront or API Gateway's 30-second origin response timeout.
- **Fix 1:** Increase memory allocation in `nextdeploy.yml` (`memory: 1792` or `2048`). AWS scales CPU linearly with allocated RAM, cutting initialization cold starts by up to 60%.
- **Fix 2:** Check database connection pooling. Ensure your Prisma/Drizzle connection pool allows fast reconnects or uses AWS RDS Proxy.
- **Fix 3:** Enable NextDeploy Lambda Response Streaming so the HTTP response headers and initial HTML chunk flush immediately to CloudFront.

### `RequestEntityTooLargeException: Request must be smaller than 6MB`
Synchronous Lambda invocations have a strict 6MB payload limit.
- **Fix:** If uploading large files, generate S3 Presigned URLs using the AWS SDK so browsers upload directly to S3 without passing payloads through Next.js server actions.

### ACM validation stuck in "Pending Validation"
If your domain uses Cloudflare DNS in front of AWS CloudFront, make sure the ACM validation CNAME records are set to **DNS only (gray cloud)**, not proxied (orange cloud). Cloudflare proxying alters CNAME resolution and blocks AWS's automated certificate verification.

---

## VPS & SSH Issues

### `Host key verification failed` or `Permission denied (publickey)`
- **Host Key Verification Failed:** The VPS host key changed (e.g. server was rebuilt). Update your known hosts file:
  ```bash
  ssh-keygen -R <server-ip>
  ```
- **Permission Denied:** Verify your SSH identity file matches the configured user:
  ```bash
  ssh -i ~/.ssh/id_ed25519 root@<server-ip>
  ```
  Ensure permissions on your private key are strict (`chmod 600 ~/.ssh/id_ed25519`).

### TLS certificate not issuing on VPS (Caddy error)
1. Confirm your domain's `A` record points to the VPS IP (`dig example.com A +short`).
2. Verify ports `80` and `443` are open in your server firewall:
   ```bash
   sudo ufw status
   sudo ufw allow 80/tcp
   sudo ufw allow 443/tcp
   ```
3. Inspect Caddy logs:
   ```bash
   journalctl -u caddy -n 100 --no-pager
   ```

### Node.js memory exhaustion (`SIGKILL` / Exit code 137)
On low-memory VPS instances (1GB RAM), Next.js builds or high traffic spikes can trigger the Linux kernel Out-Of-Memory (OOM) killer.
- **Fix 1: Add a Swapfile:**
  ```bash
  sudo fallocate -l 2G /swapfile
  sudo chmod 600 /swapfile
  sudo mkswap /swapfile
  sudo swapon /swapfile
  ```
- **Fix 2: Set memory limits in `nextdeploy.yml`:**
  ```yaml
  vps:
    resources:
      memory_limit: 1024M
  ```

---

## Agent Trace & Handoff Troubleshooting

### Checkpoint Corruption or Missing Trace
If an AI coding agent crashed mid-turn or an interrupted Git rebase disrupted `.nextdeploy/trace/`:
- Check trace health:
  ```bash
  nextdeploy trace status
  ```
- If the index is out of sync, view the latest raw checkpoints directly:
  ```bash
  ls -lat .nextdeploy/trace/checkpoints/
  cat .nextdeploy/trace/events.jsonl | tail -n 20
  ```
- To reset or re-initialize a corrupted active trace session without losing git history:
  ```bash
  rm -f .nextdeploy/trace/active_session.json
  nextdeploy trace checkpoint -m "Manual recovery checkpoint"
  ```

### Web UI Port Conflict (`:4044` already in use)
If port 4044 is occupied by an existing process:
```bash
nextdeploy trace ui --port 4055
```
Or kill the stale process:
```bash
lsof -ti :4044 | xargs kill -9
```

---

## MCP Server Troubleshooting

### Cursor or Claude Code reports "MCP server disconnected"
1. Verify the `nextdeploy-mcp` binary is compiled and available in your `PATH`:
   ```bash
   which nextdeploy-mcp || go build -o /usr/local/bin/nextdeploy-mcp ./mcp
   ```
2. Verify `nextdeploy-mcp` executes cleanly on `stdio`:
   ```bash
   echo '{"jsonrpc":"2.0","method":"tools/list","id":1}' | nextdeploy-mcp
   ```
3. In Cursor or Claude Code MCP settings, ensure `command` points to the absolute path of `nextdeploy-mcp` and `cwd` is set to your workspace root directory.
