# NextDeploy CI/CD Pipeline Integration

NextDeploy's native deployment architecture is designed to seamlessly integrate into headless CI/CD environments like GitHub Actions or GitLab CI without requiring interactive prompts.

Because we have removed Docker from the CLI and the Daemon, your deployment is faster, natively interacts with your system processes via Systemd, and supports Doppler token injection securely.

## Required Secrets

Before configuring your pipeline, ensure the following secrets are available in your repository settings:

- **`SSH_PRIVATE_KEY`**: The private key used to connect to your deployment VPS. NextDeploy will read this file during the `ship` process to upload your artifacts and trigger your daemon.
- **`DOPPLER_TOKEN`** (Optional but Recommended): The Service Token used to securely fetch and inject deployment environment variables during the Next.js execution.

## GitHub Actions Example

Below is a complete example of a GitHub Action workflow that builds a Next.js payload and ships it directly to your servers using NextDeploy:

```yaml
name: Deploy Next.js App

on:
  push:
    branches:
      - main

jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - name: Checkout Code
        uses: actions/checkout@v3

      - name: Setup Node.js
        uses: actions/setup-node@v3
        with:
          node-version: 20

      - name: Install Dependencies
        run: npm ci

      - name: Setup Go (For NextDeploy CLI)
        uses: actions/setup-go@v4
        with:
          go-version: 1.22

      - name: Install NextDeploy CLI
        run: |
          go install github.com/aynaash/NextDeploy/cli@latest
          export PATH=$PATH:$(go env GOPATH)/bin

      - name: Setup SSH Key
        run: |
          mkdir -p ~/.ssh
          echo "${{ secrets.SSH_PRIVATE_KEY }}" > ~/.ssh/nextdeploy-key.pem
          chmod 600 ~/.ssh/nextdeploy-key.pem
          # Ensure your nextdeploy config points to this key path

      - name: Build Next.js Payload
        run: nextdeploy build

      - name: Ship Application
        env:
          DOPPLER_TOKEN: ${{ secrets.DOPPLER_TOKEN }}
        run: nextdeploy ship
```

## How It Works Under The Hood

1. **`nextdeploy build`**: NextDeploy analyzes your source code. If you use standalone mode, it gathers the minimal Node.js traces required to execute your app. It consolidates these files into a `app.tar.gz` archive.
2. **`nextdeploy ship`**: The CLI connects to your targeted VPS utilizing the `SSH_PRIVATE_KEY` defined in your YAML config. It securely uploads `app.tar.gz` and sends an execution instruction to the `nextdeployd` Daemon.
3. **Daemon Execution**: The `nextdeployd` daemon unpacks the payload, updates your active `systemd` instance to safely switch versions wrapper by `doppler run -- node server.js`, and seamlessly reloads the `Caddy` reverse proxy. All zero downtime and completely native.
