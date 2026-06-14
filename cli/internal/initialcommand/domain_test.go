package initialcommand

import (
	"strings"
	"testing"

	"github.com/aynaash/nextdeploy/shared/config"
)

func TestApexZone(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"app.example.com", "example.com"},
		{"example.com", "example.com"},
		{"a.b.example.com", "example.com"},
		{"localhost", "localhost"},
		{"example.com.", "example.com"},
	}
	for _, tt := range tests {
		if got := apexZone(tt.in); got != tt.want {
			t.Errorf("apexZone(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestRenderDomainYAML(t *testing.T) {
	t.Run("empty stays a blank scalar", func(t *testing.T) {
		got := renderDomainYAML(config.DomainConfig{})
		if !strings.HasPrefix(got, `  domain: ""`) {
			t.Errorf("empty domain render = %q", got)
		}
	})

	t.Run("name-only stays a scalar", func(t *testing.T) {
		got := renderDomainYAML(config.DomainConfig{Name: "app.example.com"})
		if !strings.HasPrefix(got, "  domain: app.example.com") || strings.Contains(got, "\n") {
			t.Errorf("name-only should be a one-line scalar, got %q", got)
		}
	})

	t.Run("provider expands to a block", func(t *testing.T) {
		got := renderDomainYAML(config.DomainConfig{
			Name: "app.example.com", Provider: "cloudflare", DNS: "auto", Zone: "example.com",
		})
		for _, want := range []string{"  domain:\n", "    name: app.example.com", "    provider: cloudflare", "    dns: auto", "    zone: example.com"} {
			if !strings.Contains(got, want) {
				t.Errorf("block render missing %q in:\n%s", want, got)
			}
		}
	})
}
