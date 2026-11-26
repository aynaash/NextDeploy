package cloudflare

import (
	"strings"
	"testing"

	"github.com/aynaash/nextdeploy/internal/packaging"
	"github.com/cloudflare/cloudflare-go/v6/workers"
)

// P2's dnsRecordFQDN regression case lives in cloudflare_dns_test.go's
// TestDNSRecordFQDN (the "api.staging" row).

// P3 — only CNAME is single-valued.
func TestIsSingleValued(t *testing.T) {
	if !isSingleValued("CNAME") {
		t.Error("CNAME should be single-valued")
	}
	for _, rt := range []string{"A", "AAAA", "TXT", "MX"} {
		if isSingleValued(rt) {
			t.Errorf("%s should be multi-valued", rt)
		}
	}
}

// P1 — sqlQuote produces an escaped single-quoted literal.
func TestSQLQuote(t *testing.T) {
	if got := sqlQuote("0003_add_users.sql"); got != "'0003_add_users.sql'" {
		t.Errorf("sqlQuote = %q", got)
	}
	if got := sqlQuote("o'brien.sql"); got != "'o''brien.sql'" {
		t.Errorf("sqlQuote escape = %q", got)
	}
}

// S1 — refuseSecretWipe aborts when the incoming set drops a live secret.
func TestRefuseSecretWipe(t *testing.T) {
	live := map[string]string{"DATABASE_URL": "[secret]", "AUTH_SECRET": "[secret]"}

	// Empty incoming set with live secrets → refuse, naming the dropped ones.
	err := refuseSecretWipe(map[string]string{}, live, false)
	if err == nil || !strings.Contains(err.Error(), "DATABASE_URL") {
		t.Fatalf("expected wipe refusal naming DATABASE_URL, got %v", err)
	}

	// Superset (keeps all live names) → allowed.
	if err := refuseSecretWipe(map[string]string{"DATABASE_URL": "x", "AUTH_SECRET": "y", "NEW": "z"}, live, false); err != nil {
		t.Errorf("superset must be allowed, got %v", err)
	}

	// No live secrets → no-op.
	if err := refuseSecretWipe(map[string]string{}, nil, false); err != nil {
		t.Errorf("no live secrets must be a no-op, got %v", err)
	}
}

// TestRefuseSecretWipe_Local covers guardrail 2.
func TestRefuseSecretWipe_Local(t *testing.T) {
	live3 := map[string]string{"A": "[secret]", "B": "[secret]", "C": "[secret]"}

	cases := []struct {
		name    string
		pending map[string]string
		live    map[string]string
		allow   bool
		wantErr bool
	}{
		{"empty pending, 3 live, no override", nil, live3, false, true},
		{"empty pending, 3 live, override", nil, live3, true, false},
		{"pending covers all live", live3, live3, false, false},
		{"pending drops one live", map[string]string{"A": "x", "B": "y"}, live3, false, true},
		{"pending superset", map[string]string{"A": "x", "B": "y", "C": "z", "D": "w"}, live3, false, false},
		{"no live secrets", nil, nil, false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := refuseSecretWipe(tc.pending, tc.live, tc.allow)
			if (err != nil) != tc.wantErr {
				t.Fatalf("refuseSecretWipe err=%v, wantErr=%v", err, tc.wantErr)
			}
		})
	}

	// The error must name the dropped secrets, sorted, so the operator knows
	// exactly what's at risk.
	err := refuseSecretWipe(nil, live3, false)
	if err == nil || !strings.Contains(err.Error(), "A, B, C") {
		t.Fatalf("expected error naming A, B, C; got %v", err)
	}
}

// D2 — pickActiveVersion selects the highest-percentage version.
func TestPickActiveVersion(t *testing.T) {
	versions := []workers.DeploymentVersion{
		{VersionID: "v-canary", Percentage: 10},
		{VersionID: "v-active", Percentage: 90},
	}
	if got := pickActiveVersion(versions); got != "v-active" {
		t.Errorf("pickActiveVersion = %q, want v-active", got)
	}
	// Single version → that one.
	if got := pickActiveVersion([]workers.DeploymentVersion{{VersionID: "only", Percentage: 100}}); got != "only" {
		t.Errorf("pickActiveVersion single = %q, want only", got)
	}
}

// D1 — partitionAssets separates immutable hashed chunks from mutable assets.
func TestPartitionAssets(t *testing.T) {
	assets := []packaging.S3Asset{
		{S3Key: "index.html"},
		{S3Key: "_next/static/chunks/main-abc123.js"},
		{S3Key: "about.rsc"},
		{S3Key: "_next/static/css/app-def456.css"},
		{S3Key: "favicon.ico"},
	}
	immutable, mutable := partitionAssets(assets)
	if len(immutable) != 2 {
		t.Fatalf("expected 2 immutable, got %d: %+v", len(immutable), immutable)
	}
	for _, a := range immutable {
		if !strings.HasPrefix(a.S3Key, "_next/static/") {
			t.Errorf("immutable partition has non-hashed key %q", a.S3Key)
		}
	}
	if len(mutable) != 3 {
		t.Fatalf("expected 3 mutable, got %d: %+v", len(mutable), mutable)
	}
	for _, a := range mutable {
		if strings.HasPrefix(a.S3Key, "_next/static/") {
			t.Errorf("mutable partition leaked a hashed chunk %q (reintroduces the race)", a.S3Key)
		}
	}
}

// P6 — buildScriptMetadata rejects a secret that collides with the auto ASSETS binding.
func TestBuildScriptMetadata_RejectsDuplicateBindingName(t *testing.T) {
	secrets := map[string]string{defaultR2AssetsBindingName: "shadow"}
	_, err := buildScriptMetadata(nil, "my-bucket", "worker.mjs", noResolver, secrets)
	if err == nil || !strings.Contains(err.Error(), defaultR2AssetsBindingName) {
		t.Fatalf("expected duplicate-name error naming %q, got %v", defaultR2AssetsBindingName, err)
	}
}
