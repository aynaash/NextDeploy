package cmd

import (
	"testing"

	"github.com/aynaash/nextdeploy/shared/config"
)

func TestApplyEnvironmentOverride(t *testing.T) {
	t.Run("empty keeps the configured value", func(t *testing.T) {
		cfg := &config.NextDeployConfig{}
		cfg.App.Environment = "production"
		if err := applyEnvironmentOverride(cfg, ""); err != nil {
			t.Fatal(err)
		}
		if cfg.App.Environment != "production" {
			t.Errorf("environment = %q, want production", cfg.App.Environment)
		}
	})

	t.Run("override wins", func(t *testing.T) {
		cfg := &config.NextDeployConfig{}
		cfg.App.Environment = "production"
		if err := applyEnvironmentOverride(cfg, "pr-42"); err != nil {
			t.Fatal(err)
		}
		if cfg.App.Environment != "pr-42" {
			t.Errorf("environment = %q, want pr-42", cfg.App.Environment)
		}
	})

	t.Run("surrounding whitespace is trimmed", func(t *testing.T) {
		cfg := &config.NextDeployConfig{}
		if err := applyEnvironmentOverride(cfg, "  staging  "); err != nil {
			t.Fatal(err)
		}
		if cfg.App.Environment != "staging" {
			t.Errorf("environment = %q, want staging", cfg.App.Environment)
		}
	})

	// The value is concatenated into Cloudflare resource names. Rejecting
	// early beats letting sanitizeCFName mangle it — otherwise `-e Foo/Bar`
	// and `-e foo-bar` would silently collide on one stack.
	t.Run("rejects values that would be mangled", func(t *testing.T) {
		for _, bad := range []string{"Foo/Bar", "PR-42", "pr_42", "-lead", "trail-", "a b", ""} {
			if bad == "" {
				continue // empty is the documented no-op
			}
			cfg := &config.NextDeployConfig{}
			cfg.App.Environment = "production"
			if err := applyEnvironmentOverride(cfg, bad); err == nil {
				t.Errorf("expected %q to be rejected", bad)
			}
			if cfg.App.Environment != "production" {
				t.Errorf("%q: config mutated despite the error", bad)
			}
		}
	})

	t.Run("accepts realistic environment names", func(t *testing.T) {
		for _, ok := range []string{"pr-42", "staging", "production", "pr-1", "a", "preview-2024-01"} {
			cfg := &config.NextDeployConfig{}
			if err := applyEnvironmentOverride(cfg, ok); err != nil {
				t.Errorf("%q should be accepted: %v", ok, err)
			}
		}
	})
}
