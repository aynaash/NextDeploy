package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aynaash/nextdeploy/shared/credstore"
)

func writeTempConfig(t *testing.T, dir, yml string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, ConfigFile), []byte(yml), 0o600); err != nil {
		t.Fatalf("write %s: %v", ConfigFile, err)
	}
}

func TestResolveServers_ProcessEnvExpansion(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dir := t.TempDir()
	t.Chdir(dir)

	t.Setenv("VPS_HOST", "203.0.113.42")
	t.Setenv("VPS_USER", "deploy")
	t.Setenv("VPS_KEY", "~/.ssh/prod_ed25519")
	t.Setenv("SSH_KEY_PASSPHRASE", "super-secret-pass")

	writeTempConfig(t, dir, `version: "1"
target_type: vps
app:
  name: demo-app
  port: 3000
servers:
  - name: production
    host: ${VPS_HOST}
    username: ${VPS_USER}
    key_path: ${VPS_KEY}
`)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	srv := cfg.Servers[0]
	if srv.Host != "203.0.113.42" {
		t.Errorf("Host = %q, want 203.0.113.42", srv.Host)
	}
	if srv.Username != "deploy" {
		t.Errorf("Username = %q, want deploy", srv.Username)
	}
	if srv.KeyPath != "~/.ssh/prod_ed25519" {
		t.Errorf("KeyPath = %q, want ~/.ssh/prod_ed25519", srv.KeyPath)
	}
	if srv.KeyPassphrase != "super-secret-pass" {
		t.Errorf("KeyPassphrase = %q, want super-secret-pass", srv.KeyPassphrase)
	}
	if err := ValidateServers(cfg.Servers); err != nil {
		t.Fatalf("ValidateServers: %v", err)
	}
}

