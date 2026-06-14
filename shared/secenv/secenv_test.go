package secenv

import (
	"strings"
	"testing"

	"github.com/aynaash/nextdeploy/shared/sensitive"
)

func TestClassify_PublicVsSecret(t *testing.T) {
	env := map[string]string{
		"NEXT_PUBLIC_URL": "https://x",
		"STRIPE_SECRET":   "sk_live_123",
		"NODE_ENV":        "production",
		"ANALYTICS_ID":    "abc",
	}
	vars, secrets := Classify(env, []string{"ANALYTICS_ID"})

	// NEXT_PUBLIC_* and explicit publicKeys are vars.
	if _, ok := vars["NEXT_PUBLIC_URL"]; !ok {
		t.Error("NEXT_PUBLIC_URL should be a var")
	}
	if _, ok := vars["ANALYTICS_ID"]; !ok {
		t.Error("explicitly-public ANALYTICS_ID should be a var")
	}
	// Unknown keys default to secret (secure default).
	if _, ok := secrets["NODE_ENV"]; !ok {
		t.Error("unknown key NODE_ENV should default to secret")
	}
	if _, ok := secrets["STRIPE_SECRET"]; !ok {
		t.Error("STRIPE_SECRET should be a secret")
	}
	// No overlap.
	for k := range vars {
		if _, dup := secrets[k]; dup {
			t.Errorf("key %s appears in both vars and secrets", k)
		}
	}
}

func TestClassify_NeverNil(t *testing.T) {
	vars, secrets := Classify(nil, nil)
	if vars == nil || secrets == nil {
		t.Fatal("returned maps must not be nil")
	}
}

func TestRegisterSecrets_ScrubsValues(t *testing.T) {
	t.Cleanup(sensitive.Clear)
	sensitive.Clear()
	RegisterSecrets(map[string]string{"API_KEY": "super-secret-value-123"})
	out := sensitive.Scrub("token is super-secret-value-123 here")
	if strings.Contains(out, "super-secret-value-123") {
		t.Errorf("secret not scrubbed: %q", out)
	}
}

func TestIsLikelySecret(t *testing.T) {
	secret := []string{"STRIPE_SECRET_KEY", "AUTH_TOKEN", "DB_PASSWORD", "DATABASE_URL", "PRIVATE_KEY", "STRIPE_KEY"}
	for _, k := range secret {
		if !IsLikelySecret(k) {
			t.Errorf("%s should be flagged secret", k)
		}
	}
	notSecret := []string{"NEXT_PUBLIC_TOKEN", "PORT", "NODE_ENV", "REGION"}
	for _, k := range notSecret {
		if IsLikelySecret(k) {
			t.Errorf("%s should not be flagged secret", k)
		}
	}
}

func TestPlaintextSecretWarnings(t *testing.T) {
	warnings := PlaintextSecretWarnings(map[string]string{
		"STRIPE_SECRET_KEY": "sk_live_x",
		"EMPTY_SECRET":      "", // empty → no warning
		"NODE_ENV":          "production",
	})
	if len(warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d: %v", len(warnings), warnings)
	}
	if !strings.Contains(warnings[0], "STRIPE_SECRET_KEY") {
		t.Errorf("warning should name the key: %q", warnings[0])
	}
}

func TestPreflight(t *testing.T) {
	// Clean: gitignored env, no plaintext secrets, using a managed store.
	clean := Preflight(PreflightInput{
		Gitignore: ".env\n", HasEnvFile: true, UsingSecretStore: true,
	})
	if len(clean) != 0 {
		t.Errorf("expected no warnings, got %v", clean)
	}

	// Dirty: uncommitted-safe env, plaintext secret, no managed store.
	dirty := Preflight(PreflightInput{
		Gitignore:  "node_modules\n",
		HasEnvFile: true,
		PlainText:  map[string]string{"STRIPE_SECRET_KEY": "sk_live"},
	})
	joined := strings.Join(dirty, "\n")
	if !strings.Contains(joined, "not gitignored") {
		t.Errorf("expected gitignore warning: %v", dirty)
	}
	if !strings.Contains(joined, "STRIPE_SECRET_KEY") {
		t.Errorf("expected plaintext-secret warning: %v", dirty)
	}
	if !strings.Contains(joined, "Doppler") || !strings.Contains(joined, "Secret Store") {
		t.Errorf("expected managed-store nudge: %v", dirty)
	}
}

func TestPreflight_StoreInUseSkipsNudge(t *testing.T) {
	if w := Preflight(PreflightInput{UsingDoppler: true}); len(w) != 0 {
		t.Errorf("Doppler in use should suppress nudge, got %v", w)
	}
}

func TestGitignoreCovered(t *testing.T) {
	covered := []string{"node_modules\n.env\n", "*.log\n.env*\n", ".env.*\n"}
	for _, g := range covered {
		if !GitignoreCovered(g) {
			t.Errorf(".gitignore should be considered covered:\n%s", g)
		}
	}
	if GitignoreCovered("node_modules\ndist\n") {
		t.Error(".gitignore without env entry should not be covered")
	}
}
