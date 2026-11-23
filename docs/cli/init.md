# `nextdeploy init`

Scaffold a production-ready `nextdeploy.yml` configuration file inside your Next.js repository.

---

## Usage

```bash
nextdeploy init [flags]
```

### Flags

| Flag | Type | Description |
| :--- | :--- | :--- |
| `--target` | string | Deploy target to scaffold: `vps`, `aws`, or `cloudflare` (skips the interactive prompt). |
| `--platform` | string | Alias for `--target`. |
| `--mode` | string | Cloudflare only: `scaffold` or `sniff` (skips the interactive prompt). |
| `--db` | string | Cloudflare scaffold only: `d1`, `byo`, or `none` (skips the interactive prompt). |

---

## Interactive Prompts

When run without flags, `nextdeploy init` inspects your existing Next.js project (`package.json`, `next.config.js`, TypeScript vs JavaScript, environment files) and prompts:

1. **Deployment Target**:
   - `VPS`: Bare-metal Linux server (Ubuntu/Debian) managed by Caddy and `nextdeployd`.
   - `Serverless (Cloudflare)`: Global edge deployment via Workers, R2, and D1/KV.
   - `Serverless (AWS)`: Lambda, S3, and CloudFront.
2. **App Identifier & Domain**: Sets the slug name and routing domain.
3. **Environment & Secrets**: Asks for relevant credentials or configures `.env` reading.

---

## Deep Explanation

To inspect the initialization steps and checks:

```bash
nextdeploy init explain
```
