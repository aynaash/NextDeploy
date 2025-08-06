

# 🏗️ NextDeploy Architectural MacM

## 🎯 Mission

**NextDeploy** is a stack-agnostic, developer-first DevOps platform that enables effortless deployment of full-stack apps via two composable paths:

* **Containerized Path (VPS-first):**
  For developers who require full infrastructure control, persistent processes, or system-level integrations.

* **Serverless Path (Edge-first):**
  For developers seeking ultra-fast performance, CDN distribution, and cost-effective scaling through platforms like Cloudflare and AWS.

**One source. Two execution models. Unified by metadata. Powered by CLI and daemons.**

---

## 🧠 Core Philosophy

* **Predictable over magical** – Clear build output, transparent deploy logic.
* **Infrastructure as output** – Metadata and code determine infra needs, not the other way around.
* **Runtime-neutral** – Runs anywhere: VPS, edge, or hybrid.
* **Single config, multiple targets** – The app defines itself once. NextDeploy compiles it into multiple runtime backends.

---

## 🗺️ Architecture Overview

### 1. Build Phase (Universal Compilation)

```shell
nextdeploy build
```

Generates `.nextdeploy/` folder containing:

```
.nextdeploy/
├── metadata.json          # Declarative route + render map
├── docker/                # Docker image context
├── edge/
│   ├── cloudflare/        # Worker-compatible modules
│   └── aws/               # Lambda-compatible modules
└── assets/                # Static files, pre-rendered HTML
```

All deployments originate from this structure.

---

### 2. Deploy Phase (Target Execution)

```shell
nextdeploy deploy --target <vps|cloudflare|aws|hybrid>
```

#### ▸ `--target vps`

* Builds Docker image using `docker/`
* Deploys to VPS via SSH/Kadi daemon
* Ideal for containerized, backend-heavy, or monolithic apps

#### ▸ `--target cloudflare`

* Uploads `assets/` to Cloudflare Pages or R2
* Compiles route handlers and middleware to a Cloudflare Worker
* Uses `metadata.json` to route requests and handle fallbacks

#### ▸ `--target aws`

* Uploads `assets/` to S3
* Deploys APIs + SSR functions to AWS Lambda/Lambda\@Edge
* Uses CloudFront and API Gateway for routing and caching

#### ▸ `--target hybrid`

* Combines VPS container and Edge/CDN deploy
* Routes static content and middleware through CDN (Cloudflare)
* Routes SSR/API to VPS origin as fallback
* Enables scalability + control simultaneously

---

## 🧬 metadata.json Spec (WIP)

Defines the shape of the app post-build. Used for route generation, deployment, and server behavior.


This file is the **contract** between the app and the infrastructure. All deployments reference it.

---

## 🔧 CLI Example

```bash
# Build app
nextdeploy build

# Deploy to VPS
nextdeploy ship --target vps

# Deploy to Cloudflare Workers + CDN
nextdeploy ship --target cloudflare

# Deploy to AWS Lambda + S3 + CloudFront
nextdeploy ship --target aws

# Hybrid deployment: edge + container fallback
nextdeploy ship --target hybrid
```

---

## 🛠️ Components Map

| Component        | Role                                                               |
| ---------------- | ------------------------------------------------------------------ |
| `nextdeploy.yml` | Project-level config: target, secrets, hooks                       |
| `metadata.json`  | Build artifact that powers all routing, rendering, and infra setup |
| CLI Tool         | Developer-facing command-line interface                            |
| Build Engine     | Generates `.nextdeploy/` and interprets Next.js structure          |
| Caddy Proxy      | VPS container manager, handles TLS, routing, and deploy lifecycles |
| Edge Adapter     | Converts metadata → Cloudflare Worker or AWS Lambda logic          |
| Asset Uploader   | Sends static content to Pages, R2, or S3                           |

---

## 🌍 Future Targets

* [ ] **Fly.io**
* [ ] **Netlify Edge Functions**
* [ ] **Google Cloud Run**
* [ ] **Bun + Edge runtime targets**
* [ ] **MicroVM (Firecracker) isolated deploys**

---

## 🚀 Strategic Advantage

NextDeploy separates itself from platforms like Vercel by:

* Not locking users into a specific runtime or cloud
* Giving total control when needed (VPS) and cost-efficient scaling when required (Edge)
* Offering hybrid setups — edge speed + origin reliability
* Being fully open-source, inspectable, and extendable

---

## 📎 Summary

> NextDeploy is not a deploy tool. It's a **cross-runtime DevOps compiler** for modern full-stack apps.

It gives developers the power to:

* Build once
* Deploy anywhere
* Scale intelligently
* Own their infrastructure

All with one CLI, one config, and no platform lock-in.

---
