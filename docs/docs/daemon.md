
# 🧠 Daemon Engineering Memory Bank

---

## Core Challenges Building a Daemon in Go

1. **Fork + Exec magic**
   Go doesn't support traditional `fork()` gracefully. You must use libraries that spawn a clean child and avoid runtime issues.

2. **Zombie process hell**
   Must reap all subprocesses (e.g. `exec.Cmd`) and handle `SIGCHLD` to avoid `<defunct>` processes.

3. **Signal handling**
   Properly trap `SIGTERM`, `SIGINT`, optionally `SIGHUP`, to shutdown cleanly and flush all resources.

4. **Goroutine resilience**
   Use `errgroup` or similar to surface goroutine failures and crash the entire daemon if something critical dies.

5. **Logging & file descriptors**
   Daemons should close or redirect STDIN/OUT/ERR, write to log files or structured logging libraries (`zap`, `zerolog`), not `fmt.Println()`.

6. **Runtime configuration reloads**
   Monitor config file changes or use `SIGHUP`, update shared state safely via channels or locks.

7. **PID file hygiene**
   Write `.pid` on startups, remove it on shutdown, and check for existing live PIDs to avoid double launches.

8. **Self‑monitoring & supervision**
   Design internal health checks and expose `/healthz` or similar; consider using OS-level supervision (`systemd`, `monit`, etc.).

9. **Cleanup of external resources**
   Always clean up spawned Docker containers, temp files, DB handles—even on shutdown.

10. **Concurrency safety**
    Audit shared state access, use proper synchronization, test your goroutines rigorously.

---

## 📚 Verified Resource Map

### 📘 Foundational OS-Level Guides (C / Unix internals)

* **UNIX Daemon HOWTO** — comprehensive classic on daemon design (double-fork, setsid, umask, closing file descriptors) ([Reddit][1], [cjh.polyplex.org][2], [samuel.karp.dev][3])
* **Advanced Programming in the UNIX Environment** (Stevens & Rago) — especially Chapter 37 and examples of daemonizing ([Wikipedia][4])
* **Advanced Linux Programming** — practical focus on process control and signal handling ([cse.hcmut.edu.vn][5])
* **Linux System Programming** (Robert Love) — covers signals, process groups, PID namespace basics ([igm.univ-mlv.fr][6])

### 🧱 OS Concepts & Best Practices

* **“daemon(7)” man page on man7.org** — comparison of SysV vs new‑style daemons under systemd ([man7.org][7])
* **Samuel Karp blog “Software Daemons”** — covers daemon supervision, recovery, and restart patterns ([samuel.karp.dev][3])

### 🧰 Real-World Daemon Examples in Go

* **Prometheus Node Exporter** — a real-world Go daemon that runs metrics collection, emphasizing clean design and restarts ([GitHub][8])

---

## ✅ TL;DR Cheat Sheet

```text
Daemon Requirements:
  ← detach from terminal
  ← double-fork / setsid
  ← ignore stop signals (SIGTTIN/TTOU/TSTP)
  ← close file descriptors (stdin/out/err)
  ← write PID file
  ← trap TERM/INT (optional HUP)
  ← start goroutines under errgroup
  ← monitor children (SIGCHLD)
  ← structured logging
  ← config reload capability
  ← expose health endpoint
  ← cleanup resources on exit
```

---

## 🔧 Recommended Workflow

1. **Read the UNIX Daemon HOWTO** to internalize the exact OS-level steps.
2. **Implement a prototype in C** (no shortcuts), so you understand setsid, double-fork, etc.
3. **Build a Go version**—use `os/exec` or `go-daemon` as wrapper, but understand every step it does.
4. **Structure your Go daemon**:

   * `context.WithCancel()` + `os/signal.Notify(...)`
   * Use `errgroup` for goroutines (queue poller, config watcher, reporter)
   * Graceful shutdown logic in top-level select
5. **Add health endpoint** and expose status for external monitoring.
6. **Run via systemd** for supervision rather than building everything inside your code (cleaner, more robust).

---


[1]: https://www.reddit.com/r/rust/comments/l98jtb/cross_platform_daemonservice_library/?utm_source=chatgpt.com "Cross Platform Daemon/Service Library? : r/rust - Reddit"
[2]: https://cjh.polyplex.org/software/daemon.pdf?utm_source=chatgpt.com "[PDF] How To Write a UNIX Daemon"
[3]: https://samuel.karp.dev/blog/2019/11/software-daemons/?utm_source=chatgpt.com "Software Daemons - Samuel Karp"
[4]: https://en.wikipedia.org/wiki/Advanced_Programming_in_the_Unix_Environment?utm_source=chatgpt.com "Advanced Programming in the Unix Environment"
[5]: https://www.cse.hcmut.edu.vn/~hungnq/courses/nap/alp.pdf?utm_source=chatgpt.com "[PDF] Advanced Linux Programming"
[6]: https://igm.univ-mlv.fr/~yahya/progsys/linux.pdf?utm_source=chatgpt.com "[PDF] Linux System Programming - IGM"
[7]: https://man7.org/linux/man-pages/man7/daemon.7.html?utm_source=chatgpt.com "daemon(7) - Linux manual page - man7.org"
[8]: https://github.com/prometheus/node_exporter?utm_source=chatgpt.com "prometheus/node_exporter: Exporter for machine metrics - GitHub"
