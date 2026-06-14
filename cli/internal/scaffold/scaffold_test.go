package scaffold

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func read(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path) // #nosec G304
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

func TestScaffold_WritesDeploymentInfra(t *testing.T) {
	dir := t.TempDir()
	written, skipped, err := Scaffold(Options{AppName: "shop", DBVariant: DBD1, Dir: dir})
	if err != nil {
		t.Fatalf("Scaffold: %v", err)
	}
	if len(skipped) != 0 {
		t.Errorf("fresh scaffold should skip nothing, got %v", skipped)
	}

	// Deployment infra files (not app business logic).
	for _, rel := range []string{
		"nextdeploy.yml", "proxy.ts", "lib/env.ts",
		".github/workflows/deploy.yml", "package.json", "README.md",
		"next.config.mjs", "tsconfig.json",
		"migrations/0001_example.sql",
	} {
		if _, err := os.Stat(filepath.Join(dir, rel)); err != nil {
			t.Errorf("expected infra file %s: %v", rel, err)
		}
	}
	if len(written) < 8 {
		t.Errorf("expected several files written, got %d", len(written))
	}

	// We must NOT ship app business logic — that's the user's job.
	for _, rel := range []string{
		"lib/auth.ts", "lib/session.ts", "lib/schema.ts", "lib/db.ts",
		"app/api/login/route.ts", "drizzle.config.ts",
	} {
		if _, err := os.Stat(filepath.Join(dir, rel)); err == nil {
			t.Errorf("scaffold must NOT ship app logic file %s — NextDeploy is a deploy pipeline, not an app generator", rel)
		}
	}

	// App name substituted; no leftover tokens.
	if pkg := read(t, filepath.Join(dir, "package.json")); !strings.Contains(pkg, `"name": "shop"`) {
		t.Errorf("app name not substituted:\n%s", pkg)
	}
	for _, rel := range written {
		if strings.Contains(read(t, rel), appNameToken) {
			t.Errorf("unsubstituted token left in %s", rel)
		}
	}
}

func TestScaffold_PackageJSON_NoAppFrameworkDeps(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := Scaffold(Options{AppName: "a", Dir: dir}); err != nil {
		t.Fatal(err)
	}
	pkg := read(t, filepath.Join(dir, "package.json"))
	// Auth/ORM are the user's choices — don't impose them.
	for _, dep := range []string{"better-auth", "drizzle-orm", "drizzle-kit"} {
		if strings.Contains(pkg, dep) {
			t.Errorf("package.json should not impose app dependency %q", dep)
		}
	}
	for _, dep := range []string{"next", "react"} {
		if !strings.Contains(pkg, dep) {
			t.Errorf("package.json missing framework dep %q", dep)
		}
	}
}

func TestScaffold_D1Variant_ConfiguresD1(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := Scaffold(Options{AppName: "a", DBVariant: DBD1, Dir: dir}); err != nil {
		t.Fatal(err)
	}
	y := read(t, filepath.Join(dir, "nextdeploy.yml"))
	if !strings.Contains(y, "d1:") || !strings.Contains(y, "migrations_dir: migrations") {
		t.Errorf("d1 variant nextdeploy.yml wrong:\n%s", y)
	}
}

func TestScaffold_BYOVariant_ConfiguresHyperdrive(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := Scaffold(Options{AppName: "a", DBVariant: DBBYO, Dir: dir}); err != nil {
		t.Fatal(err)
	}
	y := read(t, filepath.Join(dir, "nextdeploy.yml"))
	if !strings.Contains(y, "hyperdrive:") || !strings.Contains(y, "origin_env: DATABASE_URL") {
		t.Errorf("byo variant nextdeploy.yml wrong:\n%s", y)
	}
}

func TestScaffold_DefaultsToD1(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := Scaffold(Options{AppName: "a", Dir: dir}); err != nil {
		t.Fatal(err)
	}
	if y := read(t, filepath.Join(dir, "nextdeploy.yml")); !strings.Contains(y, "d1:") {
		t.Errorf("empty variant should default to d1:\n%s", y)
	}
}

func TestScaffold_ConfigHasProtectionAndObservability(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := Scaffold(Options{AppName: "a", Dir: dir}); err != nil {
		t.Fatal(err)
	}
	y := read(t, filepath.Join(dir, "nextdeploy.yml"))
	for _, frag := range []string{"protection:", "rate_limit:", "secrets_store:"} {
		if !strings.Contains(y, frag) {
			t.Errorf("nextdeploy.yml missing infra block %q:\n%s", frag, y)
		}
	}
}

func TestScaffold_NeverOverwrites(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"name":"existing"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	written, skipped, err := Scaffold(Options{AppName: "new", DBVariant: DBD1, Dir: dir})
	if err != nil {
		t.Fatal(err)
	}
	if read(t, filepath.Join(dir, "package.json")) != `{"name":"existing"}` {
		t.Error("existing package.json was overwritten")
	}
	if !containsPath(skipped, filepath.Join(dir, "package.json")) {
		t.Errorf("package.json should be reported skipped, got %v", skipped)
	}
	if containsPath(written, filepath.Join(dir, "package.json")) {
		t.Error("package.json should not be in written list")
	}
}

func TestScaffold_AttributionInReadme(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := Scaffold(Options{AppName: "a", Dir: dir}); err != nil {
		t.Fatal(err)
	}
	readme := read(t, filepath.Join(dir, "README.md"))
	if !strings.Contains(readme, "aynaash") || !strings.Contains(readme, "Hersi") {
		t.Errorf("README must credit the author (aynaash/Hersi):\n%s", readme)
	}
}

func TestScaffold_Validation(t *testing.T) {
	if _, _, err := Scaffold(Options{Dir: t.TempDir()}); err == nil {
		t.Error("expected error for empty app name")
	}
	if _, _, err := Scaffold(Options{AppName: "a"}); err == nil {
		t.Error("expected error for empty dir")
	}
	if _, _, err := Scaffold(Options{AppName: "a", DBVariant: "mongo", Dir: t.TempDir()}); err == nil {
		t.Error("expected error for unknown variant")
	}
}

func TestTemplateFiles_BothVariants(t *testing.T) {
	for _, v := range []DBVariant{DBD1, DBBYO} {
		files, err := TemplateFiles(v)
		if err != nil {
			t.Fatal(err)
		}
		if !containsPath(files, "package.json") || !containsPath(files, "nextdeploy.yml") || !containsPath(files, "proxy.ts") {
			t.Errorf("variant %s missing core infra files: %v", v, files)
		}
	}
}

func containsPath(ss []string, want string) bool {
	for _, s := range ss {
		if s == want {
			return true
		}
	}
	return false
}
