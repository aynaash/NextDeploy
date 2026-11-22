# Security Policy

NextDeploy handles deployment credentials, application secrets, remote SSH
provisioning, and a privileged VPS daemon (`nextdeployd`). We take security
reports seriously and appreciate responsible disclosure.

## Supported Versions

While NextDeploy is in `0.x` development, security fixes are released on the
latest minor version:

| Version | Supported |
| ------- | --------- |
| Latest `0.x` release | Yes |
| Older `0.x` releases | No (please upgrade via `nextdeploy update`) |

## Reporting a Vulnerability

**Please do not report security vulnerabilities through public GitHub issues,
discussions, or Discord.**

Report vulnerabilities privately using GitHub Private Vulnerability Reporting:

- **Submit an advisory:** <https://github.com/aynaash/NextDeploy/security/advisories/new>

Include as much of the following as you can:

- Component affected (CLI, `nextdeployd`, `cmd/imgopt`, `shared/secrets`,
  `shared/updater`, Cloudflare/AWS/VPS adapter)
- NextDeploy version (`nextdeploy version`), OS, and deployment target
- Steps to reproduce or a minimal proof of concept
- Impact assessment and any suggested remediation

## What to Expect

- **Acknowledgment:** within 72 hours
- **Triage & fix plan:** once reproduced, we will share a target timeline and
  work with you on coordinated disclosure
- **Credit:** with your permission, we credit reporters in the GitHub Security
  Advisory and release notes

## High-Priority Security Areas

Reports in the following areas are prioritized:

- Credential or secret leakage (`shared/secrets`, `credstore`, Doppler/env
  harvesting, logs)
- `nextdeployd` command authentication (HMAC), privilege boundaries, or Caddy
  config injection
- Archive extraction path traversal or symlink escapes
- SSRF or arbitrary URL fetching in auxiliary Lambdas (`cmd/imgopt`,
  `cmd/revalidator`)
- Self-updater integrity or checksum verification (`shared/updater`)
