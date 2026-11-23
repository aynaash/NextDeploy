package envstore

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadEnvFile(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	content := "# comment\n\nFOO=bar\n  SPACED = hello world  \nMALFORMED_LINE\nURL=https://example.com?a=1&b=2\n"
	if err := os.WriteFile(envPath, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	got, err := ReadEnvFile(envPath)
	if err != nil {
		t.Fatalf("ReadEnvFile: %v", err)
	}

	want := map[string]string{
		"FOO":    "bar",
		"SPACED": "hello world",
		"URL":    "https://example.com?a=1&b=2",
	}
	if len(got) != len(want) {
		t.Fatalf("got %d entries, want %d: %v", len(got), len(want), got)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("got[%q] = %q, want %q", k, got[k], v)
		}
	}

	if _, err := ReadEnvFile(filepath.Join(dir, "missing.env")); err == nil {
		t.Fatal("expected error reading missing env file")
	}
}

func TestStore_CRUDAndFnOptions(t *testing.T) {
	s, err := New(
		CreateFn("A", "1"),
		UpdateFn("A", "2"),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	var readVal string
	if err := ReadFn("A", func(v string) error {
		readVal = v
		return nil
	})(s); err != nil || readVal != "2" {
		t.Fatalf("ReadFn: got %q, err=%v", readVal, err)
	}

	if err := s.Create("A", "dup"); err == nil {
		t.Fatal("expected error creating duplicate key")
	}
	if err := s.Update("MISSING", "x"); err == nil {
		t.Fatal("expected error updating missing key")
	}
	if _, err := s.Read("MISSING"); err == nil {
		t.Fatal("expected error reading missing key")
	}
	if err := DeleteFn[string]("A")(s); err != nil {
		t.Fatalf("DeleteFn: %v", err)
	}
	if err := s.Delete("A"); err == nil {
		t.Fatal("expected error deleting already-removed key")
	}
}

func TestStore_WithEnvFileAndGetEnv(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	if err := os.WriteFile(envPath, []byte("FILE_KEY=from_file\nOVERRIDE_KEY=from_file\n"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	s, err := New(WithEnvFile[string](envPath))
	if err != nil {
		t.Fatalf("New(WithEnvFile): %v", err)
	}
	if _, err := s.Read("FILE_KEY"); err != nil {
		t.Fatalf("Read(FILE_KEY): %v", err)
	}

	t.Setenv("OVERRIDE_KEY", "from_env")
	if got, err := s.GetEnv("OVERRIDE_KEY"); err != nil || got != "from_env" {
		t.Fatalf("GetEnv(OVERRIDE_KEY) = %q, %v; want from_env", got, err)
	}
	if got, err := s.GetEnv("FILE_KEY"); err != nil || got != "from_file" {
		t.Fatalf("GetEnv(FILE_KEY) = %q, %v; want from_file", got, err)
	}
	if _, err := s.GetEnv("ABSENT_KEY"); err == nil {
		t.Fatal("expected error for absent env variable")
	}
	if _, err := New(WithEnvFile[string](filepath.Join(dir, "nope.env"))); err == nil {
		t.Fatal("expected error for missing env file in WithEnvFile")
	}
}
