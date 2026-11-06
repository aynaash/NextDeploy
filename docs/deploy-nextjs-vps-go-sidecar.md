# Deploy a Next.js App to a VPS with NextDeploy's Go Sidecar

NextDeploy deploys a Next.js production build to a VPS behind Caddy. When
enabled, its Go sidecar sits between Caddy and the Next.js Node.js server. It
can serve eligible prerendered HTML and React Server Component (RSC) responses
from the build output, and handle supported image-optimization requests. Pages
that need runtime rendering, middleware, or other unsupported behavior continue
to Node.js.

The sidecar is an optimization path, not a replacement for Next.js. The
application keeps its Node.js server, and unsupported requests should keep
working through the Node fallback.

## What you need

- A Next.js application that builds with `next build`.
- A Linux VPS with a public IP address and a sudo-capable SSH user.
- SSH key access from the computer that runs NextDeploy.
- A domain name whose DNS can point to the VPS.
- NextDeploy installed on your local machine.

If you still need to install the CLI, use the official installer:

```bash
curl -fsSL https://hersi.dev/nextdeploy/install.sh | bash
```

The Go sidecar is available for VPS deployments. It is not part of the AWS or
Cloudflare deployment targets.

## 1. Configure SSH access to the VPS

Install your public SSH key on the VPS using the VPS provider's console or
first-login process. Then verify that you can log in with the same user and key
you intend to put in `nextdeploy.yml`:

```bash
ssh -i ~/.ssh/id_ed25519 -o IdentitiesOnly=yes ubuntu@203.0.113.10
```

Replace the example username, address, and key path with your own. Do not copy
your private key to the server or put it in the application repository.

## 2. Create the NextDeploy configuration

From the root of your Next.js project:

```bash
nextdeploy init
```

Choose **VPS (Virtual Private Server - SSH)**. When asked whether to enable the
Go sidecar, choose yes. If your installed CLI does not show that prompt, add
the configuration manually. For example:

```yaml
target_type: vps

app:
  name: my-nextjs-app
  environment: production
  domain: app.example.com
  port: 3000
  gosidecar:
    enabled: true
    image_optimization: true
    isr_cache: true
    max_image_workers: 4

servers:
  - name: production
    host: 203.0.113.10
    username: ubuntu
    key_path: ~/.ssh/id_ed25519
```

Use your own app name, hostname, VPS address, SSH username, and private-key
path. `app.name` is a lowercase slug, not the domain. Keep the private key
outside the project; `key_path` points to the local key used by the CLI.

The sidecar is opt-in and disabled in a newly generated configuration unless
you enable it. `isr_cache` and `image_optimization` default to enabled when
omitted after the sidecar itself is enabled. `max_image_workers` defaults to
4; the daemon may reduce that value to fit available memory.

## 3. Prepare the VPS

Check that the configured SSH connection works, then prepare the server:

```bash
nextdeploy prepare
```

The default agent mode installs and configures the NextDeploy daemon, Node.js
runtime, Caddy, and the supporting services. It does not require Ansible on
your computer or Python on the target VPS. Use a sudo-capable non-root SSH
account where possible.

## 4. Build and deploy

Deploy from the project root:

```bash
nextdeploy ship
```

`ship` runs the production build, packages the app, uploads it over SSH, and
activates the new release. Review its output for the app, domain, target,
server, and whether it is doing a fresh build. If you have intentionally
changed source files and need to force a fresh build, use:

```bash
nextdeploy ship --force
```

Point the domain's DNS at the VPS address if it is not already configured.
NextDeploy writes a `dns.md` guide with the needed record information when a
domain is configured.

## 5. Confirm the release is healthy

Check the application status:

```bash
nextdeploy status
```

Then check the public response:

```bash
curl -sSI https://app.example.com/
```

The exact headers depend on the route and daemon version. For requests handled
by the Go sidecar, the current implementation can report:

- `X-NextDeploy-Sidecar: go` — the request passed through the sidecar. This
  header by itself does **not** prove Go generated the response.
- `X-NextDeploy-Cache: HIT` — Go served a cached prerendered response.
- `X-NextDeploy-Cache: STALE` — Go served stale prerendered content and
  triggered background revalidation.
- `X-NextDeploy-Cache: MISS-SSR` — the request was forwarded to Node.js.
- `X-NextDeploy-Handler: go-html`, `go-rsc`, or `go-image` — the specific Go
  handler that answered, on daemon builds that include handler-class headers.
- `X-NextDeploy-Handler: node` — Node.js answered through the fallback.

Older daemon releases may not emit the handler-class header. In that case,
use the cache header and test route behavior; do not treat the generic sidecar
header as proof of a Go cache hit.

Next.js static chunks and some public assets can be served directly by Caddy.
Those are useful static-file responses, but they are not Go sidecar hits.

## Which requests use Go, and which use Node?

The sidecar is deliberately conservative. Typical cases are:

| Request type | Expected path |
| --- | --- |
| Eligible prerendered HTML | Go sidecar |
| Available prerendered RSC response | Go sidecar |
| Eligible ISR HTML | Go sidecar; stale content may revalidate through Node |
| Supported image optimization request | Go image handler, otherwise Node fallback |
| Dynamic server rendering | Node.js |
| Middleware-matched request | Node.js |
| Server Action or request with a body | Node.js |
| Preview/bypass-cookie request | Node.js |
| Unsupported route or missing RSC artifact | Node.js |
| `/_next/static/*` and matching static assets | Often Caddy directly |

