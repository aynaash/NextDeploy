package updater

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		name   string
		v1, v2 string
		want   int
	}{
		{"equal", "1.2.3", "1.2.3", 0},
		{"equal with v prefix", "v1.2.3", "1.2.3", 0},
		{"patch newer", "1.2.4", "1.2.3", 1},
		{"patch older", "1.2.3", "1.2.4", -1},
		{"minor newer", "1.3.0", "1.2.9", 1},
		{"major newer", "2.0.0", "1.9.9", 1},
		{"prerelease stripped", "1.2.3-rc1", "1.2.3", 0},
		{"different lengths equal", "1.2", "1.2.0", 0},
		{"longer is newer", "1.2.1", "1.2", 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := compareVersions(tt.v1, tt.v2); got != tt.want {
				t.Fatalf("compareVersions(%q,%q) = %d, want %d", tt.v1, tt.v2, got, tt.want)
			}
		})
	}
}

func TestIsNewer(t *testing.T) {
	if !isNewer("1.2.4", "1.2.3") {
		t.Fatal("1.2.4 should be newer than 1.2.3")
	}
	if isNewer("1.2.3", "1.2.3") {
		t.Fatal("equal versions are not newer")
	}
	if isNewer("1.2.2", "1.2.3") {
		t.Fatal("older version is not newer")
	}
}

func TestStripV(t *testing.T) {
	tests := map[string]string{"v1.0": "1.0", "1.0": "1.0", "vvv": "vv", "": ""}
	for in, want := range tests {
		if got := stripV(in); got != want {
			t.Fatalf("stripV(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSplitVer(t *testing.T) {
	tests := []struct {
		in   string
		want []int
	}{
		{"1.2.3", []int{1, 2, 3}},
		{"1.2", []int{1, 2}},
		{"1.2.x", []int{1, 2}},
		{"abc", []int{}},
		{"", []int{}},
		{"10.20", []int{10, 20}},
	}
	for _, tt := range tests {
		got := splitVer(tt.in)
		if len(got) != len(tt.want) {
			t.Fatalf("splitVer(%q) = %v, want %v", tt.in, got, tt.want)
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Fatalf("splitVer(%q) = %v, want %v", tt.in, got, tt.want)
			}
		}
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		in   int64
		want string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1048576, "1.0 MB"},
		{31457280, "30.0 MB"},
	}
	for _, tt := range tests {
		if got := formatBytes(tt.in); got != tt.want {
			t.Fatalf("formatBytes(%d) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestParseUnixTime(t *testing.T) {
	got, err := parseUnixTime("1700000000")
	if err != nil || !got.Equal(time.Unix(1700000000, 0)) {
		t.Fatalf("parseUnixTime(valid) = %v, %v", got, err)
	}
	if _, err := parseUnixTime("notanumber"); err == nil {
		t.Fatal("want error for non-numeric input")
	}
	if _, err := parseUnixTime(""); err == nil {
		t.Fatal("want error for empty input")
	}
}

func TestDownloadChecksums(t *testing.T) {
	t.Run("parses and trims", func(t *testing.T) {
		body := "abc123  nextdeploy_0.14.2_linux_amd64.tar.gz\n" +
			"def456  nextdeploy_0.14.2_darwin_arm64.tar.gz\n\n" +
			"shortline\n"
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = io.WriteString(w, body)
		}))
		defer srv.Close()

		dst := filepath.Join(t.TempDir(), "checksums.txt")
		got, err := downloadChecksums(srv.URL, dst)
		if err != nil {
			t.Fatalf("downloadChecksums: %v", err)
		}
		if got["nextdeploy_0.14.2_linux_amd64.tar.gz"] != "abc123" {
			t.Fatalf("missing/incorrect linux checksum: %v", got)
		}
		if got["nextdeploy_0.14.2_darwin_arm64.tar.gz"] != "def456" {
			t.Fatalf("missing/incorrect darwin checksum: %v", got)
		}
		if _, ok := got["shortline"]; ok {
			t.Fatal("one-field line should be skipped")
		}
	})

	t.Run("non-200", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer srv.Close()
		if _, err := downloadChecksums(srv.URL, filepath.Join(t.TempDir(), "c.txt")); err == nil {
			t.Fatal("want error on HTTP 500")
		}
	})
}

