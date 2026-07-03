package serverless

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/aynaash/nextdeploy/shared"
	"github.com/aynaash/nextdeploy/shared/config"

	"github.com/cloudflare/cloudflare-go/v6"
	"github.com/cloudflare/cloudflare-go/v6/option"
)

// --- pure helpers ------------------------------------------------------------

func TestSortedMigrationFiles_OrdersByBasenameAndFiltersNonSQL(t *testing.T) {
	dir := t.TempDir()
	// Intentionally created out of order; .txt must be ignored.
	for _, name := range []string{"0002_posts.sql", "0000_init.sql", "notes.txt", "0001_users.SQL"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("SELECT 1;"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	got, err := sortedMigrationFiles(dir)
	if err != nil {
		t.Fatalf("sortedMigrationFiles: %v", err)
	}
	var bases []string
	for _, p := range got {
		bases = append(bases, filepath.Base(p))
	}
	want := []string{"0000_init.sql", "0001_users.SQL", "0002_posts.sql"}
	if strings.Join(bases, ",") != strings.Join(want, ",") {
		t.Errorf("got %v, want %v", bases, want)
	}
}

func TestSortedMigrationFiles_MissingDirErrors(t *testing.T) {
	_, err := sortedMigrationFiles(filepath.Join(t.TempDir(), "does-not-exist"))
	if err == nil {
		t.Fatal("expected error for missing dir")
	}
	if !strings.Contains(err.Error(), "read migrations dir") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSortedMigrationFiles_EmptyDirReturnsNil(t *testing.T) {
	got, err := sortedMigrationFiles(t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected no files, got %v", got)
	}
}

func TestPendingMigrations(t *testing.T) {
	files := []string{"/m/0000_init.sql", "/m/0001_users.sql", "/m/0002_posts.sql"}
	applied := map[string]bool{"0000_init.sql": true, "0002_posts.sql": true}
	got := pendingMigrations(files, applied)
	if len(got) != 1 || filepath.Base(got[0]) != "0001_users.sql" {
		t.Errorf("got %v, want [.../0001_users.sql]", got)
	}
}

func TestPendingMigrations_AllAppliedReturnsEmpty(t *testing.T) {
	files := []string{"/m/0000_init.sql"}
	got := pendingMigrations(files, map[string]bool{"0000_init.sql": true})
	if len(got) != 0 {
		t.Errorf("expected none pending, got %v", got)
	}
}

func TestParseAppliedMigrations(t *testing.T) {
	rows := []any{
		map[string]any{"name": "0000_init.sql"},
		map[string]any{"name": "0001_users.sql"},
		map[string]any{"name": ""},   // skipped: empty
		map[string]any{"other": "x"}, // skipped: no name
		"not-a-row",                  // skipped: wrong type
	}
	got := parseAppliedMigrations(rows)
	if len(got) != 2 || !got["0000_init.sql"] || !got["0001_users.sql"] {
		t.Errorf("got %v, want {0000_init.sql, 0001_users.sql}", got)
	}
}

// --- binding emission (no network) -------------------------------------------

func TestBuildScriptMetadata_D1BindingExplicitID(t *testing.T) {
	cf := &config.CloudflareConfig{
		Bindings: &config.CFBindings{
			D1: []config.CFD1Binding{{Name: "DB", ID: "d1-uuid-123"}},
		},
	}
	meta, err := buildScriptMetadata(cf, "bucket", "worker.mjs", noResolver, nil)
	if err != nil {
		t.Fatalf("buildScriptMetadata: %v", err)
	}
	raw, _ := json.Marshal(meta)
	got := string(raw)
	for _, frag := range []string{`"name":"DB"`, `"id":"d1-uuid-123"`, `"type":"d1"`} {
		if !strings.Contains(got, frag) {
			t.Errorf("D1 binding JSON missing %q\n%s", frag, got)
		}
	}
}

func TestBuildScriptMetadata_D1RefResolvedToID(t *testing.T) {
	cf := &config.CloudflareConfig{
		Bindings: &config.CFBindings{
			D1: []config.CFD1Binding{{Name: "DB", Ref: "app-db"}},
		},
	}
	resolver := func(kind, name string) (string, bool) {
		if kind == "d1" && name == "app-db" {
			return "uuid-from-iac", true
		}
		return "", false
	}
	meta, err := buildScriptMetadata(cf, "bucket", "worker.mjs", resolver, nil)
	if err != nil {
		t.Fatalf("buildScriptMetadata: %v", err)
	}
	raw, _ := json.Marshal(meta)
	if !strings.Contains(string(raw), `"id":"uuid-from-iac"`) {
		t.Errorf("D1 ref→id resolution missing:\n%s", raw)
	}
}

func TestBuildScriptMetadata_D1RefUnresolvedErrors(t *testing.T) {
	cf := &config.CloudflareConfig{
		Bindings: &config.CFBindings{
			D1: []config.CFD1Binding{{Name: "DB", Ref: "app-db"}},
		},
	}
	_, err := buildScriptMetadata(cf, "bucket", "worker.mjs", noResolver, nil)
	if err == nil || !strings.Contains(err.Error(), `ref "app-db" not found`) {
		t.Fatalf("expected unresolved-ref error, got %v", err)
	}
}

func TestBuildScriptMetadata_D1MissingIDAndRefErrors(t *testing.T) {
	cf := &config.CloudflareConfig{
		Bindings: &config.CFBindings{
			D1: []config.CFD1Binding{{Name: "DB"}},
		},
	}
	_, err := buildScriptMetadata(cf, "bucket", "worker.mjs", noResolver, nil)
	if err == nil || !strings.Contains(err.Error(), "id or ref required") {
		t.Fatalf("expected id-or-ref error, got %v", err)
	}
}

// --- SDK-backed (httptest) ---------------------------------------------------

// d1Mock records every query SQL the provider sends and serves the minimal CF
// envelopes the D1 endpoints need. appliedNames seeds the SELECT response so we
// can assert migration idempotency.
type d1Mock struct {
	mu           sync.Mutex
	existingDBs  []map[string]string // {uuid,name} returned by list page 1
	appliedNames []string            // rows returned for SELECT name FROM tracking table
	createdNames []string            // names POSTed to the create endpoint
	querySQLs    []string            // every SQL executed against /query
}

func (m *d1Mock) handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		path := r.URL.Path
		switch {
		// list: GET /accounts/{acc}/d1/database
		case r.Method == http.MethodGet && strings.HasSuffix(path, "/d1/database"):
			page := r.URL.Query().Get("page")
			result := []map[string]string{}
			if page == "" || page == "1" {
				result = m.existingDBs
			}
			writeEnvelopeArray(w, result)
		// create: POST /accounts/{acc}/d1/database
		case r.Method == http.MethodPost && strings.HasSuffix(path, "/d1/database"):
			var body struct {
				Name string `json:"name"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			m.mu.Lock()
			m.createdNames = append(m.createdNames, body.Name)
			m.mu.Unlock()
			writeEnvelopeObject(w, map[string]string{"uuid": "new-db-uuid", "name": body.Name})
		// query: POST /accounts/{acc}/d1/database/{id}/query
		case r.Method == http.MethodPost && strings.HasSuffix(path, "/query"):
			var body struct {
				Sql string `json:"sql"`
			}
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &body)
			m.mu.Lock()
			m.querySQLs = append(m.querySQLs, body.Sql)
			m.mu.Unlock()

			var rows []map[string]any
			if strings.Contains(body.Sql, "SELECT name FROM") {
				for _, n := range m.appliedNames {
					rows = append(rows, map[string]any{"name": n})
				}
			}
			writeQueryEnvelope(w, rows)
		default:
			http.Error(w, "unexpected request: "+r.Method+" "+path, http.StatusNotFound)
		}
	}
}

func writeEnvelopeArray(w http.ResponseWriter, result any) {
	_ = json.NewEncoder(w).Encode(map[string]any{
		"success": true, "errors": []any{}, "messages": []any{},
		"result":      result,
		"result_info": map[string]int{"page": 1, "per_page": 100, "count": 1, "total_count": 1},
	})
}

func writeEnvelopeObject(w http.ResponseWriter, result any) {
	_ = json.NewEncoder(w).Encode(map[string]any{
		"success": true, "errors": []any{}, "messages": []any{}, "result": result,
	})
}

func writeQueryEnvelope(w http.ResponseWriter, rows []map[string]any) {
	if rows == nil {
		rows = []map[string]any{}
	}
	_ = json.NewEncoder(w).Encode(map[string]any{
		"success": true, "errors": []any{}, "messages": []any{},
		"result": []map[string]any{
			{"success": true, "meta": map[string]any{}, "results": rows},
		},
	})
}

func newD1TestProvider(t *testing.T, m *d1Mock) *CloudflareProvider {
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

func TestFindD1ID_ExactNameMatch(t *testing.T) {
	m := &d1Mock{existingDBs: []map[string]string{
		{"uuid": "uuid-other", "name": "other-db"},
		{"uuid": "uuid-mine", "name": "app-db"},
	}}
	p := newD1TestProvider(t, m)
	id, err := p.findD1ID(context.Background(), "app-db")
	if err != nil {
		t.Fatalf("findD1ID: %v", err)
	}
	if id != "uuid-mine" {
		t.Errorf("got %q, want uuid-mine", id)
	}
}

func TestFindD1ID_AbsentReturnsEmpty(t *testing.T) {
	p := newD1TestProvider(t, &d1Mock{existingDBs: nil})
	id, err := p.findD1ID(context.Background(), "nope")
	if err != nil {
		t.Fatalf("findD1ID: %v", err)
	}
	if id != "" {
		t.Errorf("got %q, want empty", id)
	}
}

func TestEnsureD1Database_CreatesWhenMissing(t *testing.T) {
	m := &d1Mock{existingDBs: nil}
	p := newD1TestProvider(t, m)
	id, err := p.ensureD1Database(context.Background(), config.CFD1Resource{Name: "app-db"})
	if err != nil {
		t.Fatalf("ensureD1Database: %v", err)
	}
	if id != "new-db-uuid" {
		t.Errorf("got id %q, want new-db-uuid", id)
	}
	if len(m.createdNames) != 1 || m.createdNames[0] != "app-db" {
		t.Errorf("expected create call for app-db, got %v", m.createdNames)
	}
}

func TestEnsureD1Database_ReusesExisting(t *testing.T) {
	m := &d1Mock{existingDBs: []map[string]string{{"uuid": "existing-uuid", "name": "app-db"}}}
	p := newD1TestProvider(t, m)
	id, err := p.ensureD1Database(context.Background(), config.CFD1Resource{Name: "app-db"})
	if err != nil {
		t.Fatalf("ensureD1Database: %v", err)
	}
	if id != "existing-uuid" {
		t.Errorf("got %q, want existing-uuid", id)
	}
	if len(m.createdNames) != 0 {
		t.Errorf("should not create when DB exists, got %v", m.createdNames)
	}
}

func TestApplyD1Migrations_AppliesOnlyPending(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"0000_init.sql", "0001_users.sql"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("CREATE TABLE t(id);"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	// 0000 already applied → only 0001 should run + get recorded.
	m := &d1Mock{appliedNames: []string{"0000_init.sql"}}
	p := newD1TestProvider(t, m)

	err := p.applyD1Migrations(context.Background(), "db-uuid", config.CFD1Resource{
		Name: "app-db", MigrationsDir: dir,
	})
	if err != nil {
		t.Fatalf("applyD1Migrations: %v", err)
	}

	joined := strings.Join(m.querySQLs, "\n")
	if !strings.Contains(joined, "CREATE TABLE IF NOT EXISTS "+migrationsTable) {
		t.Errorf("tracking table not ensured:\n%s", joined)
	}
	// Recording is now folded into the migration's own batch (apply + record in
	// one query) so a crash can't leave it applied-but-unrecorded. Exactly one
	// tracking INSERT — for the single pending migration, not the already-applied one.
	inserts := strings.Count(joined, "INSERT OR IGNORE INTO "+migrationsTable)
	if inserts != 1 {
		t.Errorf("expected exactly 1 migration recorded, got %d\n%s", inserts, joined)
	}
	// Apply + record must ride in the SAME query (atomicity is the whole point).
	var recordingQuery string
	for _, q := range m.querySQLs {
		if strings.Contains(q, "INSERT OR IGNORE INTO "+migrationsTable) {
			recordingQuery = q
		}
	}
	if !strings.Contains(recordingQuery, "CREATE TABLE t(id);") {
		t.Errorf("record INSERT is not batched with the migration DDL:\n%s", recordingQuery)
	}
}

func TestApplyD1Migrations_NoFilesIsNoop(t *testing.T) {
	m := &d1Mock{}
	p := newD1TestProvider(t, m)
	err := p.applyD1Migrations(context.Background(), "db-uuid", config.CFD1Resource{
		Name: "app-db", MigrationsDir: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("applyD1Migrations: %v", err)
	}
	if len(m.querySQLs) != 0 {
		t.Errorf("no SQL should run for empty migrations dir, got %v", m.querySQLs)
	}
}

func TestPlanD1_CreateWhenMissing(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "0000_init.sql"), []byte("SELECT 1;"), 0o600)
	p := newD1TestProvider(t, &d1Mock{existingDBs: nil})
	item, err := p.planD1(context.Background(), config.CFD1Resource{Name: "app-db", MigrationsDir: dir})
	if err != nil {
		t.Fatalf("planD1: %v", err)
	}
	if item.Action != PlanCreate || item.Kind != "d1" {
		t.Errorf("got %+v, want create/d1", item)
	}
	if !strings.Contains(item.Detail, "1 migration file") {
		t.Errorf("detail missing migration count: %q", item.Detail)
	}
}

func TestPlanD1_NoOpWhenExists(t *testing.T) {
	p := newD1TestProvider(t, &d1Mock{existingDBs: []map[string]string{{"uuid": "u", "name": "app-db"}}})
	item, err := p.planD1(context.Background(), config.CFD1Resource{Name: "app-db"})
	if err != nil {
		t.Fatalf("planD1: %v", err)
	}
	if item.Action != PlanNoOp {
		t.Errorf("got %v, want no-op", item.Action)
	}
}

func TestPlanD1_NameRequired(t *testing.T) {
	p := newD1TestProvider(t, &d1Mock{})
	_, err := p.planD1(context.Background(), config.CFD1Resource{})
	if err == nil || !strings.Contains(err.Error(), "name is required") {
		t.Fatalf("expected name-required error, got %v", err)
	}
}
