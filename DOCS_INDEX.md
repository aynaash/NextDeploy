# NextDeploy Master Documentation Index

A comprehensive directory of all technical, operational, architectural, and developer documentation across the NextDeploy monorepo.

---

## 1. Root Engineering Documents

| Document | Description |
| :--- | :--- |
| [README.md](./README.md) | High-level project overview, quick start, installation, key features, and demo. |
| [ARCHITECTURE.md](./ARCHITECTURE.md) | Complete technical blueprint of NextDeploy: CLI, daemon, nextcompile, targets, and data flow. |
| [CONTRIBUTING.md](./CONTRIBUTING.md) | Guide for setting up local development, running test suites, and submitting PRs. |
| [CODE_QUALITY.md](./CODE_QUALITY.md) | Go code quality standards: size targets, error handling, decoupling, and concurrency. |
| [agentrace.md](./agentrace.md) | Architectural specification of Agent-First Infrastructure and Handoff (Agent Trace). |
| [CLOUDFLARE_PARITY.md](./CLOUDFLARE_PARITY.md) | Feature matrix and parity tracker for Cloudflare Workers capabilities. |
| [CHANGELOG.md](./CHANGELOG.md) | Release history and version updates. |
| [sample.nextdeploy.yml](./sample.nextdeploy.yml) | Fully annotated reference configuration file. |
| [TODOS.md](./TODOS.md) | Engineering roadmap and prioritized TODOs across all subsystems. |

---

## 2. Frontend Application Documentation (`frontend/content/docs/`)

The documentation powering the official Next.js 16 website (`nextdeploy.org`).

### A. Getting Started
* [Installation](./frontend/content/docs/getting-started/installation.md): Install the CLI on macOS, Linux, Windows, or from Go source.
* [VPS Quickstart](./frontend/content/docs/getting-started/vps-quickstart.md): Deploy your first app to a bare-metal Linux server.
* [Cloudflare Quickstart](./frontend/content/docs/getting-started/cloudflare-quickstart.md): Edge SSR on Cloudflare Workers in under 60 seconds.
* [AWS Quickstart](./frontend/content/docs/getting-started/aws-quickstart.md): Deploy to AWS Lambda, S3, CloudFront, and ACM.

### B. Agent Trace & AI Coding Agents
* [Agent-First Infrastructure](./frontend/content/docs/agent-trace/agent-first-infra.md): Why AI coding agents need native infrastructure execution.
* [Agent Trace Architecture](./frontend/content/docs/agent-trace/architecture.md): The context continuity layer between agents and developers.
* [CLI & Human Takeover](./frontend/content/docs/agent-trace/cli-takeover.md): The `takeover -o` workflow, `:4044` web dashboard, and model resume.

### C. Model Context Protocol (MCP) & LLM Documentation
* [Model Context Protocol Server](./frontend/content/docs/mcp/server.md): Connecting Cursor, Claude Desktop, and Antigravity IDE over stdio JSON-RPC.
* [MCP Tool Schemas & API](./frontend/content/docs/mcp/tools.md): Formal JSON Schema contracts for all 8 NextDeploy tools.
* [Agent Rules & Prompt Templates](./frontend/content/docs/mcp/prompt-engineering.md): Best practices for `AGENTS.md`, `.cursorrules`, and autonomous deployments.
* [Standardized llms.txt](./frontend/public/llms.txt): Machine-readable index for LLMs and agents.
* [Full Condensed llms-full.txt](./frontend/public/llms-full.txt): Complete single-file documentation payload for offline context windows.

### D. Core Concepts
* [How It Works](./frontend/content/docs/concepts/how-it-works.md): The 5-stage deployment pipeline.
* [Build Metadata](./frontend/content/docs/concepts/metadata.md): Static AST analysis and `metadata.json` generation.
* [Architecture](./frontend/content/docs/concepts/architecture.md): High-level system design and data paths.
* [Security Model](./frontend/content/docs/concepts/security.md): ChaCha20-Poly1305 encryption, HMAC request signing, and process isolation.

### D. VPS Target & Daemon
* [VPS Overview](./frontend/content/docs/vps/overview.md): Caddy, `nextdeployd`, and systemd on Linux.
* [nextdeployd Daemon](./frontend/content/docs/vps/daemon.md): The static Go daemon running on managed hosts.
* [Go Sidecar](./frontend/content/docs/vps/go-sidecar.md): Pure-Go fast path for prerendered HTML and image optimization.
* [Zero-Downtime Cutover](./frontend/content/docs/vps/atomic-cutover.md): Dual-stage health verification and atomic symlink swap.
* [Cumulative Chunk Pooling](./frontend/content/docs/vps/chunk-pooling.md): Eliminating `ChunkLoadError` 404s in active sessions.
* [systemd Hardening](./frontend/content/docs/vps/systemd-hardening.md): `DynamicUser`, `ProtectSystem`, and cgroup v2 sandboxing.
* [Caddy & TLS](./frontend/content/docs/vps/caddy.md): Automatic Let's Encrypt TLS and NVMe byte offloading.
* [Instant Rollbacks](./frontend/content/docs/vps/rollback.md): Sub-second rollbacks via the 5-release ring buffer.

