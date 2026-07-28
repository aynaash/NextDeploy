package daemon

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// unsafeAppNames are the values that must never reach a filesystem or systemd
// call. filepath.Join CLEANS its result, so "../../../etc" under appsDir
// resolves to "/etc" — the guard, not Join, is what stops an arbitrary
// os.RemoveAll.
var unsafeAppNames = []string{
	"../../etc",
	"../../../etc",
	"..",
	"a/b",
	"foo bar",
	"",
	"My-App",
	"my_app",
	"ab",
}

func TestFilepathJoinIsNotASecurityBoundary(t *testing.T) {
	// Pin the premise the guard exists for: this is why validation must happen
	// before the path is built, not after.
	if got := filepath.Join("/opt/nextdeploy/apps", "../../../etc"); got != "/etc" {
		t.Fatalf("filepath.Join escaped to %q, expected /etc — premise changed", got)
	}
}

func TestHandleDestroyRejectsUnsafeAppName(t *testing.T) {
	// Deliberately a zero-value handler: every dependency is nil, so if the
	// guard ever stops being the first thing in the function, the handler
	// panics instead of quietly proceeding. The guard's *position* is part of
	// what's under test, not just its presence.
	ch := &CommandHandler{}

	for _, bad := range unsafeAppNames {
		resp := ch.handleDestroy(map[string]interface{}{"appName": bad})
		if resp.Success {
			t.Errorf("destroy accepted unsafe appName %q", bad)
		}
	}
}

func TestHandleStopAppRejectsUnsafeAppName(t *testing.T) {
	ch := &CommandHandler{}

	for _, bad := range unsafeAppNames {
		resp := ch.handleStopApp(map[string]interface{}{"appName": bad})
		if resp.Success {
			t.Errorf("stop accepted unsafe appName %q", bad)
		}
	}
}

func TestHandleDestroyLeavesDiskUntouched(t *testing.T) {
	// A traversal name aimed at a real tree must not delete any of it.
	victim := t.TempDir()
	canary := filepath.Join(victim, "canary")
	if err := os.WriteFile(canary, []byte("do not delete"), 0o600); err != nil {
		t.Fatal(err)
	}

	// Relative hops from appsDir back to the root, then into the temp tree.
	depth := strings.Count(strings.Trim(appsDir, "/"), "/") + 1
	traversal := strings.Repeat("../", depth) + strings.TrimPrefix(victim, "/")

	ch := &CommandHandler{}
	if resp := ch.handleDestroy(map[string]interface{}{"appName": traversal}); resp.Success {
		t.Fatalf("destroy accepted traversal appName %q", traversal)
	}
	if _, err := os.Stat(canary); err != nil {
		t.Fatalf("canary was removed: %v", err)
	}
}

func TestHandleDestroyRejectsMissingAppName(t *testing.T) {
	ch := &CommandHandler{}
	if resp := ch.handleDestroy(map[string]interface{}{}); resp.Success {
		t.Error("destroy accepted a missing appName")
	}
	if resp := ch.handleStopApp(map[string]interface{}{}); resp.Success {
		t.Error("stop accepted a missing appName")
	}
}
