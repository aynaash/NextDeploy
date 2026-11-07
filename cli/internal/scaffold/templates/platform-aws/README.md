# __APP_NAME__

A **production-ready** fullstack Next.js application configured for **AWS Serverless (Lambda + CloudFront + S3)**, powered by [NextDeploy](https://github.com/aynaash/NextDeploy).

> Scaffolding conventions and deployment architecture by [aynaash (Hersi)](https://github.com/aynaash).

---

## What's Included

- **Next.js 15 + React 19** with App Router and Standalone output.
- **Serverless Architecture**: AWS Lambda (ARM64) compute, CloudFront CDN, and S3 static asset distribution.
- **Modern Responsive UI**: Built-in CSS variables, glassmorphic dark theme, interactive health check.
- **API Health Check**: [`app/api/health/route.ts`](app/api/health/route.ts) for runtime observability.
- **Automated CI/CD**: [`.github/workflows/nextdeploy.yml`](.github/workflows/nextdeploy.yml) deploys on push to `main`.

---

## Local Development

```bash
# 1. Install dependencies
npm install

# 2. Run local development server
npm run dev
```

Open [http://localhost:3000](http://localhost:3000) in your browser.

---

## Deployment with NextDeploy

```bash
# 1. Configure AWS credentials
# (AWS_ACCESS_KEY_ID & AWS_SECRET_ACCESS_KEY or Doppler / AWS CLI profile)

# 2. Preview deployment plan
nextdeploy plan

# 3. Ship your application to AWS
nextdeploy ship
```