### E. Cloudflare Workers Target
* [Cloudflare Overview](./frontend/content/docs/cloudflare/overview.md): Compiling Next.js for Cloudflare Workers.
* [Cloudflare Quickstart](./frontend/content/docs/cloudflare/quickstart.md): Step-by-step edge deployment tutorial.
* [Workers Runtime](./frontend/content/docs/cloudflare/workers.md): `workerd` VFS runtime shims and streaming HTTP bridge.
* [Resource Bindings](./frontend/content/docs/cloudflare/bindings.md): Declarative binding configuration in YAML.
* [Cloudflare R2](./frontend/content/docs/cloudflare/r2.md): Static asset distribution with immutable caching headers.
* [Cloudflare D1 SQL](./frontend/content/docs/cloudflare/hyperdrive.md): SQL connection pooling and edge database bindings.
* [Queues & Messaging](./frontend/content/docs/cloudflare/queues.md): Background asynchronous processing.
* [Durable Objects](./frontend/content/docs/cloudflare/durable-objects.md): State synchronization and migrations.
* [KV, Vectorize & AI](./frontend/content/docs/cloudflare/kv-vectorize-ai.md): Key-value caching and AI Gateway integration.
* [Cloudflare DNS](./frontend/content/docs/cloudflare/dns.md): Automated zone DNS and custom worker domains.

### F. AWS Serverless Target
* [AWS Lambda](./frontend/content/docs/aws/lambda.md): Node.js 20.x compute packaging and cold-start optimization.
* [Amazon CloudFront](./frontend/content/docs/aws/cloudfront.md): Global CDN, Origin Access Control, and ACM SSL certificates.
* [Lambda Bridge Adapter](./frontend/content/docs/aws/lambda-bridge.md): Translating API Gateway / Lambda Function URL events to HTTP.
* [ISR & Revalidation](./frontend/content/docs/aws/isr.md): SQS revalidation queues and non-blocking cache updates.
* [Image Optimization](./frontend/content/docs/aws/image-optimization.md): Dedicated serverless image optimization function.

### G. Secrets & Configuration
* [Secrets Overview](./frontend/content/docs/secrets/overview.md): Encrypted secret lifecycle and providers.
* [Local Encryption](./frontend/content/docs/secrets/encryption.md): ChaCha20-Poly1305 key generation and secret files.
* [Doppler Integration](./frontend/content/docs/secrets/doppler.md): Dynamic secret synchronization from Doppler projects.
* [AWS Lambda Extension](./frontend/content/docs/secrets/lambda-extension.md): In-memory secret loading without env exposure.
* [YAML Reference](./frontend/content/docs/config/reference.md): Complete field-by-field reference for `nextdeploy.yml`.
* [Config Examples](./frontend/content/docs/config/examples.md): Real-world configuration templates for all targets.

### H. CLI Commands
* [CLI Command Reference](./frontend/content/docs/cli/reference.md): Syntax and flags for `init`, `plan`, `ship`, `status`, etc.
* [Trace & Takeover Reference](./frontend/content/docs/cli/trace.md): Detailed reference for `nextdeploy trace` and `takeover`.
* [CLI Self-Update](./frontend/content/docs/cli/self-update.md): Automated binary updates via `nextdeploy update`.

### I. Operations & CI/CD
* [Monitoring & Health](./frontend/content/docs/operations/monitoring.md): Dual-stage probing and log streaming.
* [Audit Logging](./frontend/content/docs/operations/audit-log.md): Structured audit records of every deployment action.
* [CI/CD Pipelines](./frontend/content/docs/operations/ci-cd.md): Automated GitHub Actions workflows for VPS and Cloudflare.

### J. Advanced Topics & Reference
* [NextCompile Engine](./frontend/content/docs/advanced/nextcompile.md): Internal compiler translating Next.js to Cloudflare Workers.
* [Next.js 16 Turbopack on Workers](./frontend/content/docs/advanced/nextjs-16-turbopack-on-cloudflare.md): Full Turbopack support.
* [Auto-Provisioning](./frontend/content/docs/advanced/auto-provisioning.md): Zero-touch cloud resource creation.
* [Local Development & Air](./frontend/content/docs/advanced/local-development.md): Hot-reloading Go binaries with Mage and Air.
* [Limitations](./frontend/content/docs/reference/limitations.md): Current runtime and size boundaries.
* [Troubleshooting](./frontend/content/docs/reference/troubleshooting.md): Diagnosis and resolution guide for common errors.
* [FAQ](./frontend/content/docs/reference/faq.md): Frequently asked technical questions.
* [Roadmap](./frontend/content/docs/roadmap.md): Architectural roadmap and planned features.

---

## 3. VitePress Documentation (`docs/`)

Maintained for static deployment at `nextdeploy.org/docs` or via `mage docs`:
* [`docs/guide/`](./docs/guide/): Getting Started, Core Architecture, Configuration Reference, Agent-First Infra.
* [`docs/targets/`](./docs/targets/): VPS, Cloudflare Workers, and AWS Serverless guides.
* [`docs/cli/`](./docs/cli/): Detailed CLI flag documentation for all commands.
* [`docs/internals/`](./docs/internals/): Low-level documentation on atomic cutover, chunk pooling, Go sidecar, and agent trace.
