# NextDeploy: Local Development & Deployment Roadmap

This roadmap outlines the steps to build out NextDeploy's local development environment, establish our core serverful deployment logic (VPS, AWS, GCP), and eventually extend into serverless territory (Cloudflare, AWS Lambda, GCP Cloud Run).

## 🛠️ Phase 1: Local Development Setup
The foundation to ensure contributors can build and test the CLI and Daemon locally.

- [ ] **Go Workspace Setup**: Ensure the Go modules for `cli/` and `daemon/` are properly linked and buildable via `Makefile`.
- [ ] **CLI Implementation - `init`**: Scaffold a default `Dockerfile` tailored for Next.js standalone builds and a default `nextdeploy.yml`.
- [ ] **CLI Implementation - `build`**: Use local Docker socket to build the Next.js image (`docker build`).
- [ ] **CLI Implementation - `runimage`**: Run the built image locally injected with Doppler secrets to verify production readiness.
- [ ] **Local VPS Simulation**: Create a `docker-compose` setup that spins up a local Ubuntu container with SSH enabled to simulate a remote VPS for testing the `provision` and `ship` commands without needing real cloud infrastructure.

## 🚀 Phase 2: Core Serverful Deployments (Ubuntu + Docker)
Our primary goal: Reliable Next.js deployments to any Ubuntu machine.

- [ ] **Provisioning (`nextdeploy provision`)**: Automate SSH installation of Docker, Caddy, and the NextDeploy Daemon (`nextdeployd`) on a fresh Ubuntu server.
- [ ] **Daemon Runtime (`nextdeployd`)**: Build the Go daemon to listen for deployment events, manage the Docker lifecycle of the Next.js app, and stream logs.
- [ ] **Shipping (`nextdeploy ship`)**: Push the built image to a remote registry (or export/import via SSH directly) and trigger the daemon to restart the container.
- [ ] **Networking & SSL**: Automate Caddy configuration via the daemon to route incoming domain traffic to the running Next.js container and auto-provision Let's Encrypt certificates.
- [ ] **Secrets Management**: Securely pass Doppler tokens or flat environment variables to the container runtime.

## ☁️ Phase 3: Cloud Provider Serverful (AWS & GCP)
Expanding the serverful model to major cloud providers.

- [ ] **Abstract Infrastructure Interfaces**: Ensure the deployment logic is completely agnostic to the host as long as it has SSH and Ubuntu.
- [ ] **AWS EC2 Support**: Add connection documentation and verify deployments on AWS Ubuntu AMIs. Ensure security groups/firewalls are properly documented for Caddy (Ports 80/443).
- [ ] **GCP Compute Engine Support**: Verify deployments on GCP Ubuntu instances and document firewall ingress rules.

## ⚡ Phase 4: Serverless Deployments (AWS, GCP, Cloudflare)
Rolling out serverless options while maintaining the same CLI experience.

- [ ] **Config Updates**: Extend `nextdeploy.yml` to support an `infrastructure: serverless` target.
- [ ] **Build Adapters (OpenNext)**: Integrate `open-next` or similar build adapters at the CLI level to compile Next.js into serverless-friendly artifacts instead of Docker images.
- [ ] **GCP Cloud Run (Easiest Entry)**: Use the existing Docker build logic to push images directly to Google Artifact Registry and deploy to Cloud Run. Keep it serverless but container-based.
- [ ] **AWS Lambda / SST**: Deploy the OpenNext artifact to AWS Lambda + CloudFront.
- [ ] **Cloudflare Pages/Workers**: Support deploying Edge-compatible Next.js apps directly to Cloudflare via Wrangler under the hood.
