# Troubleshooting Guide

Common issues and solutions when using NextDeploy.

## Installation Issues

### Docker not found

**Error**:
```
Error: docker is not installed or not functioning
```

**Solution**:
```bash
# Check Docker installation
docker --version

# Start Docker (macOS/Windows)
open -a Docker

# Start Docker (Linux)
sudo systemctl start docker

# Verify Docker is running
docker ps
```

### Permission denied (Docker)

**Error**:
```
permission denied while trying to connect to the Docker daemon socket
```

**Solution**:
```bash
# Add user to docker group (Linux)
sudo usermod -aG docker $USER

# Log out and back in, or run:
newgrp docker

# Verify
docker ps
```

### Webi install fails

**Error**:
```
curl: (7) Failed to connect to webi.sh
```

**Solution**:
```bash
# Check internet connection
ping google.com

# Try alternative installation
curl -L https://github.com/aynaash/nextdeploy/releases/latest/download/nextdeploy-$(uname -s)-$(uname -m) -o nextdeploy
chmod +x nextdeploy
sudo mv nextdeploy /usr/local/bin/
```

---

## Build Issues

### Build fails with "no Dockerfile found"

**Error**:
```
Error: no Dockerfile found in current directory
```

**Solution**:
```bash
# Initialize project first
nextdeploy init

# Or create Dockerfile manually
# See: docs/dockerfile-guide.md
```

### Out of disk space

**Error**:
```
Error: no space left on device
```

**Solution**:
```bash
# Clean up Docker
docker system prune -a

# Remove unused images
docker image prune -a

# Check disk space
df -h
```

### Build args not working

**Error**:
```
ARG NODE_ENV not found
```

**Solution**:

Update your Dockerfile:
```dockerfile
# Define ARG before FROM
ARG NODE_ENV=production

FROM node:20-alpine

# Use the ARG
ENV NODE_ENV=${NODE_ENV}
```

### npm install fails in Docker

**Error**:
```
npm ERR! network request failed
```

**Solution**:

Add to Dockerfile:
```dockerfile
# Set npm registry
RUN npm config set registry https://registry.npmjs.org/

# Or use yarn
RUN yarn install --frozen-lockfile
```

---

## Deployment Issues

### SSH connection failed

**Error**:
```
Error: SSH connection failed: Permission denied (publickey)
```

**Solution**:
```bash
# Test SSH connection
ssh -i ~/.ssh/id_rsa deploy@YOUR_SERVER_IP

# Generate new SSH key if needed
ssh-keygen -t ed25519 -C "your_email@example.com"

# Copy key to server
ssh-copy-id -i ~/.ssh/id_rsa deploy@YOUR_SERVER_IP

# Update nextdeploy.yml with correct key path
deployment:
  server:
    ssh_key: ~/.ssh/id_rsa
```

### Container fails to start

**Error**:
```
Error: container exited with code 1
```

**Solution**:
```bash
# Check container logs on server
ssh deploy@YOUR_SERVER docker logs CONTAINER_NAME

# Common issues:
# 1. Port already in use
# 2. Missing environment variables
# 3. Database connection failed

# Test locally first
nextdeploy runimage
```

### Port already in use

**Error**:
```
Error: bind: address already in use
```

**Solution**:
```bash
# Find process using port
sudo lsof -i :3000

# Kill the process
sudo kill -9 PID

# Or use different port in nextdeploy.yml
deployment:
  container:
    ports:
      - "8080:3000"
```

### Image pull failed

**Error**:
```
Error: failed to pull image: unauthorized
```

**Solution**:
```bash
# Login to registry
docker login ghcr.io

# Or use credentials flag
nextdeploy ship --credentials

# For ECR, provision user
nextdeploy build --provision-ecr-user
```

---

## Secret Management Issues

### Doppler token invalid

**Error**:
```
Error: invalid Doppler token
```

**Solution**:
```bash
# Re-login to Doppler
doppler login

# Verify setup
doppler setup

# Check token
doppler configure get token
```

### Secrets not loading

**Error**:
```
Error: DATABASE_URL is not defined
```

**Solution**:
```bash
# Verify secrets exist
doppler secrets

# Check environment
doppler configure get

# Run with Doppler
doppler run -- nextdeploy build

# Or export secrets
eval $(doppler secrets download --no-file --format env-no-quotes)
```

### Master key not found

**Error**:
```
Error: master.key not found
```

**Solution**:
```bash
# Generate new master key
nextdeploy secrets

# Or copy from backup
cp ~/backups/master.key ~/.nextdeploy/myapp/master.key
```

---

## Runtime Issues

### Application crashes on startup

