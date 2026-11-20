# `nextdeploy generate-ci`

Scaffold production-grade CI/CD automation pipelines for GitHub Actions.

---

## Usage

```bash
nextdeploy generate-ci [flags]
```

### Flags

| Flag | Type | Description |
| :--- | :--- | :--- |
| `--provider` | string | CI provider to target (currently: `github-actions`). |
| `--preview` | boolean | Include ephemeral preview deployment workflows for Pull Requests. |
| `--output` | string | Target workflow file destination (default: `.github/workflows/nextdeploy.yml`). |

---

## Generated Workflow Example

The generated GitHub Actions workflow handles automated testing, dry-run planning on PRs, and atomic shipping on merge:

```yaml
name: NextDeploy CI/CD

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-node@v4
        with:
          node-version: 20
          cache: 'npm'
      - run: npm ci
      - run: npm test

  plan:
    if: github.event_name == 'pull_request'
    needs: test
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Install NextDeploy
        run: curl -fsSL https://hersi.dev/nextdeploy/install.sh | bash
      - name: Dry-run Plan
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
        run: curl -fsSL https://hersi.dev/nextdeploy/install.sh | bash
      - name: Ship Application
        run: nextdeploy ship
        env:
          CLOUDFLARE_API_TOKEN: ${{ secrets.CLOUDFLARE_API_TOKEN }}
          CLOUDFLARE_ACCOUNT_ID: ${{ secrets.CLOUDFLARE_ACCOUNT_ID }}
```
