package updater

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// A previous attempt that timed out mid-download leaves a partial file in the
// temp dir. The retry must overwrite it, not fail with "file exists" (the bug
// that wedged `nextdeploy update` after one network hiccup).
func TestAttemptDownload_OverwritesLeftoverPartialFile(t *testing.T) {
	const body = "the-real-binary-bytes"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", fmt.Sprint(len(body)))
		_, _ = io.WriteString(w, body)
	}))
	defer srv.Close()

	dest := filepath.Join(t.TempDir(), "nextdeploy.tar.gz")
	if err := os.WriteFile(dest, []byte("partial-junk-from-a-failed-attempt"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := attemptDownload(srv.URL, dest, "nextdeploy", DefaultUpdateOptions()); err != nil {
		t.Fatalf("retry over a leftover partial file must succeed, got: %v", err)
	}
	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != body {
		t.Errorf("file content = %q, want fully overwritten %q", got, body)
	}
}

// detectArchiveName must match GoReleaser's lowercase {{ .Os }} naming exactly.
// If it title-cases the OS, the checksums.txt lookup (a literal string match)
// misses and integrity verification is silently skipped — the bug behind the
// "No checksum found for binary, skipping integrity check" log line.
func TestDetectArchiveName_MatchesGoReleaserLowercase(t *testing.T) {
	name := detectArchiveName("v0.12.1", "nextdeploy")

	ext := ".tar.gz"
	if runtime.GOOS == "windows" {
		ext = ".zip"
	}
	want := fmt.Sprintf("nextdeploy_0.12.1_%s_%s%s", runtime.GOOS, runtime.GOARCH, ext)
	if name != want {
		t.Fatalf("detectArchiveName = %q, want %q", name, want)
	}

	// Regression guard: the OS segment must be lowercase, not "Linux"/"Darwin".
	titled := strings.ToUpper(runtime.GOOS[:1]) + runtime.GOOS[1:]
	if strings.Contains(name, "_"+titled+"_") {
		t.Errorf("name %q contains title-cased OS %q — checksums.txt lookup will miss", name, titled)
	}

	// The leading v must be stripped, matching the asset/checksum entry.
	if strings.Contains(name, "_v0.12.1_") {
		t.Errorf("version not stripped of leading v: %q", name)
	}
}

// The produced name must be a key in a GoReleaser-style (lowercase) checksums
// map — i.e. verifyBinaryIntegrity would actually find it and verify.
func TestDetectArchiveName_FoundInLowercaseChecksums(t *testing.T) {
	name := detectArchiveName("0.12.1", "nextdeploy")
	checksums := map[string]string{
		fmt.Sprintf("nextdeploy_0.12.1_%s_%s.tar.gz", runtime.GOOS, runtime.GOARCH): "aaa",
		fmt.Sprintf("nextdeploy_0.12.1_%s_%s.zip", runtime.GOOS, runtime.GOARCH):    "bbb",
	}
	if _, ok := checksums[name]; !ok {
		t.Errorf("detectArchiveName %q not in lowercase checksums map — integrity check would be skipped", name)
	}
}
