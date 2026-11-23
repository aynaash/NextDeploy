package infrasniff

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/aynaash/nextdeploy/shared/config"
)

func TestSuggestedCloudflareBlock_WranglerAuthoritative(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "wrangler.jsonc", `{
  "name": "w",
  "d1_databases": [{ "binding": "DB", "database_name": "app-db", "database_id": "uuid-1" }],
  "r2_buckets": [{ "binding": "ASSETS", "bucket_name": "app-assets" }],
  "ai": { "binding": "AI" }
}`)
	res, _ := Sniff(dir)
	block := res.SuggestedCloudflareBlock()

	for _, frag := range []string{
		"cloudflare:",
		"compatibility_date:",
		"bindings:",
		"- { name: DB, id: uuid-1 }",
		"- { name: ASSETS, bucket: app-assets }",
		"ai:",
		"- { name: AI }",
	} {
		if !strings.Contains(block, frag) {
			t.Errorf("block missing %q\n%s", frag, block)
		}
	}
	// Explicit database_id should not emit a redundant resources.d1 block or a
	// non-existent migrations_dir that breaks `nextdeploy plan`.
	if strings.Contains(block, "migrations_dir: drizzle") {
		t.Errorf("should not emit migrations_dir: drizzle when directory does not exist:\n%s", block)
	}
	if strings.Contains(block, "resources:") {
		t.Errorf("should not emit resources block when all wrangler bindings have explicit IDs:\n%s", block)
	}
}

func TestSuggestedCloudflareBlock_D1MigrationsOnlyWhenPresent(t *testing.T) {
	// Without migrations/ or drizzle/ on disk: omit migrations_dir.
	noDir := t.TempDir()
	write(t, noDir, "db.ts", `const x: D1Database = null as any;`)
	resNoDir, _ := Sniff(noDir)
	yNoDir := resNoDir.RenderNextDeployYAML("myapp")
	assertValidNextDeployYAML(t, yNoDir)
	if strings.Contains(yNoDir, "migrations_dir:") {
		t.Errorf("should omit migrations_dir when dir is absent:\n%s", yNoDir)
	}

	// With drizzle/ on disk: emit migrations_dir: drizzle.
	withDir := t.TempDir()
	write(t, withDir, "db.ts", `const x: D1Database = null as any;`)
	write(t, withDir, "drizzle/0001.sql", `SELECT 1;`)
	resWithDir, _ := Sniff(withDir)
	yWithDir := resWithDir.RenderNextDeployYAML("myapp")
	assertValidNextDeployYAML(t, yWithDir)
	if !strings.Contains(yWithDir, "migrations_dir: drizzle") {
		t.Errorf("expected migrations_dir: drizzle when dir exists:\n%s", yWithDir)
	}

	// Wrangler explicit migrations_dir on ID-less D1: preserve it.
	wranglerDir := t.TempDir()
	write(t, wranglerDir, "wrangler.jsonc", `{
  "d1_databases": [
    { "binding": "DB", "database_name": "main-db", "migrations_dir": "sql/migrations" },
    { "binding": "AUDIT", "database_name": "audit-db" }
  ]
}`)
	resW, _ := Sniff(wranglerDir)
	yW := resW.RenderNextDeployYAML("myapp")
	assertValidNextDeployYAML(t, yW)
	for _, frag := range []string{
		"- { name: DB, ref: main-db }",
		"- { name: AUDIT, ref: audit-db }",
		"- { name: main-db, migrations_dir: sql/migrations }",
		"- { name: audit-db }",
	} {
		if !strings.Contains(yW, frag) {
			t.Errorf("multi-D1 yaml missing %q:\n%s", frag, yW)
		}
	}
}

func TestSuggestedCloudflareBlock_PartialKVIDsAndMultiKV(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "wrangler.jsonc", `{
  "kv_namespaces": [
    { "binding": "EXISTING", "id": "kv-real" },
    { "binding": "SESSIONS" },
    { "binding": "RATE_LIMIT" }
  ]
}`)
	res, _ := Sniff(dir)
	y := res.RenderNextDeployYAML("myapp")
	assertValidNextDeployYAML(t, y)
	for _, frag := range []string{
		"- { name: EXISTING, namespace_id: kv-real }",
		"- { name: SESSIONS, ref: app-sessions }",
		"- { name: RATE_LIMIT, ref: app-rate-limit }",
		"- { name: app-sessions }",
		"- { name: app-rate-limit }",
	} {
		if !strings.Contains(y, frag) {
			t.Errorf("partial KV yaml missing %q:\n%s", frag, y)
		}
	}
}

