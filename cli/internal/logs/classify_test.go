package logs

import (
	"testing"
)

func TestParseOwner(t *testing.T) {
	cases := []struct {
		in      string
		want    Owner
		wantErr bool
	}{
		{"", "", false},
		{"app", OwnerApp, false},
		{"APP", OwnerApp, false},
		{"nextdeploy", OwnerNextDeploy, false},
		{"nd", OwnerNextDeploy, false},
		{"platform", OwnerPlatform, false},
		{"cf", OwnerPlatform, false},
		{"cloudflare", OwnerPlatform, false},
		{"unknown", OwnerUnknown, false},
		{"bogus", "", true},
	}

	for _, tc := range cases {
		got, err := ParseOwner(tc.in)
		if (err != nil) != tc.wantErr {
			t.Errorf("ParseOwner(%q) err = %v, wantErr %v", tc.in, err, tc.wantErr)
			continue
		}
		if got != tc.want {
			t.Errorf("ParseOwner(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestDedupeMessages(t *testing.T) {
	msgs := []string{
		"GET https://nextdeploy.org/?_rsc=LJGu_tk5iPI2Zyzb",
		"⨯ \n    at worker.mjs:5804:5403",
		"⨯     at worker.mjs:5539:9298\n    at async $ (worker.mjs:5539:5737)\n    at async NextNodeServer.renderToResponseWithComponentsImpl (worker.mjs:5791:42997)",
		"    at worker.mjs:5539:9298\n    at async $ (worker.mjs:5539:5737)\n    at async NextNodeServer.renderToResponseWithComponentsImpl (worker.mjs:5791:42997)",
	}

	got := DedupeMessages(msgs)
	if len(got) != 2 {
		t.Fatalf("expected 2 deduped stack messages (echo dropped, duplicate stack collapsed), got %d: %#v", len(got), got)
	}
	if detail := BestDetail(msgs); detail == "" {
		t.Fatalf("expected non-empty BestDetail")
	}

	// If only request echo exists, keep it rather than returning empty.
	echoOnly := DedupeMessages([]string{"GET https://nextdeploy.org/api/health"})
	if len(echoOnly) != 1 {
		t.Fatalf("expected echo-only fallback to preserve 1 entry, got %d", len(echoOnly))
	}
}

func TestClassify(t *testing.T) {
	cases := []struct {
		name      string
		sig       Signal
		wantOwner Owner
		wantCode  string
	}{
		{
			name: "rsc segment prefetch in nextdeploy worker",
			sig: Signal{
				Method: "GET",
				Path:   "/",
				URL:    "https://nextdeploy.org/?_rsc=LJGu_tk5iPI2Zyzb",
				Status: 500,
				Headers: map[string]string{
					"next-router-prefetch":         "1",
					"next-router-segment-prefetch": "/__PAGE__",
				},
				Messages: []string{
					"⨯ \n    at worker.mjs:5804:5403",
					"⨯     at worker.mjs:5539:9298\n    at async NextNodeServer.renderToResponseWithComponentsImpl (worker.mjs:5791:42997)",
				},
			},
			wantOwner: OwnerNextDeploy,
			wantCode:  "nextdeploy_rsc_segment_prefetch",
		},
		{
			name: "externalRequire during docs module evaluation",
			sig: Signal{
				Method: "GET",
				Path:   "/docs/concepts/security",
				URL:    "https://nextdeploy.org/docs/concepts/security?_rsc=CKPo2T6QqhkmtnA0",
				Status: 500,
				Headers: map[string]string{
					"next-router-prefetch":         "1",
					"next-router-segment-prefetch": "/_tree",
				},
				Messages: []string{
					"⨯     at Context.externalRequire [as x] (worker.mjs:3:12269)\n    at module evaluation (worker.mjs:3062:22882)",
				},
			},
			wantOwner: OwnerApp,
			wantCode:  "app_external_require",
		},
		{
			name: "externalRequire internal next/dist module",
			sig: Signal{
				Status: 500,
				Messages: []string{
					"Error: Cannot find module 'next/dist/server/runtime' at Context.externalRequire (worker.mjs:3:12269)",
				},
			},
			wantOwner: OwnerNextDeploy,
			wantCode:  "nextdeploy_external_require",
		},
		{
			name: "nextcompile handler error",
			sig: Signal{
				Status: 500,
				Messages: []string{
					"[nextcompile nextserver handler error] TypeError: something internal",
				},
			},
			wantOwner: OwnerNextDeploy,
			wantCode:  "nextdeploy_runtime_error",
		},
		{
			name: "nextcompile cacheHandler R2 put failed",
			sig: Signal{
				Status: 500,
				Messages: []string{
					"[nextcompile cacheHandler R2 put failed] R2 bucket unavailable",
				},
			},
			wantOwner: OwnerNextDeploy,
			wantCode:  "nextdeploy_cache_r2",
		},
		{
			name: "guard kv binding missing",
			sig: Signal{
				Status: 500,
				Messages: []string{
					`rate_limit kvBinding "RATE_LIMIT" not bound`,
				},
			},
			wantOwner: OwnerNextDeploy,
			wantCode:  "nextdeploy_guard_kv",
		},
		{
			name: "missing .next manifest in vfs",
			sig: Signal{
				Status: 500,
				Messages: []string{
					"ENOENT: no such file or directory, open '.next/routes-manifest.json'",
				},
			},
			wantOwner: OwnerNextDeploy,
			wantCode:  "nextdeploy_vfs_missing",
		},
		{
			name: "app fs read missing content/docs",
			sig: Signal{
				Status: 500,
				Messages: []string{
					"ENOENT: no such file or directory, open '/var/task/content/docs/cli/reference.md'",
				},
			},
			wantOwner: OwnerApp,
			wantCode:  "app_fs_read_missing",
		},
		{
			name: "app getEnv outside request",
			sig: Signal{
				Status: 500,
				Messages: []string{
					"Error: Cloudflare env not available — getEnv() must be called inside a request",
				},
			},
			wantOwner: OwnerApp,
			wantCode:  "app_getenv_outside_request",
		},
		{
			name: "app TypeError exception",
			sig: Signal{
				Status:     500,
				Exceptions: []string{"TypeError: Cannot read properties of undefined (reading 'slug')"},
			},
			wantOwner: OwnerApp,
			wantCode:  "app_runtime_exception",
		},
		{
			name: "app 404",
			sig: Signal{
				Path:   "/does-not-exist",
				Status: 404,
			},
			wantOwner: OwnerApp,
			wantCode:  "app_http_4xx",
		},
		{
			name: "platform exceededCpu",
			sig: Signal{
				Status:  500,
				Outcome: "exceededCpu",
			},
			wantOwner: OwnerPlatform,
			wantCode:  "platform_cpu_limit",
		},
		{
			name: "platform exceededMemory",
			sig: Signal{
				Status:  500,
				Outcome: "exceededMemory",
			},
			wantOwner: OwnerPlatform,
			wantCode:  "platform_memory_limit",
		},
		{
			name: "platform 429 rate limited",
			sig: Signal{
				Status: 429,
			},
			wantOwner: OwnerPlatform,
			wantCode:  "platform_rate_limited",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Classify(tc.sig)
			if got.Owner != tc.wantOwner || got.Code != tc.wantCode {
				t.Fatalf("Classify() = (%s, %s), want (%s, %s); summary=%q",
					got.Owner, got.Code, tc.wantOwner, tc.wantCode, got.Summary)
			}
			if got.Summary == "" || got.Hint == "" {
				t.Errorf("expected non-empty Summary and Hint, got %+v", got)
			}
		})
	}
}
