package secrets

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/aynaash/nextdeploy/shared/config"
)

// fakeProvider is a no-op SecretProvider used to prove WithProvider registers
// under the requested name. Behaviour of the methods is irrelevant here; only
// identity/registration is under test.
type fakeProvider struct{ id string }

func (fakeProvider) GetSecret(string) (string, error)       { return "", nil }
func (fakeProvider) SetSecret(string, string) error         { return nil }
func (fakeProvider) DeleteSecret(string) error              { return nil }
func (fakeProvider) ListSecrets() ([]string, error)         { return nil, nil }
func (fakeProvider) Encrypt([]byte, string) ([]byte, error) { return nil, nil }
func (fakeProvider) Decrypt([]byte, string) ([]byte, error) { return nil, nil }
func (fakeProvider) GenerateMasterKey() (string, error)     { return "", nil }
func (fakeProvider) DeriveKey(string) ([]byte, error)       { return nil, nil }
func (fakeProvider) ValidateSecretFormat(string) error      { return nil }

// cfgWithName builds a minimal config carrying just the app name, which is all
// WithKeyPath consults.
func cfgWithName(name string) *config.NextDeployConfig {
	c := &config.NextDeployConfig{}
	c.App.Name = name
	return c
}

func TestNewSecretManager_WithConfig(t *testing.T) {
	cfg := cfgWithName("acme")
	sm, err := NewSecretManager(WithConfig(cfg))
	if err != nil {
		t.Fatalf("NewSecretManager: unexpected error %v", err)
	}
	if sm.cfg != cfg {
		t.Fatal("WithConfig did not set the config on the manager")
	}
	// Maps must be initialised so downstream operations don't nil-panic.
	if sm.secrets == nil || sm.keyCache == nil {
		t.Fatal("NewSecretManager left secrets/keyCache maps nil")
	}
}

func TestNewSecretManager_ConfigFallbackError(t *testing.T) {
	// No config passed and no nextdeploy.yml on disk → Load fails and the
	// constructor must surface ErrConfigNotFound (fail-closed).
	t.Chdir(t.TempDir())
	if _, err := NewSecretManager(); !errors.Is(err, ErrConfigNotFound) {
		t.Fatalf("NewSecretManager() with no config: want ErrConfigNotFound, got %v", err)
	}
}

func TestNewSecretManager_ConfigFallbackFromDisk(t *testing.T) {
	// With a nextdeploy.yml present, the nil-cfg branch loads it and succeeds.
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "nextdeploy.yml"), []byte("app:\n  name: fromdisk\n"), 0600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	t.Chdir(dir)

	sm, err := NewSecretManager()
	if err != nil {
		t.Fatalf("NewSecretManager() with on-disk config: unexpected error %v", err)
	}
	if sm.cfg == nil || sm.cfg.App.Name != "fromdisk" {
		t.Fatalf("loaded config not wired onto manager: %+v", sm.cfg)
	}
}

func TestWithKeyPath_Explicit(t *testing.T) {
	sm, err := NewSecretManager(WithConfig(cfgWithName("acme")), WithKeyPath("/etc/keys/master"))
	if err != nil {
		t.Fatalf("NewSecretManager: %v", err)
	}
	if sm.keyPath != "/etc/keys/master" {
		t.Fatalf("explicit keyPath = %q, want /etc/keys/master", sm.keyPath)
	}
}

func TestWithKeyPath_DerivedUsesAppName(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	// Config is applied before WithKeyPath so the app name is available when the
	// path is derived. Order of options is significant here.
	sm, err := NewSecretManager(WithConfig(cfgWithName("myapp")), WithKeyPath(""))
	if err != nil {
		t.Fatalf("NewSecretManager: %v", err)
	}
	want := filepath.Join(home, ".nextdeploy", "myapp")
	if sm.keyPath != want {
		t.Fatalf("derived keyPath = %q, want %q", sm.keyPath, want)
	}
}

func TestWithKeyPath_DerivedDefaultsWhenNoName(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	// WithKeyPath("") runs before any config is set, so cfg is nil and the app
	// name falls back to "default". A nextdeploy.yml is on disk so the nil-cfg
	// fallback in NewSecretManager still succeeds.
	if err := os.WriteFile(filepath.Join(home, "nextdeploy.yml"), []byte("app:\n  name: ignored\n"), 0600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	t.Chdir(home)

	sm, err := NewSecretManager(WithKeyPath(""))
	if err != nil {
		t.Fatalf("NewSecretManager: %v", err)
	}
	want := filepath.Join(home, ".nextdeploy", "default")
	if sm.keyPath != want {
		t.Fatalf("derived keyPath = %q, want %q", sm.keyPath, want)
	}
}

func TestWithKeyPath_DerivedHomeDirError(t *testing.T) {
	// UserHomeDir fails when HOME is empty on unix; the option logs and leaves
	// keyPath untouched rather than panicking.
	t.Setenv("HOME", "")
	sm, err := NewSecretManager(WithConfig(cfgWithName("acme")), WithKeyPath(""))
	if err != nil {
		t.Fatalf("NewSecretManager: %v", err)
	}
	if sm.keyPath != "" {
		t.Fatalf("keyPath = %q, want empty after home-dir lookup failure", sm.keyPath)
	}
}

func TestWithProvider_RegistersAndInitialises(t *testing.T) {
	p1 := fakeProvider{id: "one"}
	p2 := fakeProvider{id: "two"}
	sm, err := NewSecretManager(
		WithConfig(cfgWithName("acme")),
		WithProvider("aws", p1),
		WithProvider("doppler", p2),
	)
	if err != nil {
		t.Fatalf("NewSecretManager: %v", err)
	}
	if sm.manager == nil || sm.manager.providers == nil {
		t.Fatal("WithProvider did not initialise the provider manager")
	}
	if got := sm.manager.providers["aws"]; got != p1 {
		t.Fatalf("provider %q = %v, want %v", "aws", got, p1)
	}
	if got := sm.manager.providers["doppler"]; got != p2 {
		t.Fatalf("provider %q = %v, want %v", "doppler", got, p2)
	}
	if len(sm.manager.providers) != 2 {
		t.Fatalf("provider count = %d, want 2", len(sm.manager.providers))
	}
}
