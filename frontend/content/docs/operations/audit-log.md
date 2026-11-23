---
title: Audit Logging & Deployment Provenance
lede: Cryptographically verifiable, structured JSON audit logs recording every deployment, secret change, and system action.
status: ready
sourceRef: shared/logger.go, daemon/internal/daemon/
related:
  - title: Security Model
    href: /docs/concepts/security
  - title: Monitoring & Health
    href: /docs/operations/monitoring
---

## Overview

In enterprise, fintech, and healthcare environments (SOC 2, HIPAA, ISO 27001), deployment infrastructure must provide an immutable, verifiable audit trail:
- *Who initiated the deployment?*
- *What git commit and model/agent generated the code?*
- *Which environment variables or secrets were altered?*
- *What was the exact health check output before traffic cutover?*

NextDeploy writes structured JSON audit event logs for every operation.

---

## Audit Event Schema

Audit events are appended to `/var/log/nextdeploy/audit.log` (on VPS hosts) and emitted as structured JSON stdout logs in CI/CD:

```json
{
  "timestamp": "2026-11-23T15:37:00.124Z",
  "event_id": "aud_01HZX8M2P1Q8",
  "actor": {
    "user": "yussufhersi",
    "auth_method": "ssh_key_ed25519",
    "ip_address": "198.51.100.42"
  },
  "action": "SHIP_CUTOVER",
  "application": "my-app",
  "target": "vps",
  "provenance": {
    "git_commit": "8f3b21c",
    "git_branch": "main",
    "is_dirty": false,
    "nextjs_version": "16.0.10",
    "agent_session_id": "sess_9821"
  },
  "health_verification": {
    "tcp_port": 3004,
    "http_status": 200,
    "probe_duration_ms": 14
  },
  "status": "SUCCESS"
}
```

---

## Log Ingestion & SIEM Integration

Because audit logs are formatted as single-line JSON, they ingest seamlessly into standard log analytics and SIEM pipelines:
- **Datadog**: Stream via Datadog Agent reading `/var/log/nextdeploy/audit.log`.
- **AWS CloudWatch**: Ship stdout logs directly to CloudWatch Log Groups.
- **Grafana Loki / Promtail**: Collect and index logs by `event_id`, `actor`, and `application`.