func TestSuggestedCloudflareBlock_HyperdriveExplicitAndMissingID(t *testing.T) {
	// Explicit ID -> no resources.hyperdrive needed.
	explicitDir := t.TempDir()
	write(t, explicitDir, "wrangler.jsonc", `{ "hyperdrive": [{ "binding": "PG", "id": "hd-123" }] }`)
	resE, _ := Sniff(explicitDir)
	yE := resE.RenderNextDeployYAML("myapp")
	assertValidNextDeployYAML(t, yE)
	if !strings.Contains(yE, "- { name: PG, id: hd-123 }") {
		t.Errorf("missing explicit hyperdrive id:\n%s", yE)
	}
	if strings.Contains(yE, "origin_env: DATABASE_URL") {
		t.Errorf("should not emit resources.hyperdrive when wrangler provides explicit id:\n%s", yE)
	}

	// Missing ID -> ref + resources.hyperdrive (never REPLACE_WITH_HYPERDRIVE_ID).
	missingDir := t.TempDir()
	write(t, missingDir, "wrangler.jsonc", `{ "hyperdrive": [{ "binding": "PG" }] }`)
	resM, _ := Sniff(missingDir)
	yM := resM.RenderNextDeployYAML("myapp")
	assertValidNextDeployYAML(t, yM)
	if strings.Contains(yM, "REPLACE_WITH_HYPERDRIVE_ID") {
		t.Errorf("should use ref instead of placeholder:\n%s", yM)
	}
	if !strings.Contains(yM, "- { name: PG, ref: primary }") || !strings.Contains(yM, "- { name: primary, origin_env: DATABASE_URL }") {
		t.Errorf("missing hyperdrive ref/resource:\n%s", yM)
	}
}

func TestSuggestedCloudflareBlock_VectorizeAndQueues(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "wrangler.jsonc", `{
  "vectorize": [{ "binding": "MEM", "index_name": "memory-idx" }],
  "queues": { "producers": [{ "binding": "JOBS", "queue": "jobs-prod" }] }
}`)
	res, _ := Sniff(dir)
	y := res.RenderNextDeployYAML("myapp")
	assertValidNextDeployYAML(t, y)
	for _, frag := range []string{
		"vectorize:",
		"- { name: MEM, index: memory-idx }",
		"queues:",
		"- { name: JOBS, queue: jobs-prod }",
		"- { name: jobs-prod }",
		"#   - { name: memory-idx, dimensions: 1536, metric: cosine }",
	} {
		if !strings.Contains(y, frag) {
			t.Errorf("vectorize/queues yaml missing %q:\n%s", frag, y)
		}
	}
}

func TestSuggestedCloudflareBlock_HeuristicPlaceholders(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "package.json", `{"dependencies":{"@libsql/client":"^0.6.0"}}`)
	write(t, dir, "db.ts", `const x: D1Database = null as any;`)
	res, _ := Sniff(dir)
	block := res.SuggestedCloudflareBlock()
	if !strings.Contains(block, "d1:") || !strings.Contains(block, "ref: app-db") {
		t.Errorf("heuristic D1 block wrong:\n%s", block)
	}
}

func TestSuggestedCloudflareBlock_KVRefAndProvision(t *testing.T) {
	// Heuristic KV (no wrangler id) → ref binding + a provisioned resource.
	dir := t.TempDir()
	write(t, dir, "env.d.ts", `interface Env { CACHE: KVNamespace }`)
	res, _ := Sniff(dir)
	block := res.SuggestedCloudflareBlock()
	if !strings.Contains(block, "ref: app-cache") {
		t.Errorf("KV binding should use a ref:\n%s", block)
	}
	if !strings.Contains(block, "kv:\n          - { name: app-cache }") {
		t.Errorf("KV resource not declared for provisioning:\n%s", block)
	}
	if strings.Contains(block, "REPLACE_WITH_KV_ID") {
		t.Errorf("KV should auto-provision, not use a placeholder:\n%s", block)
	}
}

