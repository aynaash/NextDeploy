package protection

import (
	"bytes"
	"encoding/json"
	"slices"
	"strings"
	"testing"
)

func TestBuildRuntime_NilOrDisabledReturnsNil(t *testing.T) {
	for _, c := range []*Config{nil, {Enabled: false}, {Enabled: false, Auth: &Auth{}}} {
		rt, err := BuildRuntime(c)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if rt != nil {
			t.Errorf("expected nil runtime for %+v, got %+v", c, rt)
		}
	}
}

func TestBuildRuntime_EnabledWithoutRulesErrors(t *testing.T) {
	_, err := BuildRuntime(&Config{Enabled: true})
	if err == nil || !strings.Contains(err.Error(), "no rules set") {
		t.Fatalf("expected no-rules error, got %v", err)
	}
}

func TestBuildRuntime_AuthDefaultsApplied(t *testing.T) {
	rt, err := BuildRuntime(&Config{Enabled: true, Auth: &Auth{}})
	if err != nil {
		t.Fatalf("BuildRuntime: %v", err)
	}
	if rt.Auth == nil {
		t.Fatal("auth runtime missing")
	}
	if rt.Auth.SecretEnv != DefaultSecretEnv || rt.Auth.CookieName != DefaultCookieName || rt.Auth.LoginPath != DefaultLoginPath {
		t.Errorf("auth defaults wrong: %+v", rt.Auth)
	}
	if len(rt.Auth.ProtectedPaths) != 1 || rt.Auth.ProtectedPaths[0] != "/*" {
		t.Errorf("protected paths default wrong: %v", rt.Auth.ProtectedPaths)
	}
}

func TestBuildRuntime_AuthOverrides(t *testing.T) {
	rt, err := BuildRuntime(&Config{Enabled: true, Auth: &Auth{
		SecretEnv:      "SESSION_KEY",
		CookieName:     "sid",
		LoginPath:      "/signin",
		ProtectedPaths: []string{"/app/*", "/account"},
	}})
	if err != nil {
		t.Fatalf("BuildRuntime: %v", err)
	}
	if rt.Auth.SecretEnv != "SESSION_KEY" || rt.Auth.CookieName != "sid" || rt.Auth.LoginPath != "/signin" {
		t.Errorf("overrides not applied: %+v", rt.Auth)
	}
	if len(rt.Auth.ProtectedPaths) != 2 {
		t.Errorf("protected paths not honored: %v", rt.Auth.ProtectedPaths)
	}
}

func TestBuildRuntime_LoginPathBecomesPublic(t *testing.T) {
	rt, err := BuildRuntime(&Config{Enabled: true, Auth: &Auth{LoginPath: "/signin"}})
	if err != nil {
		t.Fatalf("BuildRuntime: %v", err)
	}
	if !contains(rt.PublicPaths, "/signin") {
		t.Errorf("login path not auto-added to publicPaths: %v", rt.PublicPaths)
	}
}

func TestBuildRuntime_AlwaysPublicIncluded(t *testing.T) {
	rt, err := BuildRuntime(&Config{Enabled: true, Deny: []string{"1.2.3.4"}})
	if err != nil {
		t.Fatalf("BuildRuntime: %v", err)
	}
	for _, p := range []string{"/_next/*", "/favicon.ico", "/robots.txt"} {
		if !contains(rt.PublicPaths, p) {
			t.Errorf("always-public path %q missing: %v", p, rt.PublicPaths)
		}
	}
}

