package cmd

import (
	"testing"

	"github.com/aynaash/nextdeploy/shared/config"
)

func TestTargetResolution(t *testing.T) {
	tests := []struct {
		name       string
		cfg        *config.NextDeployConfig
		wantDeploy string
	}{
		{
			name: "vps explicit",
			cfg: &config.NextDeployConfig{
				TargetType: "vps",
			},
			wantDeploy: "vps",
		},
		{
			name: "cloudflare serverless",
			cfg: &config.NextDeployConfig{
				TargetType: "serverless",
				Serverless: &config.ServerlessConfig{
					Provider: "cloudflare",
				},
			},
			wantDeploy: "cloudflare",
		},
		{
			name: "aws serverless",
			cfg: &config.NextDeployConfig{
				TargetType: "serverless",
				Serverless: &config.ServerlessConfig{
					Provider: "aws",
				},
			},
			wantDeploy: "aws",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			target, provider := resolveCITarget(tc.cfg)
			deployType := target
			if target == "serverless" && provider != "" {
				deployType = provider
			}
			if deployType != tc.wantDeploy {
				t.Errorf("deployment type = %q, want %q", deployType, tc.wantDeploy)
			}
		})
	}
}
