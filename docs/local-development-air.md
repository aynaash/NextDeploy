# Local Development with Air (Live-Reloading)

NextDeploy provides preconfigured [Air](https://github.com/air-verse/air) setups for rapid local iteration on the CLI and Daemon. When running with Air, every edit to Go source files triggers an automatic rebuild and updates `~/.nextdeploy/bin/nextdeploy`, making your changes immediately active in your shell.

---

## Prerequisites

1. **Go 1.26+**
2. **Mage:**
   ```bash
   brew install mage # macOS
   # or: go install github.com/magefile/mage@latest
   ```
3. **Air** (auto-installed on first `mage dev`, or install manually):
   ```bash
   go install github.com/air-verse/air@latest
   ```

---

## 1. CLI Live-Reloading (`.air.cli.toml`)

To watch CLI source code and automatically rebuild on change:

```bash
mage dev
# or: mage devCLI
# or directly: air -c .air.cli.toml
```

### What happens:
* Air watches [`cli/`](file:///Users/hersi/Developer/NextDeploy/cli) and [`shared/`](file:///Users/hersi/Developer/NextDeploy/shared) directories for `.go` and `.toml` changes.
* On change, it compiles:
  ```bash
  go build -o tmp/nextdeploy ./cli && mkdir -p $HOME/.nextdeploy/bin && cp tmp/nextdeploy $HOME/.nextdeploy/bin/nextdeploy
  ```
* Ensures `~/.nextdeploy/bin` is added to your shell's `PATH` (`~/.zshrc` / `~/.bashrc`).
* In any other terminal window, running `nextdeploy` immediately uses your latest code:
  ```bash
  nextdeploy version
  nextdeploy ship
  nextdeploy plan
  ```

---

## 2. Daemon Live-Reloading (`.air.daemon.toml`)

When developing the VPS/Docker daemon (`nextdeployd`):

```bash
mage devDaemon
# or directly: air -c .air.daemon.toml
```

### What happens:
* Air watches [`daemon/`](file:///Users/hersi/Developer/NextDeploy/daemon) and [`shared/`](file:///Users/hersi/Developer/NextDeploy/shared).
* Rebuilds and restarts the foreground daemon process on port/socket automatically.

---

## 3. Fast Feedback Preflight (`mage check` / `mage loop`)

To run rapid compilation, tests, and formatting checks alongside Air:

* **One-shot check:**
  ```bash
  mage check
  ```
* **Continuous watch feedback loop:**
  ```bash
  mage loop
  ```

---

## 4. Configuration Details

* **CLI configuration:** [`.air.cli.toml`](file:///Users/hersi/Developer/NextDeploy/.air.cli.toml)
  * Watches: `cli/`, `shared/`
  * Excludes: `tmp/`, `vendor/`, `daemon/`, `docs/`, `_test.go`
  * Destination: `tmp/nextdeploy` and `$HOME/.nextdeploy/bin/nextdeploy`
* **Daemon configuration:** [`.air.daemon.toml`](file:///Users/hersi/Developer/NextDeploy/.air.daemon.toml)
  * Watches: `daemon/`, `shared/`
  * Excludes: `tmp/`, `vendor/`, `cli/`, `docs/`, `_test.go`
