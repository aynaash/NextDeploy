

# 🧨 Docker Client Gotchas (and How to Not Get Burned)

A list of common pitfalls you'll encounter now that you're interfacing directly with the Docker Go SDK, bypassing Dockerfiles, and building a low-level platform. **Ignore these at your own risk.**

---

## ⚠️ 1. Build Context Explosion

**Problem**:
Sending large or sensitive files (e.g. `.git`, `.env`, `node_modules`) into the build context tarball causes:

* Long build times
* Unnecessary image bloat
* Leaked secrets

**Solution**:

* Build a virtual `.dockerignore` inside your tar logic
* Always exclude:

  ```
  .git
  node_modules
  .next
  *.env
  dist
  .DS_Store
  ```
* Consider allowing users to define exclusions in metadata if needed

---

## ⚠️ 2. Secrets Leaked Into Images

**Problem**:
ENV variables injected at *build time* (inside `Dockerfile`) get baked into the image permanently.

**Solution**:

* Inject secrets at *runtime* using `ContainerCreate` and `HostConfig.Env`
* Never reference secrets in Dockerfile `ENV`, `RUN`, or `COPY`
* Treat image builds as **public artifacts**

---

## ⚠️ 3. Stale Image Caching

**Problem**:
Images get cached by Docker when input doesn't change. Devs deploy and see no changes.

**Solution**:

* Hash critical files (`Dockerfile`, `package.json`, `src/`) to determine cache invalidation
* Add `--no-cache` mode to your build CLI
* Use meaningful tags (`contextbytes:<commit-hash>`)

---

## ⚠️ 4. Port Binding Conflicts

**Problem**:
Multiple containers trying to bind to the same port (e.g., `3000`) on the host.

**Solution**:

* Randomize external port binding or manage via a port allocator service
* Record port mappings in metadata store (DB or file)
* Prefer `traefik`, `caddy`, or `nginx` for dynamic proxy routing

---

## ⚠️ 5. Broken Tarball Format

**Problem**:
The tarball used in `ImageBuild` is malformed due to:

* Symbolic links
* Binary files
* Files with large size or long names

**Solution**:

* Resolve symlinks explicitly with `filepath.EvalSymlinks()`
* Filter files by type and size
* Test generated tarballs with `tar -tvf` before build

---

## ⚠️ 6. Fat Images and Slow Build Times

**Problem**:
Default base images (`node:18`) lead to >1GB image sizes.

**Solution**:

* Use `node:18-alpine` as base
* Move build artifacts to runtime stage (multi-stage builds)
* Strip devDependencies:

  ```dockerfile
  RUN npm prune --production
  ```

---

## ⚠️ 7. Wrong App Context Path

**Problem**:
Assuming the app root is `./my-next-app`, which breaks in CI or monorepos.

**Solution**:

* Always resolve and validate the `root_dir` using `filepath.Abs`
* Check for presence of `package.json`, `next.config.js`
* Fail early if root path is invalid

---

## ⚠️ 8. Lost Logs / Race Conditions

**Problem**:
Logs from container startup are missed due to late attachment to stdout/stderr streams.

**Solution**:

* Use `ContainerLogs` with `since=0` and `follow=true`
* Attach logs immediately after `ContainerStart()`
* Buffer logs and stream them to frontend or CLI in real-time

---

## ⚠️ 9. Duplicate or Unclear Image Tags

**Problem**:
Using `:latest` for all builds causes unpredictable deployments.

**Solution**:

* Use tags like:

  * `app:<commit-hash>`
  * `app:<timestamp>`
  * `app:latest` (only for development/testing)
* Store image metadata in your DB for auditability

---

## ⚠️ 10. Insufficient Docker Daemon Permissions

**Problem**:
Daemon lacks proper permissions or system limits are too low.

**Solution**:

* Ensure daemon runs with `sudo` or as `docker` group user
* Set ulimits and resource constraints explicitly:

  ```go
  HostConfig: &container.HostConfig{
      Memory: 512 * 1024 * 1024, // 512MB
      CPUQuota: 50000,           // 50% of one CPU
  }
  ```
* Add health checks and retries on daemon communication

---

## 🧠 Bonus Tips

* **Always validate metadata before build**

  * Ensure required fields: `entry_point`, `package_manager`, `node_version`
* **Track all builds**

  * Store logs, image ID, and metadata in your backend DB
* **Never trust user input blindly**

  * Sanitize file paths, commands, env vars

---

## 🔐 Security Essentials

* Strip `.env` files from build context
* Run all builds in isolated environments
* Never let user metadata touch your host FS directly

---

## ✅ Summary

| Risk                  | Solution                                            |
| --------------------- | --------------------------------------------------- |
| Build context too big | Use virtual `.dockerignore`                         |
| Secret leakage        | Inject ENV only at runtime                          |
| Image cache confusion | Use hashes and tags                                 |
| Port collisions       | Dynamic port allocation or proxy                    |
| Log loss              | Use `since=0` and `follow=true`                     |
| Image tag conflict    | Tag by SHA, time, and store in DB                   |
| Tarball corruption    | Use `archive/tar` carefully with symlink resolution |
| Fat images            | Use Alpine + prune + multi-stage                    |
| Broken builds         | Validate metadata up front                          |
| Security flaws        | Never bake secrets, sandbox everything              |

---

You're not building an app anymore. You're building an **orchestration platform**.
The tools are sharp. Use them with precision—or bleed.
