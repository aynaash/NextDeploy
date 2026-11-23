# `nextdeploy generate-ci`

> Status: ✅ **Production-Ready** for both **VPS** and **Cloudflare** targets across **GitHub Actions** and **GitLab CI**.

Scaffold production-grade CI/CD automation pipelines for GitHub Actions and GitLab CI with zero vendor lock-in and hardened Doppler-first secret management.

---

## Capabilities Matrix

| Target | Status | Key Features |
| :--- | :--- | :--- |
| **Cloudflare** | ✅ **Production-ready** | Target-aware detection, automated IaC resource reconciliation (`apply`), atomic deployment (`ship --no-provision --verify`), PR previews (`--preview`), automated teardown on PR close, encrypted state synchronization via `NEXTDEPLOY_CF_STATE_KEY`, and dry-run drift checks (`plan --detailed-exitcode`). |
| **VPS** | ✅ **Production-ready** | Target-aware detection, automated SSH key deployment (`chmod 700 ~/.ssh`), dual-key path compatibility (`id_ed25519` & `id_rsa`), host-key pinning (`SSH_KNOWN_HOSTS`), strict host-key verification (`NEXTDEPLOY_STRICT_HOST_KEY`), dual-stage health probing (`ship --verify`), and zero-downtime atomic symlink cutover. |

---

## Usage

```bash
nextdeploy generate-ci [flags]
```

### Flags

| Flag | Type | Description |
| :--- | :--- | :--- |
| `--provider` | string | CI provider to target: `github-actions` (default) or `gitlab-ci`. |
| `--target` | string | Target deployment type override: `cloudflare`, `vps`, `aws`, or `auto` (defaults to auto-resolved from `nextdeploy.yml`). |
| `--preview` | boolean | Include ephemeral preview deployment workflows for Pull Requests (`preview.yml` and `preview-cleanup.yml`, Cloudflare + GitHub Actions). |
| `--output` | string | Target workflow file destination (defaults to `.github/workflows/nextdeploy.yml` or `.gitlab-ci.yml`). |
| `--force` | boolean | Overwrite existing workflow files if they already exist. |

---

## Deployment Type Resolution

The CI pipeline is designed to know and enforce the deployment type at multiple levels:

1. **Generation-Time Knowledge**:
   When running `nextdeploy generate-ci`, NextDeploy resolves the deployment type from `nextdeploy.yml` (or `--target`) and embeds `NEXTDEPLOY_TARGET_TYPE: "cloudflare"` (or `"vps"` / `"aws"`) directly into the pipeline definition.
2. **Runtime Verification (`nextdeploy target`)**:
   During pipeline execution, the runner queries the CLI binary directly via `nextdeploy target` to dynamically verify or re-resolve the target type using NextDeploy's configuration engine.
3. **Robust Fallback**:
   If the environment variable and binary query are absent, a resilient regex inspection parses `nextdeploy.yml` so older setups continue to run seamlessly.

You can inspect your project's resolved target anytime locally or in scripts:
```bash
# Print resolved deployment type (e.g. cloudflare, vps, aws)
nextdeploy target

# Output detailed target JSON
nextdeploy target --json
```

---

## Secret Hardening: Doppler-First & Normal Way

NextDeploy CI/CD pipelines use a **Doppler-first** architecture with transparent fallback to standard repository/CI variables:

### 1. Doppler-First (Recommended)
When using Doppler, **`DOPPLER_TOKEN` is the ONLY secret you need to configure** in your CI provider (GitHub Secrets or GitLab CI/CD Variables):
- **`DOPPLER_TOKEN`**: A Doppler Service Token scoped to your environment (e.g. `prd`, `stg`).

All deployment credentials (`CLOUDFLARE_API_TOKEN`, `CLOUDFLARE_ACCOUNT_ID`, `R2_*`, `SSH_PRIVATE_KEY`, `SSH_KNOWN_HOSTS`, `CLOUDFLARE_WORKERS_SUBDOMAIN`), remote state encryption keys, and application environment variables are securely fetched directly from Doppler in memory.

