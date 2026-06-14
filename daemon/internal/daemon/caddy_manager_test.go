package daemon

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestSanitizeAppName(t *testing.T) {
	valid := []string{"app", "my-app", "my_app", "App123", "a"}
	for _, name := range valid {
		if err := sanitizeAppName(name); err != nil {
			t.Errorf("expected %q valid, got %v", name, err)
		}
	}

	// Path traversal / separators / directive-ish characters must be rejected
	// before appName reaches filepath.Join(configDir, name+".caddy").
	invalid := []string{"", "../etc/cron.d/x", "a/b", "a.b", "a b", "a;b", "..", "app$"}
	for _, name := range invalid {
		if err := sanitizeAppName(name); err == nil {
			t.Errorf("expected %q to be rejected", name)
		}
	}
}

// TestCaddyPreCommitValidation verifies a malformed fragment is trapped before
// it ever lands in the live import dir. Requires the caddy binary; skipped when
// it (or the coraza module the real Caddyfile expects) is unavailable.
func TestCaddyPreCommitValidation(t *testing.T) {
	if _, err := exec.LookPath("caddy"); err != nil {
		t.Skip("caddy binary not available; skipping pre-commit validation test")
	}

	prodDir := t.TempDir()
	cm := &CaddyManager{configDir: prodDir}

	badFragment := []byte(`example.com {
	reverse_proxy localhost:3000 {
		this_is_not_a_real_caddy_directive_xyz
	}
}
`)
	if err := cm.commitFragmentSafely("broken-app", badFragment); err == nil {
		t.Fatal("expected validation to reject the malformed fragment, but it passed")
	}
	if _, err := os.Stat(filepath.Join(prodDir, "broken-app.caddy")); !os.IsNotExist(err) {
		t.Error("malformed fragment leaked into the live config directory")
	}

	goodFragment := []byte(`example.com {
	reverse_proxy localhost:3000
}
`)
	if err := cm.commitFragmentSafely("good-app", goodFragment); err != nil {
		t.Fatalf("expected valid fragment to commit, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(prodDir, "good-app.caddy")); err != nil {
		t.Errorf("valid fragment was not committed to the live dir: %v", err)
	}
}
