
# 🛠️ Contributing to NextDeploy

## Welcome, Developer.

Thanks for your interest in contributing to **NextDeploy** — the focused, fast, and fully self-hosted deployment engine for **Next.js applications**.

We're building this project with a strong set of principles, so before you dive in, please take a moment to understand the philosophy that drives the codebase.

---

## 🚩 What NextDeploy *Is*

NextDeploy is a **Next.js-only deployment tool**.  
It gives developers full control over shipping their Next.js apps to any VPS or infrastructure they own.

- It’s **framework-specific** by design.
- It’s **plugin-free** to stay lean, testable, and secure.
- It’s **CLI-first**, with an optional daemon for orchestration and monitoring.
- It’s **open-source**, and we want contributions that push it forward without bloating it sideways.

---

## 🚫 What NextDeploy *Is Not*

To avoid confusion:

- We do **not** support other frontend frameworks (e.g., Vue, React SPA, Astro, Remix).
- We do **not** support vendor-specific hosting (e.g., Fly.io, Heroku, Vercel).
- We do **not** include a plugin system or third-party extensibility layer.
- We do **not** aim to be a one-size-fits-all DevOps tool.

This lets us keep the project simple, powerful, and laser-focused on Next.js.

---

## ✅ What We’d Love From You

If you're aligned with that vision, here’s how you can help:

- Improve the Next.js deployment flow
- Add features to the orchestration daemon
- Fix bugs or edge cases in the CLI/daemon lifecycle
- Make metrics, health checks, or logging cleaner or more insightful
- Write clear documentation, examples, and error messages
- Help us improve developer experience for real-world VPS deployments

---

## 🧱 Design Philosophy

1. **Next.js Only**  
   We don't support other frameworks. That’s a strength, not a limitation.

2. **No Plugins, No Bloat**  
   All features are native. If you want to add functionality, fork the project and propose it as a PR.

3. **Self-Hosted, Always**  
   NextDeploy gives developers freedom and control — no platform lock-in, ever.

4. **Open Source, With a Backbone**  
   We move fast, review carefully, and reject what doesn't fit the mission. It’s not personal — it's about protecting the project.

---

## 🛠️ PR Checklist

Before submitting a pull request, please:

- [ ] Make sure your feature is related to **Next.js deployment or orchestration**
- [ ] Keep your changes modular and testable
- [ ] Avoid introducing external plugins or runtime extensibility
- [ ] Document your changes clearly
- [ ] Follow the existing CLI/daemon structure

---

## 🌱 New Here? Start With These

- [x] Check out `nextdeploy.yml` usage
- [x] Run `nextdeploy init` and follow the flow
- [x] Review how the CLI interacts with Docker, SSH, and the optional daemon
- [x] Try a VPS deployment and suggest improvements to DX

---

## 🤝 Let’s Build It Right

We care deeply about developer control, simplicity, and performance.  
If you're here to help Next.js developers own their deployments, you're in the right place.

Pull requests are welcome. Issues are open. We move fast and ship clean.

Thanks for being here.

— The NextDeploy Team
