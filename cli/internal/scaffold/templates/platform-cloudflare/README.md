# __APP_NAME__

A **production-ready** fullstack Next.js application for **Cloudflare Workers**, wired with [NextDeploy](https://github.com/aynaash/NextDeploy).

> Scaffolding conventions and deployment architecture by [aynaash (Hersi)](https://github.com/aynaash).

---

## What's Included

- **Next.js 15 + React 19** with App Router and Standalone output.
- **Modern Responsive UI**: Built-in CSS variables, glassmorphic dark theme, interactive health check.
- **API Health Check**: [`app/api/health/route.ts`](app/api/health/route.ts) for runtime observability.
- **Typed Cloudflare Bindings**: [`lib/env.ts`](lib/env.ts) typed `getEnv()` accessor for D1, R2, AI, KV, and secrets.
- **Edge Middleware**: [`middleware.ts`](middleware.ts) (or `proxy.ts` on Next.js 16+) for custom routing and request shaping.
- **Edge Protection**: Pre-configured in `nextdeploy.yml` with session cookie HMAC validation and rate limiting.
- **Automated CI/CD**: [`.github/workflows/nextdeploy.yml`](.github/workflows/nextdeploy.yml) deploys on push to `main`.

---

## Local Development

```bash
# 1. Install dependencies
npm install

# 2. Run local development server
npm run dev
```

Open [http://localhost:3000](http://localhost:3000) in your browser.

---

## Deployment with NextDeploy

```bash
# 1. Set your application secrets
nextdeploy secrets set AUTH_SECRET "$(openssl rand -hex 32)"

# 2. Dry-run preview deployment plan
nextdeploy plan

# 3. Apply infrastructure resources (D1, KV, R2, Hyperdrive)
nextdeploy apply

# 4. Ship to Cloudflare Workers
nextdeploy ship
```

---

## Protection

Edge protection runs before your app routes and verifies a stateless HMAC-SHA256
session cookie (default cookie name: `session`, signed with `AUTH_SECRET`):

- Cookie format: `<base64url(JSON payload)>.<base64url(HMAC-SHA256(payload, AUTH_SECRET))>`
- The JSON payload **must** include a numeric `exp` claim (Unix timestamp in seconds).
- Requests to `protection.protected_paths` without a valid unexpired cookie are
  redirected (`302`) to `protection.auth.login_path` (HTML) or rejected with `401` (API).
- Per-IP rate limiting uses the KV namespace bound as `protection.rate_limit.kv_binding`
  (default: `RATE_LIMIT`).
