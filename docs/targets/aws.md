# AWS Serverless Target

Deploy your Next.js application to AWS using AWS Lambda for compute, Amazon S3 for static assets, and Amazon CloudFront for global CDN distribution.

---

## AWS Architecture

When targeting AWS, NextDeploy provisions and manages:

```
                    Internet
                       │
                       ▼
            ┌─────────────────────┐
            │  Amazon CloudFront  │  (Global CDN + TLS via ACM)
            └──────────┬──────────┘
                       │
         ┌─────────────┴─────────────┐
         │                           │
         ▼                           ▼
 ┌───────────────┐           ┌───────────────┐
 │   Amazon S3   │           │  AWS Lambda   │
 │ Static Assets │           │ Server Engine │
 └───────────────┘           └───────────────┘
```

- **Amazon CloudFront**: Routes `/_next/static/*` and public files directly to S3, while routing SSR, dynamic routes, and API calls to Lambda.
- **AWS Lambda**: Executes the standalone Next.js server with Node.js 20.x runtime, configured memory sizes, and execution timeouts.
- **Amazon S3**: Stores content-hashed static assets and uploads with automatic cache headers (`Cache-Control: public, max-age=31536000, immutable`).
- **AWS Certificate Manager (ACM)**: Automatically requests and validates SSL/TLS certificates for your custom domain.
- **AWS IAM**: Generates least-privilege execution roles for Lambda and CloudFront origin access.

---

## Configuration

Set `target_type: serverless` and `provider: aws` in `nextdeploy.yml`:

```yaml
version: "1.0"
target_type: serverless

app:
  name: cloud-store
  domain: store.example.com
  environment: production

serverless:
  provider: aws
  aws:
    region: us-east-1
    memory_size: 1024         # Lambda memory allocation (MB)
    timeout: 30               # Lambda execution timeout (seconds)
    cloudfront:
      price_class: PriceClass_100 # PriceClass_100 | PriceClass_200 | PriceClass_All
```

---

## Prerequisites & AWS Credentials

Ensure your AWS CLI or deployment environment has valid credentials:

```bash
export AWS_ACCESS_KEY_ID="AKIA..."
export AWS_SECRET_ACCESS_KEY="..."
export AWS_REGION="us-east-1"
```

Verify your credentials with:

```bash
nextdeploy creds
```

---

## Deploying to AWS

Deploy with:

```bash
nextdeploy ship
```

NextDeploy will:
1. Compile your Next.js application in standalone mode.
2. Package the compute layer into a minimal Lambda zip artifact.
3. Sync static files to the designated S3 bucket.
4. Deploy or update the Lambda function and publish a new version alias.
5. Invalidate CloudFront cache paths where necessary.
6. Verify CloudFront health and confirm the live cutover.

---

## Tearing Down AWS Resources

To decommission provisioned AWS infrastructure:

```bash
nextdeploy destroy
```
