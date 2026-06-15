package serverless

import (
	"strings"
	"testing"
)

// sanitizeCFName must produce names R2 and the Workers API accept: lowercase,
// [a-z0-9-] only, no leading/trailing/repeated hyphens, ≤63 chars. The first
// case is the real failure — app "NextDeploy" yielded the invalid bucket
// "nextdeploy-NextDeploy-production-assets" (Cloudflare error 10005).
func TestSanitizeCFName(t *testing.T) {
	cases := map[string]string{
		"nextdeploy-NextDeploy-production-assets": "nextdeploy-nextdeploy-production-assets",
		"NextDeploy-production":                   "nextdeploy-production",
		"pesastream-assets":                       "pesastream-assets", // already valid → unchanged
		"My App!":                                 "my-app",
		"a__b  c":                                 "a-b-c", // collapse runs of invalid chars
		"--leading-and-trailing--":                "leading-and-trailing",
		"UPPER_Snake_Case":                        "upper-snake-case",
	}
	for in, want := range cases {
		if got := sanitizeCFName(in); got != want {
			t.Errorf("sanitizeCFName(%q) = %q, want %q", in, got, want)
		}
	}
}

// The output must never exceed R2's 63-char limit and must not end on a hyphen
// after truncation.
func TestSanitizeCFName_LengthCap(t *testing.T) {
	got := sanitizeCFName(strings.Repeat("a", 80))
	if len(got) != 63 {
		t.Errorf("len = %d, want 63", len(got))
	}
	if strings.HasPrefix(got, "-") || strings.HasSuffix(got, "-") {
		t.Errorf("result has a boundary hyphen: %q", got)
	}
}
