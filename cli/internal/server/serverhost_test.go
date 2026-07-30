package server

import (
	"strings"
	"testing"

	"github.com/aynaash/nextdeploy/shared/config"
)

// The bug this guards: GetDeploymentServer returns the server NAME, and callers
// that need an address (dns.md's A record, the HTML report's "Server IP") used
// to pass that name straight through — telling operators to point their domain
// at the string "production". ServerHost is the resolution step; these cases
// pin the contract that it returns the host and never silently falls back to
// the name.
func TestServerHost(t *testing.T) {
	s := &ServerStruct{
		config: &config.NextDeployConfig{
			Servers: []config.ServerConfig{
				{Name: "production", Host: "203.0.113.10"},
				{Name: "staging", Host: "staging.example.com"},
				{Name: "hostless", Host: ""},
			},
		},
	}

	t.Run("resolves the configured host, not the name", func(t *testing.T) {
		got, err := s.ServerHost("production")
		if err != nil {
			t.Fatalf("ServerHost(production) returned error: %v", err)
		}
		if got != "203.0.113.10" {
			t.Errorf("ServerHost(production) = %q, want the host 203.0.113.10", got)
		}
	})

	t.Run("resolves a non-first server", func(t *testing.T) {
		got, err := s.ServerHost("staging")
		if err != nil {
			t.Fatalf("ServerHost(staging) returned error: %v", err)
		}
		if got != "staging.example.com" {
			t.Errorf("ServerHost(staging) = %q, want staging.example.com", got)
		}
	})

	t.Run("errors on an unknown server rather than echoing the name", func(t *testing.T) {
		got, err := s.ServerHost("nope")
		if err == nil {
			t.Fatalf("ServerHost(nope) = %q, want an error", got)
		}
		if got != "" {
			t.Errorf("ServerHost(nope) returned %q alongside its error; want empty", got)
		}
		if !strings.Contains(err.Error(), "nope") {
			t.Errorf("error %q does not name the missing server", err)
		}
	})

	t.Run("errors when the entry has no host", func(t *testing.T) {
		if _, err := s.ServerHost("hostless"); err == nil {
			t.Fatal("ServerHost(hostless) succeeded; want an error for an empty host")
		}
	})

	t.Run("errors when no config is loaded", func(t *testing.T) {
		empty := &ServerStruct{}
		if _, err := empty.ServerHost("production"); err == nil {
			t.Fatal("ServerHost on a config-less ServerStruct succeeded; want an error")
		}
	})
}