func TestBuildRuntime_RateLimitDefaultsAndValidation(t *testing.T) {
	rt, err := BuildRuntime(&Config{Enabled: true, RateLimit: &RateLimit{}})
	if err != nil {
		t.Fatalf("BuildRuntime: %v", err)
	}
	if rt.RateLimit.KVBinding != DefaultKVBinding || rt.RateLimit.RequestsPerMinute != DefaultRequestsPerM {
		t.Errorf("rate-limit defaults wrong: %+v", rt.RateLimit)
	}
	if len(rt.RateLimit.Paths) != 1 || rt.RateLimit.Paths[0] != "/*" {
		t.Errorf("rate-limit paths default wrong: %v", rt.RateLimit.Paths)
	}

	if _, err := BuildRuntime(&Config{Enabled: true, RateLimit: &RateLimit{RequestsPerMinute: -1}}); err == nil {
		t.Fatal("expected error for negative requests_per_minute")
	}
}

func TestBuildRuntime_PublicPathsDedupedAndSorted(t *testing.T) {
	rt, err := BuildRuntime(&Config{
		Enabled:     true,
		Deny:        []string{"1.1.1.1"},
		PublicPaths: []string{"/about", "/about", "/", ""},
	})
	if err != nil {
		t.Fatalf("BuildRuntime: %v", err)
	}
	// dedup: "/about" appears once; blank dropped; sorted.
	count := 0
	for _, p := range rt.PublicPaths {
		if p == "/about" {
			count++
		}
		if p == "" {
			t.Error("blank path leaked into publicPaths")
		}
	}
	if count != 1 {
		t.Errorf("publicPaths not deduped: %v", rt.PublicPaths)
	}
	if !sortedAsc(rt.PublicPaths) {
		t.Errorf("publicPaths not sorted: %v", rt.PublicPaths)
	}
}

func TestBuildRuntime_AllowDenyBlankStripped(t *testing.T) {
	rt, err := BuildRuntime(&Config{
		Enabled: true,
		Allow:   []string{"9.9.9.9", ""},
		Deny:    []string{""},
	})
	if err != nil {
		t.Fatalf("BuildRuntime: %v", err)
	}
	if len(rt.Allow) != 1 || rt.Allow[0] != "9.9.9.9" {
		t.Errorf("allow not stripped: %v", rt.Allow)
	}
	if len(rt.Deny) != 0 {
		t.Errorf("deny should be empty after stripping blanks: %v", rt.Deny)
	}
}

func TestRuntimeJSON_ShapeAndKeys(t *testing.T) {
	rt, err := BuildRuntime(&Config{
		Enabled:   true,
		Auth:      &Auth{},
		RateLimit: &RateLimit{},
		Deny:      []string{"6.6.6.6"},
	})
	if err != nil {
		t.Fatalf("BuildRuntime: %v", err)
	}
	raw, err := rt.JSON()
	if err != nil {
		t.Fatalf("JSON: %v", err)
	}
	// Must be valid JSON and round-trip back to the same camelCase keys guard.mjs reads.
	var back map[string]any
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatalf("emitted invalid JSON: %v\n%s", err, raw)
	}
	for _, frag := range []string{
		`"version": 1`, `"publicPaths"`, `"secretEnv": "AUTH_SECRET"`,
		`"cookieName": "session"`, `"kvBinding": "RATE_LIMIT"`,
		`"requestsPerMinute": 60`, `"deny"`,
	} {
		if !strings.Contains(string(raw), frag) {
			t.Errorf("emitted JSON missing %q\n%s", frag, raw)
		}
	}
}

func TestRuntimeJSON_Deterministic(t *testing.T) {
	c := &Config{Enabled: true, Auth: &Auth{}, PublicPaths: []string{"/b", "/a"}}
	rt1, _ := BuildRuntime(c)
	rt2, _ := BuildRuntime(c)
	j1, _ := rt1.JSON()
	j2, _ := rt2.JSON()
	if !bytes.Equal(j1, j2) {
		t.Errorf("JSON not deterministic:\n%s\nvs\n%s", j1, j2)
	}
}

// --- helpers -----------------------------------------------------------------

func contains(ss []string, want string) bool {
	return slices.Contains(ss, want)
}

func sortedAsc(ss []string) bool {
	for i := 1; i < len(ss); i++ {
		if ss[i-1] > ss[i] {
			return false
		}
	}
	return true
}