func TestSuggestedCloudflareBlock_KVExplicitIDFromWrangler(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "wrangler.jsonc", `{ "kv_namespaces": [{ "binding": "CACHE", "id": "kv-real" }] }`)
	res, _ := Sniff(dir)
	block := res.SuggestedCloudflareBlock()
	if !strings.Contains(block, "namespace_id: kv-real") {
		t.Errorf("explicit KV id should be used:\n%s", block)
	}
	// With an explicit id, no provisioning resource is needed.
	if strings.Contains(block, "name: app-cache }") {
		t.Errorf("should not provision KV when id is known:\n%s", block)
	}
}

func TestSuggestedCloudflareBlock_ProtectionWhenAuth(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "package.json", `{"dependencies":{"better-auth":"^1.0.0"}}`)
	res, _ := Sniff(dir)
	block := res.SuggestedCloudflareBlock()
	for _, frag := range []string{
		"protection:",
		"enabled: false",
		`public_paths: ["/", "/login", "/api/health"]`,
		`protected_paths: ["/app/*"]`,
		"rate_limit:",
		"kv_binding: RATE_LIMIT",
	} {
		if !strings.Contains(block, frag) {
			t.Errorf("protection block missing %q\n%s", frag, block)
		}
	}
	assertValidNextDeployYAML(t, res.RenderNextDeployYAML("myapp"))
}

func TestSuggestedCloudflareBlock_EmptyWhenNothingDetected(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "app/page.tsx", `export default function P(){return null}`)
	res, _ := Sniff(dir)
	if got := res.SuggestedCloudflareBlock(); got != "" {
		t.Errorf("expected empty block, got:\n%s", got)
	}
}

func TestSummary_MentionsWranglerSourceAndResources(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "wrangler.toml", `name="w"
[[d1_databases]]
binding="DB"
database_name="app-db"`)
	res, _ := Sniff(dir)
	s := res.Summary()
	if !strings.Contains(s, "wrangler.toml") || !strings.Contains(s, "d1") {
		t.Errorf("summary missing source/resource: %q", s)
	}
}

func TestRenderNextDeployYAML_DetectedAndEmpty(t *testing.T) {
	// Detected: includes the cloudflare block with bindings.
	dir := t.TempDir()
	write(t, dir, "wrangler.jsonc", `{ "d1_databases": [{ "binding": "DB", "database_id": "u1" }] }`)
	res, _ := Sniff(dir)
	y := res.RenderNextDeployYAML("myapp")
	for _, frag := range []string{"name: myapp", "provider: cloudflare", "cloudflare:", "d1:"} {
		if !strings.Contains(y, frag) {
			t.Errorf("rendered yaml missing %q\n%s", frag, y)
		}
	}
	// Domain is an active (editable) field, not a commented block.
	if !strings.Contains(y, `  domain: ""`) {
		t.Errorf("rendered yaml missing active domain field\n%s", y)
	}
	// Critical: the generated config must be valid YAML and parse into the real
	// schema — `cloudflare:` must sit under `serverless:`, not `provider:`.
	assertValidNextDeployYAML(t, y)

	// Empty: still a valid minimal cloudflare config.
	empty := t.TempDir()
	write(t, empty, "app/page.tsx", `export default function P(){return null}`)
	er, _ := Sniff(empty)
	ey := er.RenderNextDeployYAML("bare")
	if !strings.Contains(ey, "compatibility_date") {
		t.Errorf("minimal yaml missing cloudflare stub:\n%s", ey)
	}
	assertValidNextDeployYAML(t, ey)
}

// assertValidNextDeployYAML fails if s is not parseable as a NextDeployConfig —
// the regression guard for the init-generated config (it must never emit
// mis-indented YAML that the loader rejects).
func assertValidNextDeployYAML(t *testing.T, s string) {
	t.Helper()
	var c config.NextDeployConfig
	if err := yaml.Unmarshal([]byte(s), &c); err != nil {
		t.Fatalf("generated config is not valid YAML: %v\n%s", err, s)
	}
	if c.Serverless == nil || c.Serverless.Provider != "cloudflare" {
		t.Errorf("expected serverless.provider=cloudflare, got %+v\n%s", c.Serverless, s)
	}
}

func TestSummary_EmptyProject(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "app/page.tsx", `export default function P(){return null}`)
	res, _ := Sniff(dir)
	if !strings.Contains(res.Summary(), "No Cloudflare resources detected") {
		t.Errorf("unexpected summary: %q", res.Summary())
	}
}
