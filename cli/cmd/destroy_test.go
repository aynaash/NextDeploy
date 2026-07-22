package cmd

import (
	"strings"
	"testing"
)

func TestDestroyBlocked(t *testing.T) {
	blocked, reason := destroyBlocked(true, false)
	if !blocked {
		t.Fatal("deletion_protection without --force must block destroy")
	}
	if !strings.Contains(reason, "--force") || !strings.Contains(reason, "deletion_protection") {
		t.Errorf("reason should explain the override: %q", reason)
	}

	if blocked, _ := destroyBlocked(true, true); blocked {
		t.Error("--force must override deletion_protection")
	}

	if blocked, _ := destroyBlocked(false, false); blocked {
		t.Error("unprotected app should not be blocked")
	}
	if blocked, _ := destroyBlocked(false, true); blocked {
		t.Error("unprotected app should not be blocked even with force")
	}
}
