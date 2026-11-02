package secrets

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/aynaash/nextdeploy/shared/config"
)

func TestSecretManager_SetGetRoundTrip(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("HOME", tempDir)

	cfg := &config.NextDeployConfig{
		App: config.AppConfig{
			Name: "testapp",
		},
	}

	sm, err := NewSecretManager(WithConfig(cfg))
	if err != nil {
		t.Fatalf("NewSecretManager: %v", err)
	}

	const secretKey = "DATABASE_URL"
	const secretVal = "postgres://user:pass@localhost:5432/testdb"

	// 1. Set secret with encryption
	if err := sm.SetSecret(secretKey, secretVal, true); err != nil {
		t.Fatalf("SetSecret failed: %v", err)
	}

	// 2. Get secret back
	val, err := sm.GetSecret(secretKey)
	if err != nil {
		t.Fatalf("GetSecret failed: %v", err)
	}
	if val != secretVal {
		t.Fatalf("GetSecret returned %q, want %q", val, secretVal)
	}

	// 3. Flatten secrets
	flat := sm.FlattenSecrets()
	if flat[secretKey] != secretVal {
		t.Fatalf("FlattenSecrets returned %q for %s, want %q", flat[secretKey], secretKey, secretVal)
	}

	// 4. Rotate secrets
	if err := sm.RotateSecrets(); err != nil {
		t.Fatalf("RotateSecrets failed: %v", err)
	}
	valAfterRotate, err := sm.GetSecret(secretKey)
	if err != nil {
		t.Fatalf("GetSecret after rotate failed: %v", err)
	}
	if valAfterRotate != secretVal {
		t.Fatalf("GetSecret after rotate returned %q, want %q", valAfterRotate, secretVal)
	}
}

func TestSecretManager_GetKeyNilConfigSafety(t *testing.T) {
	sm := &SecretManager{
		secrets:  make(map[string]*Secret),
		keyCache: make(map[string][]byte),
		cfg:      nil, // nil config must not panic
	}

	key := sm.GetKey()
	if key != "nokey" {
		t.Fatalf("GetKey with nil cfg: got %q, want 'nokey'", key)
	}
}

func TestSecretManager_KeyFilePermissions(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("HOME", tempDir)

	cfg := &config.NextDeployConfig{
		App: config.AppConfig{
			Name: "permtest",
		},
	}

	sm, err := NewSecretManager(WithConfig(cfg))
	if err != nil {
		t.Fatalf("NewSecretManager: %v", err)
	}

	if err := sm.SetSecret("API_KEY", "super-secret-key-1234", true); err != nil {
		t.Fatalf("SetSecret: %v", err)
	}

	keyPath := filepath.Join(tempDir, ".nextdeploy", "permtest", "master.key")
	info, err := os.Stat(keyPath)
	if err != nil {
		t.Fatalf("Stat master key: %v", err)
	}

	perm := info.Mode().Perm()
	if perm != 0600 {
		t.Fatalf("master.key has permissions %o, want 0600", perm)
	}
}
