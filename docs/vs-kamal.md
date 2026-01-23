# NextDeploy vs Kamal

NextDeploy is inspired by [Kamal](https://kamal-deploy.org/) (formerly MRSK) but specialized for Next.js deployments.

## Similarities

Both NextDeploy and Kamal:
- ✅ Deploy to your own servers via SSH
- ✅ Use Docker for containerization
- ✅ Support zero-downtime deployments
- ✅ Provide simple CLI interfaces
- ✅ Avoid vendor lock-in
- ✅ Are open source

## Key Differences

### 1. **Framework Focus**

| Feature | NextDeploy | Kamal |
|---------|-----------|-------|
| **Target** | Next.js only | Any framework (Rails, Go, etc.) |
| **Optimization** | Next.js-specific builds | Generic Docker |
| **SSR/ISR** | Native support | Manual configuration |
| **API Routes** | Automatic | Manual routing |

**Why it matters**: NextDeploy understands Next.js internals, so it can optimize builds, handle SSR correctly, and configure routing automatically.

### 2. **Secret Management**

| Feature | NextDeploy | Kamal |
|---------|-----------|-------|
| **Primary method** | Doppler (first-class) | Environment variables |
| **Encryption** | Built-in ECDH | Manual |
| **Rotation** | Automatic | Manual |
| **Scoping** | dev/staging/prod | Manual |

**Example**:

**NextDeploy**:
```bash
# Secrets managed by Doppler
doppler secrets set DATABASE_URL="..."
nextdeploy ship  # Secrets auto-injected
```

**Kamal**:
```yaml
# deploy.yml
env:
  secret:
    - DATABASE_URL
# Must manage .env files manually
```

### 3. **Configuration**

**NextDeploy** (`nextdeploy.yml`):
```yaml
app:
  name: my-app
  domain: app.com
  port: 3000

docker:
  image: myapp
  registry: ghcr.io

deployment:
  server:
    host: 192.0.2.123
```

**Kamal** (`deploy.yml`):
```yaml
service: my-app
image: myapp

servers:
  web:
    - 192.0.2.123

registry:
  server: ghcr.io
```

Both are YAML-based, but NextDeploy is tailored for Next.js conventions.

### 4. **Reverse Proxy**

| Feature | NextDeploy | Kamal |
|---------|-----------|-------|
| **Default** | Caddy | Traefik |
| **HTTPS** | Automatic (Let's Encrypt) | Automatic |
| **Config** | Automatic for Next.js | Manual labels |

**NextDeploy** knows Next.js needs:
- Static file serving from `/_next/static`
- API routes at `/api/*`
- SSR for all other routes

**Kamal** requires manual Traefik labels for routing.

### 5. **Monitoring**

| Feature | NextDeploy | Kamal |
|---------|-----------|-------|
| **Health checks** | PM2-like, automatic | Manual configuration |
| **Auto-restart** | Built-in | Via Docker restart policy |
| **Metrics** | CPU, memory, disk | Via external tools |
| **Logs** | Aggregated, searchable | Docker logs |

### 6. **Build Process**

**NextDeploy**:
```bash
nextdeploy build
# - Detects Next.js version
# - Optimizes for standalone output
# - Handles static assets correctly
# - Tags with git commit hash
```

**Kamal**:
```bash
kamal build
# - Generic Docker build
# - Manual optimization needed
```

### 7. **Local Testing**

**NextDeploy**:
```bash
nextdeploy runimage
# Runs production build locally with real secrets
```

**Kamal**:
```bash
# No built-in local testing
# Must use docker run manually
```

## When to Use NextDeploy

Choose NextDeploy if you:
- ✅ **Only deploy Next.js** apps
- ✅ Want **Doppler integration** for secrets
- ✅ Need **automatic Next.js optimization**
- ✅ Want **PM2-like monitoring** built-in
- ✅ Prefer **opinionated, focused tools**

## When to Use Kamal

Choose Kamal if you:
- ✅ Deploy **multiple frameworks** (Rails, Go, etc.)
- ✅ Need **maximum flexibility**
- ✅ Want **battle-tested** deployment tool (from Basecamp)
- ✅ Prefer **generic, framework-agnostic** tools
- ✅ Already use **Traefik** for routing

## Migration from Kamal

If you're using Kamal for Next.js, migrating is straightforward:

### 1. Install NextDeploy

```bash
curl https://webi.sh/nextdeploy | sh
```

### 2. Convert Configuration

**Kamal** (`deploy.yml`):
```yaml
service: my-nextjs-app
image: user/my-app

servers:
  web:
    - 192.0.2.123

registry:
  server: ghcr.io
  username: user

env:
  secret:
    - DATABASE_URL
```

**NextDeploy** (`nextdeploy.yml`):
```yaml
app:
  name: my-nextjs-app
  domain: myapp.com
  port: 3000

docker:
  image: user/my-app
  registry: ghcr.io

deployment:
  server:
    host: 192.0.2.123
    user: deploy
    ssh_key: ~/.ssh/id_rsa
```

### 3. Move Secrets to Doppler

```bash
# Export from Kamal .env
cat .env | doppler secrets upload

# Remove .env file
rm .env
```

### 4. Deploy

```bash
nextdeploy build
nextdeploy ship
```

## Comparison Table

| Feature | NextDeploy | Kamal |
|---------|-----------|-------|
| **Framework** | Next.js only | Any |
| **Language** | Go | Ruby |
| **Config** | YAML | YAML |
| **Secrets** | Doppler-first | .env files |
| **Proxy** | Caddy | Traefik |
| **Monitoring** | Built-in | External |
| **Zero-downtime** | Blue-green | Rolling |
| **Local testing** | `runimage` | Manual |
| **Health checks** | Automatic | Manual |
| **Maturity** | New (2026) | Mature (2022) |
| **Community** | Growing | Large (Basecamp) |

## Philosophy

### Kamal
> "Deploy web apps anywhere from bare metal to cloud VMs using Docker"

**Approach**: Framework-agnostic, maximum flexibility

### NextDeploy
> "Deploy Next.js your way. No lock-in. Full control."

**Approach**: Next.js-specific, opinionated simplicity

## Credits

NextDeploy is inspired by Kamal's excellent work on making self-hosted deployments accessible. We owe a debt to the Kamal team for proving that simple, SSH-based deployments can work at scale.

**Differences**: We've specialized for Next.js to provide:
- Automatic optimization
- First-class secret management
- Built-in monitoring
- Next.js-aware routing

## Learn More

- **Kamal**: https://kamal-deploy.org/
- **NextDeploy**: https://nextdeploy.dev/
- **Kamal GitHub**: https://github.com/basecamp/kamal
- **NextDeploy GitHub**: https://github.com/aynaash/nextdeploy

---

**Both tools are excellent**. Choose based on your needs:
- **Multiple frameworks** → Kamal
- **Next.js only** → NextDeploy
