

---

# 📦 NextDeploy Metadata & Asset Extraction

This module handles metadata generation and asset extraction for the `NextDeploy` deployment pipeline. It ensures builds are deterministic and compatible with external static asset servers (e.g., Caddy).

---

## ✅ What It Does

### 1. **Runs `next build`**

Builds the Next.js project locally before containerization.

### 2. **Extracts Deploy Metadata**

Parses `.next/` outputs (TODO) to create a deploy metadata file:

```json
.nextdeploy/metadata.json
```

Contains:

* Static & dynamic routes
* Middleware files
* Public environment variables

### 3. **Copies Static Assets**

Copies all files from `public/` into:

```bash
.nextdeploy/assets/
```

These assets can be served separately by Caddy or any CDN.

### 4. **Tracks Git State**

Stores the Git commit hash and dirty state at the time of metadata generation inside:

```json
.nextdeploy/build.lock
```

Used later to detect if code has changed before rebuild.

---

## 🛠 CLI Workflow

```bash
# Initialize and generate metadata
nextdeploy metadata:generate

# Validate Git state before build
nextdeploy metadata:validate

# Proceed with Docker build
nextdeploy build
```

---

## 🔁 Git Snapshot Example

```json
{
  "git_commit": "abc123def",
  "git_dirty": false,
  "generated_at": "2025-07-12T10:45:00Z",
  "metadata_file": "metadata.json"
}
```

---

## 🔜 TODOs

* Parse `.next/routes-manifest.json`, `build-manifest.json`, etc.
* Add CI/CD hooks for metadata validation
* Add auto-upload of `.nextdeploy/assets` to CDN or server

---

Let me know if you want to add build-time environment validation or version pinning in the metadata file.
Absolutely — here are the most critical **Gotchas to Avoid** for any dev working with this system:

---

## ⚠️ Gotchas to Avoid in NextDeploy Metadata System

### ❌ 1. **Modifying Code After Metadata Generation**

* If you run `nextdeploy metadata:generate` and **then change files** in `pages/`, `app/`, `middleware.ts`, or `next.config.js`, your metadata will be stale.
* Always re-run metadata generation if core app files change.

✅ **Fix**: Use `nextdeploy metadata:validate` to check Git state before building.

---

### ❌ 2. **Forgetting to Clean the `.next/` Folder**

* If `next build` fails or is run in a dirty state, `.next/` can contain **stale artifacts**.
* This can lead to incorrect metadata extraction.

✅ **Fix**: Run `rm -rf .next/` before `nextdeploy metadata:generate` for clean builds (automate this if needed).

---

### ❌ 3. **Relying on Public Assets During Runtime**

* Next.js expects `public/` assets to be co-located with the server, but we're **decoupling** them for Caddy.

✅ **Fix**: Make sure Caddy or your CDN is correctly configured to serve `.nextdeploy/assets/` at `/`.

---

### ❌ 4. **Not Tracking `.nextdeploy` in Git**

* Don't commit `.nextdeploy/assets/`, `.nextdeploy/metadata.json`, or `.nextdeploy/build.lock` unless you want to **lock your build** to a specific state.

✅ **Fix**: Add `.nextdeploy/*` to `.gitignore`, **except** when doing reproducible builds.

---

### ❌ 5. **Hardcoding Env Variables**

* If you generate metadata with one set of `NEXT_PUBLIC_*` values, and change them in `.env.production` later, your metadata could be misleading.

✅ **Fix**: Regenerate metadata any time public envs change.

---

### ❌ 6. **Assuming Static File Names Won’t Change**

* Files in `public/` might be fingerprinted or renamed dynamically by your build process.

✅ **Fix**: Rely only on **copied** files in `.nextdeploy/assets/`, not assumptions from source.

---

### ❌ 7. **Running Build Without Validation**

* If you skip validation, you may build with **outdated metadata**, breaking production deployments.

✅ **Fix**: Always run `nextdeploy metadata:validate` before `nextdeploy build`. Make this a required CI step.

---

## 🔐 Bonus Tip: Lock & Freeze

If you're building in CI/CD and want full reproducibility, **lock your build** by committing:

* `metadata.json`
* `build.lock`

And deploy from that snapshot only.

---

Let me know if you want to add automatic validation hooks or CI examples (e.g., GitHub Actions workflow).
