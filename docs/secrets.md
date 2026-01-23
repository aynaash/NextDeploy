# Secret Management with Doppler

NextDeploy is **Doppler-first** for managing secrets. No `.env` files, no git commits with secrets, just secure, encrypted secret management.

## Why Doppler?

- 🔐 **Encrypted** - Secrets encrypted at rest and in transit
- 🌍 **Environment-scoped** - dev, staging, prod configs
- 👥 **Team-friendly** - Share secrets securely
- 🔄 **Auto-sync** - Update secrets without redeploying
- 📊 **Audit logs** - Track who changed what

## Setup

### 1. Create Doppler Account

Sign up at [doppler.com](https://doppler.com) (free tier available).

### 2. Install Doppler CLI

```bash
# macOS
brew install dopplerhq/cli/doppler

# Linux
curl -Ls https://cli.doppler.com/install.sh | sh

# Windows
scoop install doppler
```

### 3. Login

```bash
doppler login
```

### 4. Create Project

```bash
doppler projects create my-app
```

### 5. Set Up Environments

```bash
# Development
doppler setup --project my-app --config dev

# Staging
doppler setup --project my-app --config stg

# Production
doppler setup --project my-app --config prd
```

## Adding Secrets

### Via CLI

```bash
# Switch to production config
doppler setup --project my-app --config prd

# Add secrets
doppler secrets set DATABASE_URL="postgresql://..."
doppler secrets set API_KEY="sk_live_..."
doppler secrets set STRIPE_SECRET="sk_..."
```

### Via Dashboard

1. Go to [dashboard.doppler.com](https://dashboard.doppler.com)
2. Select your project
3. Select environment (dev/stg/prd)
4. Click "Add Secret"
5. Enter name and value

## Using Secrets Locally

### Development

```bash
# Run Next.js with Doppler
doppler run -- npm run dev

# Or export to shell
eval $(doppler secrets download --no-file --format env-no-quotes)
npm run dev
```

### Build with Secrets

```bash
# Build Docker image with Doppler secrets
doppler run -- nextdeploy build
```

## Using Secrets in Production

### Method 1: Doppler Service Token (Recommended)

1. **Generate service token**:
   ```bash
   doppler configs tokens create production --project my-app
   ```

2. **Add to server**:
   ```bash
   ssh deploy@your-server
   echo "DOPPLER_TOKEN=dp.st.xxx" | sudo tee -a /etc/environment
   ```

3. **Update nextdeploy.yml**:
   ```yaml
   secrets:
     provider: doppler
     project: my-app
     config: prd
   ```

4. **Deploy**:
   ```bash
   nextdeploy ship
   ```

### Method 2: Encrypted .env File

```bash
# Encrypt secrets
nextdeploy secrets

# This creates:
# - .env.encrypted
# - master.key (DO NOT COMMIT!)
```

Add to `.gitignore`:
```
.env
.env.local
master.key
```

## Environment-Specific Secrets

### Development

```bash
doppler setup --config dev
doppler secrets set DATABASE_URL="postgresql://localhost/myapp_dev"
doppler secrets set API_URL="http://localhost:3000"
```

### Staging

```bash
doppler setup --config stg
doppler secrets set DATABASE_URL="postgresql://staging-db/myapp_stg"
doppler secrets set API_URL="https://staging.myapp.com"
```

### Production

```bash
doppler setup --config prd
doppler secrets set DATABASE_URL="postgresql://prod-db/myapp_prod"
doppler secrets set API_URL="https://api.myapp.com"
```

## Common Secrets

### Database

```bash
doppler secrets set DATABASE_URL="postgresql://user:pass@host:5432/db"
doppler secrets set REDIS_URL="redis://localhost:6379"
```

### Authentication

```bash
doppler secrets set NEXTAUTH_SECRET="your-secret-here"
doppler secrets set NEXTAUTH_URL="https://myapp.com"
doppler secrets set GITHUB_CLIENT_ID="..."
doppler secrets set GITHUB_CLIENT_SECRET="..."
```

### APIs

```bash
doppler secrets set STRIPE_SECRET_KEY="sk_live_..."
doppler secrets set STRIPE_PUBLISHABLE_KEY="pk_live_..."
doppler secrets set SENDGRID_API_KEY="SG...."
doppler secrets set AWS_ACCESS_KEY_ID="..."
doppler secrets set AWS_SECRET_ACCESS_KEY="..."
```

### Next.js Specific

```bash
doppler secrets set NEXT_PUBLIC_API_URL="https://api.myapp.com"
doppler secrets set NEXT_PUBLIC_STRIPE_KEY="pk_live_..."
```

> **Note**: `NEXT_PUBLIC_*` variables are embedded in the client bundle.

## Best Practices

### 1. Never Commit Secrets

```gitignore
# .gitignore
.env
.env.*
!.env.example
master.key
*.encrypted
```

### 2. Use .env.example

```bash
# .env.example
DATABASE_URL=postgresql://localhost/myapp_dev
API_KEY=your_api_key_here
STRIPE_SECRET=sk_test_...
```

### 3. Rotate Secrets Regularly

```bash
# Generate new secret
doppler secrets set API_KEY="new_key_here"

# Restart app to pick up changes
nextdeploy ship
```

### 4. Use Service Tokens for CI/CD

```yaml
# GitHub Actions
env:
  DOPPLER_TOKEN: ${{ secrets.DOPPLER_TOKEN }}

steps:
  - name: Install Doppler
    run: curl -Ls https://cli.doppler.com/install.sh | sh
  
  - name: Build with secrets
    run: doppler run -- nextdeploy build
```

### 5. Audit Secret Access

Check who accessed secrets:

```bash
doppler activity
```

## Troubleshooting

### Doppler not found

```bash
# Install Doppler CLI
curl -Ls https://cli.doppler.com/install.sh | sh
```

### Invalid token

```bash
# Re-login
doppler login

# Verify setup
doppler setup
```

### Secrets not loading

```bash
# Check current config
doppler configure get

# Download secrets to verify
doppler secrets download --no-file
```

### Permission denied

```bash
# Check project access
doppler projects

# Request access from project admin
```

## Alternative: Manual Encryption

If you can't use Doppler:

### Encrypt Secrets

```bash
nextdeploy secrets
```

This creates:
- `.env.encrypted` - Encrypted secrets
- `master.key` - Encryption key (keep safe!)

### Deploy with Encrypted Secrets

```bash
nextdeploy ship --secrets
```

### Decrypt on Server

The daemon automatically decrypts using the master key.

## Migration from .env

### 1. Export existing secrets

```bash
# From .env file
cat .env | doppler secrets upload
```

### 2. Verify

```bash
doppler secrets
```

### 3. Remove .env

```bash
rm .env
echo ".env" >> .gitignore
```

### 4. Update scripts

```json
{
  "scripts": {
    "dev": "doppler run -- next dev",
    "build": "doppler run -- next build"
  }
}
```

## Doppler + NextDeploy Workflow

```bash
# 1. Set secrets in Doppler
doppler secrets set DATABASE_URL="..."

# 2. Build with secrets
doppler run -- nextdeploy build

# 3. Deploy (secrets auto-injected)
nextdeploy ship

# 4. Update secret (no redeploy needed)
doppler secrets set API_KEY="new_key"

# 5. Restart container to pick up changes
nextdeploy restart
```

---

**Next**: [Blue-Green Deployments →](./blue-green.md)
