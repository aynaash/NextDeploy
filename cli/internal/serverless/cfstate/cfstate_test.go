package cfstate

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/aynaash/nextdeploy/shared/secrets"
)

func testKey(t *testing.T) []byte {
	t.Helper()
	k, err := secrets.DeriveKey("test-passphrase-for-cfstate")
	if err != nil {
		t.Fatal(err)
	}
	return k
}

func TestSetGetList(t *testing.T) {
	m := New()
	m.Set("d1", "app-db", "uuid-1", "2026-06-14T00:00:00Z")
	m.Set("kv", "rate-limit", "ns-1", "2026-06-14T00:00:00Z")

	if r, ok := m.Get("d1", "app-db"); !ok || r.ID != "uuid-1" {
		t.Errorf("Get d1/app-db wrong: %+v ok=%v", r, ok)
	}
	if _, ok := m.Get("d1", "missing"); ok {
		t.Error("Get of missing resource should be !ok")
	}
	list := m.List()
	if len(list) != 2 || list[0].Kind != "d1" || list[1].Kind != "kv" {
		t.Errorf("List not sorted/complete: %+v", list)
	}
}

func TestSet_Replaces(t *testing.T) {
	m := New()
	m.Set("d1", "app-db", "old", "t1")
	m.Set("d1", "app-db", "new", "t2")
	if r, _ := m.Get("d1", "app-db"); r.ID != "new" {
		t.Errorf("Set should replace, got %q", r.ID)
	}
	if len(m.List()) != 1 {
		t.Error("replacing must not add a second record")
	}
}

func TestOrphans(t *testing.T) {
	m := New()
	m.Set("d1", "app-db", "1", "t")
	m.Set("kv", "old-cache", "2", "t")
	m.Set("queue", "jobs", "3", "t")

	// Only d1/app-db and queue/jobs still declared → kv/old-cache is orphaned.
	declared := map[string]bool{
		Key("d1", "app-db"):  true,
		Key("queue", "jobs"): true,
	}
	orphans := m.Orphans(declared)
	if len(orphans) != 1 || orphans[0].Kind != "kv" || orphans[0].Name != "old-cache" {
		t.Errorf("expected kv/old-cache orphan, got %+v", orphans)
	}
}

func TestSaveLoad_RoundTrip(t *testing.T) {
	key := testKey(t)
	path := filepath.Join(t.TempDir(), "cf-state.json.enc")

	m := New()
	m.Set("d1", "app-db", "uuid-1", "2026-06-14T00:00:00Z")
	if err := Save(path, m, key); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// File must be encrypted, not plaintext JSON.
	raw, _ := os.ReadFile(path)
	if len(raw) == 0 {
		t.Fatal("nothing written")
	}
	if string(raw[:1]) == "{" {
		t.Error("manifest appears to be plaintext JSON — must be encrypted")
	}

	loaded, err := Load(path, key)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if r, ok := loaded.Get("d1", "app-db"); !ok || r.ID != "uuid-1" {
		t.Errorf("round-trip lost data: %+v ok=%v", r, ok)
	}
}

func TestLoad_MissingFileIsFreshManifest(t *testing.T) {
	m, err := Load(filepath.Join(t.TempDir(), "nope.enc"), testKey(t))
	if err != nil {
		t.Fatalf("missing file should not error: %v", err)
	}
	if m == nil || len(m.Resources) != 0 {
		t.Errorf("expected empty manifest, got %+v", m)
	}
}

func TestLoad_WrongKeyFails(t *testing.T) {
	path := filepath.Join(t.TempDir(), "s.enc")
	m := New()
	m.Set("d1", "x", "1", "t")
	if err := Save(path, m, testKey(t)); err != nil {
		t.Fatal(err)
	}
	wrong, _ := secrets.DeriveKey("a-different-passphrase")
	if _, err := Load(path, wrong); err == nil {
		t.Error("decrypt with wrong key must fail (not silently return empty)")
	}
}
