package daemon

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"testing"
	"time"
)

func TestReleasesToPrune(t *testing.T) {
	tests := []struct {
		name  string
		names []string
		keep  int
		want  []string
	}{
		{
			name:  "fewer than keep prunes nothing",
			names: []string{"100-a", "200-b"},
			keep:  5,
			want:  nil,
		},
		{
			name:  "equal to keep prunes nothing",
			names: []string{"100-a", "200-b"},
			keep:  2,
			want:  nil,
		},
		{
			name:  "keeps newest by release-id order regardless of input order",
			names: []string{"300-c", "100-a", "500-e", "200-b", "400-d"},
			keep:  2,
			want:  []string{"100-a", "200-b", "300-c"},
		},
		{
			name:  "non-conforming name sorts deterministically, not by ReadDir luck",
			names: []string{"legacy", "300-c", "100-a", "200-b"},
			keep:  2,
			// sorted: ["100-a","200-b","300-c","legacy"] -> drop all but last 2.
			want: []string{"100-a", "200-b"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := releasesToPrune(tt.names, tt.keep); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("releasesToPrune(%v, %d) = %v, want %v", tt.names, tt.keep, got, tt.want)
			}
		})
	}
}

func TestAppLocker(t *testing.T) {
	l := newAppLocker()

	release, ok := l.tryAcquire("app1")
	if !ok {
		t.Fatal("first acquire of app1 should succeed")
	}

	if _, ok := l.tryAcquire("app1"); ok {
		t.Error("second concurrent acquire of app1 should fail fast")
	}

	// A different app is independent.
	if r2, ok := l.tryAcquire("app2"); !ok {
		t.Error("acquire of app2 should succeed while app1 is held")
	} else {
		r2()
	}

	// Releasing app1 lets it be re-acquired.
	release()
	r3, ok := l.tryAcquire("app1")
	if !ok {
		t.Error("app1 should be re-acquirable after release")
	} else {
		r3()
	}
}

// portOf extracts the TCP port an httptest server is listening on.
func portOf(t *testing.T, ts *httptest.Server) int {
	t.Helper()
	_, portStr, err := net.SplitHostPort(ts.Listener.Addr().String())
	if err != nil {
		t.Fatalf("split host port: %v", err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("atoi port: %v", err)
	}
	return port
}

func TestWaitForHealthy(t *testing.T) {
	t.Run("200 on / is healthy", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer ts.Close()
		if err := waitForHealthy(portOf(t, ts), "/", 2*time.Second); err != nil {
			t.Errorf("expected healthy, got %v", err)
		}
	})

	t.Run("custom health path is probed", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/healthz" {
				w.WriteHeader(http.StatusOK)
				return
			}
			w.WriteHeader(http.StatusNotFound)
		}))
		defer ts.Close()
		if err := waitForHealthy(portOf(t, ts), "/healthz", 2*time.Second); err != nil {
			t.Errorf("expected healthy on /healthz, got %v", err)
		}
	})

	t.Run("port open but 500 fails activation", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer ts.Close()
		// Short timeout: a TCP-only gate would pass here; the HTTP gate must not.
		if err := waitForHealthy(portOf(t, ts), "/", 600*time.Millisecond); err == nil {
			t.Error("expected health check to fail on persistent 500")
		}
	})

	t.Run("nothing listening times out", func(t *testing.T) {
		// Grab a port then release it, so the dial reliably fails.
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("listen: %v", err)
		}
		_, portStr, _ := net.SplitHostPort(ln.Addr().String())
		port, _ := strconv.Atoi(portStr)
		_ = ln.Close()
		if err := waitForHealthy(port, "/", 400*time.Millisecond); err == nil {
			t.Error("expected timeout when nothing is listening")
		}
	})

	t.Run("empty path defaults to root", func(t *testing.T) {
		var gotPath string
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			w.WriteHeader(http.StatusOK)
		}))
		defer ts.Close()
		if err := waitForHealthy(portOf(t, ts), "", 2*time.Second); err != nil {
			t.Fatalf("expected healthy, got %v", err)
		}
		if gotPath != "/" {
			t.Errorf("empty health path should probe %q, probed %q", "/", gotPath)
		}
	})
}

func TestProcessAlive(t *testing.T) {
	tests := []struct {
		name   string
		pidStr string
		want   bool
	}{
		{"non-numeric", "abc", false},
		{"zero", "0", false},
		{"negative", "-1", false},
		{"empty", "", false},
		{"unlikely-huge-pid", fmt.Sprintf("%d", 1<<30), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := processAlive(tt.pidStr); got != tt.want {
				t.Errorf("processAlive(%q) = %v, want %v", tt.pidStr, got, tt.want)
			}
		})
	}
}
