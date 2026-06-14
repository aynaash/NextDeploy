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
		"bindings:",
		"- { name: DB, id: uuid-1 }",
		"- { name: ASSETS, bucket: app-assets }",
		"ai:",
		"- { name: AI }",
		"resources:",
		"migrations_dir: drizzle",
	} {
		if !strings.Contains(block, frag) {
			t.Errorf("block missing %q\n%s", frag, block)
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
	for _, frag := range []string{"protection:", "enabled: true", "protected_paths: [\"/app/*\"]", "rate_limit:"} {
		if !strings.Contains(block, frag) {
			t.Errorf("protection block missing %q\n%s", frag, block)
		}
	}
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
