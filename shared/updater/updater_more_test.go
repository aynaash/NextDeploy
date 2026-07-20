package updater

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ExtractBinaryForCLI is the exported wrapper around extractBinary; pin that it
// forwards to the real extraction (the internal path is covered separately).
func TestExtractBinaryForCLI(t *testing.T) {
	content := []byte("#!/bin/sh\necho hi\n")

	t.Run("extracts from tar.gz", func(t *testing.T) {
		dir := t.TempDir()
		arc := makeTarGz(t, dir, "nextdeploy", content)
		dst := filepath.Join(dir, "out")
		if err := ExtractBinaryForCLI(arc, "nextdeploy", dst); err != nil {
			t.Fatalf("ExtractBinaryForCLI: %v", err)
		}
		if got, _ := os.ReadFile(dst); !bytes.Equal(got, content) {
			t.Fatalf("extracted content mismatch: %q", got)
		}
	})

	t.Run("missing binary errors", func(t *testing.T) {
		dir := t.TempDir()
		arc := makeTarGz(t, dir, "other", content)
		if err := ExtractBinaryForCLI(arc, "nextdeploy", filepath.Join(dir, "out")); err == nil {
			t.Fatal("want not-found error")
		}
	})
}

func TestAttemptDownload_ErrorBranches(t *testing.T) {
	t.Run("404 reports still-building", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer srv.Close()
		err := attemptDownload(srv.URL, filepath.Join(t.TempDir(), "b"), "nextdeploy", DefaultUpdateOptions())
		if err == nil || !strings.Contains(err.Error(), "not found") {
			t.Fatalf("404: want not-found error, got %v", err)
		}
	})

	t.Run("other non-200 surfaces status", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer srv.Close()
		err := attemptDownload(srv.URL, filepath.Join(t.TempDir(), "b"), "nextdeploy", DefaultUpdateOptions())
		if err == nil || !strings.Contains(err.Error(), "500") {
			t.Fatalf("500: want HTTP 500 error, got %v", err)
		}
	})

	t.Run("VerifySSL=false accepts self-signed cert", func(t *testing.T) {
		const body = "insecure-mirror-bytes"
		srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(body))
		}))
		defer srv.Close()

		opts := DefaultUpdateOptions()
		opts.VerifySSL = false
		dest := filepath.Join(t.TempDir(), "b")
		if err := attemptDownload(srv.URL, dest, "nextdeploy", opts); err != nil {
			t.Fatalf("VerifySSL=false against TLS server: %v", err)
		}
		if got, _ := os.ReadFile(dest); string(got) != body {
			t.Fatalf("downloaded content = %q, want %q", got, body)
		}
	})
}

func TestCopyFile(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	content := []byte("payload")
	if err := os.WriteFile(src, content, 0o644); err != nil {
		t.Fatal(err)
	}

	t.Run("copies content", func(t *testing.T) {
		dst := filepath.Join(dir, "dst")
		if err := copyFile(src, dst); err != nil {
			t.Fatalf("copyFile: %v", err)
		}
		if got, _ := os.ReadFile(dst); !bytes.Equal(got, content) {
			t.Fatalf("copied content = %q, want %q", got, content)
		}
	})

	t.Run("missing source errors", func(t *testing.T) {
		if err := copyFile(filepath.Join(dir, "nope"), filepath.Join(dir, "dst2")); err == nil {
			t.Fatal("want error copying a missing source")
		}
	})
}

// copyFileWithSudo tries the plain copy first; when the destination is writable
// it must succeed without ever shelling out to sudo.
func TestCopyFileWithSudo_PlainCopySucceeds(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	if err := os.WriteFile(src, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := copyFileWithSudo(src, filepath.Join(dir, "dst")); err != nil {
		t.Fatalf("copyFileWithSudo: %v", err)
	}
}

// atomicReplace renames within the same filesystem via os.Rename; the temp dir
// guarantees src and dst share a device so the fast path is exercised.
func TestAtomicReplace_RenameFastPath(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "new")
	dst := filepath.Join(dir, "installed")
	if err := os.WriteFile(src, []byte("new-binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := atomicReplace(src, dst); err != nil {
		t.Fatalf("atomicReplace: %v", err)
	}
	if got, _ := os.ReadFile(dst); string(got) != "new-binary" {
		t.Fatalf("dst content = %q, want new-binary", got)
	}
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Fatal("src should be gone after a rename")
	}
}

func TestSetPermissions(t *testing.T) {
	p := filepath.Join(t.TempDir(), "bin")
	if err := os.WriteFile(p, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := setPermissions(p); err != nil {
		t.Fatalf("setPermissions: %v", err)
	}
	info, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o755 {
		t.Fatalf("perm = %o, want 0755", perm)
	}
}

func TestUpdateError(t *testing.T) {
	inner := errors.New("boom")
	withErr := &UpdateError{Stage: "download", Message: "failed", Err: inner}
	if got := withErr.Error(); !strings.Contains(got, "download") || !strings.Contains(got, "boom") {
		t.Fatalf("Error() = %q, want stage+wrapped error", got)
	}
	if !errors.Is(withErr, inner) {
		t.Fatal("Unwrap must expose the wrapped error to errors.Is")
	}

	noErr := &UpdateError{Stage: "api", Message: "rate limited"}
	if got := noErr.Error(); !strings.Contains(got, "api") || strings.Contains(got, "%!") {
		t.Fatalf("Error() without wrapped err = %q", got)
	}
	if noErr.Unwrap() != nil {
		t.Fatal("Unwrap of a nil-Err UpdateError must be nil")
	}
}