When `DOPPLER_TOKEN` is detected:
- The pipeline installs the Doppler CLI idempotently.
- Deployment executes with `doppler run --`, streaming all infrastructure credentials and application environment variables into the process without ever writing secrets to disk.
- For VPS deployments, the SSH private key, known hosts, and passphrase can be retrieved dynamically from Doppler via `doppler secrets get ... --plain`.
- For PR preview deployments, `CLOUDFLARE_WORKERS_SUBDOMAIN` is pulled directly from Doppler.

### 2. Normal Way (Direct CI Secrets / Variables)
If `DOPPLER_TOKEN` is omitted, NextDeploy automatically falls back to standard provider secrets:

#### Cloudflare Target
- `CLOUDFLARE_API_TOKEN` (required) — Cloudflare API token with Workers, Pages, and R2 permissions.
- `CLOUDFLARE_ACCOUNT_ID` (optional) — Cloudflare Account ID (auto-discovered if unset).
- `NEXTDEPLOY_CF_STATE_KEY` (optional) — Secret key for encrypted remote resource state sync across CI and team members.
- `R2_ACCESS_KEY_ID` (optional) — S3-compatible R2 access key if uploading assets via S3 credentials.
- `R2_SECRET_ACCESS_KEY` (optional) — S3-compatible R2 secret key if uploading assets via S3 credentials.
- `CLOUDFLARE_WORKERS_SUBDOMAIN` (required for `--preview`) — The account's workers.dev subdomain.

#### VPS Target
- `SSH_PRIVATE_KEY` (required) — Private SSH key matching the public key authorized on your VPS (`~/.ssh/authorized_keys`).
- `SSH_KNOWN_HOSTS` (optional) — Remote server host key entries to pin in CI and prevent MITM attacks.
- `SSH_KEY_PASSPHRASE` (optional) — Passphrase if your SSH private key is encrypted.
- `NEXTDEPLOY_STRICT_HOST_KEY` (optional) — Set to `1` to fail closed if the host key is not pinned in `known_hosts`.

---

## Generated Pipelines

### GitHub Actions (`.github/workflows/nextdeploy.yml`)

The generated GitHub Actions workflow is **target-aware at runtime**: it inspects `nextdeploy.yml` on the CI runner and executes the matching deployment steps (Cloudflare, AWS, or VPS) without needing to re-scaffold CI when retargeting.

