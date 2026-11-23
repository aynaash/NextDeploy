---
title: AWS Quickstart
lede: Deploy a Next.js app to your own AWS account using AWS Lambda, Amazon S3, CloudFront, and ACM TLS — provisioned and wired from one nextdeploy.yml.
status: ready
sourceRef: cli/internal/serverless/aws*.go, docs/targets/aws.md
related:
  - title: AWS Lambda
    href: /docs/aws/lambda
  - title: CloudFront
    href: /docs/aws/cloudfront
  - title: Lambda Bridge
    href: /docs/aws/lambda-bridge
  - title: DNS Configuration
    href: /docs/dns
---

## 1. Export AWS Credentials

Provide credentials for an IAM user or role with Lambda, S3, CloudFront, ACM, and IAM permissions:

```bash
export AWS_ACCESS_KEY_ID="AKIA..."
export AWS_SECRET_ACCESS_KEY="..."
export AWS_REGION="us-east-1"
```

Verify your credentials:

```bash
nextdeploy creds
```

## 2. Configure `nextdeploy.yml`

Run `nextdeploy init` and choose **Serverless → AWS**, or create `nextdeploy.yml`:

```yaml
version: "1.0"
target_type: serverless

app:
  name: cloud-store
  domain: store.example.com
  environment: production

serverless:
  provider: aws
  region: us-east-1
  memory_size: 1024
  timeout: 30
  cloudfront_id: ${CLOUDFRONT_DISTRIBUTION_ID}
```

## 3. Plan and Ship

```bash
nextdeploy plan
nextdeploy ship
```

During `ship`, NextDeploy:

1. Builds `.next/standalone` and classifies every route (`static`, `ssg`, `isr`, `ssr`, `api`).
2. Uploads content-hashed `/_next/static/*` and `public/*` assets to Amazon S3 with immutable cache headers.
3. Packages the standalone server + Lambda bridge into a zip and updates your Lambda function + alias.
4. Provisions or updates the CloudFront distribution and ACM certificate.
5. Writes exact DNS validation and routing records to `.nextdeploy/dns.md` and an HTML deployment report to `.nextdeploy/reports/`.

## 4. Verify, Monitor, or Destroy

```bash
nextdeploy status
nextdeploy logs
nextdeploy destroy   # tears down provisioned AWS resources
```
