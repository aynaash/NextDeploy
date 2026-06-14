# __APP_NAME__

A **deployment-ready** Next.js skeleton for Cloudflare Workers, wired with
NextDeploy. This is infrastructure scaffolding — not an application. It gives you
the deploy pipeline (bindings, edge protection, provisioning, CI); you build the
app.

> Scaffolding conventions by [aynaash (Hersi)](https://github.com/aynaash), the
> developer of NextDeploy.

## What's here (infra, not app)

- `nextdeploy.yml` — the deployment config: R2/D1 (or Hyperdrive) + KV bindings,
  resources to provision, the edge **protection** guard, observability.
- `proxy.ts` — where *your* request middleware goes. NextDeploy's edge guard runs
  *before* it (auth/rate-limit, from `nextdeploy.yml`).
- `lib/env.ts` — `getEnv()` accessor for your Cloudflare bindings + secrets.
- `migrations/` — `*.sql` that `nextdeploy ship` applies to D1 (the schema is yours).
- `.github/workflows/deploy.yml` — CI that runs `nextdeploy ship`.
- `app/login`, `app/app` — empty placeholders marking public vs guarded routes.
  **Your auth UI, login, password/OAuth, and data model are yours to build.**

## Protection (edge guard)

`nextdeploy.yml` guards `/app/*`: the guard verifies a session cookie *before*
your app runs. NextDeploy does **not** implement login — you issue the cookie.
The contract the guard checks:

```
cookie value = "<payloadB64url>.<base64url(HMAC-SHA256(payloadB64url, AUTH_SECRET))>"
payload      = base64url(JSON), optional numeric "exp" (unix seconds)
```

Issue that cookie from your own login handler after you verify the user.

## Deploy

```bash
nextdeploy secrets set AUTH_SECRET "$(openssl rand -hex 32)"
nextdeploy apply   # provision D1, KV, R2 / Hyperdrive declared in nextdeploy.yml
nextdeploy ship    # build, bundle, upload the Worker
```
