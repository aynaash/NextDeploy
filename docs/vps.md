# VPS target

**Status: frozen.** Bugs are fixed; features are not added. This page is the
whole contract.

## Why it still exists

NextDeploy's investment goes to Cloudflare. VPS stays for one reason: it is the
only target whose maintenance burden doesn't track Next.js's release schedule.
It runs the real Node.js Next.js server, so a Next release works here on day one
with no adapter work — while the Workers path waits on runtime support (and
OpenNext, for anyone using it, waits too).

That makes it the answer to exactly one question: *"Next shipped something the
Workers runtime can't run yet — what do I do this quarter?"* Deploy to a VPS,
and move back when [`CLOUDFLARE_PARITY.md`](../CLOUDFLARE_PARITY.md) says the
gap closed.

It is not a second product, and it is not where new work goes. If you want a
self-hosted PaaS, Coolify and Dokploy are better at being that than a frozen
target here will be.

**Review date: 2026-03-05.** If nothing has deployed with `target_type: vps` by
then, it gets deleted — on evidence, not on argument.

## What you get

- **Caddy** in front, with automatic HTTPS. `public/` is served by Caddy
  directly rather than proxied through Node.
- **`nextdeployd`**, a daemon on each server: health-gated cutover, per-app
  locks, graceful stop, systemd units with cgroup limits, log aggregation.
- **CSP derived from the app**, via `DetectFeatures` — not a generic default.
- **Standalone-aware Caddy config**, ISR handling, rollback history.

## Flow

```bash
nextdeploy prepare        # provision the server (idempotent)
nextdeploy ship           # build, ship over SSH, health-gated cutover
nextdeploy logs -f        # stream from the daemon's aggregator
nextdeploy rollback       # previous release, or --to <commit>
nextdeploy status         # daemon-reported state per server
nextdeploy upgrade-daemon # update nextdeployd in place
```

`prepare` runs in **agent mode** by default: it installs the `nextdeployd`
static binary on the target over SSH and lets it do the provisioning there. The
server needs only a shell and `curl`/`wget` — no Python — and this machine needs
no Ansible. `--ansible` selects the legacy playbook path. Every step checks
before it changes, so re-running on a prepared server is a fast no-op.

Root SSH logins are refused unless you pass `--allow-root`.

## Config

```yaml
target_type: vps

app:
  name: example-app        # ^[a-z0-9-]+$, 3-63 chars — a slug, not a domain
  environment: production
  domain: app.example.com
  port: 3000

servers:
  - name: web-1
    host: 1.2.3.4
    username: ubuntu
    key_path: ~/.ssh/id_ed25519
```

See [`sample.nextdeploy.yml`](../sample.nextdeploy.yml) for the annotated
schema. `nextdeploy <cmd> explain` prints the per-command walkthrough.

## Known sharp edge

The CLI and `nextdeployd` version independently, and `NextCorePayload` — written
by the CLI, shipped in the tarball, unmarshalled by the daemon — carries a
`schema_version`. A daemon older than the payload fails loudly rather than
silently reading zero values. Keep them in step with `nextdeploy upgrade-daemon`
after a CLI update.
