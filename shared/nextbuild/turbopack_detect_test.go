package nextbuild

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsTurbopackOutput(t *testing.T) {
	t.Run("returns true when [turbopack] chunk is present", func(t *testing.T) {
		dir := t.TempDir()
		chunkDir := filepath.Join(dir, ".next", "server", "chunks", "ssr")
		if err := os.MkdirAll(chunkDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(
			filepath.Join(chunkDir, "[turbopack]_runtime.js"),
			[]byte("// turbopack runtime\n"),
			0o644,
		); err != nil {
			t.Fatal(err)
		}
		if !IsTurbopackOutput(dir) {
			t.Fatal("expected Turbopack output to be detected")
		}
	})

	t.Run("returns true when [root-of-the-server] chunk is present", func(t *testing.T) {
		dir := t.TempDir()
		chunkDir := filepath.Join(dir, ".next", "server", "chunks")
		if err := os.MkdirAll(chunkDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(
			filepath.Join(chunkDir, "[root-of-the-server]__abc123._.js"),
			[]byte("// turbopack root chunk\n"),
			0o644,
		); err != nil {
			t.Fatal(err)
		}
		if !IsTurbopackOutput(dir) {
			t.Fatal("expected Turbopack output to be detected")
		}
	})

	t.Run("returns false for Webpack output", func(t *testing.T) {
		dir := t.TempDir()
		chunkDir := filepath.Join(dir, ".next", "server", "chunks")
		if err := os.MkdirAll(chunkDir, 0o755); err != nil {
			t.Fatal(err)
		}
		// Webpack-style chunks: numeric IDs, no [turbopack]/[root-of-the-server] prefix.
		if err := os.WriteFile(
			filepath.Join(chunkDir, "247.js"),
			[]byte("// webpack chunk\n"),
			0o644,
		); err != nil {
			t.Fatal(err)
		}
		if IsTurbopackOutput(dir) {
			t.Fatal("expected Webpack output to NOT be flagged as Turbopack")
		}
	})

	t.Run("returns false when build dir does not exist", func(t *testing.T) {
		if IsTurbopackOutput(t.TempDir()) {
			t.Fatal("expected absent build to be reported as non-Turbopack")
		}
	})
}
