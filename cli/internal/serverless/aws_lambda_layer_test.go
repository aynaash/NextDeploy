package serverless

import (
	"strings"
	"testing"
)

func TestLayerNameAndVersion(t *testing.T) {
	cases := []struct {
		name     string
		arn      string
		wantName string
		wantVer  int64
		wantOK   bool
	}{
		{
			name:     "full arn",
			arn:      "arn:aws:lambda:us-east-1:177933569100:layer:AWS-Parameters-and-Secrets-Lambda-Extension:88",
			wantName: "arn:aws:lambda:us-east-1:177933569100:layer:AWS-Parameters-and-Secrets-Lambda-Extension",
			wantVer:  88,
			wantOK:   true,
		},
		{"missing version", "arn:aws:lambda:us-east-1:177933569100:layer:AWS-Parameters-and-Secrets-Lambda-Extension", "", 0, false},
		{"non-numeric tail", "arn:aws:lambda:us-east-1:x:layer:foo:latest", "", 0, false},
		{"empty", "", "", 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotName, gotVer, gotOK := layerNameAndVersion(tc.arn)
			if gotOK != tc.wantOK || gotName != tc.wantName || gotVer != tc.wantVer {
				t.Fatalf("layerNameAndVersion(%q) = (%q, %d, %v), want (%q, %d, %v)",
					tc.arn, gotName, gotVer, gotOK, tc.wantName, tc.wantVer, tc.wantOK)
			}
		})
	}
}

func TestSecretsExtensionLayerARN(t *testing.T) {
	// SSM-resolved ARN cached on the provider always wins over the fallback map.
	p := &AWSProvider{secretsLayerARN: "arn:aws:lambda:us-east-1:999:layer:Custom:1"}
	if got := p.secretsExtensionLayerARN("us-east-1"); got != p.secretsLayerARN {
		t.Errorf("cached ARN should win, got %q", got)
	}

	// No cache → per-region fallback map.
	p = &AWSProvider{}
	if got := p.secretsExtensionLayerARN("eu-west-1"); got != secretsExtensionLayerFallback["eu-west-1"] {
		t.Errorf("expected fallback for eu-west-1, got %q", got)
	}

	// Unknown region with no cache → "" so callers attach no layer.
	if got := p.secretsExtensionLayerARN("moon-base-1"); got != "" {
		t.Errorf("unknown region should yield empty ARN, got %q", got)
	}
}

// TestSecretsExtensionFallback_UsEast1Account guards against a regression to the
// stale/wrong owner account that produced cross-account AccessDenied on deploy.
func TestSecretsExtensionFallback_UsEast1Account(t *testing.T) {
	arn := secretsExtensionLayerFallback["us-east-1"]
	if !strings.Contains(arn, ":177933569100:") {
		t.Errorf("us-east-1 fallback must use owner account 177933569100, got %q", arn)
	}
	if strings.Contains(arn, ":177933130628:") {
		t.Errorf("us-east-1 fallback still uses the wrong account 177933130628: %q", arn)
	}
}

// TestSecretsExtensionFallback_WellFormed ensures every pinned ARN parses into a
// layer name + numeric version, so the IAM probe never silently skips.
func TestSecretsExtensionFallback_WellFormed(t *testing.T) {
	for region, arn := range secretsExtensionLayerFallback {
		if !strings.HasPrefix(arn, "arn:aws") {
			t.Errorf("%s: ARN missing arn:aws prefix: %q", region, arn)
		}
		if !strings.Contains(arn, ":layer:AWS-Parameters-and-Secrets-Lambda-Extension:") {
			t.Errorf("%s: ARN missing expected layer name: %q", region, arn)
		}
		if _, _, ok := layerNameAndVersion(arn); !ok {
			t.Errorf("%s: ARN does not parse into name+version: %q", region, arn)
		}
	}
}
