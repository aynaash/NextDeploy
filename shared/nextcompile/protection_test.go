package nextcompile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aynaash/nextdeploy/shared/protection"
)

func TestEmitProtectionConfig_NilWritesNull(t *testing.T) {
	dir := t.TempDir()
	path, err := EmitProtectionConfig(nil, dir)
	if err != nil {
		t.Fatalf("EmitProtectionConfig: %v", err)
	}
	data, err := os.ReadFile(path) // #nosec G304
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(data)) != "null" {
		t.Errorf("nil protection should emit `null`, got %q", data)
	}
	if filepath.Base(path) != "protection.json" {
		t.Errorf("unexpected path %s", path)
	}
}

func TestEmitProtectionConfig_WritesRuntimeJSON(t *testing.T) {
	rt, err := protection.BuildRuntime(&protection.Config{
		Enabled: true,
		Auth:    &protection.Auth{},
	})
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	path, err := EmitProtectionConfig(rt, dir)
	if err != nil {
		t.Fatalf("EmitProtectionConfig: %v", err)
	}
	data, err := os.ReadFile(path) // #nosec G304
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	for _, frag := range []string{`"version": 1`, `"secretEnv": "AUTH_SECRET"`, `"publicPaths"`} {
		if !strings.Contains(got, frag) {
			t.Errorf("protection.json missing %q\n%s", frag, got)
		}
	}
}

// The worker entry must import the guard + protection policy and wire both into
// the tables object so the dispatcher can run the guard.
func TestWorkerEntry_WiresGuardAndProtection(t *testing.T) {
	dir := t.TempDir()
	path, err := EmitWorkerEntry(dir)
	if err != nil {
		t.Fatalf("EmitWorkerEntry: %v", err)
	}
	data, err := os.ReadFile(path) // #nosec G304
	if err != nil {
		t.Fatal(err)
	}
	src := string(data)
	for _, frag := range []string{
		`import { runGuard } from "./runtime/guard.mjs"`,
		`import protection from "./protection.json"`,
		`guard: runGuard`,
		`protection,`,
	} {
		if !strings.Contains(src, frag) {
			t.Errorf("worker_entry.mjs missing %q\n%s", frag, src)
		}
	}
}

// The dispatcher source (shipped as-is) must invoke the guard before routing.
func TestDispatcher_InvokesGuardFirst(t *testing.T) {
	dir := t.TempDir()
	if _, err := ExtractRuntime(dir); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "_nextdeploy", "runtime", "dispatcher.mjs")) // #nosec G304
	if err != nil {
		t.Fatal(err)
	}
	src := string(data)
	if !strings.Contains(src, "tables.guard") || !strings.Contains(src, "tables.protection") {
		t.Errorf("dispatcher.mjs does not invoke the protection guard:\n%.600s", src)
	}
	// Guard must run before the short-circuit/route stages.
	guardIdx := strings.Index(src, "tables.guard")
	shortIdx := strings.Index(src, "tryShortCircuits")
	if guardIdx < 0 || shortIdx < 0 || guardIdx > shortIdx {
		t.Errorf("guard must run before tryShortCircuits (guard=%d short=%d)", guardIdx, shortIdx)
	}
}
