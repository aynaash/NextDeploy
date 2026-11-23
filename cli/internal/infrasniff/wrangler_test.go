package infrasniff

import (
	"strings"
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

func TestReadWrangler_FullEnterpriseMonorepoAndEnv(t *testing.T) {
	rootDir := t.TempDir()
	// Place wrangler.json at root
	write(t, rootDir, "wrangler.json", `{
  "name": "masomolab-web-dev",
  "compatibility_date": "2026-05-01",
  "compatibility_flags": ["nodejs_compat"],
  "durable_objects": {
    "bindings": [{ "name": "COURSE_AUTHORING", "class_name": "CourseAuthoringDO" }]
  },
  "ai": { "binding": "AI" },
  "vectorize": [{ "binding": "VECTORIZE", "index_name": "masomolab-rag-dev" }],
  "kv_namespaces": [{ "binding": "RATE_LIMITS_KV", "id": "kv-id-dev" }],
  "r2_buckets": [{ "binding": "MEDIA_BUCKET", "bucket_name": "masomolab-media-dev" }],
  "queues": {
    "producers": [{ "binding": "INGEST_Q", "queue": "masomolab-processing-dev" }],
    "consumers": [
      {
        "queue": "masomolab-processing-dev",
        "max_batch_size": 10,
        "max_batch_timeout": 30,
        "max_retries": 3,
        "dead_letter_queue": "masomolab-processing-dev-dlq"
      }
    ]
  },
  "migrations": [
    {
      "tag": "v1",
      "new_sqlite_classes": ["CourseAuthoringDO"]
    }
  ],
  "env": {
    "production": {
      "routes": [
        { "pattern": "masomolab.com", "custom_domain": true },
        { "pattern": "www.masomolab.com", "custom_domain": true }
      ],
      "triggers": {
        "crons": ["0 2 * * *"]
      }
    }
  }
}`)

	// Simulate running inside monorepo child app: apps/web
	appDir := rootDir + "/apps/web"
	write(t, appDir, "app/page.tsx", `export default function Home() { return <div>Masomo</div>; }`)

	res, err := Sniff(appDir)
	if err != nil {
		t.Fatalf("Sniff failed: %v", err)
	}

	w := res.Wrangler
	if w == nil {
		t.Fatal("wrangler config not found via parent directory traversal")
	}

	if w.Name != "masomolab-web-dev" {
		t.Errorf("worker name: got %q", w.Name)
	}
	if w.CompatibilityDate != "2026-05-01" {
		t.Errorf("compat date: got %q", w.CompatibilityDate)
	}

	// Durable Objects
	if len(w.DurableObjects) != 1 || w.DurableObjects[0].Name != "COURSE_AUTHORING" || w.DurableObjects[0].ClassName != "CourseAuthoringDO" {
		t.Errorf("DurableObjects wrong: %+v", w.DurableObjects)
	}

	// Migrations
	if len(w.Migrations) != 1 || len(w.Migrations[0].NewSQLiteClasses) != 1 || w.Migrations[0].NewSQLiteClasses[0] != "CourseAuthoringDO" {
		t.Errorf("Migrations wrong: %+v", w.Migrations)
	}

	// Queue consumers with DLQ
	if len(w.QueueConsumers) != 1 || w.QueueConsumers[0].DeadLetterQueue != "masomolab-processing-dev-dlq" {
		t.Errorf("Queue consumers wrong: %+v", w.QueueConsumers)
	}

	// Merged from env.production: custom domain and crons
	if len(w.CustomDomains) == 0 || w.CustomDomains[0] != "masomolab.com" {
		t.Errorf("CustomDomains merged wrong: %+v", w.CustomDomains)
	}
	if len(w.Crons) != 1 || w.Crons[0] != "0 2 * * *" {
		t.Errorf("Crons merged wrong: %+v", w.Crons)
	}

	// Test generated YAML output
	y := res.RenderNextDeployYAML("masomolab-web")
	assertValidNextDeployYAML(t, y)

	for _, frag := range []string{
		`domain: "masomolab.com"`,
		`compatibility_date: "2026-05-01"`,
		`compatibility_flags: [nodejs_compat]`,
		`- { name: COURSE_AUTHORING, class_name: CourseAuthoringDO }`,
		`- { name: INGEST_Q, queue: masomolab-processing-dev }`,
		`queue: masomolab-processing-dev`,
		`dead_letter_queue: masomolab-processing-dev-dlq`,
		`tag: v1`,
		`new_sqlite_classes: [CourseAuthoringDO]`,
		`crons:`,
		`- "0 2 * * *"`,
	} {
		if !strings.Contains(y, frag) {
			t.Errorf("generated YAML missing %q\n%s", frag, y)
		}
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
