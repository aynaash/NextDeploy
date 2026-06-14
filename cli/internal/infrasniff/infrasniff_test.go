package infrasniff

import (
	"os"
	"path/filepath"
	"testing"
)

func write(t *testing.T, dir, rel, content string) {
	t.Helper()
	p := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func hasResource(r *Result, want Resource) bool {
	for _, got := range r.Resources() {
		if got == want {
			return true
		}
	}
	return false
}

func reasonFor(r *Result, res Resource) []string {
	var out []string
	for _, s := range r.Signals {
		if s.Resource == res {
			out = append(out, s.Reason)
		}
	}
	return out
}

func TestSniff_D1FromImportAndDep(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "package.json", `{"dependencies":{"drizzle-orm":"^0.30.0","@libsql/client":"^0.6.0"}}`)
	write(t, dir, "src/db.ts", `import { drizzle } from "drizzle-orm/d1";
export function db(env: { DB: D1Database }) { return drizzle(env.DB); }`)
	res, err := Sniff(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !hasResource(res, ResD1) {
		t.Fatalf("expected D1 detected, signals=%v", res.Signals)
	}
	// Both the dep and the source type should contribute reasons.
	reasons := reasonFor(res, ResD1)
	if len(reasons) < 2 {
		t.Errorf("expected multiple D1 reasons, got %v", reasons)
	}
}

func TestSniff_HyperdriveFromPostgres(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "package.json", `{"dependencies":{"postgres":"^3.4.0"}}`)
	write(t, dir, "lib/db.ts", `const url = process.env.DATABASE_URL!; // postgres://user:pw@host/db`)
	res, err := Sniff(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !hasResource(res, ResHyperdrive) {
		t.Errorf("expected Hyperdrive, signals=%v", res.Signals)
	}
	if !contains(res.Secrets, "DATABASE_URL") {
		t.Errorf("expected DATABASE_URL secret, got %v", res.Secrets)
	}
}

func TestSniff_AIFromModelIDAndBinding(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "app/api/chat/route.ts", `export async function POST(req, { env }) {
  return env.AI.run("@cf/meta/llama-3-8b-instruct", { prompt: "hi" });
}`)
	res, err := Sniff(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !hasResource(res, ResAI) {
		t.Errorf("expected AI, signals=%v", res.Signals)
	}
}

func TestSniff_R2KVVectorizeQueueTypes(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "worker-env.d.ts", `interface Env {
  ASSETS: R2Bucket;
  CACHE: KVNamespace;
  MEM: VectorizeIndex;
  JOBS: Queue<string>;
}`)
	res, err := Sniff(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []Resource{ResR2, ResKV, ResVectorize, ResQueue} {
		if !hasResource(res, want) {
			t.Errorf("expected %s detected, signals=%v", want, res.Signals)
		}
	}
}

func TestSniff_AuthDetection(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "package.json", `{"dependencies":{"better-auth":"^1.0.0"}}`)
	res, err := Sniff(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Auth {
		t.Error("expected Auth=true for better-auth dependency")
	}
	if !contains(res.Secrets, "AUTH_SECRET") {
		t.Errorf("expected AUTH_SECRET suggested, got %v", res.Secrets)
	}
}

func TestSniff_SecretsExcludePublicAndBindings(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "app/page.tsx", `const a = process.env.STRIPE_SECRET_KEY;
const b = process.env.NEXT_PUBLIC_URL;     // public, not a secret
const c = env.DB;                          // binding, not a secret
const d = process.env.RESEND_API_KEY;`)
	res, err := Sniff(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !contains(res.Secrets, "STRIPE_SECRET_KEY") || !contains(res.Secrets, "RESEND_API_KEY") {
		t.Errorf("missing expected secrets: %v", res.Secrets)
	}
	if contains(res.Secrets, "NEXT_PUBLIC_URL") {
		t.Error("NEXT_PUBLIC_ vars must not be treated as secrets")
	}
	if contains(res.Secrets, "DB") {
		t.Error("binding names must not be treated as secrets")
	}
}

func TestSniff_SkipsNodeModulesAndBuildDirs(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "node_modules/pkg/index.js", `const x = R2Bucket; process.env.LEAKED_SECRET;`)
	write(t, dir, ".next/server/app.js", `D1Database; process.env.ALSO_LEAKED;`)
	write(t, dir, "app/page.tsx", `export default function P(){return null}`)
	res, err := Sniff(dir)
	if err != nil {
		t.Fatal(err)
	}
	if hasResource(res, ResR2) || hasResource(res, ResD1) {
		t.Errorf("must not scan node_modules/.next, signals=%v", res.Signals)
	}
	if contains(res.Secrets, "LEAKED_SECRET") || contains(res.Secrets, "ALSO_LEAKED") {
		t.Errorf("must not collect secrets from skipped dirs: %v", res.Secrets)
	}
}

func TestSniff_EmptyProjectNoSignals(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "app/page.tsx", `export default function Home(){ return <div/> }`)
	res, err := Sniff(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Resources()) != 0 {
		t.Errorf("expected no resources, got %v", res.Resources())
	}
	if res.FilesParsed != 1 {
		t.Errorf("expected 1 file parsed, got %d", res.FilesParsed)
	}
}

func TestSniff_MissingDirIsNotFatal(t *testing.T) {
	res, err := Sniff(filepath.Join(t.TempDir(), "nope"))
	if err != nil {
		t.Fatalf("missing dir should not error: %v", err)
	}
	if len(res.Resources()) != 0 {
		t.Errorf("expected empty result, got %v", res.Resources())
	}
}

func contains(ss []string, want string) bool {
	for _, s := range ss {
		if s == want {
			return true
		}
	}
	return false
}
