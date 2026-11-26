package daemon

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/aynaash/nextdeploy/daemon/internal/types"
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

func signTestCommand(t *testing.T, secret, cmdType string, args map[string]interface{}, nonce string) types.Command {
	t.Helper()
	cmd := types.Command{
		Type:      cmdType,
		Args:      args,
		Timestamp: time.Now().Unix(),
		Nonce:     nonce,
	}
	payload, err := json.Marshal(map[string]interface{}{
		"type":      cmd.Type,
		"args":      cmd.Args,
		"timestamp": cmd.Timestamp,
		"nonce":     cmd.Nonce,
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	h := hmac.New(sha256.New, []byte(secret))
	h.Write(payload)
	cmd.Signature = hex.EncodeToString(h.Sum(nil))
	return cmd
}

func readAuditEntries(t *testing.T, path string) ([]byte, []AuditEntry) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read audit log %s: %v", path, err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	entries := make([]AuditEntry, 0, len(lines))
	for _, line := range lines {
		if line == "" {
			continue
		}
		var entry AuditEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Fatalf("unmarshal audit entry %q: %v", line, err)
		}
		entries = append(entries, entry)
	}
	return data, entries
}

func TestSanitizeAuditArgs(t *testing.T) {
	in := map[string]interface{}{
		"action":       "set",
		"appName":      "myapp",
		"key":          "DATABASE_URL",
		"tarball":      "/opt/nextdeploy/uploads/app.tar.gz",
		"value":        "postgres://secret",
		"dopplerToken": "dp.st.secret",
	}

	got := sanitizeAuditArgs(in)
	want := map[string]interface{}{
		"action":  "set",
		"appName": "myapp",
		"key":     "DATABASE_URL",
		"tarball": "/opt/nextdeploy/uploads/app.tar.gz",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("sanitizeAuditArgs() = %#v, want %#v", got, want)
	}
	if in["value"] != "postgres://secret" || in["dopplerToken"] != "dp.st.secret" {
		t.Errorf("sanitizeAuditArgs mutated input map in place: %#v", in)
	}
	if sanitizeAuditArgs(nil) != nil {
		t.Error("sanitizeAuditArgs(nil) should return nil")
	}
}

func TestHandleCommandAuditRedactsSecrets(t *testing.T) {
	withTempSecretsDir(t)
	ch := newTestHandler(t, nil)

	const secretVal = "postgres://user:super-secret-value@host/db"

	// 1. secrets set: raw value must not be logged in args or message.
	setCmd := signTestCommand(t, "test-secret", "secrets", map[string]interface{}{
		"action":  "set",
		"appName": "audit-test-app",
		"key":     "DATABASE_URL",
		"value":   secretVal,
	}, "nonce-set")
	setResp := ch.HandleCommand(setCmd, localSocketIdentity, true)
	if !setResp.Success {
		t.Fatalf("secrets set failed: %+v", setResp)
	}

	raw, entries := readAuditEntries(t, ch.auditLogger.path)
	if strings.Contains(string(raw), secretVal) {
		t.Fatalf("audit log leaked secret value on secrets set: %s", string(raw))
	}
	last := entries[len(entries)-1]
	argsMap, ok := last.Args.(map[string]interface{})
	if !ok {
		t.Fatalf("expected Args map in audit entry, got %T", last.Args)
	}
	if argsMap["key"] != "DATABASE_URL" || argsMap["action"] != "set" {
		t.Errorf("unexpected audit Args: %#v", argsMap)
	}
	if _, hasValue := argsMap["value"]; hasValue {
		t.Errorf("audit Args still contained 'value': %#v", argsMap)
	}

	// 2. secrets get: CLI response must return the secret, audit log must redact it.
	getCmd := signTestCommand(t, "test-secret", "secrets", map[string]interface{}{
		"action":  "get",
		"appName": "audit-test-app",
		"key":     "DATABASE_URL",
	}, "nonce-get")
	getResp := ch.HandleCommand(getCmd, localSocketIdentity, true)
	if !getResp.Success || getResp.Message != secretVal {
		t.Fatalf("secrets get = %+v, want success with secret value", getResp)
	}

	raw, entries = readAuditEntries(t, ch.auditLogger.path)
	if strings.Contains(string(raw), secretVal) {
		t.Fatalf("audit log leaked secret value on secrets get: %s", string(raw))
	}
	last = entries[len(entries)-1]
	if last.ErrorDetails != "secret value retrieved" {
		t.Errorf("secrets get ErrorDetails = %q, want %q", last.ErrorDetails, "secret value retrieved")
	}

	// 3. secrets list: CLI gets key list, audit log gets fixed summary.
	listCmd := signTestCommand(t, "test-secret", "secrets", map[string]interface{}{
		"action":  "list",
		"appName": "audit-test-app",
	}, "nonce-list")
	listResp := ch.HandleCommand(listCmd, localSocketIdentity, true)
	if !listResp.Success || listResp.Message != "DATABASE_URL" {
		t.Fatalf("secrets list = %+v, want success with DATABASE_URL", listResp)
	}

	_, entries = readAuditEntries(t, ch.auditLogger.path)
	last = entries[len(entries)-1]
	if last.ErrorDetails != "secret keys listed" {
		t.Errorf("secrets list ErrorDetails = %q, want %q", last.ErrorDetails, "secret keys listed")
	}

	// 4. secrets get for missing key: failure details are preserved.
	missingCmd := signTestCommand(t, "test-secret", "secrets", map[string]interface{}{
		"action":  "get",
		"appName": "audit-test-app",
		"key":     "DOES_NOT_EXIST",
	}, "nonce-missing")
	missingResp := ch.HandleCommand(missingCmd, localSocketIdentity, true)
	if missingResp.Success {
		t.Fatalf("expected missing secret lookup to fail, got %+v", missingResp)
	}

	_, entries = readAuditEntries(t, ch.auditLogger.path)
	last = entries[len(entries)-1]
	if last.Result != "false" || !strings.Contains(last.ErrorDetails, "not found") {
		t.Errorf("missing secret audit entry = %+v, want Result=false and ErrorDetails containing 'not found'", last)
	}
}
