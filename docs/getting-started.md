# Getting Started with NextDeploy

Welcome to NextDeploy! This guide will help you deploy your first Next.js application in under 5 minutes.

## What is NextDeploy?

NextDeploy is a self-hosted deployment platform built exclusively for Next.js applications. Think of it as your own personal Vercel, but with:

- ✅ **Full control** - Deploy to any VPS you own
- ✅ **Zero lock-in** - No vendor dependencies
- ✅ **Transparent** - See every step of the deployment
- ✅ **Cost-effective** - Pay only for your VPS ($5-20/month)

## Prerequisites

Before you begin, make sure you have:

- **A Next.js project** (any version)
- **Docker installed** on your local machine
- **A VPS server** (DigitalOcean, Hetzner, AWS, etc.)
- **SSH access** to your server

## Installation

### CLI (All Platforms)

Install the NextDeploy CLI using webi:

```bash
curl https://webi.sh/nextdeploy | sh
```

Or download directly from [GitHub Releases](https://github.com/aynaash/nextdeploy/releases).

### Daemon (Linux Servers Only)

On your VPS, install the daemon:

```bash
curl https://webi.sh/nextdeployd | sh
sudo systemctl enable nextdeployd
sudo systemctl start nextdeployd
```

## Quick Start

### 1. Initialize Your Project

Navigate to your Next.js project and run:

```bash
cd my-nextjs-app
nextdeploy init
```

This creates:
- `nextdeploy.yml` - Configuration file
- `Dockerfile` - Optimized for Next.js

### 2. Configure Deployment

Edit `nextdeploy.yml`:

```yaml
version: "1.0"

app:
  name: my-app
  environment: production
  domain: myapp.com
  port: 3000

docker:
  image: username/my-app
  registry: ghcr.io
  push: true

deployment:
  server:
    host: 192.0.2.123  # Your VPS IP
    user: deploy
    ssh_key: ~/.ssh/id_rsa
```

### 3. Build Your Image

```bash
nextdeploy build
```

This will:
- Build an optimized Docker image
- Tag it with your git commit hash
- Optionally push to your registry

### 4. Test Locally

Before deploying, test your production build:

```bash
nextdeploy runimage
```

Your app will run at `http://localhost:3000` with production settings.

### 5. Deploy to Your Server

```bash
nextdeploy ship
```

This will:
- Transfer necessary files to your server
- Pull the Docker image
- Deploy the container
- Verify the deployment

### 6. Enable HTTPS (Optional)

```bash
nextdeploy ship --serve
```

This configures Caddy for automatic HTTPS with Let's Encrypt.

## Next Steps

- [Configuration Guide](./configuration.md) - Detailed config options
- [Secret Management](./secrets.md) - Using Doppler for secrets
- [Blue-Green Deployments](./blue-green.md) - Zero-downtime deploys
- [Monitoring](./monitoring.md) - Health checks and logs

## Getting Help

- **Documentation**: [Full docs](https://nextdeploy.dev/docs)
- **GitHub Issues**: [Report bugs](https://github.com/aynaash/nextdeploy/issues)
- **Community**: [Discussions](https://github.com/aynaash/nextdeploy/discussions)

## Common Issues

### Docker not found

Make sure Docker is installed and running:

```bash
docker --version
docker ps
```

### SSH connection failed

Verify your SSH key:

```bash
ssh -i ~/.ssh/id_rsa deploy@YOUR_VPS_IP
```

### Build fails

Check your Dockerfile and ensure all dependencies are listed in `package.json`.

---

**Next**: [Configuration Guide →](./configuration.md)
