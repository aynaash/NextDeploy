package serverless

import (
	"errors"
	"strings"
	"testing"
)

// hintCloudflarePermissions must turn a raw CF auth failure into actionable
// token-scope guidance, while passing non-auth errors through untouched. This
// is what makes a 403 during resource provisioning diagnosable.
func TestHintCloudflarePermissions(t *testing.T) {
	cases := map[string]bool{ // error text -> should add the token-scope hint
		"403 Forbidden": true,
		`{"code":10000,"message":"Authentication error"}`: true,
		"Authentication error":                            true,
		"connection refused":                              false,
		"bucket name is not valid":                        false,
	}
	for in, wantHint := range cases {
		got := hintCloudflarePermissions(errors.New(in))
		if hasHint := strings.Contains(got.Error(), "api-tokens"); hasHint != wantHint {
			t.Errorf("hintCloudflarePermissions(%q): hint=%v, want %v", in, hasHint, wantHint)
		}
		if !strings.Contains(got.Error(), in) {
			t.Errorf("hintCloudflarePermissions(%q) dropped the original message: %q", in, got.Error())
		}
	}
}