This behavior lets a deployment retain Next.js runtime compatibility while
offloading requests the sidecar can safely serve. Do not assume every page is
pre-rendered: inspect the production build output and exercise representative
routes from your own app.

## 6. Verify with representative routes

Test more than the homepage. Include:

1. A route known to be statically prerendered.
2. A route with an RSC/client-navigation request.
3. An ISR route, checked once fresh and again after its revalidation interval.
4. A dynamic route that should be rendered by Node.
5. A route with middleware.
6. API GET and POST requests, including any body-dependent behavior.
7. An image-optimization request using the formats your app requests.
8. A 404 route and any authentication or preview-cookie flows.

For a route that should be served by Go, confirm `X-NextDeploy-Cache: HIT` or
the appropriate `X-NextDeploy-Handler` value. For a route that needs runtime
execution, verify the response is correct through Node fallback. Check response
status and body as well as headers.

The sample app and benchmark routes used in our live experiment are in the
`nextdeploy-vps-template` project under `/sidecar-test/*`; they are test routes,
not routes automatically added to every application.

## 7. Measure the effect instead of assuming it

Compare the same build and request path through the sidecar and Node, using the
same method, headers, concurrency, and cache state. Measure first-request
(cold) and repeated (warm) behavior separately. Record:

- TTFB and full-response latency (p50 and p95).
- HTTP status, response size, and errors.
- Confirmed Go HTML/RSC/image hits versus Node fallbacks.
- Throughput at low, controlled concurrency.
- VPS CPU and memory before and after each run.
- Build/release version, daemon version, timestamp, network path, and sample
  count.

Do not compare public HTTPS latency directly with an SSH-tunneled loopback
measurement; DNS, TLS, Caddy, and the client network add different costs.
Avoid high-concurrency stress tests on a small production VPS.

### Results from the MiniStudio test deployment

On 2026-11-05, we tested the sample routes on an Ubuntu VPS with 1 vCPU and 1 GB
RAM. The app was deployed at `https://ministudio.site`. The saved CSVs contain
10 sequential requests per route for sidecar-direct, Node-direct, and public
Caddy paths. The table shows median / approximate p95 TTFB in milliseconds:

| Case | Sidecar direct TTFB | Node direct TTFB | Sidecar evidence |
| --- | ---: | ---: | --- |
| Static HTML | 843 / 898 ms | 872 / 929 ms | 10/10 Go prerender `HIT` |
| Static RSC | 834 / 865 ms | 835 / 895 ms | 10/10 Go prerender `HIT` |
| Generated product route | 816 / 890 ms | 849 / 886 ms | 10/10 Go prerender `HIT` |
| ISR | 864 / 962 ms | 837 / 860 ms | 9 `HIT`, 1 `STALE` |
| Dynamic route | 836 / 893 ms | 840 / 862 ms | Node fallback (`MISS-SSR`) |
| API GET | 842 / 856 ms | 781 / 824 ms | Node fallback (`MISS-SSR`) |
| API POST | 866 / 920 ms | 807 / 832 ms | Node fallback (`MISS-SSR`) |

Public Caddy median TTFB for the Go-hit examples was about 1.1 seconds:
1,120 ms for static HTML, 1,088 ms for static RSC, 1,126 ms for the generated
product route, and 1,121 ms for ISR. These public measurements include the
external network and Caddy and should not be directly compared with the
SSH-tunneled direct measurements.

All recorded test requests returned their expected status, with no request
errors in these CSVs. The data confirms the Go prerender path and Node fallback
worked for the tested requests. It does **not** prove a general speedup:
samples were small (the reported p95 is especially noisy with ten requests),
the ISR median was slower through the sidecar, and timing varies by route. The
middleware test was mistakenly served as a Go cache hit,
so middleware execution was not verified. The image response was unclassified,
so Go image optimization was not verified either.

The raw benchmark data and detailed methodology are saved in the
`nextdeploy-vps-template` project:

- `experiments/2026-11-05-sidecar-direct.csv`
- `experiments/2026-11-05-node-direct.csv`
- `experiments/2026-11-05-public-caddy.csv`
- `experiment.md`

Treat these as a real deployment smoke-test and initial timing sample, not a
capacity benchmark or a universal performance claim.

## Troubleshooting

- **SSH authentication fails:** verify the configured username and private key
  path, and confirm the corresponding public key is authorized on the VPS.
- **The app deploys but the domain does not load:** check DNS points to the VPS,
  then inspect `nextdeploy status` and `nextdeploy logs`.
- **A response has `MISS-SSR`:** the sidecar intercepted it but delegated it
  to Node. This is expected for dynamic or unsupported requests.
- **A route expected to be prerendered falls back:** confirm it is present in
  the production build output and that middleware, cookies, query parameters,
  or request headers do not require runtime handling.
- **Image requests fall back or fail:** confirm the Next.js image config and
  requested format are supported. Keep Node fallback enabled and verify both
  optimized output and source-image behavior.
- **The sidecar is not enabled:** confirm `target_type: vps` and
  `app.gosidecar.enabled: true`, then deploy a new release.

For app logs:

```bash
nextdeploy logs -f
```

## Rollback

If the new release has a problem, use NextDeploy's rollback command:

```bash
nextdeploy rollback
```

The Node.js server remains part of the deployment; turning off the sidecar in
configuration and shipping a new release returns the app to the Node-backed
request path.
