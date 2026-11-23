package stress

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

// BenchmarkMetrics records latency and status codes during high-concurrency runs.
type BenchmarkMetrics struct {
	TotalRequests uint64
	Success200    uint64
	Errors5xx     uint64
	OtherCodes    uint64
	Latencies     []time.Duration
	NodeHits      map[string]uint64
	mu            sync.Mutex
}

func (m *BenchmarkMetrics) Record(statusCode int, duration time.Duration, hostname string) {
	atomic.AddUint64(&m.TotalRequests, 1)
	if statusCode == http.StatusOK {
		atomic.AddUint64(&m.Success200, 1)
	} else if statusCode >= 500 {
		atomic.AddUint64(&m.Errors5xx, 1)
	} else {
		atomic.AddUint64(&m.OtherCodes, 1)
	}

	m.mu.Lock()
	m.Latencies = append(m.Latencies, duration)
	if hostname != "" {
		m.NodeHits[hostname]++
	}
	m.mu.Unlock()
}

func (m *BenchmarkMetrics) Summary() string {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.Latencies) == 0 {
		return "No requests recorded"
	}

	sort.Slice(m.Latencies, func(i, j int) bool {
		return m.Latencies[i] < m.Latencies[j]
	})

	var sum time.Duration
	for _, l := range m.Latencies {
		sum += l
	}
	avg := sum / time.Duration(len(m.Latencies))
	p50 := m.Latencies[len(m.Latencies)*50/100]
	p95 := m.Latencies[len(m.Latencies)*95/100]
	p99 := m.Latencies[len(m.Latencies)*99/100]
	max := m.Latencies[len(m.Latencies)-1]

	return fmt.Sprintf(
		"Total: %d | 200 OK: %d (%.2f%%) | 5xx Errors: %d | Avg: %v | p50: %v | p95: %v | p99: %v | Max: %v",
		m.TotalRequests,
		m.Success200,
		float64(m.Success200)/float64(m.TotalRequests)*100,
		m.Errors5xx,
		avg,
		p50,
		p95,
		p99,
		max,
	)
}

// TestClusterConnectivity verifies that all 3 container nodes and the NGINX LB are operational.
func TestClusterConnectivity(t *testing.T) {
	keyBytes, err := os.ReadFile("test_ed25519")
	if err != nil {
		t.Skipf("Skipping live cluster test (test_ed25519 not found; run with Docker cluster active): %v", err)
	}

	// Quick check if cluster port is open before running subtests
	conn, err := net.DialTimeout("tcp", "127.0.0.1:2201", 1*time.Second)
	if err != nil {
		t.Skipf("Cluster nodes not running on 127.0.0.1:2201 (%v). Run ./scripts/run-linux-stress-test.sh to start the cluster.", err)
	}
	conn.Close()

	signer, err := ssh.ParsePrivateKey(keyBytes)
	if err != nil {
		t.Fatalf("Failed to parse private key: %v", err)
	}

	sshConfig := &ssh.ClientConfig{
		User:            "root",
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         3 * time.Second,
	}

	nodes := []struct {
		name string
		port int
	}{
		{"web-01", 2201},
		{"web-02", 2202},
		{"web-03", 2203},
	}

	for _, node := range nodes {
		t.Run(node.name, func(t *testing.T) {
			addr := fmt.Sprintf("127.0.0.1:%d", node.port)
			client, err := ssh.Dial("tcp", addr, sshConfig)
			if err != nil {
				t.Fatalf("SSH connection to %s (%s) failed: %v", node.name, addr, err)
			}
			defer client.Close()

			session, err := client.NewSession()
			if err != nil {
				t.Fatalf("Failed to create SSH session on %s: %v", node.name, err)
			}
			defer session.Close()

			output, err := session.CombinedOutput("/usr/local/bin/nextdeployd version")
			if err != nil {
				t.Fatalf("Failed to execute nextdeployd version on %s: %v\nOutput: %s", node.name, err, output)
			}
			t.Logf("Node %s active: %s", node.name, string(output))
		})
	}

	// Verify NGINX Load Balancer health
	t.Run("load-balancer", func(t *testing.T) {
		resp, err := http.Get("http://127.0.0.1:8080/lb-healthz")
		if err != nil {
			t.Fatalf("Load balancer http://127.0.0.1:8080/lb-healthz unreachable: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("Load balancer health check failed with status: %d", resp.StatusCode)
		}
		t.Logf("Load balancer is healthy (HTTP 200)")
	})
}

// TestZeroDowntimeHighConcurrency blasts requests at the load balancer while verifying 0% 5xx errors.
func TestZeroDowntimeHighConcurrency(t *testing.T) {
	resp, err := http.Get("http://127.0.0.1:8080/lb-healthz")
	if err != nil {
		t.Skip("Load balancer not running, skipping stress load test")
	}
	resp.Body.Close()

	metrics := &BenchmarkMetrics{
		NodeHits: make(map[string]uint64),
	}

	client := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        500,
			MaxIdleConnsPerHost: 200,
			IdleConnTimeout:     60 * time.Second,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	concurrency := 40
	var wg sync.WaitGroup

	t.Logf("Firing sustained concurrency load (%d workers) at http://127.0.0.1:8080...", concurrency)

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				default:
					start := time.Now()
					r, reqErr := client.Get("http://127.0.0.1:8080/api/heavy-ssr")
					elapsed := time.Since(start)

					if reqErr != nil {
						metrics.Record(500, elapsed, "error")
						continue
					}

					body, _ := io.ReadAll(r.Body)
					r.Body.Close()

					var payload struct {
						Hostname string `json:"hostname"`
						Release  string `json:"release"`
					}
					_ = json.Unmarshal(body, &payload)

					metrics.Record(r.StatusCode, elapsed, payload.Hostname)
				}
			}
		}()
	}

	wg.Wait()

	t.Logf("=== STRESS BENCHMARK RESULTS ===")
	t.Logf("%s", metrics.Summary())
	t.Logf("Traffic Distribution Across Cluster Nodes: %+v", metrics.NodeHits)

	if metrics.Errors5xx > 0 {
		t.Errorf("FAIL: Encountered %d 5xx errors during high-concurrency run!", metrics.Errors5xx)
	}
}

// Suppress unused compiler warning for crypto/ed25519
var _ = ed25519.PublicKey(nil)
