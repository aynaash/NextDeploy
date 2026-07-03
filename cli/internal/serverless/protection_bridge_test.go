package serverless

import (
	"slices"
	"testing"

	"github.com/aynaash/nextdeploy/shared/config"
)

func TestBuildProtectionRuntime_NilWhenAbsent(t *testing.T) {
	cases := []*config.NextDeployConfig{
		nil,
		{},
		{Serverless: &config.ServerlessConfig{}},
		{Serverless: &config.ServerlessConfig{Cloudflare: &config.CloudflareConfig{}}},
	}
	for i, c := range cases {
		rt, err := buildProtectionRuntime(c)
		if err != nil {
			t.Fatalf("case %d: %v", i, err)
		}
		if rt != nil {
			t.Errorf("case %d: expected nil runtime, got %+v", i, rt)
		}
	}
}

func TestBuildProtectionRuntime_MapsAllFields(t *testing.T) {
	cfg := &config.NextDeployConfig{
		Serverless: &config.ServerlessConfig{
			Cloudflare: &config.CloudflareConfig{
				Protection: &config.CFProtection{
					Enabled:     true,
					PublicPaths: []string{"/marketing/*"},
					Deny:        []string{"6.6.6.6"},
					Auth: &config.CFAuth{
						SecretEnv:      "SESSION_KEY",
						CookieName:     "sid",
						LoginPath:      "/signin",
						ProtectedPaths: []string{"/app/*"},
					},
					RateLimit: &config.CFRateLimit{
						KVBinding:         "RL",
						RequestsPerMinute: 30,
						Paths:             []string{"/api/*"},
					},
				},
			},
		},
	}
	rt, err := buildProtectionRuntime(cfg)
	if err != nil {
		t.Fatalf("buildProtectionRuntime: %v", err)
	}
	if rt == nil {
		t.Fatal("expected non-nil runtime")
	}
	if rt.Auth == nil || rt.Auth.SecretEnv != "SESSION_KEY" || rt.Auth.CookieName != "sid" || rt.Auth.LoginPath != "/signin" {
		t.Errorf("auth mapping wrong: %+v", rt.Auth)
	}
	if rt.RateLimit == nil || rt.RateLimit.KVBinding != "RL" || rt.RateLimit.RequestsPerMinute != 30 {
		t.Errorf("rate-limit mapping wrong: %+v", rt.RateLimit)
	}
	if len(rt.Deny) != 1 || rt.Deny[0] != "6.6.6.6" {
		t.Errorf("deny mapping wrong: %v", rt.Deny)
	}
	if !containsStr(rt.PublicPaths, "/marketing/*") || !containsStr(rt.PublicPaths, "/signin") {
		t.Errorf("public paths missing custom/login entries: %v", rt.PublicPaths)
	}
}

func TestBuildProtectionRuntime_DisabledReturnsNil(t *testing.T) {
	cfg := &config.NextDeployConfig{
		Serverless: &config.ServerlessConfig{
			Cloudflare: &config.CloudflareConfig{
				Protection: &config.CFProtection{Enabled: false, Auth: &config.CFAuth{}},
			},
		},
	}
	rt, err := buildProtectionRuntime(cfg)
	if err != nil {
		t.Fatalf("buildProtectionRuntime: %v", err)
	}
	if rt != nil {
		t.Errorf("disabled protection should yield nil runtime, got %+v", rt)
	}
}

func containsStr(ss []string, want string) bool {
	return slices.Contains(ss, want)
}
