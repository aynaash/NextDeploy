package config

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func sampleConfig() *NextDeployConfig {
	return &NextDeployConfig{
		Version:    "1",
		TargetType: "serverless",
		App: AppConfig{
			Name:               "demo",
			Port:               3000,
			Environment:        "production",
			DeletionProtection: true,
		},
	}
}

func TestSave_WritesAndReloads(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, ConfigFile)
	cfg := sampleConfig()

	if err := Save(cfg, p); err != nil {
		t.Fatalf("Save: %v", err)
	}

	info, err := os.Stat(p)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("file mode = %o, want 0600", perm)
	}

	t.Chdir(dir)
	got, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !reflect.DeepEqual(got, cfg) {
		t.Fatalf("round-trip mismatch:\n got %+v\nwant %+v", got, cfg)
	}
}

func TestLoad_Errors(t *testing.T) {
	t.Run("missing file", func(t *testing.T) {
		t.Chdir(t.TempDir())
		cfg, err := Load()
		if err == nil {
			t.Fatal("want error for missing file")
		}
		if !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("want os.ErrNotExist, got %v", err)
		}
		if cfg != nil {
			t.Fatalf("want nil cfg on error, got %+v", cfg)
		}
	})

	t.Run("invalid yaml", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, ConfigFile), []byte("::: not yaml :::"), 0o600); err != nil {
			t.Fatal(err)
		}
		t.Chdir(dir)
		cfg, err := Load()
		if err == nil {
			t.Fatal("want parse error for invalid yaml")
		}
		if cfg != nil {
			t.Fatalf("want nil cfg on parse error, got %+v", cfg)
		}
	})

	t.Run("empty file is a zero-value config", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, ConfigFile), []byte(""), 0o600); err != nil {
			t.Fatal(err)
		}
		t.Chdir(dir)
		cfg, err := Load()
		if err != nil {
			t.Fatalf("empty file: unexpected error %v", err)
		}
		if cfg == nil {
			t.Fatal("want non-nil cfg for empty doc")
		}
	})

	t.Run("valid minimal parses fields", func(t *testing.T) {
		dir := t.TempDir()
		yml := "version: \"1\"\napp:\n  name: demo\n  port: 3000\n"
		if err := os.WriteFile(filepath.Join(dir, ConfigFile), []byte(yml), 0o600); err != nil {
			t.Fatal(err)
		}
		t.Chdir(dir)
		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if cfg.App.Name != "demo" || cfg.App.Port != 3000 || cfg.Version != "1" {
			t.Fatalf("parsed fields wrong: %+v", cfg)
		}
	})
}

// LoadConfig duplicates Load; pin that they agree so an edit to one is caught.
func TestLoadConfig_MatchesLoad(t *testing.T) {
	dir := t.TempDir()
	if err := Save(sampleConfig(), filepath.Join(dir, ConfigFile)); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)

	a, errA := Load()
	b, errB := LoadConfig()
	if errA != nil || errB != nil {
		t.Fatalf("Load=%v LoadConfig=%v", errA, errB)
	}
	if !reflect.DeepEqual(a, b) {
		t.Fatalf("Load and LoadConfig disagree:\n%+v\n%+v", a, b)
	}
}

func TestSave_Errors(t *testing.T) {
	t.Run("unwritable path", func(t *testing.T) {
		dir := t.TempDir()
		// Make a regular file, then try to write "under" it as if it were a dir.
		blocker := filepath.Join(dir, "blocker")
		if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
		err := Save(sampleConfig(), filepath.Join(blocker, "nextdeploy.yml"))
		if err == nil {
			t.Fatal("want error writing under a file path")
		}
	})
}

// SaveConfig has reversed arg order vs Save: (path, cfg). Lock it.
func TestSaveConfig_ArgOrder(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, ConfigFile)
	if err := SaveConfig(p, sampleConfig()); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}
	t.Chdir(dir)
	got, err := Load()
	if err != nil {
		t.Fatalf("Load after SaveConfig: %v", err)
	}
	if got.App.Name != "demo" {
		t.Fatalf("SaveConfig round-trip wrong: %+v", got)
	}
}
