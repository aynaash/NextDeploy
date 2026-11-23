package initialcommand

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/aynaash/nextdeploy/shared/config"
)

func TestSetGoSidecarEnabled(t *testing.T) {
	raw := config.GetSampleConfigTemplate("vps")

	got, err := setGoSidecarEnabled(raw)
	if err != nil {
		t.Fatalf("enable Go sidecar: %v", err)
	}
	if !strings.Contains(got, "enabled: true # Opt in after reviewing the sidecar's supported request paths") {
		t.Fatal("enabled Go sidecar setting or its comment was not preserved")
	}

	var cfg config.NextDeployConfig
	if err := yaml.Unmarshal([]byte(got), &cfg); err != nil {
		t.Fatalf("parse generated VPS config: %v", err)
	}
	if cfg.App.GoSidecar == nil || !cfg.App.GoSidecar.Enabled {
		t.Fatal("generated VPS config should enable the Go sidecar")
	}
}

func TestSetGoSidecarEnabledRequiresVPSSetting(t *testing.T) {
	if _, err := setGoSidecarEnabled(config.GetSampleConfigTemplate("serverless")); err == nil {
		t.Fatal("expected an error when the Go sidecar setting is missing")
	}
}

func TestNormalizeInitTarget(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{input: "vps", want: "vps"},
		{input: " AWS ", want: "serverless"},
		{input: "serverless", want: "serverless"},
		{input: "Cloudflare", want: "cloudflare"},
		{input: "invalid", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := normalizeInitTarget(tt.input); got != tt.want {
				t.Fatalf("normalizeInitTarget(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestRunInitCommandCloudflareScaffoldFlagsAreNonInteractive(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"name":"demo","dependencies":{"next":"15.1.0"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	// Simulate an existing src/app/ project to verify infra-only scaffolding
	// doesn't drop conflicting root app/ starter pages.
	if err := os.MkdirAll(filepath.Join(dir, "src", "app"), 0o750); err != nil {
		t.Fatal(err)
	}

	cmd := &cobra.Command{}
	cmd.Flags().String("target", "cloudflare", "")
	cmd.Flags().String("platform", "", "")
	cmd.Flags().String("mode", "scaffold", "")
	cmd.Flags().String("db", "d1", "")

	if err := RunInitCommand(cmd, nil); err != nil {
		t.Fatalf("RunInitCommand with non-interactive flags: %v", err)
	}
	configPath := filepath.Join(dir, "nextdeploy.yml")
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read generated nextdeploy.yml: %v", err)
	}
	if !strings.Contains(string(content), "provider: cloudflare") {
		t.Fatal("generated config does not target Cloudflare")
	}
	// Infra files should be written (src/middleware.ts for Next 15 src/app layout),
	// while root app/ starter pages must NOT be dropped into an existing Next.js repo.
	for _, rel := range []string{"src/middleware.ts", "lib/env.ts", "migrations/0001_example.sql", ".github/workflows/nextdeploy.yml"} {
		if _, err := os.Stat(filepath.Join(dir, rel)); err != nil {
			t.Errorf("expected infra file %s: %v", rel, err)
		}
	}
	for _, unexpected := range []string{"app/page.tsx", "app/layout.tsx", "app/app/page.tsx", "app/login/page.tsx"} {
		if _, err := os.Stat(filepath.Join(dir, unexpected)); err == nil {
			t.Errorf("existing-project scaffold must not create starter page %s", unexpected)
		}
	}
}

func TestRunInitCommand_CloudflareSniff_ExistingAppSanitizesScopedName(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"name":"@acme/my_web","dependencies":{"next":"15.1.0"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := &cobra.Command{}
	cmd.Flags().String("target", "cloudflare", "")
	cmd.Flags().String("platform", "", "")
	cmd.Flags().String("mode", "sniff", "")
	cmd.Flags().String("db", "", "")

	if err := RunInitCommand(cmd, nil); err != nil {
		t.Fatalf("RunInitCommand sniff: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "nextdeploy.yml"))
	if err != nil {
		t.Fatal(err)
	}
	var cfg config.NextDeployConfig
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		t.Fatalf("unmarshal generated nextdeploy.yml: %v\n%s", err, raw)
	}
	if err := config.ValidateAppName(cfg.App.Name); err != nil {
		t.Fatalf("generated app.name %q failed ValidateAppName: %v", cfg.App.Name, err)
	}
	if cfg.App.Name != "acme-my-web" {
		t.Errorf("expected slugified app.name %q, got %q", "acme-my-web", cfg.App.Name)
	}
}

func TestRunInitCommand_CloudflareSniffRejectsDBFlag(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"name":"demo","dependencies":{"next":"15.1.0"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := &cobra.Command{}
	cmd.Flags().String("target", "cloudflare", "")
	cmd.Flags().String("platform", "", "")
	cmd.Flags().String("mode", "sniff", "")
	cmd.Flags().String("db", "d1", "")

	if err := RunInitCommand(cmd, nil); err == nil {
		t.Fatal("expected error when combining --mode=sniff with --db=d1")
	}
}

func TestRunInitCommand_CloudflareSniffFailsWithoutNextProject(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	cmd := &cobra.Command{}
	cmd.Flags().String("target", "cloudflare", "")
	cmd.Flags().String("platform", "", "")
	cmd.Flags().String("mode", "sniff", "")
	cmd.Flags().String("db", "", "")

	if err := RunInitCommand(cmd, nil); err == nil {
		t.Fatal("expected error when running --mode=sniff outside a Next.js project")
	}
}

func TestRunInitCommand_InvalidModeRejectedWithoutNextProject(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	cmd := &cobra.Command{}
	cmd.Flags().String("target", "cloudflare", "")
	cmd.Flags().String("platform", "", "")
	cmd.Flags().String("mode", "bogus", "")
	cmd.Flags().String("db", "", "")

	if err := RunInitCommand(cmd, nil); err == nil {
		t.Fatal("expected error for invalid --mode value")
	}
}

func TestRunInitCommand_CloudflareScaffold_NoDB(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"name":"docs-site","dependencies":{"next":"15.1.0"}}`), 0o600); err != nil {
		t.Fatal(err)
	}

	cmd := &cobra.Command{}
	cmd.Flags().String("target", "cloudflare", "")
	cmd.Flags().String("platform", "", "")
	cmd.Flags().String("mode", "scaffold", "")
	cmd.Flags().String("db", "none", "")

	if err := RunInitCommand(cmd, nil); err != nil {
		t.Fatalf("RunInitCommand with --db=none: %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(dir, "nextdeploy.yml"))
	if err != nil {
		t.Fatal(err)
	}
	var cfg config.NextDeployConfig
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		t.Fatalf("unmarshal nextdeploy.yml: %v\n%s", err, raw)
	}
	if err := config.ValidateAppName(cfg.App.Name); err != nil {
		t.Fatalf("invalid app.name: %v", err)
	}
	if cfg.Serverless == nil || cfg.Serverless.Cloudflare == nil {
		t.Fatalf("missing serverless.cloudflare block:\n%s", raw)
	}
	if b := cfg.Serverless.Cloudflare.Bindings; b != nil && (len(b.D1) > 0 || len(b.Hyperdrive) > 0) {
		t.Errorf("--db=none should not declare D1 or Hyperdrive bindings: %+v", b)
	}
	if r := cfg.Serverless.Cloudflare.Resources; r != nil && (len(r.D1) > 0 || len(r.Hyperdrive) > 0) {
		t.Errorf("--db=none should not declare D1 or Hyperdrive resources: %+v", r)
	}
	if _, err := os.Stat(filepath.Join(dir, "migrations")); err == nil {
		t.Error("--db=none should not create migrations/ directory")
	}
}

func TestRunInitCommand_NoNextProject_ScaffoldsVPS(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	cmd := &cobra.Command{}
	cmd.Flags().String("target", "vps", "")
	cmd.Flags().String("platform", "", "")
	cmd.Flags().String("mode", "", "")
	cmd.Flags().String("db", "", "")

	if err := RunInitCommand(cmd, nil); err != nil {
		t.Fatalf("RunInitCommand VPS: %v", err)
	}

	for _, expectedFile := range []string{
		"package.json", "next.config.mjs", "tsconfig.json", "Dockerfile",
		"nextdeploy.yml", "app/globals.css", "app/page.tsx", "app/api/health/route.ts",
	} {
		if _, err := os.Stat(filepath.Join(dir, expectedFile)); err != nil {
			t.Errorf("expected scaffolded file %s: %v", expectedFile, err)
		}
	}

	content, err := os.ReadFile(filepath.Join(dir, "nextdeploy.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "target: vps") {
		t.Errorf("expected target: vps in nextdeploy.yml:\n%s", string(content))
	}
}

func TestRunInitCommand_NoNextProject_ScaffoldsAWS(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	cmd := &cobra.Command{}
	cmd.Flags().String("target", "aws", "")
	cmd.Flags().String("platform", "", "")
	cmd.Flags().String("mode", "", "")
	cmd.Flags().String("db", "", "")

	if err := RunInitCommand(cmd, nil); err != nil {
		t.Fatalf("RunInitCommand AWS: %v", err)
	}

	for _, expectedFile := range []string{
		"package.json", "next.config.mjs", "tsconfig.json",
		"nextdeploy.yml", "app/globals.css", "app/page.tsx", "app/api/health/route.ts",
	} {
		if _, err := os.Stat(filepath.Join(dir, expectedFile)); err != nil {
			t.Errorf("expected scaffolded file %s: %v", expectedFile, err)
		}
	}

	content, err := os.ReadFile(filepath.Join(dir, "nextdeploy.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "target: serverless") || !strings.Contains(string(content), "provider: aws") {
		t.Errorf("expected AWS serverless in nextdeploy.yml:\n%s", string(content))
	}
}
