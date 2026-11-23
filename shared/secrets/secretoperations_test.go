package secrets

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
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

func TestValidateSecretName(t *testing.T) {
	tests := []struct {
		name    string
		key     string
		wantErr error
	}{
		{name: "valid uppercase", key: "DATABASE_URL"},
		{name: "valid api key", key: "API_KEY"},
		{name: "valid underscore prefix", key: "_CUSTOM_1"},
		{name: "empty", key: "", wantErr: ErrInvalidSecretName},
		{name: "too long", key: strings.Repeat("A", 257), wantErr: ErrInvalidSecretName},
		{name: "starts with digit", key: "123ABC", wantErr: ErrInvalidSecretName},
		{name: "contains hyphen", key: "BAD-KEY", wantErr: ErrInvalidSecretName},
		{name: "contains dot", key: "BAD.KEY", wantErr: ErrInvalidSecretName},
		{name: "contains equals", key: "KEY=VAL", wantErr: ErrInvalidSecretName},
		{name: "contains space", key: "A B", wantErr: ErrInvalidSecretName},
		{name: "reserved PATH", key: "PATH", wantErr: ErrReservedSecretName},
		{name: "reserved HOME lowercase", key: "home", wantErr: ErrReservedSecretName},
		{name: "reserved LD_PRELOAD", key: "LD_PRELOAD", wantErr: ErrReservedSecretName},
		{name: "reserved DYLD_INSERT_LIBRARIES", key: "DYLD_INSERT_LIBRARIES", wantErr: ErrReservedSecretName},
		{name: "reserved PORT", key: "PORT", wantErr: ErrReservedSecretName},
		{name: "reserved ND_ prefix", key: "ND_SECRET_NAME", wantErr: ErrReservedSecretName},
		{name: "reserved NEXTDEPLOY_ prefix", key: "NEXTDEPLOY_ENV", wantErr: ErrReservedSecretName},
		{name: "reserved DOPPLER_ prefix", key: "DOPPLER_TOKEN", wantErr: ErrReservedSecretName},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSecretName(tt.key)
			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("ValidateSecretName(%q): unexpected error: %v", tt.key, err)
				}
				return
			}
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("ValidateSecretName(%q): got %v, want %v", tt.key, err, tt.wantErr)
			}
		})
	}

	t.Run("SetSecret rejects invalid and reserved keys", func(t *testing.T) {
		sm, err := NewSecretManager(WithConfig(&config.NextDeployConfig{
			App: config.AppConfig{Name: "nametest"},
		}))
		if err != nil {
			t.Fatalf("NewSecretManager: %v", err)
		}
		if err := sm.SetSecret("BAD-KEY", "val", false); !errors.Is(err, ErrInvalidSecretName) {
			t.Fatalf("SetSecret(BAD-KEY): got %v, want ErrInvalidSecretName", err)
		}
		if err := sm.SetSecret("PATH", "val", false); !errors.Is(err, ErrReservedSecretName) {
			t.Fatalf("SetSecret(PATH): got %v, want ErrReservedSecretName", err)
		}
	})
}

func assertEncryptionMetadataFields(t *testing.T, sec *Secret) {
	t.Helper()
	if sec.Alg != CurrentEncryptionAlg {
		t.Fatalf("Alg = %q, want %q", sec.Alg, CurrentEncryptionAlg)
	}
	saltBytes, err := base64.StdEncoding.DecodeString(sec.Salt)
	if err != nil || len(saltBytes) != 16 {
		t.Fatalf("Salt invalid: len=%d err=%v", len(saltBytes), err)
	}
	ivBytes, err := base64.StdEncoding.DecodeString(sec.IV)
	if err != nil || len(ivBytes) != 12 {
		t.Fatalf("IV invalid: len=%d err=%v", len(ivBytes), err)
	}
}

func TestSecretManager_EncryptionMetadata(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("HOME", tempDir)

	cfg := &config.NextDeployConfig{
		App: config.AppConfig{Name: "metaapp"},
	}
	sm, err := NewSecretManager(WithConfig(cfg))
	if err != nil {
		t.Fatalf("NewSecretManager: %v", err)
	}

	const key = "JWT_SECRET"
	const val = "super-secret-jwt-value"
	if err := sm.SetSecret(key, val, true); err != nil {
		t.Fatalf("SetSecret: %v", err)
	}

	sec := sm.secrets[key]
	assertEncryptionMetadataFields(t, sec)

	exportPath := filepath.Join(tempDir, "secrets.json")
	if err := sm.ExportSecrets(exportPath); err != nil {
		t.Fatalf("ExportSecrets: %v", err)
	}
	rawJSON, err := os.ReadFile(exportPath)
	if err != nil {
		t.Fatalf("ReadFile export: %v", err)
	}
	var exported map[string]Secret
	if err := json.Unmarshal(rawJSON, &exported); err != nil {
		t.Fatalf("Unmarshal export: %v", err)
	}
	if exported[key].Alg != CurrentEncryptionAlg || exported[key].Salt != sec.Salt || exported[key].IV != sec.IV {
		t.Fatalf("exported metadata mismatch: %+v", exported[key])
	}

	sm2, err := NewSecretManager(WithConfig(cfg))
	if err != nil {
		t.Fatalf("NewSecretManager 2: %v", err)
	}
	if err := sm2.ImportSecrets(exportPath); err != nil {
		t.Fatalf("ImportSecrets: %v", err)
	}
	if got, err := sm2.GetSecret(key); err != nil || got != val {
		t.Fatalf("GetSecret after import: got=%q err=%v", got, err)
	}
}