**Symptoms**:
- Container starts then immediately exits
- Health checks fail
- 502 Bad Gateway

**Debug steps**:
```bash
# 1. Check logs
nextdeploy logs --tail 100

# 2. Test locally
nextdeploy runimage

# 3. Check environment variables
ssh deploy@SERVER docker exec CONTAINER env

# 4. Check database connection
ssh deploy@SERVER docker exec CONTAINER nc -zv db-host 5432
```

### High memory usage

**Symptoms**:
- Container gets killed
- OOM errors in logs

**Solution**:
```bash
# Check memory usage
nextdeploy status

# Increase container memory limit
# In nextdeploy.yml:
deployment:
  container:
    memory: 2g
    memory_swap: 2g

# Or upgrade server
```

### Slow response times

**Debug steps**:
```bash
# 1. Check server resources
ssh deploy@SERVER htop

# 2. Check database performance
# 3. Enable Next.js profiling
# 4. Check network latency

# Optimize:
# - Enable caching
# - Use CDN for static assets
# - Optimize database queries
```

---

## Health Check Issues

### Health check always failing

**Error**:
```
Health check failed: connection refused
```

**Solution**:

1. **Create health endpoint** (`app/api/health/route.ts`):
```typescript
export async function GET() {
  return Response.json({ status: 'ok' });
}
```

2. **Update nextdeploy.yml**:
```yaml
deployment:
  container:
    healthcheck:
      path: /api/health
      interval: 30s
      timeout: 5s
      retries: 3
```

3. **Test endpoint**:
```bash
curl http://localhost:3000/api/health
```

---

## Database Issues

### Connection refused

**Error**:
```
Error: connect ECONNREFUSED 127.0.0.1:5432
```

**Solution**:
```bash
# Check database is running
ssh deploy@SERVER systemctl status postgresql

# Check connection from container
ssh deploy@SERVER docker exec CONTAINER nc -zv db-host 5432

# Update DATABASE_URL in Doppler
# Use server's IP, not localhost
DATABASE_URL=postgresql://user:pass@192.168.1.10:5432/db
```

### Migration fails

**Error**:
```
Error: relation "users" does not exist
```

**Solution**:
```bash
# Run migrations manually
ssh deploy@SERVER docker exec CONTAINER npm run migrate

# Or enable auto-migration
# In nextdeploy.yml:
database:
  migrate_on_deploy: true
```

---

## SSL/HTTPS Issues

### Certificate not issued

**Error**:
```
Error: failed to obtain certificate
```

**Solution**:
```bash
# Check DNS is pointing to server
dig +short yourdomain.com

# Verify port 80 is open
ssh deploy@SERVER sudo ufw status

# Check Caddy logs
ssh deploy@SERVER journalctl -u caddy -f

# Manually trigger certificate
ssh deploy@SERVER sudo caddy reload --config /etc/caddy/Caddyfile
```

### Mixed content warnings

**Error**:
```
Mixed Content: The page was loaded over HTTPS, but requested an insecure resource
```

**Solution**:

Update `next.config.js`:
```javascript
module.exports = {
  async headers() {
    return [
      {
        source: '/:path*',
        headers: [
          {
            key: 'Content-Security-Policy',
            value: "upgrade-insecure-requests"
          }
        ]
      }
    ];
  }
};
```

---

## Performance Issues

### Slow builds

**Solutions**:
```bash
# 1. Use build cache
nextdeploy build  # Don't use --no-cache

# 2. Optimize Dockerfile
# - Use multi-stage builds
# - Copy package.json before source
# - Use .dockerignore

# 3. Use faster registry
# - Use local registry
# - Use regional mirror
```

### Slow deployments

**Solutions**:
```bash
# 1. Use image registry
# Push once, pull on server
nextdeploy build --push

# 2. Optimize image size
# - Use alpine base images
# - Remove dev dependencies
# - Minimize layers

# 3. Use blue-green deployment
# Zero downtime
nextdeploy ship --bluegreen
```

---

## Getting Help

### Enable debug mode

```bash
# Verbose output
nextdeploy build --verbose

# Debug mode
DEBUG=true nextdeploy ship
```

### Collect diagnostic info

```bash
# System info
nextdeploy version
docker version
docker info

# Configuration
cat nextdeploy.yml

# Logs
nextdeploy logs --tail 200 > logs.txt
```

### Report a bug

1. Check [existing issues](https://github.com/aynaash/nextdeploy/issues)
2. Create new issue with:
   - NextDeploy version
   - Operating system
   - Steps to reproduce
   - Error messages
   - Diagnostic info

---

**Need more help?** [Open an issue](https://github.com/aynaash/nextdeploy/issues/new)
