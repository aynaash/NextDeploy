package initialcommand

import (
	"strings"
	"testing"

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
