package cmd

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/aynaash/nextdeploy/shared/config"
)

func TestStatusTargetResolution(t *testing.T) {
	tests := []struct {
		name       string
		cfg        config.NextDeployConfig
		metaTarget string
		want       string
	}{
		{
			name: "explicit target_type serverless",
			cfg: config.NextDeployConfig{
				TargetType: "serverless",
			},
			want: "serverless",
		},
		{
			name: "explicit target serverless alias",
			cfg: config.NextDeployConfig{
				Target: "serverless",
			},
			want: "serverless",
		},
		{
			name: "inferred from serverless block",
			cfg: config.NextDeployConfig{
				Serverless: &config.ServerlessConfig{
					Provider: "cloudflare",
				},
			},
			want: "serverless",
		},
		{
			name: "default to vps when empty",
			cfg: config.NextDeployConfig{
				App: config.AppConfig{
					Name: "test-app",
				},
			},
			want: "vps",
		},
		{
			name: "explicit vps target_type",
			cfg: config.NextDeployConfig{
				TargetType: "vps",
			},
			want: "vps",
		},
		{
			name: "fallback to metaTarget",
			cfg:  config.NextDeployConfig{},
			metaTarget: "serverless",
			want:       "serverless",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.cfg.ResolveTargetType(tc.metaTarget)
			if got != tc.want {
				t.Errorf("ResolveTargetType() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestProbeEndpoint(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	}))
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	res := probeEndpoint(ctx, ts.URL)
	if res.err != nil {
		t.Fatalf("probeEndpoint returned error: %v", res.err)
	}
	if res.statusCode != http.StatusOK {
		t.Errorf("probeEndpoint statusCode = %d, want %d", res.statusCode, http.StatusOK)
	}
}

func TestProbeEndpointError(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	// Probe non-routable address
	res := probeEndpoint(ctx, "http://192.0.2.1:1")
	if res.err == nil && res.statusCode < 400 {
		t.Errorf("expected error or failure probing unreachable endpoint, got: %v", res)
	}
}
