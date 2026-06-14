package infrasniff

import (
	"testing"
)

func TestStripJSONC_RemovesCommentsAndTrailingCommas(t *testing.T) {
	in := []byte(`{
  // line comment
  "name": "app", /* block */
  "url": "https://x/y", // keep the // inside the string above
  "list": [1, 2, 3,],
}`)
	out := stripJSONC(in)
	for _, frag := range []string{`"name": "app"`, `"url": "https://x/y"`, `[1, 2, 3]`} {
		if !containsBytes(out, frag) {
			t.Errorf("stripped JSONC missing %q\n%s", frag, out)
		}
	}
	if containsBytes(out, "line comment") || containsBytes(out, "block") {
		t.Errorf("comments not stripped:\n%s", out)
	}
}

func TestReadWrangler_JSONCBindings(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "wrangler.jsonc", `{
  // worker config
  "name": "my-worker",
  "d1_databases": [
    { "binding": "DB", "database_name": "app-db", "database_id": "uuid-1" },
  ],
  "kv_namespaces": [{ "binding": "CACHE", "id": "kv-1" }],
  "r2_buckets": [{ "binding": "ASSETS", "bucket_name": "app-assets" }],
  "ai": { "binding": "AI" },
  "vars": { "STAGE": "prod" },
}`)
	res, err := Sniff(dir)
	if err != nil {
		t.Fatal(err)
	}
	w := res.Wrangler
	if w == nil {
		t.Fatal("wrangler config not parsed")
	}
	if w.Name != "my-worker" {
		t.Errorf("name: got %q", w.Name)
	}
	if len(w.D1) != 1 || w.D1[0].Name != "DB" || w.D1[0].Resource != "app-db" || w.D1[0].ID != "uuid-1" {
		t.Errorf("D1 binding wrong: %+v", w.D1)
	}
	if len(w.R2) != 1 || w.R2[0].Resource != "app-assets" {
		t.Errorf("R2 binding wrong: %+v", w.R2)
	}
	if w.AI != "AI" {
		t.Errorf("AI binding: got %q", w.AI)
	}
	// authoritative signals must surface on the result
	if !hasResource(res, ResD1) || !hasResource(res, ResKV) || !hasResource(res, ResR2) || !hasResource(res, ResAI) {
		t.Errorf("wrangler signals missing: %v", res.Resources())
	}
}

func TestReadWrangler_TOMLBindings(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "wrangler.toml", `name = "toml-worker"

[[d1_databases]]
binding = "DB"
database_name = "t-db"
database_id = "t-uuid"

[[hyperdrive]]
binding = "PG"
id = "hd-1"

[[queues.producers]]
binding = "JOBS"
queue = "jobs-prod"
`)
	res, err := Sniff(dir)
	if err != nil {
		t.Fatal(err)
	}
	w := res.Wrangler
	if w == nil {
		t.Fatal("toml wrangler not parsed")
	}
	if w.Name != "toml-worker" {
		t.Errorf("name: got %q", w.Name)
	}
	if len(w.D1) != 1 || w.D1[0].ID != "t-uuid" {
		t.Errorf("D1 wrong: %+v", w.D1)
	}
	if len(w.Hyperdrive) != 1 || w.Hyperdrive[0].Name != "PG" {
		t.Errorf("hyperdrive wrong: %+v", w.Hyperdrive)
	}
	if len(w.Queues) != 1 || w.Queues[0].Resource != "jobs-prod" {
		t.Errorf("queue producer wrong: %+v", w.Queues)
	}
	if !hasResource(res, ResHyperdrive) || !hasResource(res, ResQueue) {
		t.Errorf("toml signals missing: %v", res.Resources())
	}
}

func TestReadWrangler_PrefersJsoncOverToml(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "wrangler.jsonc", `{ "name": "from-jsonc" }`)
	write(t, dir, "wrangler.toml", `name = "from-toml"`)
	res, err := Sniff(dir)
	if err != nil {
		t.Fatal(err)
	}
	if res.Wrangler == nil || res.Wrangler.Name != "from-jsonc" {
		t.Errorf("expected jsonc to win, got %+v", res.Wrangler)
	}
}

func TestReadWrangler_AbsentIsNil(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "app/page.tsx", `export default function P(){return null}`)
	res, err := Sniff(dir)
	if err != nil {
		t.Fatal(err)
	}
	if res.Wrangler != nil {
		t.Errorf("expected nil wrangler, got %+v", res.Wrangler)
	}
}

func containsBytes(b []byte, sub string) bool {
	return len(sub) == 0 || (len(b) >= len(sub) && indexOf(string(b), sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
