package serverless

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/cloudflare/cloudflare-go/v6"
	"github.com/cloudflare/cloudflare-go/v6/option"

	"github.com/aynaash/nextdeploy/shared"
)

// teardownMock records the (method, path) of every request and replies with the
// configured status + a Cloudflare-shaped envelope.
type teardownMock struct {
	mu       sync.Mutex
	requests []string
	status   int // 0 → 200
}

func (m *teardownMock) handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		m.mu.Lock()
		m.requests = append(m.requests, r.Method+" "+r.URL.Path)
		m.mu.Unlock()

		st := m.status
		if st == 0 {
			st = http.StatusOK
		}
		w.Header().Set("content-type", "application/json")
		w.WriteHeader(st)
		if st >= 400 {
			_, _ = io.WriteString(w, `{"success":false,"errors":[{"code":10000,"message":"nope"}],"messages":[],"result":null}`)
			return
		}
		_, _ = io.WriteString(w, `{"success":true,"errors":[],"messages":[],"result":{}}`)
	})
}

func (m *teardownMock) hit(substr string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, req := range m.requests {
		if strings.Contains(req, substr) {
			return true
		}
	}
	return false
}

func newTeardownTestProvider(t *testing.T, m *teardownMock) *CloudflareProvider {
	t.Helper()
	srv := httptest.NewServer(m.handler())
	t.Cleanup(srv.Close)
	client := cloudflare.NewClient(
		option.WithBaseURL(srv.URL+"/"),
		option.WithAPIToken("test-token"),
		option.WithMaxRetries(0),
	)
	return &CloudflareProvider{
		log:       shared.PackageLogger("test", ""),
		cf:        client,
		accountID: "acc123",
	}
}

func TestDeleteProvisionedResource_UnknownKind(t *testing.T) {
	p := &CloudflareProvider{log: shared.PackageLogger("test", "")}
	if err := p.deleteProvisionedResource(context.Background(), "mystery", "x"); err == nil {
		t.Fatal("expected an error for an unknown resource kind")
	}
}

func TestDeleteProvisionedResource_EachKind(t *testing.T) {
	cases := map[string]struct{ kind, id, pathPart string }{
		"d1":         {"d1", "db-uuid", "/d1/database/db-uuid"},
		"kv":         {"kv", "ns-id", "/storage/kv/namespaces/ns-id"},
		"hyperdrive": {"hyperdrive", "hd-id", "/hyperdrive/configs/hd-id"},
		"vectorize":  {"vectorize", "idx-name", "/vectorize/v2/indexes/idx-name"},
		"queue":      {"queue", "q-id", "/queues/q-id"},
		"ai_gateway": {"ai_gateway", "gw-slug", "gateways/gw-slug"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			m := &teardownMock{}
			p := newTeardownTestProvider(t, m)
			if err := p.deleteProvisionedResource(context.Background(), tc.kind, tc.id); err != nil {
				t.Fatalf("delete %s: %v", tc.kind, err)
			}
			if !m.hit("DELETE") || !m.hit(tc.pathPart) {
				t.Errorf("%s: expected a DELETE hitting %q; requests=%v", tc.kind, tc.pathPart, m.requests)
			}
		})
	}
}

func TestIsCFNotFound(t *testing.T) {
	// A 404 from the API → isCFNotFound true (treated as already-gone).
	m404 := &teardownMock{status: http.StatusNotFound}
	p404 := newTeardownTestProvider(t, m404)
	err := p404.deleteProvisionedResource(context.Background(), "d1", "gone")
	if err == nil || !isCFNotFound(err) {
		t.Fatalf("expected a 404 recognized by isCFNotFound, got %v", err)
	}

	// A 500 → not a not-found (must surface as a real failure).
	m500 := &teardownMock{status: http.StatusInternalServerError}
	p500 := newTeardownTestProvider(t, m500)
	err = p500.deleteProvisionedResource(context.Background(), "d1", "boom")
	if err == nil || isCFNotFound(err) {
		t.Fatalf("expected a non-404 error, got %v", err)
	}
}
