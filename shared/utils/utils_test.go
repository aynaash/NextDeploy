package utils

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/aynaash/nextdeploy/shared/nextcore"
)

// silentLogger satisfies the package-private logger interface without
// printing — CreateTarball is chatty and the test only cares about output.
type silentLogger struct{}

func (silentLogger) Info(string, ...interface{})  {}
func (silentLogger) Warn(string, ...interface{})  {}
func (silentLogger) Error(string, ...interface{}) {}

// readTarGz returns name→content for every regular file in a gzipped tar.
func readTarGz(t *testing.T, path string) map[string]string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open tarball: %v", err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatalf("gzip reader: %v", err)
	}
	defer gz.Close()

	out := map[string]string{}
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err != nil {
			break
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		buf, err := io.ReadAll(tr)
		if err != nil {
			t.Fatalf("read entry %s: %v", hdr.Name, err)
		}
		out[hdr.Name] = string(buf)
	}
	return out
}

// TestCreateTarballRoundTrip exercises the bounded read pipeline with far more
// files than maxPending (so the look-ahead semaphore must cycle) plus a file
// larger than largeFileThreshold (so the streamed, non-buffered path runs).
// It asserts every file round-trips with its exact bytes — which only holds if
// the out-of-order reassembly stays correct under the new backpressure.
func TestCreateTarballRoundTrip(t *testing.T) {
	src := t.TempDir()

	want := map[string]string{}
	// 3× maxPending small files forces the dispatcher to block on `pending`
	// and the writer to drain it repeatedly.
	for i := 0; i < maxPending*3; i++ {
		name := fmt.Sprintf("file_%04d.txt", i)
		content := fmt.Sprintf("content of file %d\n", i)
		if err := os.WriteFile(filepath.Join(src, name), []byte(content), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
		want[name] = content
	}

	// One file above the threshold: exercises the deferred/streamed branch
	// (readFile returns data=nil, writeTarEntry copies from disk).
	big := make([]byte, largeFileThreshold+1024)
	for i := range big {
		big[i] = byte('a' + i%26)
	}
	if err := os.WriteFile(filepath.Join(src, "big.bin"), big, 0o600); err != nil {
		t.Fatalf("write big: %v", err)
	}
	want["big.bin"] = string(big)

	// A nested directory entry, to cover the dir-header phase.
	if err := os.MkdirAll(filepath.Join(src, "nested"), 0o750); err != nil {
		t.Fatalf("mkdir nested: %v", err)
	}
	if err := os.WriteFile(filepath.Join(src, "nested", "deep.txt"), []byte("deep\n"), 0o600); err != nil {
		t.Fatalf("write nested: %v", err)
	}
	want["nested/deep.txt"] = "deep\n"

	target := filepath.Join(t.TempDir(), "app.tar.gz")
	payload := &nextcore.NextCorePayload{OutputMode: nextcore.OutputModeStandalone}
	if err := CreateTarball(src, target, "vps", payload, silentLogger{}); err != nil {
		t.Fatalf("CreateTarball: %v", err)
	}

	got := readTarGz(t, target)
	if len(got) != len(want) {
		gotNames := make([]string, 0, len(got))
		for n := range got {
			gotNames = append(gotNames, n)
		}
		sort.Strings(gotNames)
		t.Fatalf("file count: got %d, want %d (got: %v)", len(got), len(want), gotNames)
	}
	for name, content := range want {
		if got[name] != content {
			t.Errorf("file %s: content mismatch (got %d bytes, want %d)", name, len(got[name]), len(content))
		}
	}
}
