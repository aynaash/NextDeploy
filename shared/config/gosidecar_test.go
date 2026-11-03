package config

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func TestGoSidecarConfigResolvesAndAppliesDefaults(t *testing.T) {
	var cfg NextDeployConfig
	input := []byte("app:\n  gosidecar:\n    enabled: true\n")
	if err := yaml.Unmarshal(input, &cfg); err != nil {
		t.Fatal(err)
	}
	sidecar := cfg.ResolveGoSidecar()
	if sidecar == nil || !sidecar.Enabled {
		t.Fatal("app-level GoSidecar config was not decoded")
	}
	if !sidecar.ISRCacheEnabled() || !sidecar.ImageOptEnabled() || sidecar.Workers() != 4 {
		t.Fatalf("unexpected defaults: ISR=%v image=%v workers=%d", sidecar.ISRCacheEnabled(), sidecar.ImageOptEnabled(), sidecar.Workers())
	}
}

func TestGoSidecarConfigTopLevelAndOverrides(t *testing.T) {
	var cfg NextDeployConfig
	input := []byte("gosidecar:\n  enabled: true\n  image_optimization: false\n  isr_cache: false\n  max_image_workers: 8\napp:\n  gosidecar:\n    enabled: true\n    image_optimization: true\n")
	if err := yaml.Unmarshal(input, &cfg); err != nil {
		t.Fatal(err)
	}
	resolved := cfg.ResolveGoSidecar()
	if resolved == nil || !resolved.ImageOptEnabled() {
		t.Fatal("app-level config should override top-level GoSidecar config")
	}
	if !resolved.ISRCacheEnabled() || resolved.Workers() != 4 {
		t.Fatalf("unexpected app-level settings: ISR=%v workers=%d", resolved.ISRCacheEnabled(), resolved.Workers())
	}

	cfg.App.GoSidecar = nil
	resolved = cfg.ResolveGoSidecar()
	if resolved.ImageOptEnabled() || resolved.ISRCacheEnabled() || resolved.Workers() != 8 {
		t.Fatalf("top-level config was not decoded: %+v", resolved)
	}
}
