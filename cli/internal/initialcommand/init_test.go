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
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"name":"demo"}`), 0o600); err != nil {
		t.Fatal(err)
	}

	cmd := &cobra.Command{}
	cmd.Flags().String("target", "cloudflare", "")
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
}
