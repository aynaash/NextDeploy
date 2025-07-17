
// TODO(nextdeploy):
// - Add TLS and optional mTLS support
// - Move hardcoded paths to XDG_CONFIG_HOME-compatible locations
// - Add log streaming to external observability tools (e.g. Loki, Datadog)
// - Implement structured audit logging
// - Gracefully handle corrupted state or missing config files
// TODO(nextdeploy): Critical daemon improvements for production readiness
//
// ─────────────────────────────────────────────────────────────────────────────
// 🔒 SECURITY / PORTABILITY
// ─────────────────────────────────────────────────────────────────────────────
// - [ ] Add TLS support for both main and metrics servers (self-signed or config-driven).
// - [ ] Replace hardcoded paths (/var/lib, /var/run, /var/log) with configurable or XDG-based paths.
// - [ ] Add ENV var + config file (YAML/TOML) support using a config loader (e.g., Viper or custom).
//
// ─────────────────────────────────────────────────────────────────────────────
// ⚙️  SYSTEM RESILIENCE / ABSTRACTION
// ─────────────────────────────────────────────────────────────────────────────
// - [ ] Wrap all goroutines (e.g., startServer) with panic recovery + stack trace logging.
// - [ ] Create a `DaemonApp` struct to encapsulate and manage lifecycle of components (keyManager, servers, etc).
// - [ ] Decouple core component initialization out of main(); use start()/stop() methods per subsystem.
// - [ ] Add startup readiness vs liveness health endpoints (/health/startup, /health/live).
//
// ─────────────────────────────────────────────────────────────────────────────
// 📈 OBSERVABILITY
// ─────────────────────────────────────────────────────────────────────────────
// - [ ] Add request correlation ID middleware to all HTTP handlers (traceability).
// - [ ] Integrate OpenTelemetry support for tracing + metrics.
// - [ ] Expand /metrics to include more detailed runtime stats (GC, memory, goroutines, etc).
//
// ─────────────────────────────────────────────────────────────────────────────
// 🤖 BACKGROUND DAEMONS + SUPERVISION
// ─────────────────────────────────────────────────────────────────────────────
// - [ ] Build a Supervisor pattern to manage background daemons (deployment monitor, log stream, container checker).
// - [ ] Add daemon health probes + restart policies for background workers.
//
// ─────────────────────────────────────────────────────────────────────────────
// 🧠 FUTURE-SCALING ARCHITECTURE
// ─────────────────────────────────────────────────────────────────────────────
// - [ ] Prepare coordination/locking layer (Redis, etcd, or file-based) for future HA/multi-node deployments.
// - [ ] Allow dynamic reloading of config (SIGHUP logic needs to reload flags/config/env).
