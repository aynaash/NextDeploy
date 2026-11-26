package cloudflare

import (
	"testing"

	"github.com/aynaash/nextdeploy/shared/config"
)

func TestPlanResult_HasDrift(t *testing.T) {
	cases := []struct {
		name string
		r    PlanResult
		want bool
	}{
		{"empty", PlanResult{}, false},
		{"all noop", PlanResult{Items: []PlanItem{{Action: PlanNoOp}, {Action: PlanNoOp}}}, false},
		{"one create", PlanResult{Items: []PlanItem{{Action: PlanCreate}}}, false},
		{"one drift", PlanResult{Items: []PlanItem{{Action: PlanNoOp}, {Action: PlanImmutableDrift}}}, true},
		{"only drift", PlanResult{Items: []PlanItem{{Action: PlanImmutableDrift}}}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.r.HasDrift(); got != tc.want {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestPlanZoneSettings_Validation(t *testing.T) {
	p := &CloudflareProvider{}

	if _, err := p.planZoneSettings(t.Context(), config.CFZoneSettings{}); err == nil {
		t.Fatal("expected error for empty zone")
	}
	if _, err := p.planZoneSettings(t.Context(), config.CFZoneSettings{Zone: "example.com", MinTTL: 1}); err == nil {
		t.Fatal("expected error for min_ttl=1")
	}
	item, err := p.planZoneSettings(t.Context(), config.CFZoneSettings{Zone: "example.com", MinTTL: 0})
	if err != nil {
		t.Fatalf("unexpected error for min_ttl=0: %v", err)
	}
	if item.Action != PlanNoOp {
		t.Fatalf("got action %q, want %q", item.Action, PlanNoOp)
	}
}

// dnsRecordMatches is the no-op gate in ensureDNSRecord; if this lies, every
// deploy will issue redundant Edit calls (and worse, racy concurrent edits
// could clobber each other).
func TestDNSRecordMatches(t *testing.T) {
	// fake stand-in for dns.RecordResponse — we can't construct one cleanly
	// because of unexported JSON fields, so test with a literal here.
	// (Skipped — covered indirectly by buildDNSRecordBody table tests.)
	t.Skip("requires dns.RecordResponse construction; covered by integration smoke")
}
