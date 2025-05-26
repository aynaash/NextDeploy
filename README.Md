
---

## 🛠️ NextDeploy CLI — Real DevOps for Developers Who Give a Damn

Sick of being trapped in Vercel’s walled garden? Done fighting opaque CI/CD pipelines and vendor secrets sprawl?

**NextDeploy CLI** is your escape hatch.

It deploys **Next.js apps to your own VPS** — with real secrets, real Docker, and zero bullsh\*t. You get full-stack control from local dev to live prod using nothing but your terminal and Doppler.

---

### ⚙️ What It Actually Does

* 🧱 Builds Docker images for your Next.js app — production-ready, no guesswork
* 🔐 SSH-deploys your container to any VPS (Hetzner, DO, AWS, bare metal — doesn't matter)
* 🔑 Injects secrets directly from **Doppler**, securely and dynamically
* 📦 Ships local or CI builds with full transparency
* 📊 Streams logs + metrics from your container
* 🧪 Localtest: run prod-like builds locally using real secrets

One tool. One config. No dashboards. No YAML hell.

---

### 🔐 Doppler First. No `.env` Theater.

NextDeploy is built **Doppler-first**, because `.env` file juggling in 2025 is amateur hour.

* Load secrets dynamically at deploy time
* No hardcoding, no copying, no leaking
* Full environment scoping: dev, staging, prod
* Change a secret → restart container → done
* Works identically in CI or local dev

---

### 🚀 Install It. Use It. Forget the Old Way.

```bash
go install github.com/nextdeploy/cli@latest
```

```bash
nextdeploy init       # Dockerfile + nextdeploy.yml scaffold  
nextdeploy build      # Create production image  
nextdeploy deploy     # Push to your VPS via SSH + Doppler secrets  
```

Need to debug with prod config before deploying?

```bash
nextdeploy localtest --env=prod
```

---

### 🧠 Why This Exists

Because modern DevOps has become a damn puppet show.

You don’t own your pipeline. You don’t see your deploy. You don’t control your secrets.
You pay more, wait longer, and trust more than you should.

**NextDeploy is the antidote.** You deploy what you understand. You keep it lean, fast, and auditable.

---

### 🧪 Good Fit If You:

* Deploy Next.js or full-stack apps regularly
* Run small to mid-sized projects on VPS or bare metal
* Hate black-box PaaS solutions and "magic" deployments
* Want **security without a security team**
* Are building real things, fast, and want fewer moving parts

---

### 📍 Roadmap (Shipping Fast)

* ✅ Docker builds & SSH deploy
* ✅ Doppler secret injection
* ✅ Container metrics + logs
* ✅ `localtest` command
* 🔄 GitHub webhook CI/CD
* ⏪ Rollbacks + release tracking
* 🔌 Stack plugins (Rails, Go, Astro, Bun, etc.)
* 🌐 Team dashboard + multitenant support

---

### 🧱 OSS + Community-Driven

We're building NextDeploy for developers who value **autonomy, simplicity, and control**.

Want to help?

* 🔌 Stack plugin authors (Rust, Deno, Rails, etc.)
* 📡 Metrics/daemon engineers (Go or Rust)
* 🛡️ Security flow reviewers
* 🧠 Product contributors who *use* their own tools

This is for devs, by devs — not suits or VCs.

---

### 🌐 Links

* Website: [nextdeploy.one](https://nextdeploy.one)
* GitHub: [github.com/nextdeploy/cli](https://github.com/nextdeploy/cli)
* Twitter/X: [@nextdeploydev](https://twitter.com/nextdeploydev)

---

## 🧨 NextDeploy — Finally, Deployment That Respects You

Everything else wraps you in handcuffs and sells you enterprise features.

NextDeploy gives you root access to your stack — and gets the hell out of your way.

Run your infra like a pro. Deploy like it’s 2025.
**No magic. No middleware. Just you and your server.**

---
