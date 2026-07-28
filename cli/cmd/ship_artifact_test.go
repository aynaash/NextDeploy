package cmd

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// makeTarball writes a .tar.gz containing the given name→content entries.
func makeTarball(t *testing.T, entries map[string]string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "app.tar.gz")
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	for name, body := range entries {
		if err := tw.WriteHeader(&tar.Header{
			Name:     name,
			Mode:     0o600,
			Size:     int64(len(body)),
			Typeflag: tar.TypeReg,
		}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestTarballAppNameReadsMetadata(t *testing.T) {
	for _, entry := range []string{
		".nextdeploy/metadata.json",
		"./.nextdeploy/metadata.json",
		"metadata.json",
		"./metadata.json",
	} {
		p := makeTarball(t, map[string]string{
			entry:            `{"app_name":"ressencesystems","domain":"ressencesystems.com"}`,
			"server.js":      "console.log(1)",
			"public/x.txt":   "x",
			"other/note.txt": "n",
		})
		got, err := tarballAppName(p)
		if err != nil {
			t.Errorf("entry %q: %v", entry, err)
			continue
		}
		if got != "ressencesystems" {
			t.Errorf("entry %q: got %q, want ressencesystems", entry, got)
		}
	}
}

func TestTarballAppNameStaleArtifactIsDetectable(t *testing.T) {
	// The failure this guards: config says "ressencesystems", the cached
	// tarball still carries the old domain-shaped name the daemon rejects.
	p := makeTarball(t, map[string]string{
		".nextdeploy/metadata.json": `{"app_name":"ressencesystems.com"}`,
	})
	got, err := tarballAppName(p)
	if err != nil {
		t.Fatal(err)
	}
	if got == "ressencesystems" {
		t.Fatal("stale name was not surfaced")
	}
	if got != "ressencesystems.com" {
		t.Errorf("got %q, want ressencesystems.com", got)
	}
}

func TestTarballAppNameMissingMetadata(t *testing.T) {
	p := makeTarball(t, map[string]string{"server.js": "x"})
	_, err := tarballAppName(p)
	if !errors.Is(err, errNoMetadata) {
		t.Errorf("want errNoMetadata, got %v", err)
	}
}

func TestTarballAppNameIgnoresUnrelatedMetadataJSON(t *testing.T) {
	// A nested metadata.json (e.g. inside node_modules) must not be mistaken
	// for the artifact's own — the daemon only reads the two canonical paths.
	p := makeTarball(t, map[string]string{
		"node_modules/pkg/metadata.json": `{"app_name":"not-the-app"}`,
	})
	_, err := tarballAppName(p)
	if !errors.Is(err, errNoMetadata) {
		t.Errorf("want errNoMetadata for a nested metadata.json, got %v", err)
	}
}

func TestTarballAppNameNotAnArchive(t *testing.T) {
	p := filepath.Join(t.TempDir(), "bogus.tar.gz")
	if err := os.WriteFile(p, []byte("definitely not gzip"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := tarballAppName(p); err == nil {
		t.Error("want an error for a non-archive")
	}
}

func TestTarballAppNameMissingFile(t *testing.T) {
	if _, err := tarballAppName(filepath.Join(t.TempDir(), "absent.tar.gz")); err == nil {
		t.Error("want an error for a missing file")
	}
}
