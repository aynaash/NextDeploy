// Package secenv enforces a secure split between non-sensitive *vars* and
// *secrets*, and provides commit-safety + redaction helpers. It encodes one
// rule: anything not explicitly public is treated as a secret. Vars become
// plain_text Worker bindings (visible); secrets become secret_text (encrypted
// in the Worker) and are registered for log redaction.
//
// It composes with the existing shared/secrets (encryption at rest, Doppler)
// and shared/sensitive (log scrubbing) packages rather than replacing them.
package secenv

import (
	"sort"
	"strings"

	"github.com/aynaash/nextdeploy/shared/sensitive"
)

// PublicPrefix marks env vars that are intentionally public (Next.js inlines
// these into the client bundle, so they are never secret).
const PublicPrefix = "NEXT_PUBLIC_"

// secretHints are case-insensitive substrings that strongly imply a value is
// sensitive. Used only for *warnings* (e.g. a hinted key sitting in plaintext
// YAML) — classification itself defaults unknown keys to secret regardless.
var secretHints = []string{
	"SECRET", "TOKEN", "PASSWORD", "PASSWD", "PRIVATE",
	"CREDENTIAL", "APIKEY", "API_KEY", "ACCESS_KEY", "DATABASE_URL", "DSN",
}

// Classify splits env into vars (public) and secrets. A key is a var iff it has
// the NEXT_PUBLIC_ prefix or is listed in publicKeys; everything else is a
// secret. Secure-by-default: an unrecognized key is treated as a secret, not
// leaked as a visible plain_text binding. Returned maps are never nil.
func Classify(env map[string]string, publicKeys []string) (vars, secrets map[string]string) {
	pub := make(map[string]bool, len(publicKeys))
	for _, k := range publicKeys {
		pub[k] = true
	}
	vars = map[string]string{}
	secrets = map[string]string{}
	for k, v := range env {
		if strings.HasPrefix(k, PublicPrefix) || pub[k] {
			vars[k] = v
		} else {
			secrets[k] = v
		}
	}
	return vars, secrets
}

// RegisterSecrets registers every secret value with the sensitive package so it
// is scrubbed from any logged output. Call before any deploy logging.
func RegisterSecrets(secrets map[string]string) {
	vals := make([]string, 0, len(secrets))
	for _, v := range secrets {
		vals = append(vals, v)
	}
	sensitive.Register(vals...)
}

// IsLikelySecret reports whether a key name looks sensitive (used for
// commit-safety warnings, not classification).
func IsLikelySecret(key string) bool {
	if strings.HasPrefix(key, PublicPrefix) {
		return false
	}
	upper := strings.ToUpper(key)
	for _, h := range secretHints {
		if strings.Contains(upper, h) {
			return true
		}
	}
	// A trailing _KEY (e.g. STRIPE_KEY) is sensitive; a leading KEY_ (KEY_PATH)
	// is not necessarily. Check word boundary at the end.
	return strings.HasSuffix(upper, "_KEY") || upper == "KEY"
}

// PlaintextSecretWarnings flags keys in an inline (e.g. committed-YAML) map that
// look like secrets and therefore should not be stored in plaintext. Returns
// sorted human-readable warnings.
func PlaintextSecretWarnings(inline map[string]string) []string {
	var keys []string
	for k, v := range inline {
		if v != "" && IsLikelySecret(k) {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	var out []string
	for _, k := range keys {
		out = append(out, "secret-looking key "+k+" is set in plaintext — move it to an encrypted store (Doppler, the credstore, or CF Secret Store)")
	}
	return out
}

// PreflightInput aggregates what the secret-hygiene check needs. All fields are
// optional; zero values simply skip the corresponding check.
type PreflightInput struct {
	Gitignore        string            // .gitignore content ("" if absent)
	HasEnvFile       bool              // a .env file exists on disk
	PlainText        map[string]string // declared plain_text bindings (name→value)
	UsingDoppler     bool              // Doppler is configured/active
	UsingSecretStore bool              // a CF Secret Store binding is declared
}

// Preflight returns non-fatal security warnings about secret hygiene: a
// committable .env, secret-looking values stored in plaintext, and (when
// neither a managed store is in use) a nudge toward Doppler or Cloudflare
// Secret Store. Empty slice means all clear.
func Preflight(in PreflightInput) []string {
	var w []string
	if in.HasEnvFile && !GitignoreCovered(in.Gitignore) {
		w = append(w, ".env exists but is not gitignored — add \".env*\" to .gitignore so secrets aren't committed")
	}
	w = append(w, PlaintextSecretWarnings(in.PlainText)...)
	if !in.UsingDoppler && !in.UsingSecretStore {
		w = append(w, "tip: prefer Doppler or Cloudflare Secret Store over plaintext/.env for secrets (safer rotation + audit; for CF use bindings.secrets_store)")
	}
	return w
}

// GitignoreCovered reports whether the given .gitignore content ignores env
// files. Accepts the raw file content; returns false when env files could be
// committed (so the caller can warn).
func GitignoreCovered(gitignore string) bool {
	for line := range strings.SplitSeq(gitignore, "\n") {
		switch strings.TrimSpace(line) {
		case ".env", ".env*", "*.env", ".env.*", ".env.local":
			return true
		}
	}
	return false
}
