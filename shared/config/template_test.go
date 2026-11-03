package config

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestVPSInitTemplateIncludesOptInGoSidecarFields(t *testing.T) {
	raw := GetSampleConfigTemplate("vps")
	var cfg NextDeployConfig
	if err := yaml.Unmarshal([]byte(raw), &cfg); err != nil {
		t.Fatalf("parse generated VPS config: %v", err)
	}
	sidecar := cfg.App.GoSidecar
	if sidecar == nil {
		t.Fatal("generated VPS config is missing app.gosidecar fields")
	}
	if sidecar.Enabled {
		t.Fatal("generated VPS config must leave the sidecar disabled by default")
	}
	if sidecar.ImageOptimization == nil || !*sidecar.ImageOptimization {
		t.Fatal("generated VPS config should enable image optimization when the sidecar is opted in")
	}
	if sidecar.ISRCache == nil || !*sidecar.ISRCache {
		t.Fatal("generated VPS config should enable ISR caching when the sidecar is opted in")
	}
	if sidecar.MaxImageWorkers != 4 {
		t.Fatalf("max_image_workers = %d, want 4", sidecar.MaxImageWorkers)
	}
	if !strings.Contains(raw, "Requests requiring middleware or SSR continue to use Node.js.") {
		t.Fatal("generated config should document the Node.js fallback")
	}
}

func TestServerlessInitTemplatesDoNotIncludeGoSidecar(t *testing.T) {
	for _, target := range []string{"serverless", "cloudflare"} {
		raw := GetSampleConfigTemplate(target)
		if strings.Contains(raw, "gosidecar:") {
			t.Errorf("%s init template should not advertise the VPS-only Go sidecar", target)
		}
	}
}
