package daemon

import (
	"strings"
	"testing"

	"github.com/aynaash/nextdeploy/shared/config"
)

func TestRenderResourceLimits(t *testing.T) {
	tests := []struct {
		name           string
		limits         *config.ResourceLimits
		expectContains []string
		expectAbsent   []string
	}{
		{
			name:         "nil limits (default off)",
			limits:       nil,
			expectAbsent: []string{"CPUQuota=", "MemoryMax=", "MemoryHigh="},
		},
		{
			name:         "empty block (default off)",
			limits:       &config.ResourceLimits{},
			expectAbsent: []string{"CPUQuota=", "MemoryMax=", "MemoryHigh="},
		},
		{
			name:           "memory only",
			limits:         &config.ResourceLimits{MemoryMax: "512M"},
			expectContains: []string{"MemoryAccounting=true", "MemoryMax=512M"},
			expectAbsent:   []string{"CPUQuota=", "MemoryHigh="},
		},
		{
			name: "full throttle",
			limits: &config.ResourceLimits{
				CPUQuota:   "50%",
				MemoryMax:  "2G",
				MemoryHigh: "1.6G",
			},
			expectContains: []string{"CPUQuota=50%", "MemoryMax=2G", "MemoryHigh=1.6G", "CPUAccounting=true"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := renderResourceLimits(tt.limits)
			for _, want := range tt.expectContains {
				if !strings.Contains(out, want) {
					t.Errorf("expected %q in rendered block, got:\n%s", want, out)
				}
			}
			for _, absent := range tt.expectAbsent {
				if strings.Contains(out, absent) {
					t.Errorf("unexpected %q leaked into rendered block, got:\n%s", absent, out)
				}
			}
		})
	}
}

func TestResourceLimitsValidate(t *testing.T) {
	valid := []*config.ResourceLimits{
		nil,
		{},
		{CPUQuota: "80%"},
		{CPUQuota: "150%"},
		{MemoryMax: "1G", MemoryHigh: "800M"},
		{MemoryMax: "512"},
	}
	for _, v := range valid {
		if err := v.Validate(); err != nil {
			t.Errorf("expected %+v to be valid, got %v", v, err)
		}
	}

	// Anything outside the grammar — especially injection of extra directives —
	// must be rejected before it can reach the unit file.
	invalid := []*config.ResourceLimits{
		{CPUQuota: "80"}, // missing %
		{CPUQuota: "0%"}, // zero quota
		{CPUQuota: "80%\nExecStartPre=/bin/rm -rf /"}, // directive injection
		{MemoryMax: "1 Gigabyte"},                     // bad unit
		{MemoryHigh: "lots"},                          // not a size
		{MemoryMax: "1G\nUser=root"},                  // directive injection
	}
	for _, v := range invalid {
		if err := v.Validate(); err == nil {
			t.Errorf("expected %+v to be rejected, but it passed validation", v)
		}
	}
}
