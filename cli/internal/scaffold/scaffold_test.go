package scaffold

import (
	"os"
	"path/filepath"
	"slices"
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
		"nextdeploy.yml", "middleware.ts", "lib/env.ts",
		".github/workflows/nextdeploy.yml", "package.json", "README.md",
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

func TestScaffold_NoneVariant_OmitsDatabase(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := Scaffold(Options{AppName: "marketing", DBVariant: DBNone, Dir: dir}); err != nil {
		t.Fatal(err)
	}
	y := read(t, filepath.Join(dir, "nextdeploy.yml"))
	for _, forbidden := range []string{"d1:", "hyperdrive:", "migrations_dir:"} {
		if strings.Contains(y, forbidden) {
			t.Errorf("none variant should not include %q:\n%s", forbidden, y)
		}
	}
	for _, want := range []string{"provider: cloudflare", `domain: ""`, "r2:", "kv:", "protection:"} {
		if !strings.Contains(y, want) {
			t.Errorf("none variant missing %q:\n%s", want, y)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "migrations")); err == nil {
		t.Error("none variant should not create migrations/ directory")
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

func TestTemplateFiles_AllVariants(t *testing.T) {
	for _, v := range []DBVariant{DBD1, DBBYO, DBNone} {
		files, err := TemplateFiles(v)
		if err != nil {
			t.Fatal(err)
		}
		if !containsPath(files, "package.json") || !containsPath(files, "nextdeploy.yml") || !containsPath(files, "middleware.ts") {
			t.Errorf("variant %s missing core infra files: %v", v, files)
		}
	}
}

func containsPath(ss []string, want string) bool {
	return slices.Contains(ss, want)
}

func TestScaffold_VPSPlatform(t *testing.T) {
	dir := t.TempDir()
	written, skipped, err := Scaffold(Options{
		AppName:         "vps-app",
		Platform:        PlatformVPS,
		EnableGoSidecar: true,
		Dir:             dir,
	})
	if err != nil {
		t.Fatalf("Scaffold VPS: %v", err)
	}
	if len(skipped) != 0 {
		t.Errorf("fresh VPS scaffold should skip nothing, got %v", skipped)
	}

	for _, rel := range []string{
		"nextdeploy.yml", "package.json", "next.config.mjs", "tsconfig.json",
		"Dockerfile", ".dockerignore", "README.md",
		"app/globals.css", "app/layout.tsx", "app/page.tsx", "app/api/health/route.ts",
	} {
		if _, err := os.Stat(filepath.Join(dir, rel)); err != nil {
			t.Errorf("expected VPS file %s: %v", rel, err)
		}
	}

	// Should NOT include Cloudflare-specific files.
	for _, rel := range []string{"middleware.ts", "proxy.ts"} {
		if _, err := os.Stat(filepath.Join(dir, rel)); err == nil {
			t.Errorf("VPS scaffold should not include %s", rel)
		}
	}

	// Check nextdeploy.yml content
	yml := read(t, filepath.Join(dir, "nextdeploy.yml"))
	if !strings.Contains(yml, "target: vps") {
		t.Errorf("VPS config missing 'target: vps':\n%s", yml)
	}
	if !strings.Contains(yml, "enabled: true") {
		t.Errorf("VPS config should have sidecar enabled:\n%s", yml)
	}
	if len(written) < 6 {
		t.Errorf("expected several files written, got %d", len(written))
	}
}

func TestScaffold_AWSPlatform(t *testing.T) {
	dir := t.TempDir()
	written, skipped, err := Scaffold(Options{
		AppName:  "aws-app",
		Platform: PlatformAWS,
		Dir:      dir,
	})
	if err != nil {
		t.Fatalf("Scaffold AWS: %v", err)
	}
	if len(skipped) != 0 {
		t.Errorf("fresh AWS scaffold should skip nothing, got %v", skipped)
	}

	for _, rel := range []string{
		"nextdeploy.yml", "package.json", "next.config.mjs", "tsconfig.json",
		"README.md", "app/globals.css", "app/layout.tsx", "app/page.tsx",
		"app/api/health/route.ts",
	} {
		if _, err := os.Stat(filepath.Join(dir, rel)); err != nil {
			t.Errorf("expected AWS file %s: %v", rel, err)
		}
	}

	// Check nextdeploy.yml content
	yml := read(t, filepath.Join(dir, "nextdeploy.yml"))
	if !strings.Contains(yml, "target: serverless") || !strings.Contains(yml, "provider: aws") {
		t.Errorf("AWS config missing serverless provider aws:\n%s", yml)
	}
	if len(written) < 6 {
		t.Errorf("expected several files written, got %d", len(written))
	}
}

func TestScaffold_CloudflareInfraOnlySkipsStarterAppPages(t *testing.T) {
	dir := t.TempDir()
	written, _, err := Scaffold(Options{
		AppName:   "existing-app",
		Platform:  PlatformCloudflare,
		DBVariant: DBD1,
		Dir:       dir,
		InfraOnly: true,
	})
	if err != nil {
		t.Fatalf("Scaffold InfraOnly: %v", err)
	}
	for _, want := range []string{
		"nextdeploy.yml", "middleware.ts", "lib/env.ts",
		"migrations/0001_example.sql", ".github/workflows/nextdeploy.yml",
	} {
		if _, err := os.Stat(filepath.Join(dir, want)); err != nil {
			t.Errorf("expected infra file %s: %v", want, err)
		}
	}
	for _, forbidden := range []string{
		"package.json", "tsconfig.json", "next.config.mjs", "README.md",
		"app/page.tsx", "app/layout.tsx", "app/app/page.tsx", "app/login/page.tsx",
	} {
		if _, err := os.Stat(filepath.Join(dir, forbidden)); err == nil {
			t.Errorf("InfraOnly must not create %s", forbidden)
		}
	}
	yml := read(t, filepath.Join(dir, "nextdeploy.yml"))
	if !strings.Contains(yml, `domain: ""`) {
		t.Errorf("scaffolded Cloudflare nextdeploy.yml missing app.domain field:\n%s", yml)
	}
	if len(written) != 5 {
		t.Errorf("expected 5 infra files written, got %d: %v", len(written), written)
	}
}

func TestScaffold_CloudflareNext16UsesProxyAndSrcLayout(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "src", "app"), 0o750); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Scaffold(Options{
		AppName:     "next16-app",
		Platform:    PlatformCloudflare,
		DBVariant:   DBNone,
		Dir:         dir,
		InfraOnly:   true,
		NextVersion: "^16.0.0",
	}); err != nil {
		t.Fatalf("Scaffold Next 16: %v", err)
	}
	proxyPath := filepath.Join(dir, "src", "proxy.ts")
	body := read(t, proxyPath)
	if !strings.Contains(body, "export default function proxy(") {
		t.Errorf("expected Next 16 proxy export in src/proxy.ts:\n%s", body)
	}
	if _, err := os.Stat(filepath.Join(dir, "middleware.ts")); err == nil {
		t.Error("Next 16 src/app project should not create root middleware.ts")
	}
	if _, err := os.Stat(filepath.Join(dir, "proxy.ts")); err == nil {
		t.Error("src/app project should place proxy.ts inside src/, not root")
	}
}

func TestScaffold_CloudflareSkipsWhenInterceptorAlreadyExists(t *testing.T) {
	dir := t.TempDir()
	existing := filepath.Join(dir, "src", "middleware.ts")
	if err := os.MkdirAll(filepath.Dir(existing), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(existing, []byte("export function middleware() {}"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, skipped, err := Scaffold(Options{
		AppName:     "has-mw",
		Platform:    PlatformCloudflare,
		DBVariant:   DBNone,
		Dir:         dir,
		InfraOnly:   true,
		NextVersion: "^16.0.0",
	})
	if err != nil {
		t.Fatalf("Scaffold: %v", err)
	}
	if !containsPath(skipped, existing) {
		t.Errorf("expected existing %s to be skipped, got %v", existing, skipped)
	}
	if _, err := os.Stat(filepath.Join(dir, "proxy.ts")); err == nil {
		t.Error("must not create conflicting proxy.ts when src/middleware.ts already exists")
	}
}
