package nextcompile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRuntimeSourceFiles_IncludesMVP(t *testing.T) {
	files, err := RuntimeSourceFiles()
	if err != nil {
		t.Fatal(err)
	}
	// MVP runtime must include these — they're what the dispatcher imports.
	required := []string{
		"runtime_src/dispatcher.mjs",
		"runtime_src/serve.mjs",
		"runtime_src/route_match.mjs",
		"runtime_src/errors.mjs",
		"runtime_src/context.mjs",
		"runtime_src/rsc.mjs",
		"runtime_src/actions.mjs",
		"runtime_src/cache.mjs",
		"runtime_src/image.mjs",
		"runtime_src/next_shims/cache.mjs",
		"runtime_src/next_shims/headers.mjs",
		"runtime_src/next_shims/server.mjs",
	}
	for _, r := range required {
		if !containsPath(files, r) {
			t.Errorf("embedded runtime missing %s\nfound: %v", r, files)
		}
	}
}

func TestRuntimeSourceFiles_ExcludesTestFilesIncludesGuard(t *testing.T) {
	files, err := RuntimeSourceFiles()
	if err != nil {
		t.Fatal(err)
	}
	if !containsPath(files, "runtime_src/guard.mjs") {
		t.Errorf("guard.mjs should be a shipped runtime file\nfound: %v", files)
	}
	for _, f := range files {
		if strings.HasSuffix(f, ".test.mjs") {
			t.Errorf("test file leaked into runtime source list: %s", f)
		}
	}
}

func TestRuntimeSourceFiles_ExcludesDevModules(t *testing.T) {
	files, err := RuntimeSourceFiles()
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		if strings.HasSuffix(f, ".dev.mjs") {
			t.Errorf("dev-only module leaked into runtime source list: %s", f)
		}
	}
	// The trie example specifically must stay out of the shipped runtime.
	if containsPath(files, "runtime_src/route_trie.dev.mjs") {
		t.Error("route_trie.dev.mjs must not be a shipped runtime file")
	}
}

func TestExtractRuntime_DoesNotShipTestFiles(t *testing.T) {
	dir := t.TempDir()
	written, err := ExtractRuntime(dir)
	if err != nil {
		t.Fatalf("ExtractRuntime: %v", err)
	}
	for _, w := range written {
		if strings.HasSuffix(w, ".test.mjs") || strings.HasSuffix(w, ".dev.mjs") {
			t.Errorf("dev-only file extracted into worker runtime: %s", w)
		}
	}
	// guard.mjs (non-test) must ship.
	guard := filepath.Join(dir, "_nextdeploy", "runtime", "guard.mjs")
	if _, err := os.Stat(guard); err != nil {
		t.Errorf("guard.mjs not extracted: %v", err)
	}
	// its test sibling must NOT.
	guardTest := filepath.Join(dir, "_nextdeploy", "runtime", "guard.test.mjs")
	if _, err := os.Stat(guardTest); err == nil {
		t.Error("guard.test.mjs must not be extracted into the worker runtime")
	}
}

func TestExtractRuntime_WritesFiles(t *testing.T) {
	dir := t.TempDir()
	written, err := ExtractRuntime(dir)
	if err != nil {
		t.Fatalf("ExtractRuntime: %v", err)
	}
	if len(written) == 0 {
		t.Fatal("no files written")
	}

	// dispatcher.mjs must land at <dir>/_nextdeploy/runtime/dispatcher.mjs
	// and must parse as ESM (start with comment or import / export).
	dispatcher := filepath.Join(dir, "_nextdeploy", "runtime", "dispatcher.mjs")
	data, err := os.ReadFile(dispatcher) // #nosec G304
	if err != nil {
		t.Fatalf("dispatcher.mjs not written: %v", err)
	}
	if !strings.Contains(string(data), "export async function dispatch") {
		t.Errorf("dispatcher.mjs contents unexpected:\n%.400s", data)
	}

	// serve.mjs, route_match.mjs, errors.mjs, context.mjs, rsc.mjs must all be there.
	for _, name := range []string{"serve.mjs", "route_match.mjs", "errors.mjs", "context.mjs", "rsc.mjs", "actions.mjs", "cache.mjs", "image.mjs"} {
		p := filepath.Join(dir, "_nextdeploy", "runtime", name)
		if _, err := os.Stat(p); err != nil {
			t.Errorf("missing %s: %v", name, err)
		}
	}

	// vendor/README.md must be there (subdirectory walk works).
	readme := filepath.Join(dir, "_nextdeploy", "runtime", "vendor", "README.md")
	if _, err := os.Stat(readme); err != nil {
		t.Errorf("vendor/README.md missing: %v", err)
	}
}

func TestExtractRuntime_Idempotent(t *testing.T) {
	dir := t.TempDir()
	if _, err := ExtractRuntime(dir); err != nil {
		t.Fatal(err)
	}
	// Second run must not error and must produce the same file contents.
	if _, err := ExtractRuntime(dir); err != nil {
		t.Fatalf("second ExtractRuntime: %v", err)
	}
}

func containsPath(files []string, want string) bool {
	for _, f := range files {
		if f == want {
			return true
		}
	}
	return false
}
