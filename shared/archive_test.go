package shared

import (
	"archive/tar"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"
)

// writeTarGz builds a .tar.gz at path from the given headers. For regular files
// the body is taken from bodies[name] and the header Size is set accordingly.
func writeTarGz(t *testing.T, path string, headers []tar.Header, bodies map[string]string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create %s: %v", path, err)
	}
	defer f.Close()
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	for _, h := range headers {
		hdr := h
		if body, ok := bodies[hdr.Name]; ok {
			hdr.Size = int64(len(body))
		}
		if err := tw.WriteHeader(&hdr); err != nil {
			t.Fatalf("write header %s: %v", hdr.Name, err)
		}
		if body, ok := bodies[hdr.Name]; ok {
			if _, err := tw.Write([]byte(body)); err != nil {
				t.Fatalf("write body %s: %v", hdr.Name, err)
			}
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestExtractTarGz_Valid(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "ok.tar.gz")
	writeTarGz(t, src,
		[]tar.Header{
			{Name: "sub", Typeflag: tar.TypeDir, Mode: 0o755},
			{Name: "sub/app.js", Typeflag: tar.TypeReg, Mode: 0o644},
		},
		map[string]string{"sub/app.js": "console.log('hi')"},
	)

	dest := filepath.Join(dir, "out")
	if err := ExtractTarGz(src, dest); err != nil {
		t.Fatalf("expected clean extraction, got: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(dest, "sub", "app.js"))
	if err != nil {
		t.Fatalf("expected extracted file: %v", err)
	}
	if string(got) != "console.log('hi')" {
		t.Fatalf("unexpected content: %q", got)
	}
}

func TestExtractTarGz_RejectsTraversal(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "evil.tar.gz")
	writeTarGz(t, src,
		[]tar.Header{{Name: "../escaped.txt", Typeflag: tar.TypeReg, Mode: 0o644}},
		map[string]string{"../escaped.txt": "pwned"},
	)

	dest := filepath.Join(dir, "out")
	if err := ExtractTarGz(src, dest); err == nil {
		t.Fatal("expected traversal entry to be rejected")
	}
	if _, err := os.Stat(filepath.Join(dir, "escaped.txt")); err == nil {
		t.Fatal("traversal entry escaped the destination directory")
	}
}

func TestExtractTarGz_RejectsSymlinkEscape(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "symlink.tar.gz")
	writeTarGz(t, src,
		[]tar.Header{{Name: "link", Typeflag: tar.TypeSymlink, Linkname: "../../etc/cron.d", Mode: 0o777}},
		nil,
	)

	dest := filepath.Join(dir, "out")
	if err := ExtractTarGz(src, dest); err == nil {
		t.Fatal("expected escaping symlink to be rejected")
	}
}

// Next.js catch-all route directories like "[...slug]" contain "..", which a
// naive strings.Contains(name, "..") check wrongly rejected ("unsafe archive
// entry path"). They must extract fine; genuine traversal is still blocked by
// withinDir (see TestExtractTarGz_RejectsTraversal).
func TestExtractTarGz_AllowsNextCatchAllRoute(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "next.tar.gz")
	const file = "app/(docs)/docs/[...slug]/page.js"
	writeTarGz(t, src,
		[]tar.Header{
			{Name: "app/(docs)/docs/[...slug]", Typeflag: tar.TypeDir, Mode: 0o755},
			{Name: file, Typeflag: tar.TypeReg, Mode: 0o644},
		},
		map[string]string{file: "export default Page"},
	)

	dest := filepath.Join(dir, "out")
	if err := ExtractTarGz(src, dest); err != nil {
		t.Fatalf("catch-all route path must extract, got: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dest, "app", "(docs)", "docs", "[...slug]", "page.js")); err != nil {
		t.Fatalf("expected extracted catch-all file: %v", err)
	}
}
