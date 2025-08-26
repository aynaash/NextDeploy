## 🛠️ NextDeploy CLI — DevOps for Developers Who Care

Tired of opaque platforms and complex CI/CD? **NextDeploy CLI** is your path to simplicity and control.

It deploys **Next.js apps directly to your own VPS** — with proper secrets management, real Docker, and minimal friction. You get full-stack control from local development to live production, all from your terminal.

---

### ⚙️ What It Does

*   🧱 **Builds** production-ready Docker images for your Next.js app.
*   🚀 **Deploys** your container to any VPS (Hetzner, DigitalOcean, AWS, bare metal) via SSH.
*   🔐 **Injects secrets** securely and dynamically directly from **Doppler**.
*   📦 **Ships** builds from your local machine or CI environment with full transparency.
*   📊 **Streams** logs and metrics directly from your running container.
*   🧪 **Runimage:** Test production-like builds locally using real secrets.

One tool. One config. No mandatory dashboards.

---

### 🔐 Doppler Integrated. No `.env` Files.

NextDeploy is built **Doppler-first**, eliminating the risk of unsecured `.env` files.

*   Load secrets dynamically at deploy time.
*   Secrets are encrypted and managed securely in Doppler.
*   No hardcoding, no manual copying, no risk of leaking.
*   Full environment scoping: dev, staging, prod.
*   Change a secret → restart container → done.
*   Works identically in CI or local development.

---

### 🚀 Quick Start

```bash
go install github.com/aynaash/nextdeploy@latest
```

```bash
nextdeploy init       # Scaffolds a Dockerfile and nextdeploy.yml config
nextdeploy build      # Creates a production-optimized Docker image
nextdeploy secrets    # Manage your Doppler secret integrations
nextdeploy runimage   # Run your image locally with secrets injected
nextdeploy provision  # Set up a new server with the built-in setup
nextdeploy prepare    # Prepare a server to receive deployments
nextdeploy ship       # Deploy to your VPS via SSH with Doppler secrets
nextdeploy serve      # Serve your app online
```

Test with production configuration before deploying:

```bash
nextdeploy runimage --prod
```

---

### 🧠 Our Philosophy

Modern DevOps can become complex and opaque. You often don't own your pipeline, see your deployments, or fully control your secrets.

**NextDeploy is the alternative.** It's built for clarity and ownership. You deploy what you understand, keeping the process lean, fast, and completely auditable.

---

### ✅ Ideal For Developers Who:

*   Deploy Next.js or full-stack apps regularly.
*   Run projects on VPS or bare metal and want more control.
*   Prefer transparent tools over opaque "magic" platforms.
*   Need strong security practices without a dedicated security team.
*   Value simplicity and want to reduce moving parts.

---

### 📍 Roadmap

*   ✅ Docker builds & SSH deploy
*   ✅ Doppler secret injection
*   ✅ Container metrics + logs
*   ✅ `runimage` command for local testing
*   🔄 GitHub webhook CI/CD automation
*   ⏪ Rollbacks and release tracking
*   🔌 Stack plugins (Rails, Go, Astro, Bun, etc.)
*   🌐 Team dashboard and multitenant support(in the future)

---

### 🧱 Open Source & Community-Driven

NextDeploy is built for developers who value **autonomy, simplicity, and control**.

We welcome contributors:
*   📡 **Systems Engineers:** Help enhance the daemon.
*   🛡️ **Security Reviewers:** Help us strengthen our security flows.
*   🧠 **Product Contributors:** Developers who build and use their own tools.

This is for devs, by devs.

---

### 🌐 Links

*   Website: [nextdeploy.one](https://nextdeploy.one)
*   GitHub: [github.com/aynaash/nextdeploy/cli](https://github.com/aynaash/nextdeploy/cli)
*   Twitter/X: [@nextdeploydev](https://twitter.com/nextdeploydev)

---

## 🚀 NextDeploy — Transparent Deployment, Under Your Control

Other platforms can lock you in and upsell you enterprise features. NextDeploy gives you root access to your stack and then gets out of your way.

Run your infrastructure professionally. Deploy with modern best practices.
**No magic. No middleware. Just you and your server.**

---
# ⚡ NextDeploy

NextDeploy is an **open-source CLI and daemon** for deploying and managing Next.js applications on your own infrastructure (VPS, cloud, or bare metal). It offers a simple, transparent alternative to closed-platform services.

---

## 🚀 Features
- `nextdeploy` CLI – Generate configs, build, ship, and manage apps.
- `nextdeployd` daemon – Handles logging, health checks, and deployment automation on the server.
- Cross-platform binaries (Linux, macOS, Windows).
- Built with Go for speed and portability.

---

## 📦 Installation

Download the latest binaries from the [Releases](https://github.com/aynaash/nextdeploy/releases) page.

**Example for Linux:**

```bash
# Download and install the CLI
curl -L -o nextdeploy https://github.com/aynaash/nextdeploy/releases/download/v0.1.0/nextdeploy-linux-amd64
chmod +x nextdeploy
sudo mv nextdeploy /usr/local/bin/

# Download and install the daemon
curl -L -o nextdeployd https://github.com/aynaash/nextdeploy/releases/download/v0.1.0/nextdeployd-linux-amd64
chmod +x nextdeployd
sudo mv nextdeployd /usr/local/bin/
```