func TestResolveServers_PrecedenceEnvNextdeployEnvDotenvCredstore(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dir := t.TempDir()
	t.Chdir(dir)

	if err := credstore.Save("vps", map[string]string{
		"host":     "198.51.100.4",
		"username": "cred-user",
	}); err != nil {
		t.Fatalf("credstore.Save: %v", err)
	}

	yml := `version: "1"
target_type: vps
app:
  name: demo-app
  port: 3000
servers:
  - name: production
    host: ${VPS_HOST}
    username: ubuntu
    key_path: ~/.ssh/id_ed25519
`
	writeTempConfig(t, dir, yml)

	// 4. Credstore wins when no env or dotenv exists.
	t.Setenv("VPS_HOST", "")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load (credstore): %v", err)
	}
	if cfg.Servers[0].Host != "198.51.100.4" {
		t.Errorf("credstore fallback Host = %q, want 198.51.100.4", cfg.Servers[0].Host)
	}
	// Literal username in nextdeploy.yml must NOT be overwritten by credstore.
	if cfg.Servers[0].Username != "ubuntu" {
		t.Errorf("literal Username overwritten = %q, want ubuntu", cfg.Servers[0].Username)
	}

	// 3. .env overrides credstore.
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("VPS_HOST=198.51.100.3 # root dotenv\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err = Load()
	if err != nil {
		t.Fatalf("Load (.env): %v", err)
	}
	if cfg.Servers[0].Host != "198.51.100.3" {
		t.Errorf(".env Host = %q, want 198.51.100.3", cfg.Servers[0].Host)
	}

	// 2. .nextdeploy/.env overrides .env.
	if err := os.MkdirAll(filepath.Join(dir, ".nextdeploy"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".nextdeploy", ".env"), []byte("export VPS_HOST=\"198.51.100.2\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err = Load()
	if err != nil {
		t.Fatalf("Load (.nextdeploy/.env): %v", err)
	}
	if cfg.Servers[0].Host != "198.51.100.2" {
		t.Errorf(".nextdeploy/.env Host = %q, want 198.51.100.2", cfg.Servers[0].Host)
	}

	// 1. Process env overrides .nextdeploy/.env, .env, and credstore.
	t.Setenv("VPS_HOST", "198.51.100.1")
	cfg, err = Load()
	if err != nil {
		t.Fatalf("Load (process env): %v", err)
	}
	if cfg.Servers[0].Host != "198.51.100.1" {
		t.Errorf("process env Host = %q, want 198.51.100.1", cfg.Servers[0].Host)
	}
}

func TestResolveServers_OmittedHostAndCredstore(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dir := t.TempDir()
	t.Chdir(dir)

	if err := credstore.Save("vps", map[string]string{
		"host":     "203.0.113.55",
		"username": "deployer",
		"key_path": "~/.ssh/vps_key",
	}); err != nil {
		t.Fatalf("credstore.Save: %v", err)
	}

	writeTempConfig(t, dir, `version: "1"
target_type: vps
app:
  name: demo-app
  port: 3000
servers:
  - name: production
`)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	srv := cfg.Servers[0]
	if srv.Host != "203.0.113.55" || srv.Username != "deployer" || srv.KeyPath != "~/.ssh/vps_key" {
		t.Fatalf("resolved server = %+v, want host=203.0.113.55 username=deployer key_path=~/.ssh/vps_key", srv)
	}
}

func TestResolveServers_MultiServerDefaultsAndLiteralSafety(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dir := t.TempDir()
	t.Chdir(dir)

	// Generic "vps" credstore must not clobber literal hosts or unrelated multi-server slots.
	if err := credstore.Save("vps", map[string]string{"host": "192.0.2.99"}); err != nil {
		t.Fatal(err)
	}
	if err := credstore.Save("vps-staging", map[string]string{"host": "198.51.100.77"}); err != nil {
		t.Fatal(err)
	}
	t.Setenv("VPS_HOST_PRODUCTION", "203.0.113.10")
	t.Setenv("VPS_USER", "")

	writeTempConfig(t, dir, `version: "1"
target_type: vps
app:
  name: demo-app
  port: 3000
servers:
  - name: production
    username: ${VPS_USER:-ubuntu}
    key_path: ~/.ssh/id_ed25519
  - name: staging
    host: ${STAGING_HOST}
    username: ubuntu
    key_path: ~/.ssh/id_ed25519
  - name: literal-box
    host: 203.0.113.200
    username: ubuntu
    key_path: ~/.ssh/id_ed25519
`)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Servers[0].Host != "203.0.113.10" {
		t.Errorf("production Host = %q, want 203.0.113.10", cfg.Servers[0].Host)
	}
	if cfg.Servers[0].Username != "ubuntu" {
		t.Errorf("production Username default = %q, want ubuntu", cfg.Servers[0].Username)
	}
	if cfg.Servers[1].Host != "198.51.100.77" {
		t.Errorf("staging Host = %q, want 198.51.100.77", cfg.Servers[1].Host)
	}
	if cfg.Servers[2].Host != "203.0.113.200" {
		t.Errorf("literal-box Host = %q, want 203.0.113.200", cfg.Servers[2].Host)
	}
}

func TestResolveServers_UnresolvedPlaceholderPreservedThenValidated(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dir := t.TempDir()
	t.Chdir(dir)
	t.Setenv("VPS_HOST", "")

	writeTempConfig(t, dir, `version: "1"
target_type: vps
app:
  name: demo-app
  port: 3000
servers:
  - name: production
    host: ${VPS_HOST}
    username: ubuntu
    key_path: ~/.ssh/id_ed25519
`)

	// Load must succeed so non-SSH commands (target, generate-ci, build) work.
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load should succeed with unresolved placeholder, got: %v", err)
	}
	if cfg.Servers[0].Host != "${VPS_HOST}" {
		t.Fatalf("Host = %q, want preserved ${VPS_HOST}", cfg.Servers[0].Host)
	}

	// ValidateServers must fail with actionable guidance.
	err = ValidateServers(cfg.Servers)
	if err == nil {
		t.Fatal("ValidateServers succeeded, want error for unresolved ${VPS_HOST}")
	}
	msg := err.Error()
	for _, want := range []string{"production", "VPS_HOST", "export VPS_HOST=", "nextdeploy creds set vps"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error %q missing %q", msg, want)
		}
	}

	// Empty host must also fail with actionable guidance.
	errEmpty := ValidateServerConfig(ServerConfig{Name: "production", Host: ""})
	if errEmpty == nil {
		t.Fatal("ValidateServerConfig succeeded for empty host, want error")
	}
	if !strings.Contains(errEmpty.Error(), "nextdeploy creds set vps") {
		t.Errorf("empty host error %q missing credstore hint", errEmpty.Error())
	}
}
