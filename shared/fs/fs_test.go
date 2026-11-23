package fs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFileWriter_WriteCreatesNewFile(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "config.txt")
	fw := NewFileWriter(false, false)

	if err := fw.Write(target, []byte("hello")); err != nil {
		t.Fatalf("Write: %v", err)
	}

	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(data) != "hello" {
		t.Fatalf("content = %q, want %q", data, "hello")
	}

	info, err := os.Stat(target)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("mode = %o, want 0600", perm)
	}
}

func TestFileWriter_WriteSkipsExisting(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "keep.txt")
	if err := os.WriteFile(target, []byte("original"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	fw := NewFileWriter(false, false)
	if err := fw.Write(target, []byte("updated")); err != nil {
		t.Fatalf("Write: %v", err)
	}

	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(data) != "original" {
		t.Fatalf("content = %q, want original", data)
	}
}

func TestFileWriter_WriteOverwritesWhenForced(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "overwrite.txt")
	if err := os.WriteFile(target, []byte("old"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	fw := NewFileWriter(true, false)
	if err := fw.Write(target, []byte("new")); err != nil {
		t.Fatalf("Write: %v", err)
	}

	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(data) != "new" {
		t.Fatalf("content = %q, want new", data)
	}
}

func TestFileWriter_WriteDryRun(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "dry.txt")
	fw := NewFileWriter(true, true)

	if err := fw.Write(target, []byte("content")); err != nil {
		t.Fatalf("Write: %v", err)
	}

	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("expected file %s not to exist in dryRun mode, err=%v", target, err)
	}
}
