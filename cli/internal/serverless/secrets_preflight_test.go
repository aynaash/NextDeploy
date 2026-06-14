package serverless

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aynaash/nextdeploy/shared/config"
)

func TestSecretsPreflightDir_FlagsUncommittedEnvAndPlaintextSecret(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("X=1"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("node_modules\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := &config.NextDeployConfig{
		Serverless: &config.ServerlessConfig{
			Cloudflare: &config.CloudflareConfig{
				Bindings: &config.CFBindings{
					PlainText: []config.CFPlainTextBinding{{Name: "DB_PASSWORD", Value: "hunter2"}},
				},
			},
		},
	}
	warnings := secretsPreflightDir(cfg, dir)
	joined := strings.Join(warnings, "\n")
	if !strings.Contains(joined, "not gitignored") {
		t.Errorf("expected uncommitted-env warning: %v", warnings)
	}
	if !strings.Contains(joined, "DB_PASSWORD") {
		t.Errorf("expected plaintext-secret warning for DB_PASSWORD: %v", warnings)
	}
}

func TestSecretsPreflightDir_CleanWithSecretStore(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(".env*\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := &config.NextDeployConfig{
		Serverless: &config.ServerlessConfig{
			Cloudflare: &config.CloudflareConfig{
				Bindings: &config.CFBindings{
					SecretsStore: []config.CFSecretStoreBinding{{Name: "K", StoreID: "s", SecretName: "n"}},
				},
			},
		},
	}
	if w := secretsPreflightDir(cfg, dir); len(w) != 0 {
		t.Errorf("expected no warnings (gitignored, secret store in use), got %v", w)
	}
}

func TestHasSecretsStoreBinding(t *testing.T) {
	if hasSecretsStoreBinding(nil) {
		t.Error("nil config should report no secret store")
	}
	cfg := &config.NextDeployConfig{
		Serverless: &config.ServerlessConfig{
			Cloudflare: &config.CloudflareConfig{
				Bindings: &config.CFBindings{SecretsStore: []config.CFSecretStoreBinding{{Name: "X"}}},
			},
		},
	}
	if !hasSecretsStoreBinding(cfg) {
		t.Error("should detect declared secret store binding")
	}
}
