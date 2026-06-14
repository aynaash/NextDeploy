package serverless

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/aynaash/nextdeploy/shared"
	"github.com/aynaash/nextdeploy/shared/config"

	"github.com/cloudflare/cloudflare-go/v6"
	"github.com/cloudflare/cloudflare-go/v6/option"
)

// kvMock serves the KV namespace list/create endpoints and records creations.
type kvMock struct {
	mu       sync.Mutex
	existing []map[string]string // {id,title} returned by list page 1
	created  []string            // titles POSTed to the create endpoint
}

func (m *kvMock) handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/storage/kv/namespaces"):
			page := r.URL.Query().Get("page")
			result := []map[string]string{}
			if page == "" || page == "1" {
				result = m.existing
			}
			writeEnvelopeArray(w, result)
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/storage/kv/namespaces"):
			var body struct {
				Title string `json:"title"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			m.mu.Lock()
			m.created = append(m.created, body.Title)
			m.mu.Unlock()
			writeEnvelopeObject(w, map[string]string{"id": "new-ns-id", "title": body.Title})
		default:
			http.Error(w, "unexpected: "+r.Method+" "+r.URL.Path, http.StatusNotFound)
		}
	}
}

func newKVTestProvider(t *testing.T, m *kvMock) *CloudflareProvider {
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

func TestEnsureKVNamespace_CreatesWhenMissing(t *testing.T) {
	m := &kvMock{existing: nil}
	p := newKVTestProvider(t, m)
	id, err := p.ensureKVNamespace(context.Background(), "app-rate-limit")
	if err != nil {
		t.Fatalf("ensureKVNamespace: %v", err)
	}
	if id != "new-ns-id" {
		t.Errorf("got id %q, want new-ns-id", id)
	}
	if len(m.created) != 1 || m.created[0] != "app-rate-limit" {
		t.Errorf("expected create for app-rate-limit, got %v", m.created)
	}
}

func TestEnsureKVNamespace_ReusesExisting(t *testing.T) {
	m := &kvMock{existing: []map[string]string{
		{"id": "other", "title": "something-else"},
		{"id": "mine", "title": "app-rate-limit"},
	}}
	p := newKVTestProvider(t, m)
	id, err := p.ensureKVNamespace(context.Background(), "app-rate-limit")
	if err != nil {
		t.Fatalf("ensureKVNamespace: %v", err)
	}
	if id != "mine" {
		t.Errorf("got %q, want mine", id)
	}
	if len(m.created) != 0 {
		t.Errorf("should not create when namespace exists, got %v", m.created)
	}
}

func TestFindKVNamespaceID_Absent(t *testing.T) {
	p := newKVTestProvider(t, &kvMock{existing: nil})
	id, err := p.findKVNamespaceID(context.Background(), "nope")
	if err != nil {
		t.Fatalf("findKVNamespaceID: %v", err)
	}
	if id != "" {
		t.Errorf("got %q, want empty", id)
	}
}

func TestPlanKV(t *testing.T) {
	create := newKVTestProvider(t, &kvMock{existing: nil})
	item, err := create.planKV(context.Background(), config.CFKVResource{Name: "rl"})
	if err != nil {
		t.Fatal(err)
	}
	if item.Action != PlanCreate || item.Kind != "kv" {
		t.Errorf("got %+v, want create/kv", item)
	}

	exists := newKVTestProvider(t, &kvMock{existing: []map[string]string{{"id": "x", "title": "rl"}}})
	item, err = exists.planKV(context.Background(), config.CFKVResource{Name: "rl"})
	if err != nil {
		t.Fatal(err)
	}
	if item.Action != PlanNoOp {
		t.Errorf("got %v, want no-op", item.Action)
	}

	if _, err := create.planKV(context.Background(), config.CFKVResource{}); err == nil {
		t.Error("expected name-required error")
	}
}

// --- binding ref resolution (no network) ------------------------------------

func TestBuildScriptMetadata_KVRefResolved(t *testing.T) {
	cf := &config.CloudflareConfig{
		Bindings: &config.CFBindings{KV: []config.CFKVBinding{{Name: "RATE_LIMIT", Ref: "app-rl"}}},
	}
	resolver := func(kind, name string) (string, bool) {
		if kind == "kv" && name == "app-rl" {
			return "ns-uuid-1", true
		}
		return "", false
	}
	meta, err := buildScriptMetadata(cf, "bucket", "worker.mjs", resolver, nil)
	if err != nil {
		t.Fatalf("buildScriptMetadata: %v", err)
	}
	raw, _ := json.Marshal(meta)
	if !strings.Contains(string(raw), `"namespace_id":"ns-uuid-1"`) {
		t.Errorf("KV ref→id resolution missing:\n%s", raw)
	}
}

func TestBuildScriptMetadata_KVExplicitIDStillWorks(t *testing.T) {
	cf := &config.CloudflareConfig{
		Bindings: &config.CFBindings{KV: []config.CFKVBinding{{Name: "CACHE", NamespaceID: "explicit-id"}}},
	}
	meta, err := buildScriptMetadata(cf, "bucket", "worker.mjs", noResolver, nil)
	if err != nil {
		t.Fatalf("buildScriptMetadata: %v", err)
	}
	if !strings.Contains(mustJSON(t, meta), `"namespace_id":"explicit-id"`) {
		t.Errorf("explicit KV id lost")
	}
}

func TestBuildScriptMetadata_KVMissingIDAndRefErrors(t *testing.T) {
	cf := &config.CloudflareConfig{
		Bindings: &config.CFBindings{KV: []config.CFKVBinding{{Name: "CACHE"}}},
	}
	_, err := buildScriptMetadata(cf, "bucket", "worker.mjs", noResolver, nil)
	if err == nil || !strings.Contains(err.Error(), "namespace_id or ref required") {
		t.Fatalf("expected namespace_id-or-ref error, got %v", err)
	}
}

func TestBuildScriptMetadata_KVUnresolvedRefErrors(t *testing.T) {
	cf := &config.CloudflareConfig{
		Bindings: &config.CFBindings{KV: []config.CFKVBinding{{Name: "CACHE", Ref: "missing"}}},
	}
	_, err := buildScriptMetadata(cf, "bucket", "worker.mjs", noResolver, nil)
	if err == nil || !strings.Contains(err.Error(), `ref "missing" not found`) {
		t.Fatalf("expected unresolved-ref error, got %v", err)
	}
}
