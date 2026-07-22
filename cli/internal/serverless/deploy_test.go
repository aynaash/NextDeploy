package serverless

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aynaash/nextdeploy/shared/config"
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

// TestLoadLocalSecrets_Precedence verifies the merge order:
//
//	.env  <  cfg.Secrets.Files  <  managed JSON store
//
// This is the regression net for the YAML-merge fix in deploy.go.
func TestLoadLocalSecrets_Precedence(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	// 1. Project-root .env (lowest precedence)
	writeFile(t, ".env", "FOO=from_dotenv\nSHARED=dotenv_loses\nDOTENV_ONLY=1\n")

	// 2. YAML-declared file (middle precedence)
	yamlFile := filepath.Join(dir, "secrets.env")
	writeFile(t, yamlFile, "BAR=from_yaml\nSHARED=yaml_loses\nYAML_ONLY=1\n")

	// 3. Managed JSON store (highest precedence)
	if err := os.MkdirAll(".nextdeploy", 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, ".nextdeploy/.env", `{
  "BAZ": {"value": "from_managed", "is_encrypted": false},
  "SHARED": {"value": "managed_wins", "is_encrypted": false},
  "MANAGED_ONLY": {"value": "1", "is_encrypted": false}
}`)

	cfg := &config.NextDeployConfig{
		Secrets: config.SecretsConfig{
			Files: []config.SecretFile{{Path: yamlFile}},
		},
	}

	got, err := loadLocalSecrets(cfg)
	if err != nil {
		t.Fatalf("loadLocalSecrets: %v", err)
	}

	// Precedence checks
	want := map[string]string{
		"FOO":          "from_dotenv",
		"BAR":          "from_yaml",
		"BAZ":          "from_managed",
		"SHARED":       "managed_wins",
		"DOTENV_ONLY":  "1",
		"YAML_ONLY":    "1",
		"MANAGED_ONLY": "1",
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("key %q: got %q, want %q", k, got[k], v)
		}
	}
}

// TestLoadLocalSecrets_NoSources returns an empty map without error.
func TestLoadLocalSecrets_NoSources(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	got, err := loadLocalSecrets(&config.NextDeployConfig{})
	if err != nil {
		t.Fatalf("loadLocalSecrets: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty map, got %d entries", len(got))
	}
}

// TestLoadLocalSecrets_MissingYamlFile fails loudly (no silent skip).
func TestLoadLocalSecrets_MissingYamlFile(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	cfg := &config.NextDeployConfig{
		Secrets: config.SecretsConfig{
			Files: []config.SecretFile{{Path: "does/not/exist.env"}},
		},
	}
	_, err := loadLocalSecrets(cfg)
	if err == nil {
		t.Fatal("expected error for missing YAML-declared file, got nil")
	}
}

func TestSecretsEqual(t *testing.T) {
	cases := []struct {
		name string
		a, b map[string]string
		want bool
	}{
		{"both nil", nil, nil, true},
		{"empty equal", map[string]string{}, map[string]string{}, true},
		{"same content", map[string]string{"k": "v"}, map[string]string{"k": "v"}, true},
		{"len differs", map[string]string{"k": "v"}, map[string]string{"k": "v", "x": "y"}, false},
		{"value differs", map[string]string{"k": "v"}, map[string]string{"k": "w"}, false},
		{"key missing", map[string]string{"k": "v"}, map[string]string{"x": "v"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := secretsEqual(tc.a, tc.b); got != tc.want {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}
// chdir switches CWD for the duration of a test, restoring on cleanup.
func chdir(t *testing.T, dir string) {
	t.Helper()
	prev, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(prev) })
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

// TestPushSecrets_FailsClosedOnLoadError proves the S1 guardrail-1 condition:
// a declared secret source that fails to load must be a fatal combination, not
// a silent empty-set substitution.
func TestPushSecrets_FailsClosedOnLoadError(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	cfg := &config.NextDeployConfig{
		Secrets: config.SecretsConfig{
			Files: []config.SecretFile{{Path: "does/not/exist.env"}},
		},
	}
	if !secretsDeclared(cfg) {
		t.Fatal("secretsDeclared should be true when Secrets.Files is set")
	}
	if _, err := loadLocalSecrets(cfg); err == nil {
		t.Fatal("loadLocalSecrets should error on an unreadable declared file")
	}
	// declared==true AND load error ⇒ pushSecrets returns the fatal error.
}

// TestRefuseSecretWipe covers guardrail 2.
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