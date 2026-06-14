package config

import (
	"testing"

	"gopkg.in/yaml.v3"
)

// TestDomainConfigUnmarshalScalar guards backward compatibility: existing
// configs with a bare `domain: example.com` must keep parsing.
func TestDomainConfigUnmarshalScalar(t *testing.T) {
	var app AppConfig
	if err := yaml.Unmarshal([]byte("name: web\ndomain: example.com\n"), &app); err != nil {
		t.Fatalf("unmarshal scalar domain: %v", err)
	}
	if app.Domain.Name != "example.com" {
		t.Errorf("Name = %q, want example.com", app.Domain.Name)
	}
	if app.Domain.Provider != "" || app.Domain.DNS != "" || app.Domain.Zone != "" {
		t.Errorf("scalar form should leave provider/dns/zone empty, got %+v", app.Domain)
	}
}

func TestDomainConfigUnmarshalBlock(t *testing.T) {
	in := `name: web
domain:
  name: example.com
  provider: cloudflare
  dns: auto
  zone: example.com
`
	var app AppConfig
	if err := yaml.Unmarshal([]byte(in), &app); err != nil {
		t.Fatalf("unmarshal block domain: %v", err)
	}
	want := DomainConfig{Name: "example.com", Provider: "cloudflare", DNS: "auto", Zone: "example.com"}
	if app.Domain != want {
		t.Errorf("got %+v, want %+v", app.Domain, want)
	}
}

func TestDomainConfigMarshalRoundTrip(t *testing.T) {
	tests := []struct {
		name string
		in   DomainConfig
	}{
		{"scalar-only", DomainConfig{Name: "example.com"}},
		{"full-block", DomainConfig{Name: "example.com", Provider: "namecheap", DNS: "manual", Zone: "example.com"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := yaml.Marshal(tt.in)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			var got DomainConfig
			if err := yaml.Unmarshal(out, &got); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if got != tt.in {
				t.Errorf("round trip: got %+v, want %+v (yaml: %s)", got, tt.in, out)
			}
		})
	}
}

// TestDomainConfigMarshalCompact asserts a name-only domain serializes back to
// the compact scalar form rather than an expanded block.
func TestDomainConfigMarshalCompact(t *testing.T) {
	out, err := yaml.Marshal(DomainConfig{Name: "example.com"})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if got := string(out); got != "example.com\n" {
		t.Errorf("compact form = %q, want %q", got, "example.com\n")
	}
}
