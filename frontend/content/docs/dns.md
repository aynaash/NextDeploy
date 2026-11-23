---
title: DNS Configuration
lede: "Comprehensive DNS setup for VPS (A + CNAME), AWS CloudFront + ACM validation, and Cloudflare Workers automatic custom domains — plus apex domain flattening and automated .nextdeploy/dns.md guides."
status: ready
sourceRef: cli/internal/dns/guide.go, cli/internal/serverless/cloudflare.go, cli/internal/serverless/aws.go
related:
  - title: Cloudflare DNS
    href: /docs/cloudflare/dns
  - title: Caddy & TLS
    href: /docs/vps/caddy
  - title: Amazon CloudFront
    href: /docs/aws/cloudfront
---

## Automated DNS Guides (`.nextdeploy/dns.md`)

Whenever you deploy a custom domain to VPS bare metal or AWS Serverless, NextDeploy automatically calculates the exact DNS records needed and writes them into `.nextdeploy/dns.md`.

Because `.nextdeploy/` is gitignored:
- Deploys never dirty your Git working tree or create merge conflicts.
- You or your team can inspect copy-pasteable DNS records instantly without searching through provider dashboards.
- AI coding agents can read `.nextdeploy/dns.md` via the `nextdeploy_dns_guide` MCP tool and provide immediate guidance.

---

## Target-by-Target DNS Setup

### 1. Cloudflare Workers (Fully Automated)

When your application targets Cloudflare Workers and `serverless.cloudflare.zone_name` matches a domain in your Cloudflare account, **DNS configuration is 100% automated**:

```yaml
serverless:
  cloudflare:
    custom_domains:
      - hostname: app.example.com
        zone_name: example.com
      - hostname: example.com
        zone_name: example.com
```

- NextDeploy invokes the Cloudflare Custom Domains API.
- Cloudflare creates internal `AAAA 100::` routing records automatically.
- Cloudflare issues and manages dedicated edge SSL/TLS certificates with auto-renewal.
- Zero manual DNS records required.

---

### 2. Linux VPS (Caddy Auto-TLS)

For bare-metal Linux VPS deployments, Caddy requires your domain to resolve to your server's public IPv4/IPv6 addresses so Let's Encrypt can solve the HTTP-01 or TLS-ALPN-01 ACME challenges.

Add the following records at your DNS registrar (Namecheap, GoDaddy, Porkbun, Cloudflare DNS, Route53):

| Record Type | Host / Name | Target / Value | TTL | Note |
| :--- | :--- | :--- | :--- | :--- |
| `A` | `@` (or apex) | `203.0.113.10` (Your VPS IPv4) | Auto / 300s | Directs root traffic to VPS |
| `AAAA` *(Optional)* | `@` (or apex) | `2001:db8::1` (Your VPS IPv6) | Auto / 300s | If IPv6 is enabled on server |
| `CNAME` | `www` | `example.com` | Auto / 300s | Subdomain alias |

**Automatic TLS Provisioning:** Once records propagate, Caddy obtains an SSL certificate automatically on the very first HTTPS connection. No `certbot` or cron renewal scripts needed.

---

### 3. AWS Serverless (CloudFront + ACM)

Deploying to AWS Serverless uses Amazon CloudFront for edge caching and AWS Certificate Manager (ACM) for free SSL/TLS certificates.

#### Step 1: ACM SSL Certificate Validation
NextDeploy requests an ACM certificate in `us-east-1` (required for CloudFront). AWS generates an ACM validation CNAME:

| Record Type | Host / Name | Target / Value | Cloudflare Proxy Status |
| :--- | :--- | :--- | :--- |
| `CNAME` | `_a7b8c9d0...example.com` | `_x1y2z3.acm-validations.aws.` | **DNS Only (Gray Cloud)** |

> [!WARNING]
> If your domain uses Cloudflare DNS in front of AWS, you **MUST** set the ACM validation CNAME to **DNS Only (Gray Cloud)**. If proxied (Orange Cloud), Cloudflare intercepts DNS resolution and ACM certificate validation will hang indefinitely.

#### Step 2: Routing Traffic to CloudFront
Once the ACM certificate is verified (typically 60-120 seconds), point your domain to the CloudFront distribution:

| Record Type | Host / Name | Target / Value |
| :--- | :--- | :--- |
| `ALIAS` or `ANAME` | `@` | `d12345678abcdef.cloudfront.net` |
| `CNAME` | `www` | `d12345678abcdef.cloudfront.net` |

---

## Apex Domain CNAME Flattening Mechanics

Under RFC 1034 (Section 3.6.2), a `CNAME` record cannot coexist with other record types (such as `SOA` or `NS`) on the zone apex (`example.com`).

Here is how to route apex domains across different DNS providers:

| DNS Provider | Apex Strategy | Instructions |
| :--- | :--- | :--- |
| **Cloudflare DNS** | CNAME Flattening | Create a `CNAME` on `@` pointing to your target. Cloudflare automatically resolves and flattens it to `A` records at runtime. |
| **AWS Route 53** | ALIAS Record | Create an `A` record on `@`, check "Alias", and select the CloudFront distribution. |
| **Namecheap** | ALIAS Record | Choose record type `ALIAS Record` on `@` pointing to the target domain. |
| **DNSimple / DNS Made Easy** | ANAME Record | Choose `ANAME` record on `@` to dynamically track the target IP. |
| **Traditional DNS (No Flattening)** | Direct `A` Record | Use an `A` record pointing to the direct IP (VPS only; not recommended for AWS CloudFront). |

---

## Verifying DNS Propagation

Use `dig` or `nslookup` to verify propagation from multiple DNS resolvers:

```bash
# Check VPS A record
dig example.com A +short

# Check AWS CloudFront CNAME / ALIAS resolution
dig www.example.com CNAME +short

# Check Google Public DNS (8.8.8.8)
dig @8.8.8.8 example.com A +short

# Check Cloudflare DNS (1.1.1.1)
dig @1.1.1.1 example.com A +short

# Inspect full DNS trace
dig +trace example.com
```

Once records resolve correctly, run:
```bash
nextdeploy status
```
NextDeploy verifies end-to-end HTTPS handshake and reports healthy status.
