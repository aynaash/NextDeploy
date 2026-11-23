# Multi-Server Enterprise Stress & Chaos Testing

> Emulate, stress test, and chaos-engineer real-world multi-server VPS clusters locally on Linux with zero cloud costs.

NextDeploy includes an enterprise-grade multi-server emulation test harness (`test/stress/`) designed to push Next.js applications, the Go Sidecar, and NGINX load balancers to their absolute limits under sustained high concurrency.

---

## Why Test on a Real Linux Machine?

While macOS Docker runs inside an emulated virtualization layer (HyperKit / Virtualization.framework), a native Linux machine executes containers directly against the host Linux kernel:
* **Zero VM Overhead**: Reaches line-rate loopback throughput (40+ Gbps in memory).
* **True cgroups v2 Enforcement**: Emulates realistic VPS resource constraints (`cpus: 2.0`, `memory: 1024M`).
* **True Kernel `epoll` & Sockets**: Sustains 10,000 to 50,000+ requests/second without socket exhaustion.

---

## Architecture of the Test Cluster

```
                                [ Stress Traffic Generator ]
                              Go Load Engine / k6 (Host Machine)
                            1,000 – 50,000 req/sec Continuous Load
                                              │
                                              ▼
                             ┌──────────────────────────────────┐
                             │       NGINX Ingress (lb-01)      │
                             │   Port: 8080 (172.28.0.10)       │
                             │  • least_conn; keepalive 128;    │
                             │  • Failover & connection retry   │
                             └────────────────┬─────────────────┘
                                              │
                      ┌───────────────────────┼───────────────────────┐
                      │ (33% traffic)         │ (33% traffic)         │ (33% traffic)
                      ▼                       ▼                       ▼
          ┌──────────────────────┐┌──────────────────────┐┌──────────────────────┐
          │     Node: web-01     ││     Node: web-02     ││     Node: web-03     │
          │   172.28.0.11:3000   ││   172.28.0.12:3000   ││   172.28.0.13:3000   │
          │   SSH Port: 2201     ││   SSH Port: 2202     ││   SSH Port: 2203     │
          │ • systemd init       ││ • systemd init       ││ • systemd init       │
          │ • nextdeployd daemon ││ • nextdeployd daemon ││ • nextdeployd daemon │
          │ • Go Sidecar         ││ • Go Sidecar         ││ • Go Sidecar         │
          │ • Next.js Heavy SSR  ││ • Next.js Heavy SSR  ││ • Next.js Heavy SSR  │
          └──────────────────────┘└──────────────────────┘└──────────────────────┘
                      ▲
                      │ While Under Full Saturation:
                      │ `nextdeploy ship` performs zero-downtime rolling update
```

The harness runs 4 Docker containers connected over a dedicated bridge network (`172.28.0.0/16`):
1. **`nextdeploy-stress-lb-01` (`172.28.0.10`)**: NGINX configured with `least_conn` upstream, keepalive pooling, and automatic failover retries.
2. **`nextdeploy-stress-web-01` (`172.28.0.11`)**: Ubuntu 24.04 virtual VPS running `systemd`, `sshd` (port 2201), Node.js 22, Bun, and `nextdeployd`.
3. **`nextdeploy-stress-web-02` (`172.28.0.12`)**: Ubuntu 24.04 virtual VPS (port 2202).
4. **`nextdeploy-stress-web-03` (`172.28.0.13`)**: Ubuntu 24.04 virtual VPS (port 2203).

---

## Quickstart: Running on Linux

From the repository root on any Linux machine:

```bash
# 1. Run the all-in-one automated benchmark runner
./scripts/run-linux-stress-test.sh
```

The runner script:
1. Optimizes file descriptor limits (`ulimit -n 65535`).
2. Compiles `nextdeploy` CLI and `nextdeployd` daemon.
3. Boots the 3 VPS nodes and NGINX load balancer.
4. Waits for SSH readiness across all nodes.
5. Executes the Go high-concurrency benchmark and chaos tests.
6. Automatically cleans up all containers on exit.

---

## What the Benchmark Tests

### 1. High-Concurrency Heavy SSR
The benchmark app (`test/stress/app/app/api/heavy-ssr/route.ts`) executes server-side cryptographic hashing (5,000 SHA-256 iterations) on every request to heavily saturate the vCPUs.

The Go test engine fires a pool of concurrent workers against `http://localhost:8080/api/heavy-ssr` and computes:
* Total requests sent
* Status code distribution (200 OK vs 5xx errors)
* Latency percentiles: Average, $p50$, $p95$, $p99$, and Max
* Real-time node hit distribution (verifying NGINX balances across `web-01`, `web-02`, `web-03`)

### 2. Zero-Downtime Rolling Deployment Under 100% Load
While the load generator is saturating the cluster:
* `nextdeploy ship` deploys a new release.
* Nodes are drained and updated one by one (`batch_size: 1`).
* **Pass Assertion**: **0.00% 5xx errors** throughout the rollout.

### 3. Chaos Engineering: Node Crash Simulation
While traffic is in flight:
* Node 2 is killed (`docker stop nextdeploy-stress-web-02`).
* **Assertion**: NGINX automatically routes around the dead node within milliseconds. All client requests receive HTTP 200 via `web-01` and `web-03`.

---

## Fine-Tuning the Tests for Your Hardware

### Adjusting Concurrency & Duration
In [test/stress/stress_test.go](file:///Users/yussufhersi/Music/workspace/NextDeploy/test/stress/stress_test.go):

```go
// Increase concurrency for beefier multi-core machines:
concurrency := 100 // Default: 40

// Adjust sustained duration:
ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second) // Default: 20s
```

### Adjusting Container CPU / Memory Quotas
In [test/stress/docker-compose.cluster.yml](file:///Users/yussufhersi/Music/workspace/NextDeploy/test/stress/docker-compose.cluster.yml):

```yaml
deploy:
  resources:
    limits:
      cpus: "4.0"     # Give each virtual VPS 4 vCPUs
      memory: 2048M   # Give each virtual VPS 2GB RAM
```

### Tuning Linux Host Kernel Limits for Max Throughput
Before running large-scale benchmarks on your Linux host:

```bash
# Increase file descriptor limit
ulimit -n 65535

# Increase connection backlog and local port range
sudo sysctl -w net.core.somaxconn=65535
sudo sysctl -w net.ipv4.tcp_max_syn_backlog=65535
sudo sysctl -w net.ipv4.ip_local_port_range="1024 65535"
```
