// Package protection builds the edge-guard runtime config from a user's
// protection spec. It is deliberately decoupled from the config (YAML) layer —
// callers map config.CFProtection → protection.Config — so the normalizer and
// the emitted JSON contract can be unit-tested in isolation and reused by the
// nextcompile runtime without importing the whole config package.
//
// The output (Runtime) is serialized to _nextdeploy/protection.json and read by
// runtime_src/guard.mjs at the edge. All defaults are resolved here so the JS
// side never has to guess.
package protection

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
)

// Config is the decoupled mirror of config.CFProtection.
type Config struct {
	Enabled     bool
	PublicPaths []string
	Auth        *Auth
	RateLimit   *RateLimit
	Allow       []string
	Deny        []string
}

type Auth struct {
	SecretEnv      string
	CookieName     string
	ProtectedPaths []string
	LoginPath      string
}

type RateLimit struct {
	KVBinding         string
	RequestsPerMinute int
	Paths             []string
}

// Defaults applied when the user leaves a field blank.
const (
	DefaultSecretEnv    = "AUTH_SECRET"
	DefaultCookieName   = "session"
	DefaultLoginPath    = "/login"
	DefaultKVBinding    = "RATE_LIMIT"
	DefaultRequestsPerM = 60
)

// alwaysPublic paths must never be guarded — the guard runs ahead of static
// asset serving, so blocking these would break the app shell itself.
var alwaysPublic = []string{"/_next/*", "/favicon.ico", "/robots.txt"}

// Runtime is the normalized, fully-defaulted shape guard.mjs consumes.
type Runtime struct {
	Version     int          `json:"version"`
	PublicPaths []string     `json:"publicPaths"`
	Allow       []string     `json:"allow,omitempty"`
	Deny        []string     `json:"deny,omitempty"`
	Auth        *AuthRT      `json:"auth,omitempty"`
	RateLimit   *RateLimitRT `json:"rateLimit,omitempty"`
}

type AuthRT struct {
	SecretEnv      string   `json:"secretEnv"`
	CookieName     string   `json:"cookieName"`
	LoginPath      string   `json:"loginPath"`
	ProtectedPaths []string `json:"protectedPaths"`
}

type RateLimitRT struct {
	KVBinding         string   `json:"kvBinding"`
	RequestsPerMinute int      `json:"requestsPerMinute"`
	Paths             []string `json:"paths"`
}

// BuildRuntime validates c and resolves defaults. Returns (nil, nil) when
// protection is absent or disabled (caller emits no guard). Returns an error
// when Enabled but no actual rule (auth / rate-limit / allow / deny) is set —
// a guard that does nothing is almost always a config mistake.
func BuildRuntime(c *Config) (*Runtime, error) {
	if c == nil || !c.Enabled {
		return nil, nil
	}
	if c.Auth == nil && c.RateLimit == nil && len(nonEmpty(c.Allow)) == 0 && len(nonEmpty(c.Deny)) == 0 {
		return nil, errors.New("protection enabled but no rules set (configure auth, rate_limit, allow, or deny)")
	}

	rt := &Runtime{
		Version: 1,
		Allow:   nonEmpty(c.Allow),
		Deny:    nonEmpty(c.Deny),
	}

	public := append([]string{}, alwaysPublic...)
	public = append(public, nonEmpty(c.PublicPaths)...)

	if c.Auth != nil {
		auth := &AuthRT{
			SecretEnv:      defaultStr(c.Auth.SecretEnv, DefaultSecretEnv),
			CookieName:     defaultStr(c.Auth.CookieName, DefaultCookieName),
			LoginPath:      defaultStr(c.Auth.LoginPath, DefaultLoginPath),
			ProtectedPaths: defaultSlice(nonEmpty(c.Auth.ProtectedPaths), []string{"/*"}),
		}
		// The login path must be public, otherwise redirecting an
		// unauthenticated user there loops forever.
		public = append(public, auth.LoginPath)
		rt.Auth = auth
	}

	if c.RateLimit != nil {
		rpm := c.RateLimit.RequestsPerMinute
		if rpm < 0 {
			return nil, fmt.Errorf("rate_limit.requests_per_minute must be >= 0, got %d", rpm)
		}
		if rpm == 0 {
			rpm = DefaultRequestsPerM
		}
		rt.RateLimit = &RateLimitRT{
			KVBinding:         defaultStr(c.RateLimit.KVBinding, DefaultKVBinding),
			RequestsPerMinute: rpm,
			Paths:             defaultSlice(nonEmpty(c.RateLimit.Paths), []string{"/*"}),
		}
	}

	rt.PublicPaths = dedup(public)
	return rt, nil
}

// JSON renders the runtime config deterministically (stable key order via the
// struct field order; slices are already deduped/sorted where it matters).
func (r *Runtime) JSON() ([]byte, error) {
	return json.MarshalIndent(r, "", "  ")
}

// --- helpers -----------------------------------------------------------------

func defaultStr(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

func defaultSlice(v, def []string) []string {
	if len(v) == 0 {
		return def
	}
	return v
}

// nonEmpty returns the input with blank entries removed (nil if none remain).
func nonEmpty(in []string) []string {
	var out []string
	for _, s := range in {
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

// dedup removes duplicates and sorts for a stable, deterministic emission.
func dedup(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}
