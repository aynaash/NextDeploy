# Configuration Guide

Complete reference for `nextdeploy.yml` configuration.

## File Structure

```yaml
version: "1.0"
app: { }
docker: { }
deployment: { }
database: { }
monitoring: { }
backup: { }
ssl: { }
```

## App Configuration

```yaml
app:
  name: my-app              # Unique app name
  environment: production   # development | staging | production
  domain: app.example.com   # Your domain
  port: 3000               # Internal app port
```

### Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | ✅ | Unique identifier for your app |
| `environment` | string | ✅ | Deployment environment |
| `domain` | string | ⚠️ | Required for HTTPS setup |
| `port` | number | ✅ | Port your Next.js app listens on |

## Docker Configuration

```yaml
docker:
  image: username/my-app    # Image name
  registry: ghcr.io         # ghcr.io | docker.io | ECR
  push: true                # Auto-push after build
  target: production        # Build target (multi-stage)
  platform: linux/amd64     # Target platform
  alwaysPull: true          # Always pull base images
```

### Registry Options

#### GitHub Container Registry (ghcr.io)

```yaml
docker:
  registry: ghcr.io
  username: your-github-username
  # Use GitHub Personal Access Token in Doppler
```

#### AWS ECR

```yaml
docker:
  registry: 123456789.dkr.ecr.us-east-1.amazonaws.com
  # AWS credentials in Doppler or environment
```

#### Docker Hub

```yaml
docker:
  registry: docker.io
  username: your-dockerhub-username
  # Password in Doppler
```

## Deployment Configuration

```yaml
deployment:
  server:
    host: 192.0.2.123           # VPS IP address
    user: deploy                # SSH user
    ssh_key: ~/.ssh/id_rsa      # Path to SSH key
    use_sudo: false             # Use sudo for Docker commands
  
  container:
    name: my-app                # Container name
    restart: always             # always | on-failure | unless-stopped
    env_file: .env              # Environment file (optional)
    volumes:
      - ./data:/app/data        # Volume mounts
    ports:
      - "80:3000"               # Port mapping
    
    healthcheck:
      path: /api/health         # Health check endpoint
      interval: 30s             # Check interval
      timeout: 5s               # Timeout per check
      retries: 3                # Retries before restart
```

### SSH Configuration

#### Using SSH Key

```yaml
deployment:
  server:
    ssh_key: ~/.ssh/nextdeploy_rsa
```

#### Using SSH Agent

```yaml
deployment:
  server:
    ssh_key: agent  # Use SSH agent
```

### Container Restart Policies

| Policy | Behavior |
|--------|----------|
| `always` | Always restart, even after manual stop |
| `on-failure` | Restart only on failure (non-zero exit) |
| `unless-stopped` | Restart unless manually stopped |
| `no` | Never restart automatically |

## Database Configuration

```yaml
database:
  type: postgres              # postgres | mysql
  host: 192.0.2.124          # Database host
  port: 5432                 # Database port
  username: dbuser           # Database user
  password: secret           # Use Doppler for this!
  name: myapp_db             # Database name
  migrate_on_deploy: true    # Run migrations on deploy
```

### Supported Databases

- **PostgreSQL** - Recommended
- **MySQL/MariaDB** - Supported
- **MongoDB** - Coming soon
- **Redis** - For caching/sessions

## Monitoring Configuration

```yaml
monitoring:
  enabled: true
  cpu_threshold: 80          # Alert at 80% CPU
  memory_threshold: 75       # Alert at 75% memory
  disk_threshold: 90         # Alert at 90% disk
  
  alert:
    email: ops@example.com
    slack_webhook: https://hooks.slack.com/...
    notify_on:
      - crash
      - healthcheck_failed
      - high_cpu
      - high_memory
```

## Backup Configuration

```yaml
backup:
  enabled: true
  frequency: daily           # hourly | daily | weekly
  retention_days: 7          # Keep backups for 7 days
  
  storage:
    provider: s3             # s3 | gcs | azure
    bucket: my-backups
    region: us-east-1
    access_key: YOUR_KEY     # Use Doppler!
    secret_key: YOUR_SECRET  # Use Doppler!
```

## SSL Configuration

```yaml
ssl:
  enabled: true
  provider: letsencrypt      # letsencrypt | custom
  email: admin@example.com   # For renewal notices
  auto_renew: true           # Auto-renew certificates
```

### Custom SSL Certificates

```yaml
ssl:
  enabled: true
  provider: custom
  cert_path: /path/to/cert.pem
  key_path: /path/to/key.pem
```

## Environment Variables

### Using Doppler (Recommended)

```yaml
secrets:
  provider: doppler
  project: my-app
  config: production
```

### Using .env File

```yaml
deployment:
  container:
    env_file: .env.production
```

### Inline Variables

```yaml
deployment:
  container:
    environment:
      NODE_ENV: production
      API_URL: https://api.example.com
```

## Complete Example

```yaml
version: "1.0"

app:
  name: my-production-app
  environment: production
  domain: app.example.com
  port: 3000

docker:
  image: mycompany/my-app
  registry: ghcr.io
  push: true
  platform: linux/amd64
  alwaysPull: true

deployment:
  server:
    host: 192.0.2.123
    user: deploy
    ssh_key: ~/.ssh/id_rsa
    use_sudo: false
  
  container:
    name: my-app
    restart: always
    ports:
      - "80:3000"
    healthcheck:
      path: /api/health
      interval: 30s
      timeout: 5s
      retries: 3

database:
  type: postgres
  host: db.example.com
  port: 5432
  username: appuser
  name: myapp_production
  migrate_on_deploy: true

monitoring:
  enabled: true
  cpu_threshold: 80
  memory_threshold: 75
  alert:
    email: ops@example.com

ssl:
  enabled: true
  provider: letsencrypt
  email: admin@example.com
  auto_renew: true
```

## Validation

Validate your configuration:

```bash
nextdeploy validate
```

## Environment-Specific Configs

Create multiple config files:

```
nextdeploy.yml           # Default (development)
nextdeploy.staging.yml   # Staging
nextdeploy.prod.yml      # Production
```

Use with:

```bash
nextdeploy build --config nextdeploy.prod.yml
```

---

**Next**: [Secret Management →](./secrets.md)
