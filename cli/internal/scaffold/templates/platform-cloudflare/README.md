# __APP_NAME__

A **production-ready** fullstack Next.js application for **Cloudflare Workers**, wired with [NextDeploy](https://github.com/aynaash/NextDeploy).

> Scaffolding conventions and deployment architecture by [aynaash (Hersi)](https://github.com/aynaash).

---

## What's Included

- **Next.js 15 + React 19** with App Router and Standalone output.
- **Modern Responsive UI**: Built-in CSS variables, glassmorphic dark theme, interactive health check.
- **API Health Check**: [`app/api/health/route.ts`](app/api/health/route.ts) for runtime observability.
- **Typed Cloudflare Bindings**: [`lib/env.ts`](lib/env.ts) typed `getEnv()` accessor for D1, R2, AI, KV, and secrets.
- **Edge Proxy**: [`proxy.ts`](proxy.ts) for custom routing and request shaping.
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

# 2. Apply infrastructure resources (D1, KV, R2)
nextdeploy apply

# 3. Dry-run preview deployment plan
nextdeploy plan

# 4. Ship to Cloudflare Workers
nextdeploy ship
```
