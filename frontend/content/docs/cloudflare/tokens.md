---
title: Cloudflare API Token Permissions
lede: Complete reference for Cloudflare API Token permissions, resource scoping, Doppler setup, and troubleshooting for NextDeploy.
status: ready
sourceRef: cli/internal/serverless/cloudflare*.go, shared/config/
related:
  - title: Cloudflare Quickstart
    href: /docs/cloudflare/quickstart
  - title: Cloudflare Overview
    href: /docs/cloudflare/overview
  - title: Cloudflare DNS
    href: /docs/cloudflare/dns
  - title: Bindings
    href: /docs/cloudflare/bindings
---

NextDeploy interacts with Cloudflare's REST APIs to compile and deploy Workers, synchronize versioned static assets to R2, reconcile bindings (D1, KV, Queues, Vectorize, AI, Hyperdrive), bind custom domains, and purge edge caches upon deployment.

To execute these operations, NextDeploy requires an **API Token** with the appropriate permission policies.

---

## 1. Quick Reference: Recommended Token Permissions

When creating a custom API Token in the [Cloudflare Dashboard](https://dash.cloudflare.com/profile/api-tokens), configure the following permissions:

| Scope | Permission Group | Access Level | Why NextDeploy Needs It |
| :--- | :--- | :--- | :--- |
| **Account** | **Workers Scripts** | **Edit** | Upload and publish the compiled `worker.mjs` bundle and secrets. |
| **Account** | **Workers R2 Storage** | **Edit** | Provision R2 asset buckets and upload static `.html`, `.rsc`, and `/_next/` files. |
| **Account** | **Account Settings** | **Read** | Automatically detect and resolve your Cloudflare Account ID. |
| **Zone** | **Zone** | **Read** | Look up your domain's Zone ID for custom domain routing. |
| **Zone** | **DNS** | **Edit** | Automatically attach and configure custom domains (`app.example.com`). |
| **Zone** | **Cache Purge** | **Purge** | **Required for zero-stale releases.** Automatically invalidates CDN edge caches on ship. |
| **Zone** | **Workers Routes** | **Edit** | Attach route patterns and custom domain bindings. |

### Optional Binding Permissions (Only if declared in `nextdeploy.yml`)

If your application declares additional serverless bindings under `serverless.cloudflare.bindings`, add the corresponding edit permissions:

| Binding in `nextdeploy.yml` | Cloudflare Permission | Access Level |
| :--- | :--- | :--- |
| `kv:` | **Account → Workers KV Storage** | **Edit** |
| `d1:` | **Account → D1** | **Edit** |
| `queues:` | **Account → Workers Queues** | **Edit** |
| `vectorize:` | **Account → Workers Vectorize** | **Edit** |
| `ai:` | **Account → Workers AI** | **Read / Edit** |
| `hyperdrive:` | **Account → Workers Hyperdrive** | **Edit** |

---

## 2. User API Tokens vs. Account API Tokens

> [!IMPORTANT]
> Always create a **User API Token** (`My Profile → API Tokens`), **not** an Account API Token (`Manage Account → Account API tokens`).

Cloudflare has two distinct types of tokens:
1. **User API Tokens (`cfut_...`):** Created at `https://dash.cloudflare.com/profile/api-tokens`. They have access to both Account-level resources (Workers, R2) and Zone-level resources (DNS, Cache Purge). **NextDeploy is designed for User API Tokens.**
2. **Account API Tokens (`cfat_...`):** Created at `Manage Account → Account API tokens`. They are scoped strictly to account administrative resources and often fail on Zone operations like `POST /zones/:id/purge_cache`.

---

## 3. Step-by-Step: Creating Your Token in Cloudflare

1. Log into the Cloudflare Dashboard and navigate to [My Profile → API Tokens](https://dash.cloudflare.com/profile/api-tokens).
2. Click **Create Token**.
3. Under *Custom Token* (at the bottom), click **Get started**.
4. Set a **Token name** (e.g. `NextDeploy Deployer`).
5. Under **Permissions**, add the following policies:

   **Account Permissions:**
   * `Account` → `Workers Scripts` → `Edit`
   * `Account` → `Workers R2 Storage` → `Edit`
   * `Account` → `Account Settings` → `Read`
   * *(Optional: Add KV, D1, Queues, Vectorize if your project uses them)*

   **Zone Permissions:**
   * `Zone` → `Zone` → `Read`
   * `Zone` → `DNS` → `Edit`
   * `Zone` → `Cache Purge` → `Purge`
   * `Zone` → `Workers Routes` → `Edit`

6. Under **Account Resources**, select:
   * **Include** → **All accounts** (or your specific account).
7. Under **Zone Resources**, select:
   * **Include** → **All zones** (or your specific domains).
8. Click **Continue to summary**.
9. On the summary page, verify all permissions and click **Create Token**.
10. Copy the generated token immediately (Cloudflare will not display it again).

---

## 4. Injecting Credentials

### Option A: Doppler (Recommended for Teams & Production)
Store your token in Doppler:
```bash
doppler secrets set CLOUDFLARE_API_TOKEN="cfut_your_token_here"
```
Deploy via Doppler:
```bash
doppler run -- nextdeploy ship
```

### Option B: Local Environment Variables
Export the token in your shell or CI runner:
```bash
export CLOUDFLARE_API_TOKEN="cfut_your_token_here"
# Optional: if you have multiple accounts, you can specify your Account ID:
export CLOUDFLARE_ACCOUNT_ID="your_account_id_here"
```

### Option C: NextDeploy Credential Store
Store the token securely encrypted in the local OS keychain:
```bash
nextdeploy creds set --provider cloudflare
```

---

## 5. Troubleshooting & Verification

### Verifying Token Validity
You can test whether Cloudflare recognizes your token from your terminal:

```bash
curl -s -H "Authorization: Bearer $CLOUDFLARE_API_TOKEN" \
  https://api.cloudflare.com/client/v4/user/tokens/verify | jq .
```
Expected output:
```json
{
  "result": { "id": "...", "status": "active" },
  "success": true,
  "errors": [],
  "messages": [{ "code": 10000, "message": "This API Token is valid and active" }]
}
```

### Error: `401 Unauthorized` on Cache Purge
```text
ERROR 💥 Cache invalidation failed (non-fatal): cache purge failed:
POST "https://api.cloudflare.com/client/v4/zones/:id/purge_cache": 401 Unauthorized
{"code": 10000, "message": "Authentication error"}
```
**Cause:** Your API Token is missing the **Zone → Cache Purge → Purge** permission, or you are using an Account API Token (`cfat_...`) rather than a User API Token (`cfut_...`).  
**Fix:** Edit your token at `My Profile → API Tokens`, add `Zone → Cache Purge → Purge`, and click **Update Token**.

### Error: `403 Forbidden` on Worker Upload
```text
ERROR 💥 Worker upload failed: 403 Forbidden
```
**Cause:** The token lacks `Account → Workers Scripts → Edit`.  
**Fix:** Add `Account → Workers Scripts → Edit` to your token policies and verify the account ID matches your target account.
