
---

# ✅ TODO.md – NextDeploy Daemon: Path to Vercel-Level Infra

> **Goal**: Extend NextDeploy daemon to provide full production-grade infra like Vercel – monitoring, scaling, logging, health, and more – on any VPS.

---

## 1. 🧠 Unify Configuration

* [ ] Parse a single `nextdeploy.yml` (already defined)
* [ ] Validate schema with strict types: ports, DB, memory, health checks, etc.
* [ ] Add "environments" support (`dev`, `staging`, `prod`)
* [ ] Allow daemon flags to override config (e.g., `--env=prod`)

---

## 2. 🔁 Daemon Bootstrap

* [ ] Spawn workers (goroutines) for:

  * Request capturing
  * Response logging
  * System stats collection
  * Container health checks
  * Failover routing (to backup containers)

* [ ] Add worker registry system to track status, failures, and restarts

* [ ] Graceful shutdown + restart hooks

---

## 3. 📦 App Container Management

* [ ] Watch deployed container status
* [ ] Auto-restart if container crashes
* [ ] Auto-scale horizontally based on:

  * CPU/mem usage
  * HTTP traffic spikes
* [ ] Log container uptime and restart reasons
* [ ] Cache container images if same app is deployed multiple times

---

## 4. 🌐 HTTP Traffic Interception

* [ ] Reverse proxy incoming traffic
* [ ] Capture **incoming request metadata**:

  * Path, headers, body (if small)
  * Geolocation via IP
* [ ] Record **outgoing response**:

  * Status code, latency, headers
* [ ] Export full request/response logs to central dashboard

---

## 5. 📊 Metrics & System Health

* [ ] Track:

  * CPU, RAM, disk usage per container and host
  * Network in/out
  * Open ports, zombie processes
* [ ] Publish metrics to Prometheus-compatible endpoint (or internal API)
* [ ] Flag anomalies (e.g. RAM over 90%) and send alert
* [ ] Expose `/metrics` and `/healthz` endpoints

---

## 6. 🪵 Log Stream + Storage

* [ ] Tail `stdout` and `stderr` from all app containers
* [ ] Timestamp, container ID, app name per log line
* [ ] Stream logs to:

  * Developer dashboard in real-time (via WebSocket)
  * Optional: Long-term storage (e.g., S3 or local volume rotation)
* [ ] Add log filters: by app, by level, by container

---

## 7. 🛡️ Failover & Mirroring

* [ ] If main container fails health check:

  * Route to mirrored container instantly
  * Retry original container after cooldown
* [ ] Optional config: `mirror: true` in `nextdeploy.yml`
* [ ] Sync state from primary to mirror (for session persistence)

---

## 8. ⚙️ Deployment Binary Builder

* [ ] Compile binaries for:

  * `linux/amd64`
  * `darwin/arm64`
  * `windows/amd64`
* [ ] Use `goreleaser` or `xgo` for cross-platform builds
* [ ] Output CLI + Daemon `.tar.gz` artifacts for distribution
* [ ] Build Webby-compatible installer shell script for easy self-hosted VPS deployment

---

## 9. 🧪 Observability Testing

* [ ] Load test with 10k req/min
* [ ] Kill containers randomly and ensure daemon recovers
* [ ] Simulate disk full, RAM maxed, network drop
* [ ] Ensure no silent crashes or zombie processes

---

## 10. 🧬 Developer Dashboard Integration

* [ ] Expose REST API for:

  * Logs
  * Metrics
  * Health
  * Daemon status
* [ ] Add basic auth or JWT token auth for endpoints
* [ ] Make dashboard pluggable via WebSocket for real-time events

---

## 11. 🚀 Future Ideas (Post-MVP)

* [ ] Add rate limiting + throttling per IP/app
* [ ] Add usage billing tracker (RAM hours, bandwidth, req count)
* [ ] Add Sentry-style error tracking + alerts
* [ ] Integrate runtime config editing from dashboard (like Vercel env panel)

---

## ⚠️ Critical Philosophy

* 🧨 **Don't crash**. Recover from all panics.
* 🧠 **Be observable**. Every error, restart, request, container, or metric must be visible.
* ⚡ **Be fast**. Zero-lag deployment and auto-recovery.
* 🔐 **Be secure**. No port open without a reason. No root exec without control.
* 💬 **Give feedback**. Devs should *feel* like they're on Vercel — even when it's a \$5 VPS.

---
