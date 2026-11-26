package cmd

import (
	"testing"
)

func TestResolveCredsTarget(t *testing.T) {
	tests := []struct {
		name         string
		providerFlag string
		serverFlag   string
		args         []string
		wantProvider string
		wantKey      string
		wantErr      bool
	}{
		{
			name:         "vps positional shorthand",
			args:         []string{"vps"},
			wantProvider: "vps",
			wantKey:      "vps",
		},
		{
			name:         "vps via --provider flag",
			providerFlag: "vps",
			wantProvider: "vps",
			wantKey:      "vps",
		},
		{
			name:         "vps with --server scopes storeKey",
			args:         []string{"vps"},
			serverFlag:   "Staging-01",
			wantProvider: "vps",
			wantKey:      "vps-staging-01",
		},
		{
			name:         "cloudflare positional",
			args:         []string{"cloudflare"},
			wantProvider: "cloudflare",
			wantKey:      "cloudflare",
		},
		{
			name:         "aws flag",
			providerFlag: "aws",
			wantProvider: "aws",
			wantKey:      "aws",
		},
		{
			name:    "missing provider",
			wantErr: true,
		},
		{
			name:    "unknown provider",
			args:    []string{"gcp"},
			wantErr: true,
		},
		{
			name:         "conflicting arg and flag",
			providerFlag: "aws",
			args:         []string{"vps"},
			wantErr:      true,
		},
		{
			name:       "--server rejected for non-vps provider",
			args:       []string{"cloudflare"},
			serverFlag: "prod",
			wantErr:    true,
		},
		{
			name:       "invalid server path traversal rejected",
			args:       []string{"vps"},
			serverFlag: "../evil",
			wantErr:    true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotProvider, gotKey, err := resolveCredsTarget(tc.providerFlag, tc.serverFlag, tc.args)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("resolveCredsTarget(%q, %q, %v) = (%q, %q), want error",
						tc.providerFlag, tc.serverFlag, tc.args, gotProvider, gotKey)
				}
				return
			}
			if err != nil {
				t.Fatalf("resolveCredsTarget(%q, %q, %v) unexpected error: %v",
					tc.providerFlag, tc.serverFlag, tc.args, err)
			}
			if gotProvider != tc.wantProvider || gotKey != tc.wantKey {
				t.Fatalf("resolveCredsTarget = (%q, %q), want (%q, %q)",
					gotProvider, gotKey, tc.wantProvider, tc.wantKey)
			}
		})
	}
}

func TestVPSCredsSchema(t *testing.T) {
	schema, ok := providerSchemas["vps"]
	if !ok {
		t.Fatal("providerSchemas missing \"vps\"")
	}
	wantKeys := map[string]bool{
		"host":           true,
		"username":       false,
		"key_path":       false,
		"key_passphrase": false,
	}
	seen := make(map[string]bool)
	for _, f := range schema {
		req, expected := wantKeys[f.Key]
		if !expected {
			t.Errorf("unexpected vps field %q", f.Key)
			continue
		}
		if f.Required != req {
			t.Errorf("vps field %q Required = %v, want %v", f.Key, f.Required, req)
		}
		seen[f.Key] = true
	}
	for k := range wantKeys {
		if !seen[k] {
			t.Errorf("vps schema missing field %q", k)
		}
	}
}