func TestSecretManager_EncryptionMetadataRotateAndTamper(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("HOME", tempDir)

	cfg := &config.NextDeployConfig{
		App: config.AppConfig{Name: "metaapp-rotate"},
	}
	sm, err := NewSecretManager(WithConfig(cfg))
	if err != nil {
		t.Fatalf("NewSecretManager: %v", err)
	}

	const key = "JWT_SECRET"
	const val = "super-secret-jwt-value"
	if err := sm.SetSecret(key, val, true); err != nil {
		t.Fatalf("SetSecret: %v", err)
	}

	sec := sm.secrets[key]
	oldSalt, oldIV := sec.Salt, sec.IV
	if err := sm.RotateSecrets(); err != nil {
		t.Fatalf("RotateSecrets: %v", err)
	}
	if sec.Version != 2 || sec.Salt == oldSalt || sec.IV == oldIV || sec.Alg != CurrentEncryptionAlg {
		t.Fatalf("RotateSecrets did not update metadata/version: %+v", sec)
	}
	if got, err := sm.GetSecret(key); err != nil || got != val {
		t.Fatalf("GetSecret after rotate: got=%q err=%v", got, err)
	}

	savedIV := sec.IV
	sec.IV = base64.StdEncoding.EncodeToString(make([]byte, 12))
	if _, err := sm.GetSecret(key); !errors.Is(err, ErrDecryptionFailed) {
		t.Fatalf("expected ErrDecryptionFailed for tampered IV, got %v", err)
	}
	sec.IV = savedIV
	sec.Salt = base64.StdEncoding.EncodeToString(make([]byte, 16))
	if _, err := sm.GetSecret(key); !errors.Is(err, ErrDecryptionFailed) {
		t.Fatalf("expected ErrDecryptionFailed for tampered Salt, got %v", err)
	}
}

func TestNewSecretManager_LoadsEnvWithoutConfig(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	envContent := "DATABASE_URL=postgres://localhost/db\nAPI_KEY=secret123456\nPATH=/usr/bin\n"
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte(envContent), 0o600); err != nil {
		t.Fatalf("WriteFile .env: %v", err)
	}

	sm, err := NewSecretManager()
	if err != nil {
		t.Fatalf("NewSecretManager without config: %v", err)
	}

	gotDB, err := sm.GetSecret("DATABASE_URL")
	if err != nil || gotDB != "postgres://localhost/db" {
		t.Fatalf("GetSecret(DATABASE_URL) = %q, %v", gotDB, err)
	}

	flat := sm.FlattenSecrets()
	if flat["API_KEY"] != "secret123456" {
		t.Fatalf("FlattenSecrets[API_KEY] = %q, want secret123456", flat["API_KEY"])
	}
	if _, exists := flat["PATH"]; exists {
		t.Fatal("reserved key PATH should not be loaded from .env")
	}
}

func TestNewSecretManager_WithEnvFile(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, "custom.env")
	if err := os.WriteFile(envPath, []byte("STRIPE_KEY=sk_test_123456\n"), 0o600); err != nil {
		t.Fatalf("WriteFile custom.env: %v", err)
	}

	cfg := &config.NextDeployConfig{App: config.AppConfig{Name: "envfiletest"}}
	sm, err := NewSecretManager(WithConfig(cfg), WithEnvFile(envPath))
	if err != nil {
		t.Fatalf("NewSecretManager(WithEnvFile): %v", err)
	}

	got, err := sm.GetSecret("STRIPE_KEY")
	if err != nil || got != "sk_test_123456" {
		t.Fatalf("GetSecret(STRIPE_KEY) = %q, %v", got, err)
	}

	if _, err := NewSecretManager(WithConfig(cfg), WithEnvFile(filepath.Join(dir, "missing.env"))); err == nil {
		t.Fatal("expected error for missing explicit env file")
	}
}
