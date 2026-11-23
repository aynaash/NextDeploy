---
title: Secrets Management Overview
lede: "End-to-end secret lifecycle: local authenticated encryption, cloud synchronization, and runtime injection without plaintext exposure."
status: ready
sourceRef: shared/secrets/, cli/cmd/secrets.go
related:
  - title: Local ChaCha20 Encryption
    href: /docs/secrets/encryption
  - title: AWS Lambda Extension
    href: /docs/secrets/lambda-extension
  - title: Doppler Integration
    href: /docs/secrets/doppler
---

## Principles of Secret Security

Managing production secrets for Next.js applications (database connection strings, Stripe API keys, OAuth client secrets, authentication tokens) is critical. NextDeploy adheres to strict security invariants:

1. **Zero Plaintext Secrets in Repositories**: Plaintext secrets are never committed to git.
2. **Authenticated Symmetric Encryption**: Secrets stored locally or in CI/CD are encrypted with **ChaCha20-Poly1305** (or AES-256-GCM) authenticated encryption with unique nonces.
3. **Least-Privilege Cloud Ingestion**:
   - On **VPS**: Secrets are injected into the daemon's memory space and passed to systemd processes via secure IPC pipes; never stored unencrypted in `/etc/environment` or bash history.
   - On **Cloudflare Workers**: Secrets are written directly to encrypted Worker Secret bindings via the Cloudflare API.
   - On **AWS**: Stored in AWS Secrets Manager and retrieved in-memory via the Lambda Extension.

---

## Secret Providers Supported

NextDeploy supports multiple secret providers configured in `nextdeploy.yml`:

```yaml
secrets:
  provider: local # 'local', 'doppler', or 'aws'
  encryption: chacha20 # 'chacha20' or 'aes256'
```

| Provider | Best For | Storage Location | Key Commands |
| :--- | :--- | :--- | :--- |
| **`local` (Default)** | Solo developers, fast iteration, self-hosted | Local encrypted vault (`.secrets`) | `nextdeploy secrets set/get/list` |
| **`doppler`** | Teams, centralized secret rotation, multi-env | Doppler Secret Platform | `doppler run -- nextdeploy ship` |
| **`aws`** | AWS enterprise compliance, multi-account | AWS Secrets Manager | Automated via Lambda Extension |
| **`cloudflare`** | Edge workers | Cloudflare Encrypted Worker Secrets | Reconciled during `nextdeploy ship` |

---

## Secret Operations CLI

Manage secrets effortlessly with the unified CLI:

```bash
# Set an encrypted secret
nextdeploy secrets set DATABASE_URL=postgres://user:pass@ep-cool.us-east-1.neon.tech/db

# View all secret keys (values masked by default)
nextdeploy secrets list

# Retrieve a secret value
nextdeploy secrets get DATABASE_URL

# Load variables from a .env file into the secret store
nextdeploy secrets load .env.production

# Remove a secret permanently
nextdeploy secrets unset OLD_TOKEN

# Sync secrets to the remote daemon or cloud provider
nextdeploy secrets sync

# Preview or remove orphaned remote secrets
nextdeploy secrets prune --apply
```

---

## Pre-Deploy Integration Verification

During `nextdeploy plan` and `nextdeploy ship`, NextDeploy's introspection engine (`shared/nextcore`) cross-references the external services detected in your compiled bundles with your configured secrets:

- If Stripe SDK is detected in `node_modules` but `STRIPE_SECRET_KEY` is missing, NextDeploy warns immediately.
- If Supabase or Prisma is detected but `DATABASE_URL` is empty, deployment is halted before pushing broken code to production.
