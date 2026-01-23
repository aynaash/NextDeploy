---

# ⚡ NextDeploy

NextDeploy is an **open-source CLI + daemon** for deploying and managing **Next.js applications** on your own infrastructure.
No lock-in. No magic. Just **Docker, SSH, and full control**.

---

## 🚀 Why NextDeploy?

* 🧱 **Builds** Docker images optimized for Next.js
* 🚀 **Ships** to any VPS (Hetzner, DigitalOcean, AWS, bare metal) via SSH
* 🔐 **Injects secrets** securely with [Doppler](https://doppler.com)
* 📊 **Streams logs & metrics** from running containers
* 🧪 **Runimage:** test production builds locally with real secrets
* 🛠️ **Daemon support:** health checks, logs, and automation on servers

One tool. One config. Full transparency.

---

## 📦 Installation

Choose your platform:

**Linux**

```bash
curl -fsSL https://nextdeploy.one/linux-cli.sh | sh
```

**macOS**

```bash
curl -fsSL https://nextdeploy.one/mac-cli.sh | sh
```

**Windows (PowerShell, Run as Admin)**

```powershell
iwr -useb https://nextdeploy.one/windows.ps1 | iex
```

**Daemon (Linux/Mac)**

```bash
curl -fsSL https://nextdeploy.one/nextdeployd.sh | sh
```

⚠️ **Pro tip:** version your installers. For production use:

```bash
curl -fsSL https://nextdeploy.one/install.sh | sh          # latest stable
curl -fsSL https://nextdeploy.one/install/v0.1.0.sh | sh   # pinned version
```

---

## ⚡ Quick Start

```bash
nextdeploy init       # Scaffold Dockerfile + nextdeploy.yml
nextdeploy build      # Build production Docker image
nextdeploy runimage   # Run locally with Doppler secrets
nextdeploy provision  # Prepare a fresh VPS
nextdeploy ship       # Deploy to your server
nextdeploy serve      # Serve app online
```

Test with production config before shipping:

```bash
nextdeploy runimage --prod
```

---

## 🔐 Secrets Done Right

NextDeploy is **Doppler-first** — no more `.env` files:

* Secrets injected at deploy/runtime
* Fully encrypted + scoped (dev/staging/prod)
* Update → restart → done
* Works the same locally and in CI

---

## 🧠 Philosophy

Other platforms abstract until you lose control.
**NextDeploy flips that.** You own the pipeline. You see every step.

No black boxes. No middleware. Just you and your server.

**Inspired by**: [Kamal](https://kamal-deploy.org/) - We loved their approach to self-hosted deployments and specialized it for Next.js.

---

## ✅ Perfect For Developers Who

* Deploy **Next.js** or full-stack apps to VPS/bare metal
* Want **transparent, auditable DevOps**
* Need strong **security practices** without complexity
* Care about **simplicity over vendor lock-in**

---

## 🛠️ Roadmap

* ✅ Docker builds & SSH deploy
* ✅ Doppler integration
* ✅ Logs + metrics
* ✅ `runimage` for local testing
* 🔄 CI/CD via GitHub webhooks
* ⏪ Rollbacks & release tracking
* 🔌 Stack plugins (Rails, Go, Bun, Astro…)
* 🌐 Dashboard & multitenant support

---

## 🌐 Links

* Website → [nextdeploy.one](https://nextdeploy.one)
* GitHub → [github.com/aynaash/nextdeploy/cli](https://github.com/aynaash/nextdeploy/cli)
* Twitter/X → [@nextdeploy](https://twitter.com/nextdeploy)

---

## 👥 Community

We welcome contributors:

* Systems engineers (daemon/logging)
* Security reviewers
* Product-minded devs

---
🔥 **NextDeploy — Transparent Deployment, Under Your Control.**
---

