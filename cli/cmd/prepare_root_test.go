package cmd

import (
	"strings"
	"testing"
)

func TestRootCredentialBlocked(t *testing.T) {
	// root without override → blocked, with guidance.
	blocked, reason := rootCredentialBlocked("root", false)
	if !blocked {
		t.Fatal("root login without --allow-root must be blocked")
	}
	if !strings.Contains(reason, "sudo-capable") || !strings.Contains(reason, "--allow-root") {
		t.Errorf("reason should guide the user: %q", reason)
	}

	// root with override → allowed.
	if blocked, _ := rootCredentialBlocked("root", true); blocked {
		t.Error("--allow-root must override the root block")
	}

	// non-root → allowed.
	for _, u := range []string{"ubuntu", "deploy", "ec2-user", ""} {
		if blocked, _ := rootCredentialBlocked(u, false); blocked {
			t.Errorf("non-root user %q should not be blocked", u)
		}
	}
}
