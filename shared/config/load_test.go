package config

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// sampleConfig is deliberately more than the minimum: it carries a Serverless
// block with Cloudflare D1/R2/KV bindings so Save->Load exercises the nested
// pointer/slice types, not just the flat top-level scalars.
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
		Serverless: &ServerlessConfig{
			Provider: "cloudflare",
			Region:   "auto",
			Cloudflare: &CloudflareConfig{
				CompatibilityDate:  "2025-04-01",
				CompatibilityFlags: []string{"nodejs_compat_v2"},
				Bindings: &CFBindings{
					D1: []CFD1Binding{{Name: "DB", ID: "11111111-2222-3333-4444-555555555555"}},
					R2: []CFR2Binding{{Name: "ASSETS", Bucket: "demo-assets"}},
					KV: []CFKVBinding{{Name: "RATE_LIMIT", Ref: "demo-rate-limit"}},
				},
			},
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

	// DeepEqual on the whole struct would still pass if the bindings had been
	// dropped on BOTH sides by a bad yaml tag, so assert them explicitly.
	b := got.Serverless.Cloudflare.Bindings
	if len(b.D1) != 1 || b.D1[0].Name != "DB" || b.D1[0].ID == "" {
		t.Errorf("D1 binding did not survive round-trip: %+v", b.D1)
	}
	if len(b.R2) != 1 || b.R2[0].Bucket != "demo-assets" {
		t.Errorf("R2 binding did not survive round-trip: %+v", b.R2)
	}
	if len(b.KV) != 1 || b.KV[0].Ref != "demo-rate-limit" {
		t.Errorf("KV binding did not survive round-trip: %+v", b.KV)
	}
}

// A block-form domain whose name is a sequence, not a scalar, hits the
// decode-error branch of DomainConfig.UnmarshalYAML — the one path that both
// the scalar form and the happy block form skip past.
func TestLoad_UndecodableDomainBlock(t *testing.T) {
	dir := t.TempDir()
	yml := "version: \"1\"\napp:\n  name: demo\n  domain:\n    name: [1, 2]\n"
	if err := os.WriteFile(filepath.Join(dir, ConfigFile), []byte(yml), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	cfg, err := Load()
	if err == nil {
		t.Fatal("want decode error for a non-scalar domain name")
	}
	if cfg != nil {
		t.Fatalf("want nil cfg on decode error, got %+v", cfg)
	}
}

// DomainConfig has a hand-written marshaler pair: it collapses to a bare scalar
// when only Name is set and expands to a block otherwise. Both directions have
// to survive Save->Load or older scalar-form configs break.
func TestSave_DomainFormRoundTrips(t *testing.T) {
	tests := []struct {
		name       string
		domain     DomainConfig
		wantScalar bool
	}{
		{
			name:       "name only collapses to scalar",
			domain:     DomainConfig{Name: "example.com"},
			wantScalar: true,
		},
		{
			name:   "full block form",
			domain: DomainConfig{Name: "example.com", Provider: "cloudflare", DNS: "auto", Zone: "example.com"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			p := filepath.Join(dir, ConfigFile)
			cfg := sampleConfig()
			cfg.App.Domain = tt.domain

			if err := Save(cfg, p); err != nil {
				t.Fatalf("Save: %v", err)
			}

			raw, err := os.ReadFile(p)
			if err != nil {
				t.Fatal(err)
			}
			// The scalar form must not emit a nested "name:" key.
			gotScalar := strings.Contains(string(raw), "domain: example.com")
			if gotScalar != tt.wantScalar {
				t.Errorf("scalar form = %v, want %v; yaml:\n%s", gotScalar, tt.wantScalar, raw)
			}

			t.Chdir(dir)
			got, err := Load()
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if !reflect.DeepEqual(got.App.Domain, tt.domain) {
				t.Fatalf("domain round-trip: got %+v, want %+v", got.App.Domain, tt.domain)
			}
		})
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

	t.Run("empty file is rejected for a missing app.name", func(t *testing.T) {
		// Every command that calls Load operates on a named app, so a config
		// without app.name is not a usable zero value — catch it here rather
		// than three layers down in the daemon.
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, ConfigFile), []byte(""), 0o600); err != nil {
			t.Fatal(err)
		}
		t.Chdir(dir)
		cfg, err := Load()
		if err == nil {
			t.Fatal("want error for a config with no app.name")
		}
		if !strings.Contains(err.Error(), "app.name is required") {
			t.Fatalf("error should name the missing field, got %v", err)
		}
		if cfg != nil {
			t.Fatalf("want nil cfg on validation error, got %+v", cfg)
		}
	})

	t.Run("domain as app.name is rejected", func(t *testing.T) {
		dir := t.TempDir()
		yml := "version: \"1\"\napp:\n  name: ressencesystems.com\n  port: 3000\n"
		if err := os.WriteFile(filepath.Join(dir, ConfigFile), []byte(yml), 0o600); err != nil {
			t.Fatal(err)
		}
		t.Chdir(dir)
		if _, err := Load(); err == nil {
			t.Fatal("want error for a domain in app.name")
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

	// Documented actual behavior, not a wish: a nil config is NOT rejected —
	// yaml marshals it to "null" and Save reports success. The damage surfaces
	// later, at Load, as a confusing app-name error. Pinned so that if anyone
	// adds a nil guard, they do it deliberately and update this test.
	t.Run("nil cfg writes null and does not error", func(t *testing.T) {
		dir := t.TempDir()
		p := filepath.Join(dir, ConfigFile)
		if err := Save(nil, p); err != nil {
			t.Fatalf("Save(nil) = %v, want nil error", err)
		}
		raw, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		if string(raw) != "null\n" {
			t.Fatalf("Save(nil) wrote %q, want %q", raw, "null\n")
		}

		t.Chdir(dir)
		if _, err := Load(); err == nil {
			t.Fatal("Load of a null config should fail app-name validation")
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
