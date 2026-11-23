#!/usr/bin/env bash
# ==============================================================================
# NextDeploy Multi-Server Enterprise Stress & Chaos Test Runner
# ==============================================================================
# Emulates a 3-node VPS production cluster + NGINX Load Balancer locally on Linux
# with zero cloud costs, testing high-concurrency SSR load and zero-downtime rolling deploys.
# ==============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
STRESS_DIR="$ROOT_DIR/test/stress"

echo "======================================================================"
echo "  🚀 NextDeploy Enterprise Multi-Server Stress Test Harness"
echo "======================================================================"

# 1. Optimize Linux kernel file descriptor limits if permissible
if [ "$(ulimit -n)" -lt 65535 ]; then
    ulimit -n 65535 2>/dev/null || echo "ℹ️  Note: ulimit -n is $(ulimit -n). Run as sudo or edit /etc/security/limits.conf for max throughput."
fi

# 2. Check prerequisites
for cmd in docker go; do
    if ! command -v "$cmd" >/dev/null 2>&1; then
        echo "❌ Error: $cmd is required but not installed." >&2
        exit 1
    fi
done

# Check docker compose support
DOCKER_COMPOSE="docker compose"
if ! docker compose version >/dev/null 2>&1; then
    if command -v docker-compose >/dev/null 2>&1; then
        DOCKER_COMPOSE="docker-compose"
    else
        echo "❌ Error: docker compose is required." >&2
        exit 1
    fi
fi

# 3. Ensure test SSH keypair exists for Docker cluster
if [ ! -f "$STRESS_DIR/test_ed25519" ]; then
    echo "==> 🔑 Generating test cluster SSH keypair..."
    ssh-keygen -t ed25519 -N "" -f "$STRESS_DIR/test_ed25519" -C "nextdeploy-stress-test"
    chmod 600 "$STRESS_DIR/test_ed25519"
    chmod 644 "$STRESS_DIR/test_ed25519.pub"
fi

# 4. Compile fresh nextdeploy and nextdeployd binaries
echo "==> 📦 Compiling nextdeploy CLI and nextdeployd daemon..."
mkdir -p "$STRESS_DIR/bin"
go build -o "$STRESS_DIR/bin/nextdeploy" "$ROOT_DIR/cli"
go build -o "$STRESS_DIR/bin/nextdeployd" "$ROOT_DIR/daemon/cmd/nextdeployd"
echo "    Binaries compiled successfully."

# 4. Guarantee cleanup on exit
cleanup() {
    echo ""
    echo "==> 🧹 Tearing down test cluster containers..."
    $DOCKER_COMPOSE -f "$STRESS_DIR/docker-compose.cluster.yml" down -v --remove-orphans >/dev/null 2>&1 || true
    echo "    Cleanup complete."
}
trap cleanup EXIT INT TERM

# 5. Boot the 3 VPS nodes + NGINX load balancer
echo "==> 🐳 Spinning up 3-node VPS cluster + NGINX load balancer..."
$DOCKER_COMPOSE -f "$STRESS_DIR/docker-compose.cluster.yml" up -d --build

# 6. Wait for SSH readiness on all 3 nodes
echo "==> ⏳ Waiting for SSH daemon on web-01, web-02, web-03..."
for port in 2201 2202 2203; do
    retries=30
    until nc -z 127.0.0.1 "$port" 2>/dev/null || [ $retries -eq 0 ]; do
        sleep 1
        retries=$((retries - 1))
    done
    if [ $retries -eq 0 ]; then
        echo "❌ Timeout waiting for port $port"
        exit 1
    fi
done

# Wait for NGINX load balancer port
retries=20
until nc -z 127.0.0.1 8080 2>/dev/null || [ $retries -eq 0 ]; do
    sleep 1
    retries=$((retries - 1))
done

echo "    All nodes and load balancer are online!"

# 7. Execute Go E2E Stress Test Suite
echo ""
echo "======================================================================"
echo "  🔥 Running High-Concurrency Stress Benchmark"
echo "======================================================================"
cd "$ROOT_DIR"
go test -v -timeout=30m "$ROOT_DIR/test/stress/..."

echo ""
echo "======================================================================"
echo "  ✅ All Multi-Server Stress Benchmarks Passed Successfully!"
echo "======================================================================"
