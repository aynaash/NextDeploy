package serverless

import (
	"strings"
	"testing"

	"github.com/aynaash/nextdeploy/shared/config"
)

func TestIsUnmanaged(t *testing.T) {
	declared := map[string]struct{}{"shop-db": {}}
	cases := []struct {
		name string
		want bool
		why  string
	}{
		{"shop-db", false, "declared → managed"},
		{"shop-db-old", true, "matches prefix, not declared → looks like a rename leftover"},
		{"shop", true, "bare app name matches the convention"},
		{"other-db", false, "different app on the same account → not our business"},
		{"shopify-db", false, "prefix must be followed by a hyphen, not just any suffix"},
		{"SHOP-CACHE", true, "match is case-insensitive"},
		{"", false, "empty name is never reported"},
	}
	for _, c := range cases {
		if got := isUnmanaged(c.name, "shop", declared); got != c.want {
			t.Errorf("isUnmanaged(%q) = %v, want %v (%s)", c.name, got, c.want, c.why)
		}
	}
}

func TestUnmanagedScanPrefix_EmptyAppDisablesScan(t *testing.T) {
	if got := unmanagedScanPrefix("  "); got != "" {
		t.Errorf("blank app name should disable the scan, got %q", got)
	}
	if got := unmanagedScanPrefix("  Shop "); got != "shop" {
		t.Errorf("prefix = %q, want shop", got)
	}
}

func TestUnmanagedWarnings(t *testing.T) {
	if got := unmanagedWarnings(nil); got != nil {
		t.Errorf("no findings should produce no warnings, got %v", got)
	}
	lines := unmanagedWarnings([]UnmanagedResource{
		{Kind: "d1", Name: "shop-db-old"},
		{Kind: "kv", Name: "shop-cache-v1"},
	})
	joined := strings.Join(lines, "\n")
	for _, want := range []string{"shop-db-old", "shop-cache-v1", "will NOT rename", "orphaned"} {
		if !strings.Contains(joined, want) {
			t.Errorf("warning text missing %q:\n%s", want, joined)
		}
	}
}

func TestQueueConsumerWarnings(t *testing.T) {
	cf := &config.CloudflareConfig{
		Resources: &config.CFResources{
			Queues: []config.CFQueueResource{{Name: "jobs"}},
		},
		Bindings: &config.CFBindings{
			Queues: &config.CFQueueBindings{
				Consumers: []config.CFQueueConsumer{
					{Queue: "jobs"},    // declared → no warning
					{Queue: "unknown"}, // not declared → warning
					{Queue: ""},        // ignored
				},
			},
		},
	}
	got := queueConsumerWarnings(cf)
	if len(got) != 1 {
		t.Fatalf("want 1 warning, got %d: %v", len(got), got)
	}
	if !strings.Contains(got[0], "unknown") {
		t.Errorf("warning should name the undeclared queue: %q", got[0])
	}
}

func TestQueueConsumerWarnings_NilSafe(t *testing.T) {
	if got := queueConsumerWarnings(nil); got != nil {
		t.Errorf("nil config should yield no warnings, got %v", got)
	}
	if got := queueConsumerWarnings(&config.CloudflareConfig{}); got != nil {
		t.Errorf("empty config should yield no warnings, got %v", got)
	}
}
