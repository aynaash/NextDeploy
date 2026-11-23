---
title: AWS Lambda Compute Engine
lede: Packaging, memory sizing, cold start mitigation, and lifecycle execution of Next.js standalone on AWS Lambda.
status: ready
sourceRef: cli/internal/serverless/aws_lambda.go, shared/config/types.go
related:
  - title: Lambda Bridge Adapter
    href: /docs/aws/lambda-bridge
  - title: Secrets & Lambda Extension
    href: /docs/secrets/lambda-extension
  - title: CloudFront Edge CDN
    href: /docs/aws/cloudfront
---

## Packaging Next.js Standalone

When deploying to AWS Lambda, NextDeploy packages the output produced by `output: 'standalone'` in your `next.config.mjs`:

1. **Selective Inclusion**: NextDeploy bundles only `.next/standalone`, necessary server files, and minimal node_modules required for server execution.
2. **Asset Stripping**: Static assets (`.next/static/` and `public/`) are excluded from the zip bundle and uploaded directly to S3. This keeps the Lambda deployment package well under the AWS unzipped 250MB limit (typically **18MB to 35MB**).
3. **Architecture Optimization**: Automatically compiles and packages for `arm64` (AWS Graviton2/3) by default for up to 34% better price-performance compared to `x86_64`.

---

## Configuration Reference

Customize Lambda runtime parameters in `nextdeploy.yml`:

```yaml
target:
  type: serverless
  provider: aws

serverless:
  provider: aws
  region: us-east-1
  memory_size: 1536       # Megabytes (default: 1024, recommended: 1536)
  timeout: 30             # Execution timeout in seconds (default: 30)
  runtime: nodejs20.x     # Node.js runtime version
  handler: server.handler # Exported bridge handler
  warmer: true            # Enables EventBridge warmer ping every 5 minutes
```

---

## Memory Sizing & CPU Allocation

AWS Lambda couples CPU allocation directly to configured memory:

| Memory (MB) | vCPU Equivalent | Cold Start | Typical SSR Latency | Cost Impact |
| :--- | :--- | :--- | :--- | :--- |
| **512 MB** | 0.3 vCPU | ~1200ms | 85ms | Lowest baseline |
| **1024 MB** | 0.6 vCPU | ~650ms | 38ms | Good balance |
| **1536 MB (Recommended)** | 1.0 Full vCPU | ~380ms | 18ms | Optimal execution speed |
| **3008 MB** | 2.0 vCPU | ~250ms | 14ms | High-concurrency heavy compute |

---

## Cold Start Mitigation

NextDeploy provides two built-in mechanisms to eliminate cold start penalties:

### 1. Built-in Lambda Warmer (`warmer: true`)
When enabled, NextDeploy provisions an Amazon EventBridge scheduled rule that pings the Lambda function every 5 minutes with a lightweight payload `{ "source": "nextdeploy-warmer" }`. The bridge detects this ping and immediately returns without invoking the Next.js routing pipeline, keeping execution environments warm 24/7.

### 2. AWS Secrets Manager Extension
Instead of loading secrets over slow HTTPS API calls inside the handler, NextDeploy mounts the official AWS Secrets Manager Extension layer, caching secrets in a local HTTP daemon memory buffer.
