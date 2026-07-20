package serverless

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aynaash/nextdeploy/shared/config"
)

func TestSecretsDeclared(t *testing.T){
	dir := t.TempDir()
	chdir(t,dir)
	cases := []struct {
		name string 
		cfg *config.NextDeployConfig
		want bool 
	}{
		{"nil",nil, false},
		{"empty"}
	}
}
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

func TestLoadLocalSecrets_Precedence(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	writeFile(t, ".env", "FOO=from_dotenv\nSHARED=dotenv_loses\nDOTENV_ONLY=1\n")

	yamlFile := filepath.Join(dir, "secrets.env")
	writeFile(t, yamlFile, "BAR=from_yaml\nSHARED=yaml_loses\nYAML_ONLY=1\n")

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
