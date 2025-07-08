
# 🧠 nextcore: The Next.js Deployment Compiler

`nextcore` is the build-time intelligence engine behind NextDeploy. It transforms **project-specific metadata** into fully optimized, production-grade Docker images for any Next.js app — including support for SSR, SSG, middleware, edge functions, and static export.

### ⚙️ What It Does

- Ingests a structured `nextcore.config.json` or auto-detected metadata
- Validates rendering modes, project setup, and dependencies
- Renders a custom Dockerfile suited to the app’s actual behavior
- Builds a deployable image with zero manual config
- Outputs everything needed for CI/CD and runtime diagnostics

### 🧩 Why nextcore Exists

Next.js is powerful — but notoriously hard to self-host.

- Too many hidden modes (standalone, static, custom server)
- Middleware conflicts with static export
- Sharp doesn’t work on Alpine
- SSR, ISR, edge functions, rewrites — all need runtime awareness

> Vercel built a platform that hides all this.  
> We're building a platform that **exposes it intelligently.**

`nextcore` gives you full visibility and control while maintaining zero-config simplicity for developers.

---

## 🔍 Example Use Case

```bash
nextdeploy build --meta ./nextcore.config.json
