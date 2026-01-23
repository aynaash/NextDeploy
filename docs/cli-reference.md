# CLI Reference

Complete command reference for the NextDeploy CLI.

## Global Flags

All commands support these flags:

| Flag | Short | Description |
|------|-------|-------------|
| `--config` | `-c` | Path to config file (default: `nextdeploy.yml`) |
| `--verbose` | `-v` | Enable verbose logging |
| `--help` | `-h` | Show help for command |
| `--version` | | Show version information |

## Commands

### `nextdeploy init`

Initialize a Next.js project for deployment.

```bash
nextdeploy init [flags]
```

**What it does**:
- Creates `nextdeploy.yml` configuration
- Generates optimized `Dockerfile`
- Sets up `.dockerignore`

**Flags**:
- `--template <name>` - Use a specific template
- `--force` - Overwrite existing files

**Example**:
```bash
nextdeploy init
nextdeploy init --template minimal
```

---

### `nextdeploy build`

Build a Docker image for your application.

```bash
nextdeploy build [flags]
```

**What it does**:
- Builds Docker image with smart tagging
- Tags with git commit hash
- Optionally pushes to registry

**Flags**:
- `--no-cache` - Build without cache
- `--push` - Push to registry after build
- `--provision-ecr-user` - Create ECR user (AWS only)
- `--fresh` - Delete existing ECR user and create new one

**Examples**:
```bash
# Basic build
nextdeploy build

# Build and push
nextdeploy build --push

# Production build (no cache)
NODE_ENV=production nextdeploy build --no-cache
```

**Output**:
```
Building myapp:abc123...
✓ Image built successfully
✓ Tagged as myapp:abc123
```

---

### `nextdeploy runimage`

Test your production build locally.

```bash
nextdeploy runimage [flags]
```

**What it does**:
- Runs the built Docker image locally
- Injects secrets from Doppler
- Exposes on localhost

**Flags**:
- `--port <number>` - Custom port (default: 3000)
- `--prod` - Use production secrets

**Examples**:
```bash
# Run with dev secrets
nextdeploy runimage

# Run with production secrets
nextdeploy runimage --prod

# Custom port
nextdeploy runimage --port 8080
```

---

### `nextdeploy ship`

Deploy your application to a server.

```bash
nextdeploy ship [flags]
```

**What it does**:
1. Verifies server connectivity
2. Transfers configuration files
3. Pulls Docker image
4. Deploys container
5. Verifies deployment

**Flags**:
- `--serve` | `-s` - Configure Caddy for HTTPS
- `--dry-run` | `-d` - Simulate deployment
- `--new` | `-n` - New application deployment
- `--bluegreen` | `-b` - Use blue-green deployment
- `--credentials` | `-c` - Use registry credentials

**Examples**:
```bash
# Basic deployment
nextdeploy ship

# With HTTPS setup
nextdeploy ship --serve

# Blue-green deployment
nextdeploy ship --bluegreen

# Dry run (test without deploying)
nextdeploy ship --dry-run
```

**Output**:
```
=== PHASE 1: Pre-deployment checks ===
✓ Server connectivity verified
✓ Docker available

=== PHASE 2: File transfers ===
✓ Configuration transferred

=== PHASE 3: Container deployment ===
✓ Image pulled
✓ Container started

=== PHASE 4: Post-deployment verification ===
✓ Health check passed

🎉 Deployment completed successfully!
```

---

### `nextdeploy provision`

Provision a new VPS server.

```bash
nextdeploy provision [flags]
```

**What it does**:
- Installs Docker
- Configures firewall
- Sets up SSH keys
- Installs nextdeployd daemon

**Flags**:
- `--provider <name>` - Cloud provider (digitalocean, hetzner, aws)
- `--region <name>` - Server region
- `--size <name>` - Server size

**Examples**:
```bash
# Interactive provisioning
nextdeploy provision

# Provision on DigitalOcean
nextdeploy provision --provider digitalocean --region nyc3 --size s-1vcpu-1gb
```

---

### `nextdeploy secrets`

Manage encrypted secrets.

```bash
nextdeploy secrets [command]
```

**Commands**:
- `encrypt` - Encrypt .env file
- `decrypt` - Decrypt secrets
- `rotate` - Rotate encryption key

**Examples**:
```bash
# Encrypt secrets
nextdeploy secrets encrypt

# Decrypt for local use
nextdeploy secrets decrypt
```

---

### `nextdeploy logs`

View application logs.

```bash
nextdeploy logs [flags]
```

**Flags**:
- `--follow` | `-f` - Follow log output
- `--tail <number>` - Number of lines to show
- `--since <time>` - Show logs since timestamp

**Examples**:
```bash
# View last 100 lines
nextdeploy logs

# Follow logs in real-time
nextdeploy logs --follow

# Last 50 lines
nextdeploy logs --tail 50

# Logs from last hour
nextdeploy logs --since 1h
```

---

### `nextdeploy status`

Check deployment status.

```bash
nextdeploy status
```

**Output**:
```
Application: myapp
Status: running
Uptime: 2d 5h 32m
Health: healthy
CPU: 12%
Memory: 256MB / 1GB
Restarts: 0
```

---

### `nextdeploy restart`

Restart the application.

```bash
nextdeploy restart
```

**What it does**:
- Gracefully stops container
- Starts new container
- Verifies health

---

### `nextdeploy stop`

Stop the application.

```bash
nextdeploy stop
```

---

### `nextdeploy start`

Start the application.

```bash
nextdeploy start
```

---

### `nextdeploy rollback`

Rollback to previous deployment.

```bash
nextdeploy rollback [version]
```

**Examples**:
```bash
# Rollback to previous version
nextdeploy rollback

# Rollback to specific version
nextdeploy rollback v1.2.3
```

---

## Environment Variables

### Build-time

```bash
# Use production mode
NODE_ENV=production nextdeploy build

# Custom registry
DOCKER_REGISTRY=ghcr.io nextdeploy build
```

### Runtime

```bash
# Custom config file
NEXTDEPLOY_CONFIG=nextdeploy.prod.yml nextdeploy ship

# Debug mode
DEBUG=true nextdeploy ship
```

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | General error |
| 2 | Configuration error |
| 3 | Build failed |
| 4 | Deployment failed |
| 5 | Connection error |

## Configuration File

Default locations (in order):
1. `./nextdeploy.yml`
2. `./nextdeploy.yaml`
3. `./.nextdeploy/config.yml`

Override with:
```bash
nextdeploy build --config custom.yml
```

## Examples

### Complete Deployment Workflow

```bash
# 1. Initialize
nextdeploy init

# 2. Configure
vim nextdeploy.yml

# 3. Build
nextdeploy build

# 4. Test locally
nextdeploy runimage

# 5. Deploy
nextdeploy ship --serve

# 6. Check status
nextdeploy status

# 7. View logs
nextdeploy logs --follow
```

### CI/CD Pipeline

```bash
# In GitHub Actions
- name: Build
  run: nextdeploy build --push

- name: Deploy to staging
  run: nextdeploy ship --config nextdeploy.staging.yml

- name: Deploy to production
  if: github.ref == 'refs/heads/main'
  run: nextdeploy ship --config nextdeploy.prod.yml
```

### Blue-Green Deployment

```bash
# Deploy new version
nextdeploy ship --bluegreen --new

# Traffic is automatically switched
# Old version kept running for rollback

# If issues, rollback
nextdeploy rollback
```

---

**Next**: [Daemon Reference →](./daemon-reference.md)
