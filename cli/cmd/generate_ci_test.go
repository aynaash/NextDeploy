package cmd

import (
	"strings"
	"testing"

	"github.com/aynaash/nextdeploy/shared/config"
)

func TestResolveCITarget(t *testing.T) {
	tests := []struct {
		name         string
		cfg          *config.NextDeployConfig
		wantTarget   string
		wantProvider string
	}{
		{
			name: "explicit vps target",
			cfg: &config.NextDeployConfig{
				TargetType: "vps",
			},
			wantTarget:   "vps",
			wantProvider: "",
		},
		{
			name: "default target is vps",
			cfg: &config.NextDeployConfig{
				App: config.AppConfig{Name: "my-app"},
			},
			wantTarget:   "vps",
			wantProvider: "",
		},
		{
			name: "serverless cloudflare",
			cfg: &config.NextDeployConfig{
				TargetType: "serverless",
				Serverless: &config.ServerlessConfig{
					Provider: "cloudflare",
				},
			},
			wantTarget:   "serverless",
			wantProvider: "cloudflare",
		},
		{
			name: "serverless cloudflare inferred without target_type",
			cfg: &config.NextDeployConfig{
				Serverless: &config.ServerlessConfig{
					Provider: "cloudflare",
				},
			},
			wantTarget:   "serverless",
			wantProvider: "cloudflare",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotTarget, gotProvider := resolveCITarget(tc.cfg)
			if gotTarget != tc.wantTarget {
				t.Errorf("resolveCITarget() target = %q, want %q", gotTarget, tc.wantTarget)
			}
			if gotProvider != tc.wantProvider {
				t.Errorf("resolveCITarget() provider = %q, want %q", gotProvider, tc.wantProvider)
			}
		})
	}
}

func TestSecretsForTarget(t *testing.T) {
	t.Run("vps secrets include SSH key and hardening hints", func(t *testing.T) {
		secrets := secretsForTarget("vps", "")
		joined := strings.Join(secrets, " ")
		for _, required := range []string{"SSH_PRIVATE_KEY", "SSH_KNOWN_HOSTS", "SSH_KEY_PASSPHRASE", "NEXTDEPLOY_STRICT_HOST_KEY"} {
			if !strings.Contains(joined, required) {
				t.Errorf("VPS secrets missing %q: got %v", required, secrets)
			}
		}
	})

	t.Run("cloudflare secrets include token and CF/R2 credentials", func(t *testing.T) {
		secrets := secretsForTarget("serverless", "cloudflare")
		joined := strings.Join(secrets, " ")
		for _, required := range []string{"CLOUDFLARE_API_TOKEN", "CLOUDFLARE_ACCOUNT_ID", "NEXTDEPLOY_CF_STATE_KEY", "R2_ACCESS_KEY_ID"} {
			if !strings.Contains(joined, required) {
				t.Errorf("Cloudflare secrets missing %q: got %v", required, secrets)
			}
		}
	})
}

func TestNormalizeCIProvider(t *testing.T) {
	tests := []struct {
		input     string
		want      string
		wantError bool
	}{
		{"", "github-actions", false},
		{"github-actions", "github-actions", false},
		{"github", "github-actions", false},
		{"GITHUB", "github-actions", false},
		{"gitlab-ci", "gitlab-ci", false},
		{"gitlab", "gitlab-ci", false},
		{"GitLab", "gitlab-ci", false},
		{"circleci", "", true},
		{"travis", "", true},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got, err := normalizeCIProvider(tc.input)
			if tc.wantError && err == nil {
				t.Errorf("normalizeCIProvider(%q) expected error, got nil", tc.input)
			}
			if !tc.wantError && err != nil {
				t.Errorf("normalizeCIProvider(%q) unexpected error: %v", tc.input, err)
			}
			if got != tc.want {
				t.Errorf("normalizeCIProvider(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}