```yaml
# Generated by nextdeploy generate-ci
name: deploy

on:
  push:
    branches: [main]
  workflow_dispatch:

concurrency:
  group: nextdeploy-${{ github.ref }}
  cancel-in-progress: false

permissions:
  contents: read

jobs:
  deploy:
    runs-on: ubuntu-latest
    timeout-minutes: 30
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-node@v4
        with:
          node-version: 22

      - name: Install app dependencies
        run: |
          if [ -f bun.lockb ] || [ -f bun.lock ]; then
            curl -fsSL https://bun.sh/install | bash
            echo "$HOME/.bun/bin" >> "$GITHUB_PATH"
            "$HOME/.bun/bin/bun" install --frozen-lockfile
          elif [ -f pnpm-lock.yaml ]; then
            corepack enable
            pnpm install --frozen-lockfile
          else
            npm ci
          fi

      - name: Install nextdeploy CLI (pinned)
        run: |
          # Pinned release download with sha256 checksum verification
          ...

      - name: Detect deployment type
        id: detect
        run: |
          if grep -qE '^[[:space:]]*provider:[[:space:]]*["'\'']?cloudflare["'\'']?' nextdeploy.yml; then
            type=cloudflare
          elif grep -qE '^[[:space:]]*provider:[[:space:]]*["'\'']?aws["'\'']?' nextdeploy.yml; then
            type=aws
          elif grep -qE '^[[:space:]]*(target_type|target):[[:space:]]*["'\'']?vps["'\'']?' nextdeploy.yml || grep -qE '^[[:space:]]*(deployment|server|servers):' nextdeploy.yml || ! grep -qE '^[[:space:]]*provider:' nextdeploy.yml; then
            type=vps
          else
            echo "::error::Could not determine deployment type from nextdeploy.yml"; exit 1
          fi
          echo "type=$type" >> "$GITHUB_OUTPUT"

      - name: Deploy to Cloudflare
        if: steps.detect.outputs.type == 'cloudflare'
        env:
          DOPPLER_TOKEN: ${{ secrets.DOPPLER_TOKEN }}
          CLOUDFLARE_API_TOKEN: ${{ secrets.CLOUDFLARE_API_TOKEN }}
          CLOUDFLARE_ACCOUNT_ID: ${{ secrets.CLOUDFLARE_ACCOUNT_ID }}
          NEXTDEPLOY_CF_STATE_KEY: ${{ secrets.NEXTDEPLOY_CF_STATE_KEY }}
        run: |
          if [ -n "${DOPPLER_TOKEN:-}" ]; then
            if ! command -v doppler >/dev/null 2>&1; then
              (curl -Ls --tlsv1.2 --proto "=https" --retry 3 https://cli.doppler.com/install.sh || wget -t 3 -qO- https://cli.doppler.com/install.sh) | sudo sh
            fi
            doppler run -- nextdeploy apply --yes
            doppler run -- nextdeploy ship --no-provision --verify
          else
            nextdeploy apply --yes
            nextdeploy ship --no-provision --verify
          fi

      - name: Deploy to VPS
        if: steps.detect.outputs.type == 'vps'
        env:
          DOPPLER_TOKEN: ${{ secrets.DOPPLER_TOKEN }}
          SSH_PRIVATE_KEY: ${{ secrets.SSH_PRIVATE_KEY }}
          SSH_KNOWN_HOSTS: ${{ secrets.SSH_KNOWN_HOSTS }}
          SSH_KEY_PASSPHRASE: ${{ secrets.SSH_KEY_PASSPHRASE }}
          NEXTDEPLOY_STRICT_HOST_KEY: ${{ secrets.NEXTDEPLOY_STRICT_HOST_KEY }}
        run: |
          mkdir -p ~/.ssh
          chmod 700 ~/.ssh
          if [ -n "${DOPPLER_TOKEN:-}" ]; then
            if ! command -v doppler >/dev/null 2>&1; then
              (curl -Ls --tlsv1.2 --proto "=https" --retry 3 https://cli.doppler.com/install.sh || wget -t 3 -qO- https://cli.doppler.com/install.sh) | sudo sh
            fi
            KEY=$(doppler secrets get SSH_PRIVATE_KEY --plain 2>/dev/null || echo "$SSH_PRIVATE_KEY")
            HOSTS=$(doppler secrets get SSH_KNOWN_HOSTS --plain 2>/dev/null || echo "$SSH_KNOWN_HOSTS")
          else
            KEY="$SSH_PRIVATE_KEY"
            HOSTS="$SSH_KNOWN_HOSTS"
          fi
          if [ -n "$KEY" ]; then
            echo "$KEY" > ~/.ssh/id_ed25519
            chmod 600 ~/.ssh/id_ed25519
            cp ~/.ssh/id_ed25519 ~/.ssh/id_rsa
            chmod 600 ~/.ssh/id_rsa
          fi
          if [ -n "$HOSTS" ]; then
            echo "$HOSTS" >> ~/.ssh/known_hosts
            chmod 600 ~/.ssh/known_hosts
          fi
          if [ -n "${DOPPLER_TOKEN:-}" ]; then
            doppler run -- nextdeploy ship --verify
          else
            nextdeploy ship --verify
          fi
```

### GitLab CI (`.gitlab-ci.yml`)

Generate with:
```bash
nextdeploy generate-ci --provider=gitlab-ci
```

This generates a `.gitlab-ci.yml` pipeline with:
- `node:22-bookworm` container runtime.
- Pinned NextDeploy CLI binary with SHA256 checksum verification.
- Runtime target autodetection for Cloudflare, AWS, and VPS.
- Automated Doppler-first execution when `DOPPLER_TOKEN` is configured in GitLab CI/CD Variables.
- SSH key directory and permission hardening (`chmod 700 ~/.ssh`, `chmod 600`).

