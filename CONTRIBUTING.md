
# 🛠️ CONTRIBUTING TO NEXTDEPLOY

## 🔥 Welcome to the Engine Room

First, understand this:

**NextDeploy is not a general-purpose deployment tool.**

It is the **Next.js deployment engine** — focused, minimal, and engineered for developers who value control over convenience, clarity over abstraction, and performance over compatibility.

If that’s you, read on. If not, fork it and build your own thing.

---

## 📜 Our Mantra

> “If you want to build with something other than Next.js — build your own tool.”

NextDeploy **does one thing** and does it **violently well**:  
Deploying **Next.js applications** to your own infrastructure, with zero fluff and full control.

---

## 💡 What We Accept

- 🔧 Improvements to the Next.js deployment flow
- 🧠 Performance enhancements
- 📈 Metrics, logging, monitoring for Next.js containers
- 🛡️ Security hardening
- ⚙️ Daemon updates that enhance orchestration
- 🧪 New CLI commands that stay within the Next.js lifecycle
- 📖 Clear, useful documentation improvements
- 🧼 Bug fixes that keep the engine stable

---

## 🚫 What We *Don't* Accept

Don’t waste your time. If your PR includes any of the following, it will be **closed immediately**:

- ❌ Support for other frameworks (React, Vue, Remix, etc.)
- ❌ Plugin systems or dynamic runtime extensions
- ❌ Hosting provider lock-in (we’re infra-agnostic — use your own VPS)
- ❌ “Make it work with Docker Compose, Heroku, Fly.io” — no.
- ❌ Generic build tools, webhooks, or CI/CD for non-Next.js use cases

We are not interested in becoming a bloated platform. This project is for **Next.js developers** who want to **own their deployments**.

---

## 🧱 Design Philosophy

### 1. **Next.js or Nothing**

NextDeploy will **never** support other frontend frameworks.  
That’s a feature, not a limitation.

### 2. **CLI-First, Daemon-Optional**

The CLI is 100% open-source and self-contained.  
The daemon exists to enhance orchestration, not to replace or bloat the core.

### 3. **No Plugin System**

Want new behavior? Fork the project, add it yourself, and make a PR.  
We don’t do runtime plugins. We ship fast, tight, readable Go code.

### 4. **OSS with Teeth**

This is open source — not open-ended.  
The project moves fast, breaks what needs to break, and stays lean.

---

## 🧬 Branching Model

- `main`: latest stable release
- `next`: active dev work
- Feature branches: `feat/your-feature-name`
- Fix branches: `fix/issue-description`

Make PRs into `next`, not `main`.

---

## ✅ Contribution Checklist

Before submitting a pull request:

- [ ] You’re fixing/improving something related to **Next.js deployment**
- [ ] Your code is **tested** and **documented**
- [ ] You’re not trying to add a plugin system
- [ ] You’ve followed the repo structure and style
- [ ] You understand that **Next.js is non-negotiable**

---

## ✊ Our Promise

If your contribution makes **Next.js deployment faster, clearer, or more powerful**, it will be reviewed, discussed, and — if it meets the bar — merged fast.

But we don't merge compromises.

---

## 🗣️ Join the Conversation

Open an issue, ask a question, propose a feature. But remember:

We are **not building a platform**.  
We are building the **Next.js developer's deployment engine of choice**.

---

Built for developers.  
Forged for freedom.  
No fluff. No apologies.

— The NextDeploy Core Team [Just me now]
