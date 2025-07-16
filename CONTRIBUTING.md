
# 🛠️ Contributing to NextDeploy

## Welcome to the Future of Next.js Deployment.

Thanks for your interest in contributing to **NextDeploy** — the fast, focused, and fully self-hosted deployment engine built **exclusively** for **Next.js**.

This project is not a tech experiment. It’s a battle-tested infrastructure tool powering real production apps — and we’re building it to **end the era of black-box deployment platforms** like Vercel.

If you're here, it means you believe developers should control their own tools, infrastructure, and stack. So do we.

---

## 🚩 What NextDeploy *Is*

NextDeploy is a **Next.js-only** deployment ecosystem.  
It gives developers full autonomy over how their Next.js apps ship, scale, and operate on their own VPS infrastructure.

- It's **framework-specific**, for precision and performance.
- It's **CLI-first**, with a long-running **daemon** for orchestration and monitoring.
- It’s **cloud-agnostic** — run it on any server, anywhere.
- It’s **open-source**, and built to be used in the real world — not just in demos.

It’s the deployment platform *we* use — because nothing else gave us the clarity, control, and composability we needed.

---

## 🚫 What NextDeploy *Is Not*

We don’t compromise on our scope. We don't chase trends. We don’t build for everyone.

- ❌ We do **not** support other frontend frameworks (Vue, Astro, Remix, etc.)
- ❌ We do **not** integrate with vendor platforms (Fly.io, Heroku, Vercel)
- ❌ We do **not** offer plugin APIs or runtime injection systems
- ❌ We do **not** aim to be a general-purpose DevOps toolkit

This is deliberate. **Focus is a feature.**

---

## ✅ What We’re Looking For

We’re inviting contributors who align with this mission and want to push NextDeploy forward, without bloating it sideways.

Here’s what we’d love your help with:

- 🔧 Tightening the Next.js deployment flow (SSR, API routes, middleware, edge cases)
- 📦 Improving the orchestration daemon (Docker lifecycle, container health, etc.)
- 📈 Enhancing system-level monitoring, logs, metrics
- 🧹 Cleaning up configuration and YAML DX
- 🧠 Improving deployment speed, resilience, and edge-case handling
- 📘 Writing great docs, error messages, and usage examples
- 🧪 Running real deployments and submitting bugs/feedback from real-world usage

---

## 🧱 Core Principles

> Read this section twice. It defines the soul of the project.

### 1. **Next.js Only**
NextDeploy is laser-focused on Next.js. That’s not a limitation — it’s our superpower. We optimize every line of code around how *Next.js actually works in production*.

### 2. **Self-Hosted, Fully**
You own the infrastructure. You own the app. You own the deploy flow. We’ll never lock you into anything.

### 3. **No Plugins. No Black Boxes.**
NextDeploy is simple by design. No plugin systems. No runtime extensibility. What you see is what runs. If you want more power, fork it, or open a PR.

### 4. **Built From the Trenches**
This isn't a theory project. It runs production apps: pharmacies, hospitals, dashboards — deployed by the same tool you’re improving. **Every change should earn its keep.**

### 5. **Open Source With Boundaries**
We move fast, we review hard, and we reject what doesn’t align. It’s not personal — it’s to protect the long-term clarity and usability of the ecosystem.

---

## 📦 PR Checklist

Before submitting a PR:

- [ ] Is this related to **Next.js deployment or orchestration**?
- [ ] Is this focused, modular, and testable?
- [ ] Have you avoided introducing plugin systems, vendor dependencies, or feature creep?
- [ ] Have you documented what changed and why?
- [ ] Have you followed the CLI + Daemon architecture?

If in doubt, open an issue first and let’s discuss.

---

## 🌱 First Steps

If you're new here, here’s how to get started:

- [x] Clone the repo and run `nextdeploy init`
- [x] Deploy a sample Next.js app to a VPS using `nextdeploy deploy`
- [x] Review how the CLI interacts with Docker, SSH, and the orchestration daemon
- [x] Read and tweak the `nextdeploy.yml` config
- [x] Submit feedback, edge case issues, or suggestions for improvements

---

## 🧪 Power User Contributions

We especially want contributions from developers who:

- Use NextDeploy in real apps (like we do)
- Find edge cases from SSR/ISR/streaming
- Push deployments to low-resource servers
- Care deeply about speed, reliability, and transparency
- Want to make DevOps better *for the frontend dev*

---

## 🤝 Let’s Build It Right

NextDeploy is for developers who are done with platform lock-in, hidden limits, and black-box magic.  
It’s for builders who want to ship fast **and** sleep at night.

If that’s you — welcome. Pull requests are open. The CLI is live. The engine runs in production.

Let’s make Next.js deployment simple, powerful, and ours again.

— **The NextDeploy Team** *(just me... for now)*
