package nextcore

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aynaash/nextdeploy/shared/config"
)

func fpConfig() *config.NextDeployConfig {
	return &config.NextDeployConfig{
		TargetType: "vps",
		App: config.AppConfig{
			Name:        "ressencesystems",
			Port:        3000,
			Environment: "production",
			Domain:      config.DomainConfig{Name: "ressencesystems.com"},
		},
	}
}

func TestConfigFingerprintIsStable(t *testing.T) {
	a := ConfigFingerprint(fpConfig())
	b := ConfigFingerprint(fpConfig())
	if a == "" {
		t.Fatal("fingerprint is empty for a valid config")
	}
	if a != b {
		t.Errorf("fingerprint not stable: %q vs %q", a, b)
	}
}

func TestConfigFingerprintChangesWithBuildAffectingFields(t *testing.T) {
	base := ConfigFingerprint(fpConfig())

	mutations := map[string]func(*config.NextDeployConfig){
		"app.name":        func(c *config.NextDeployConfig) { c.App.Name = "othername" },
		"app.domain":      func(c *config.NextDeployConfig) { c.App.Domain.Name = "other.com" },
		"app.environment": func(c *config.NextDeployConfig) { c.App.Environment = "staging" },
		"app.port":        func(c *config.NextDeployConfig) { c.App.Port = 4000 },
		"target_type":     func(c *config.NextDeployConfig) { c.TargetType = "serverless" },
		"cdn_enabled":     func(c *config.NextDeployConfig) { c.App.CDNEnabled = true },
	}
	for field, mutate := range mutations {
		cfg := fpConfig()
		mutate(cfg)
		if got := ConfigFingerprint(cfg); got == base {
			t.Errorf("changing %s did not change the fingerprint", field)
		}
	}
}

func TestConfigFingerprintIgnoresUnrelatedFields(t *testing.T) {
	// Reformatting the YAML or editing an unrelated setting must not force a
	// rebuild — the key covers the inputs that reach the artifact, no more.
	cfg := fpConfig()
	cfg.Version = "99"
	cfg.App.DeletionProtection = true
	if ConfigFingerprint(cfg) != ConfigFingerprint(fpConfig()) {
		t.Error("an unrelated field changed the fingerprint")
	}
}

func TestConfigFingerprintFieldsCannotRunTogether(t *testing.T) {
	// NUL separators mean adjacent fields can't be shuffled into the same hash.
	a := fpConfig()
	a.App.Name = "ab"
	a.App.Environment = "c"
	b := fpConfig()
	b.App.Name = "a"
	b.App.Environment = "bc"
	if ConfigFingerprint(a) == ConfigFingerprint(b) {
		t.Error("adjacent fields collided — separators are not doing their job")
	}
}

func TestConfigFingerprintNilConfig(t *testing.T) {
	if got := ConfigFingerprint(nil); got != "" {
		t.Errorf("ConfigFingerprint(nil) = %q, want \"\"", got)
	}
}

// writeLock places a build.lock in a temp cwd so ValidateBuildState can read it.
func writeLock(t *testing.T, lock BuildLock) {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".nextdeploy"), 0o750); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(lock)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".nextdeploy", "build.lock"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
}

func TestValidateBuildStateRejectsNilConfig(t *testing.T) {
	// A caller that can't supply the config can't prove the key is unchanged,
	// so it must not be allowed to skip.
	writeLock(t, BuildLock{ConfigHash: ConfigFingerprint(fpConfig())})
	if err := ValidateBuildState(nil); err == nil {
		t.Error("ValidateBuildState(nil) returned nil — a build would be skipped on an unproven key")
	}
}

func TestValidateBuildStateRejectsLegacyLockWithoutHash(t *testing.T) {
	// A lock written before config_hash existed carries no key at all. Treat
	// "unknown" as "rebuild" rather than trusting a partial match.
	writeLock(t, BuildLock{GitCommit: "deadbeef"})
	err := ValidateBuildState(fpConfig())
	if err == nil {
		t.Fatal("a lock with no config_hash should force a rebuild")
	}
	if !strings.Contains(err.Error(), "config") && !strings.Contains(err.Error(), "commit") {
		t.Errorf("error should explain what changed, got %v", err)
	}
}

func TestValidateBuildStateReportsConfigChange(t *testing.T) {
	// Same (fake) commit, different config → the git-only key would have said
	// "skip". This is the exact shape of the original incident.
	writeLock(t, BuildLock{GitCommit: "deadbeef", ConfigHash: ConfigFingerprint(fpConfig())})

	changed := fpConfig()
	changed.App.Name = "renamed"
	err := ValidateBuildState(changed)
	if err == nil {
		t.Fatal("a config change with an unchanged commit should force a rebuild")
	}
}
