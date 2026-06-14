package serverless

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/aynaash/nextdeploy/shared/config"
)

func boolPtr(b bool) *bool { return &b }

func TestBuildObservability_DefaultsEnabled(t *testing.T) {
	obs := buildObservability(nil)
	raw, _ := json.Marshal(obs)
	got := string(raw)
	for _, frag := range []string{`"enabled":true`, `"head_sampling_rate":1`, `"invocation_logs":true`} {
		if !strings.Contains(got, frag) {
			t.Errorf("default observability missing %q\n%s", frag, got)
		}
	}
}

func TestBuildObservability_ExplicitDisable(t *testing.T) {
	obs := buildObservability(&config.CloudflareConfig{
		Observability: &config.CFObservability{Enabled: boolPtr(false)},
	})
	raw, _ := json.Marshal(obs)
	if !strings.Contains(string(raw), `"enabled":false`) {
		t.Errorf("expected disabled observability:\n%s", raw)
	}
}

func TestBuildObservability_SamplingRateClampedAndApplied(t *testing.T) {
	obs := buildObservability(&config.CloudflareConfig{
		Observability: &config.CFObservability{Enabled: boolPtr(true), HeadSamplingRate: 0.25},
	})
	if !strings.Contains(mustJSON(t, obs), `"head_sampling_rate":0.25`) {
		t.Errorf("sampling rate not applied:\n%s", mustJSON(t, obs))
	}

	// Out-of-range clamps to 1.
	high := buildObservability(&config.CloudflareConfig{
		Observability: &config.CFObservability{HeadSamplingRate: 5},
	})
	if !strings.Contains(mustJSON(t, high), `"head_sampling_rate":1`) {
		t.Errorf("rate >1 should clamp to 1:\n%s", mustJSON(t, high))
	}
}

// Observability must land in the uploaded script metadata by default.
func TestBuildScriptMetadata_IncludesObservability(t *testing.T) {
	meta, err := buildScriptMetadata(nil, "auto-bucket", "worker.mjs", noResolver, nil)
	if err != nil {
		t.Fatalf("buildScriptMetadata: %v", err)
	}
	raw, _ := json.Marshal(meta)
	if !strings.Contains(string(raw), `"observability"`) || !strings.Contains(string(raw), `"invocation_logs":true`) {
		t.Errorf("script metadata missing observability:\n%s", raw)
	}
}

func mustJSON(t *testing.T, v interface{}) string {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}
