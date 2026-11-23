---
title: Doppler Integration
lede: "Inject encrypted secrets seamlessly into nextdeploy plan, build, and ship using Doppler across local development, staging branches, and automated CI/CD pipelines."
status: ready
sourceRef: shared/secrets/, shared/config/
related:
  - title: Secrets Overview
    href: /docs/secrets/overview
  - title: CI/CD Pipelines
    href: /docs/operations/ci-cd
  - title: Local Development
    href: /docs/advanced/local-development
---

## Why Use Doppler with NextDeploy

NextDeploy is designed with a **Zero-Secrets-on-Disk** security philosophy. Storing plaintext `.env` or `.env.production` files on developer laptops or remote servers risks accidental Git commits and secret leakage.

By integrating Doppler:
- Secrets are fetched just-in-time into process memory during `nextdeploy plan` and `nextdeploy ship`.
- `nextdeploy.yml` natively interpolates `${ENV_VAR}` placeholders from the injected Doppler environment.
- On VPS deployments, secrets are written directly to systemd environment files with `chmod 600` permissions or injected via the NextDeploy encrypted vault.
- On Cloudflare and AWS, secrets are automatically synchronized to Worker environment variables or AWS Secrets Manager.

---

## The `--` Syntax Rule

When invoking NextDeploy through Doppler CLI, always include the `--` argument separator:

```bash
# Correct: doppler passes all flags directly to nextdeploy
doppler run -- nextdeploy plan
doppler run -- nextdeploy ship
doppler run -- nextdeploy status

# Incorrect: Doppler attempts to parse nextdeploy flags
doppler run nextdeploy ship --env prod   # Doppler might intercept --env!
```

---

## Configuration Example

Declare your secret bindings in `nextdeploy.yml` using `${VARIABLE_NAME}` syntax:

```yaml
app_name: saas-web
target:
  type: serverless
  provider: cloudflare

serverless:
  cloudflare:
    account_id: ${CLOUDFLARE_ACCOUNT_ID}
    api_token: ${CLOUDFLARE_API_TOKEN}
    zone_name: example.com
    resources:
      hyperdrive:
        - name: primary-db
          binding: HYPERDRIVE
          origin: ${DATABASE_URL}
      r2:
        assets_bucket: saas-assets
        custom_buckets:
          - name: saas-uploads
            binding: UPLOADS

env:
  NEXT_PUBLIC_API_URL: ${NEXT_PUBLIC_API_URL}
  STRIPE_SECRET_KEY: ${STRIPE_SECRET_KEY}
  AUTH_SECRET: ${AUTH_SECRET}
```

---

## Multi-Environment Workflows (Dev, Staging, Prod)

Doppler provides first-class environment configurations (`dev`, `stg`, `prd`). You can target specific configurations using Doppler's `--config` flag:

```bash
# Deploy to Staging using Staging credentials
doppler run --config stg -- nextdeploy ship

# Deploy to Production using Production credentials
doppler run --config prd -- nextdeploy ship
```

---

## Integrating Doppler in GitHub Actions CI/CD

Store a single `DOPPLER_TOKEN` service token in GitHub Actions Repository Secrets:

```yaml
name: Deploy Next.js to Cloudflare via NextDeploy & Doppler

on:
  push:
    branches: [main]

jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - name: Checkout Code
        uses: actions/checkout@v4

      - name: Install Doppler CLI
        uses: dopplerhq/cli-action@v3

      - name: Install NextDeploy
        run: curl -fsSL https://nextdeploy.org/install.sh | bash

      - name: Run NextDeploy Preflight Plan
        env:
          DOPPLER_TOKEN: ${{ secrets.DOPPLER_TOKEN_PROD }}
        run: doppler run -- nextdeploy plan --detailed-exitcode

      - name: Deploy Production Release
        env:
          DOPPLER_TOKEN: ${{ secrets.DOPPLER_TOKEN_PROD }}
        run: doppler run -- nextdeploy ship
```

---

## Troubleshooting Doppler Invocations

- **`DOPPLER_TOKEN not found`:** Ensure `doppler login` has been run locally, or `DOPPLER_TOKEN` is exported in your environment.
- **`Variable ${NAME} is empty`:** Check your Doppler dashboard for the active project/config to ensure the variable is defined and unlocked.
- **Token Scope:** Ensure the Doppler service token has read access to the relevant Doppler Project and Config.
