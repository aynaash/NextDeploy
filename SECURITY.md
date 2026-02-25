# Security Policy

## Supported Versions

The following versions of NextDeploy are currently receiving security updates:

| Version | Supported          |
| ------- | ------------------ |
| 1.0.x   | :white_check_mark: |
| < 1.0   | :x:                |

## Reporting a Vulnerability

**Please do not report security vulnerabilities through public GitHub issues.**

If you discover a security vulnerability in NextDeploy, please report it responsibly:

1. **Email**: [yussuf@hersi.dev](mailto:yussuf@hersi.dev) or open a [GitHub private security advisory](https://github.com/aynaash/NextDeploy/security/advisories/new)
2. **Include** as much of the following as possible:
   - Type of vulnerability (e.g. remote code execution, credential leak, SSRF)
   - Full path of the affected source file(s)
   - Steps to reproduce or a proof-of-concept
   - Potential impact and attack scenario

### What to expect

| Timeline | Action |
| -------- | ------ |
| Within **48 hours** | Acknowledgement of your report |
| Within **7 days** | Initial triage and severity assessment |
| Within **30 days** | Patch or mitigation plan communicated |
| Post-fix | Public disclosure coordinated with you |

If a vulnerability is **accepted**, you will be credited in the release notes (unless you prefer to remain anonymous).

If a vulnerability is **declined** (e.g. out of scope or not reproducible), you will receive a clear explanation.

## Scope

The following are considered in scope:

- `cli/` — the `nextdeploy` CLI binary
- `daemon/` — the `nextdeployd` server daemon
- `shared/` — shared libraries used by both
- Ansible provisioning playbooks in `cli/cmd/ansible/`

The following are **out of scope**:

- Vulnerabilities in third-party dependencies (please report those upstream)
- Issues in example configs or documentation
- Social engineering attacks

## Security Best Practices for Users

- Store SSH private keys with `chmod 600` and never commit them to source control
- Use Doppler or another secrets manager — never put secrets directly in `nextdeploy.yml`
- Rotate your deployment server SSH keys periodically
- Keep the `nextdeploy` binary up to date to receive the latest security patches