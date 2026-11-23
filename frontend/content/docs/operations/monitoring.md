---
title: Monitoring & Health Checks
lede: "Monitor live deployments with nextdeploy status, real-time log streaming, post-deploy SmokeVerify probes, HTML deployment reports, Prometheus metrics, and automated alerts."
status: ready
sourceRef: cli/cmd/status.go, cli/cmd/logs.go, cli/internal/serverless/smoke.go, cli/internal/serverless/resource_view.go, daemon/internal/metrics/
related:
  - title: Audit Log
    href: /docs/operations/audit-log
  - title: CI/CD Pipelines
    href: /docs/operations/ci-cd
  - title: Troubleshooting
    href: /docs/reference/troubleshooting
---

## Real-Time Observability Overview

NextDeploy provides end-to-end monitoring across VPS bare metal, Cloudflare Workers, and AWS Lambda deployments:

1. **CLI Status & Health Probes**: Real-time snapshot of active release IDs, git hashes, uptime, memory/CPU consumption, and active ports.
2. **Unified Log Streaming**: Live log tailing that multiplexes across systemd journald (VPS), CloudWatch Logs (AWS), or Tail Workers (Cloudflare).
3. **Automated Smoke Verification (`SmokeVerify`)**: Post-deploy validation that tests HTTP response codes and latencies across public endpoints before closing cutover.
4. **Prometheus Metrics Endpoint**: Native `/metrics` exporter in `nextdeployd` for Grafana dashboards.
5. **Alert Webhooks**: Configurable Slack, Discord, or generic JSON webhooks triggered on crash or health check failure.

---

## Live Status & Log Streaming

### Inspect Live Status (`nextdeploy status`)

```bash
# Output human-readable terminal table
nextdeploy status

# Output structured JSON for AI agents and automated monitors
nextdeploy status --json
```

Sample output:
```text
App: saas-dashboard
Target: VPS (203.0.113.10)
Release: 2026-11-23_14-20-00 (Git: 8f4a1c2 - "feat: add stripe checkout")
Uptime: 4h 12m
Status: HEALTHY (HTTP 200 OK - 14ms)
Memory: 184MB / 1024MB (18.0%)
CPU: 2.1%
Active Port: 3002 (Caddy routing via 127.0.0.1:3002)
Historical Releases: 5 retained (Rollback target: 2026-11-23_10-15-00)
```

### Tail Live Logs (`nextdeploy logs`)

```bash
# Follow logs in real-time
nextdeploy logs -f

# Limit output to last 100 lines
nextdeploy logs -n 100

# Filter by error level
nextdeploy logs --filter "level=error"
```

- On **VPS**: Streams directly from `journalctl -u nextdeploy@<app> -f`.
- On **Cloudflare**: Connects to Cloudflare Workers Trace/Tail WebSocket stream.
- On **AWS Lambda**: Tail streams from `/aws/lambda/<app>-serverless` via AWS CloudWatch Logs Live Tail.

---

## Post-Deploy Smoke Verification (`SmokeVerify`)

After every serverless or VPS cutover, NextDeploy automatically probes your custom domain root `/` plus critical application routes before marking the deployment successful:

```yaml
# nextdeploy.yml
monitoring:
  smoke_verify:
    enabled: true
    timeout_seconds: 15
    retries: 3
    retry_delay_seconds: 2
    paths:
      - /
      - /pricing
      - /api/health
    expected_status: [200, 307] # 307 allowed for authenticated redirects
```

### Smoke Verification Rules:
- Probes run with custom user-agent `NextDeploy-SmokeVerify/1.0`.
- Automatically ignores internal Next.js error paths (`/_not-found`, `/_global-error`, `/404`, `/500`).
- If an endpoint returns 5xx status codes consecutively across retries, `nextdeploy ship` halts, triggers an automatic rollback to the previous release, and logs full trace provenance.

---

## HTML Deployment Reports

Every deployment generates a self-contained, interactive HTML architecture and DNS report inside `.nextdeploy/reports/` (gitignored so deploys never dirty your Git tree):

- `.nextdeploy/reports/<app>-<timestamp>-report.html`: Historical immutable deploy log.
- `.nextdeploy/reports/<app>-latest.html`: Always points to the current active release.

The report includes:
- Git commit metadata (author, branch, commit message, commit hash).
- Active Agent Trace session summary and prompt trails.
- Serverless resource ARN / Cloudflare ID mappings.
- Exact DNS routing verification records.
- Latency waterfall from the SmokeVerify phase.

---

## Prometheus Metrics Exporter (VPS Daemon)

The `nextdeployd` systems daemon exposes a Prometheus-compatible metrics endpoint at `http://127.0.0.1:9100/metrics` (or configured port):

```text
# HELP nextdeploy_app_up Whether the Next.js application process is healthy (1) or down (0).
# TYPE nextdeploy_app_up gauge
nextdeploy_app_up{app="saas-dashboard"} 1

# HELP nextdeploy_app_memory_rss_bytes Resident Memory Set in bytes.
# TYPE nextdeploy_app_memory_rss_bytes gauge
nextdeploy_app_memory_rss_bytes{app="saas-dashboard"} 192937984

# HELP nextdeploy_app_cpu_percent Total CPU usage percentage.
# TYPE nextdeploy_app_cpu_percent gauge
nextdeploy_app_cpu_percent{app="saas-dashboard"} 2.45

# HELP nextdeploy_http_response_time_seconds HTTP probe response time in seconds.
# TYPE nextdeploy_http_response_time_seconds histogram
nextdeploy_http_response_time_seconds_bucket{app="saas-dashboard",le="0.05"} 1420
```

Add to your `prometheus.yml`:
```yaml
scrape_configs:
  - job_name: "nextdeploy"
    static_configs:
      - targets: ["203.0.113.10:9100"]
```

---

## Automated Webhook Alerts (Slack / Discord)

Configure failure notifications in `nextdeploy.yml`:

```yaml
monitoring:
  enabled: true
  cpu_threshold: 85
  memory_threshold: 85
  alert:
    slack_webhook: ${SLACK_WEBHOOK_URL}
    discord_webhook: ${DISCORD_WEBHOOK_URL}
    notify_on:
      - crash
      - healthcheck_failed
      - cutover_aborted
      - high_memory_sustained
```

When an alert triggers, NextDeploy posts a payload containing:
- Application name and environment.
- Active release ID and Git commit hash.
- The last 25 lines of application stderr.
- One-click rollback command (`nextdeploy rollback`).
