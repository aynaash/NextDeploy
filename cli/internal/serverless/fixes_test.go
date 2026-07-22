package serverless

import (
	"strings"
	"testing"

	"github.com/aynaash/nextdeploy/internal/packaging"
	"github.com/aynaash/nextdeploy/shared/config"
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

// S1 — secretsDeclared is true only when a secret source is configured.
func TestSecretsDeclared(t *testing.T) {
	if secretsDeclared(nil) {
		t.Error("nil config declares no secrets")
	}
	if secretsDeclared(&config.NextDeployConfig{}) {
		t.Error("empty config declares no secrets")
	}
	withFiles := &config.NextDeployConfig{Secrets: config.SecretsConfig{Files: []config.SecretFile{{}}}}
	if !secretsDeclared(withFiles) {
		t.Error("Secrets.Files should count as declared")
	}
	withDoppler := &config.NextDeployConfig{Secrets: config.SecretsConfig{Doppler: &config.DopplerConfig{}}}
	if !secretsDeclared(withDoppler) {
		t.Error("Secrets.Doppler should count as declared")
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
