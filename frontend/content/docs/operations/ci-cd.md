---
title: CI/CD Pipelines
lede: Automate drift detection on pull requests with nextdeploy plan and zero-downtime deployments on merge with nextdeploy ship using GitHub Actions.
status: ready
sourceRef: cli/cmd/generate_ci.go, cli/internal/cicd/workflow.go, docs/cli/ci-cd.md
related:
  - title: Doppler Integration
    href: /docs/secrets/doppler
  - title: CLI Reference
    href: /docs/cli/reference
---

## Scaffold a Workflow Automatically

Generate a GitHub Actions pipeline tailored to your `nextdeploy.yml` target:

```bash
nextdeploy generate-ci
```

## Example GitHub Actions Workflow (Cloudflare Workers)

```yaml
name: NextDeploy CI/CD

on:
  pull_request:
    branches: [main]
  push:
    branches: [main]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-node@v4
        with:
          node-version: "22"
      - run: npm ci
      - run: npm test

  plan:
    if: github.event_name == 'pull_request'
    needs: test
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Install NextDeploy
        run: curl -fsSL https://nextdeploy.org/install.sh | bash
      - name: Dry-Run Plan
        run: nextdeploy plan --detailed-exitcode
        env:
          CLOUDFLARE_API_TOKEN: ${{ secrets.CLOUDFLARE_API_TOKEN }}
          CLOUDFLARE_ACCOUNT_ID: ${{ secrets.CLOUDFLARE_ACCOUNT_ID }}

  deploy:
    if: github.ref == 'refs/heads/main' && github.event_name == 'push'
    needs: test
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Install NextDeploy
        run: curl -fsSL https://nextdeploy.org/install.sh | bash
      - name: Ship Application
        run: nextdeploy ship
        env:
          CLOUDFLARE_API_TOKEN: ${{ secrets.CLOUDFLARE_API_TOKEN }}
          CLOUDFLARE_ACCOUNT_ID: ${{ secrets.CLOUDFLARE_ACCOUNT_ID }}
```

For VPS deploys, add your `SSH_PRIVATE_KEY` to GitHub Secrets (loaded via `ssh-agent`) and run `nextdeploy ship` on merge to `main`.
