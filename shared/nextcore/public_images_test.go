package nextcore

import (
	"os"
	"path/filepath"
	"testing"
)

// A Next.js app is not required to have a public/ directory. A missing one must
// yield "no public images", not a fatal metadata-generation error (regression:
// `nextdeploy ship` failed with "lstat .../public: no such file or directory").
func TestFindPublicImages_MissingDirIsNotAnError(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "public")
	imgs, err := findPublicImages(missing, t.TempDir())
	if err != nil {
		t.Fatalf("missing public/ should be tolerated, got: %v", err)
	}
	if len(imgs) != 0 {
		t.Errorf("expected no images for a missing public/, got %d", len(imgs))
	}
}

func TestFindPublicImages_FindsImagesWhenPresent(t *testing.T) {
	dir := t.TempDir()
	pub := filepath.Join(dir, "public")
	if err := os.MkdirAll(pub, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pub, "logo.png"), []byte("\x89PNG"), 0o600); err != nil {
		t.Fatal(err)
	}
	imgs, err := findPublicImages(pub, dir)
	if err != nil {
		t.Fatalf("scan present public/: %v", err)
	}
	if len(imgs) != 1 {
		t.Errorf("expected 1 image, got %d: %+v", len(imgs), imgs)
	}
}