func TestVerifyBinaryIntegrity(t *testing.T) {
	dir := t.TempDir()
	binPath := filepath.Join(dir, "bin")
	content := []byte("the binary bytes")
	// #nosec G306 -- test binary must be executable for the updater to exec it
	if err := os.WriteFile(binPath, content, 0o755); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(content)
	realHex := hex.EncodeToString(sum[:])

	tests := []struct {
		name      string
		checksums map[string]string
		binName   string
		wantErr   bool
	}{
		{"nil checksums skips", nil, "nextdeploy", false},
		{"name absent skips", map[string]string{"other": realHex}, "nextdeploy", false},
		{"match", map[string]string{"nextdeploy": realHex}, "nextdeploy", false},
		{"mismatch", map[string]string{"nextdeploy": "deadbeef"}, "nextdeploy", true},
		{"exe fallback", map[string]string{"nextdeploy.exe": realHex}, "nextdeploy", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := verifyBinaryIntegrity(binPath, tt.binName, tt.checksums)
			if tt.wantErr && err == nil {
				t.Fatal("want error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}

	t.Run("file missing", func(t *testing.T) {
		err := verifyBinaryIntegrity(filepath.Join(dir, "nope"), "nextdeploy",
			map[string]string{"nextdeploy": realHex})
		if err == nil {
			t.Fatal("want error opening missing file")
		}
	})
}

func makeTarGz(t *testing.T, dir, entryName string, content []byte) string {
	t.Helper()
	p := filepath.Join(dir, "archive.tar.gz")
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	hdr := &tar.Header{Name: entryName, Mode: 0o755, Size: int64(len(content)), Typeflag: tar.TypeReg}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return p
}

func makeZip(t *testing.T, dir, entryName string, content []byte) string {
	t.Helper()
	p := filepath.Join(dir, "archive.zip")
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zw := zip.NewWriter(f)
	w, err := zw.Create(entryName)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestExtractBinary(t *testing.T) {
	content := []byte("#!/bin/sh\necho hi\n")

	t.Run("tar.gz happy", func(t *testing.T) {
		dir := t.TempDir()
		arc := makeTarGz(t, dir, "nextdeploy", content)
		dst := filepath.Join(dir, "out")
		if err := extractBinary(arc, "nextdeploy", dst); err != nil {
			t.Fatalf("extractBinary: %v", err)
		}
		if got, _ := os.ReadFile(dst); !bytes.Equal(got, content) {
			t.Fatal("extracted content mismatch")
		}
	})

	t.Run("zip happy", func(t *testing.T) {
		dir := t.TempDir()
		arc := makeZip(t, dir, "nextdeploy", content)
		dst := filepath.Join(dir, "out")
		if err := extractBinary(arc, "nextdeploy", dst); err != nil {
			t.Fatalf("extractBinary: %v", err)
		}
		if got, _ := os.ReadFile(dst); !bytes.Equal(got, content) {
			t.Fatal("extracted content mismatch")
		}
	})

	t.Run("nested basename match", func(t *testing.T) {
		dir := t.TempDir()
		arc := makeTarGz(t, dir, "some/dir/nextdeploy", content)
		dst := filepath.Join(dir, "out")
		if err := extractBinary(arc, "nextdeploy", dst); err != nil {
			t.Fatalf("extractBinary nested: %v", err)
		}
	})

	t.Run("not found", func(t *testing.T) {
		dir := t.TempDir()
		arc := makeTarGz(t, dir, "somethingelse", content)
		if err := extractBinary(arc, "nextdeploy", filepath.Join(dir, "out")); err == nil {
			t.Fatal("want not-found error")
		}
	})
}

// zeroReader yields an endless stream of zero bytes.
type zeroReader struct{}

func (zeroReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 0
	}
	return len(p), nil
}

func TestCopyBounded(t *testing.T) {
	t.Run("under limit", func(t *testing.T) {
		var dst bytes.Buffer
		if err := copyBounded(&dst, bytes.NewReader([]byte("small"))); err != nil {
			t.Fatalf("copyBounded: %v", err)
		}
		if dst.String() != "small" {
			t.Fatalf("got %q", dst.String())
		}
	})

	t.Run("over limit trips guard", func(t *testing.T) {
		if testing.Short() {
			t.Skip("copies >512MiB; skipped in -short")
		}
		err := copyBounded(io.Discard, zeroReader{})
		if err == nil {
			t.Fatal("want decompression-bomb error")
		}
	})
}
