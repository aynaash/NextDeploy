package core

import (
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/aynaash/nextdeploy/shared"
	"github.com/aynaash/nextdeploy/shared/config"
	"github.com/aynaash/nextdeploy/shared/envstore"
	"github.com/aynaash/nextdeploy/shared/secrets"
)

// LoadLocalSecrets merges secrets from every supported source for the current
// project. Precedence (lowest → highest):
//
//  1. Auto-detected dotenv file at project root (`.env`)
//  2. Files declared in `nextdeploy.yml` under `secrets.files[]` (in order)
//  3. Doppler — when running under `doppler run -- nextdeploy ship`, or when
//     `secrets.doppler.inject_env: true` is set in nextdeploy.yml, the
//     process environment is harvested and merged. See harvestDopplerEnv.
//  4. The managed JSON store at `.nextdeploy/.env`, populated by
//     `nextdeploy secrets set/load`
//
// Higher-precedence sources override lower ones. The managed store wins
// because it represents explicit user intent via the CLI.
func LoadLocalSecrets(cfg *config.NextDeployConfig) (map[string]string, error) {
	return loadLocalSecrets(cfg)
}

// SecretsDeclared reports whether the project references any secret source.
// When true, a LoadLocalSecrets failure must abort the deploy rather than ship
// an empty set — CF Worker uploads are replace-not-merge, so an empty set
// strips every live secret. When false (no declared sources) an empty set is
// legitimately the intended state.
func SecretsDeclared(cfg *config.NextDeployConfig) bool {
	return secretsDeclared(cfg)
}

func secretsDeclared(cfg *config.NextDeployConfig) bool {
	if cfg == nil {
		return false
	}
	if len(cfg.Secrets.Files) > 0 {
		return true
	}
	if cfg.Secrets.Doppler != nil || cfg.Secrets.Vault != nil {
		return true
	}
	// Managed store, populated by `nextdeploy secrets set/load`.
	if _, err := os.Stat(filepath.Join(".nextdeploy", ".env")); err == nil {
		return true
	}
	return false
}

func loadLocalSecrets(cfg *config.NextDeployConfig) (map[string]string, error) {
	log := shared.PackageLogger("serverless", "🔐 SECRETS")
	merged := map[string]string{}

	// 1. Project-root .env (dotenv format) — silently skipped if missing.
	if env, err := envstore.ReadEnvFile(".env"); err == nil {
		mergeInto(merged, env)
		log.Info("Loaded %d secrets from .env", len(env))
	} else if !os.IsNotExist(err) {
		log.Warn("Failed to read .env (non-fatal): %v", err)
	}

	// 2. YAML-declared files (cfg.Secrets.Files). Each file is parsed as
	// dotenv. We do not silently swallow parse errors here — a misconfigured
	// secrets file should be loud.
	for _, sf := range cfg.Secrets.Files {
		if sf.Path == "" {
			continue
		}
		env, err := envstore.ReadEnvFile(sf.Path)
		if err != nil {
			return nil, fmt.Errorf("failed to read secrets file %s: %w", sf.Path, err)
		}
		mergeInto(merged, env)
		log.Info("Loaded %d secrets from %s", len(env), sf.Path)
	}

	// 3. Doppler — harvest process env when running under `doppler run --`.
	if dopplerEnv, source := harvestDopplerEnv(cfg); source != "" {
		mergeInto(merged, dopplerEnv)
		log.Info("Loaded %d secrets from Doppler (%s)", len(dopplerEnv), source)
	}

	// 4. Managed JSON store (.nextdeploy/.env). Highest precedence.
	sm, err := secrets.NewSecretManager(secrets.WithConfig(cfg))
	if err != nil {
		return nil, fmt.Errorf("failed to init secret manager: %w", err)
	}
	managedPath := filepath.Join(".nextdeploy", ".env")
	if _, statErr := os.Stat(managedPath); statErr == nil {
		if err := sm.ImportSecrets(managedPath); err != nil {
			return nil, fmt.Errorf("failed to load secrets from %s: %w", managedPath, err)
		}
		managed := sm.FlattenSecrets()
		mergeInto(merged, managed)
		log.Info("Loaded %d secrets from managed store", len(managed))
	}

	log.Info("Total secrets to sync: %d", len(merged))
	return merged, nil
}

// harvestDopplerEnv inspects the process environment for Doppler-injected
// secrets and returns them as a name→value map. Returns an empty map when
// Doppler is not in play.
//
// Activation, in order:
//
//  1. The standard variables `doppler run` injects (DOPPLER_PROJECT,
//     DOPPLER_CONFIG, DOPPLER_ENVIRONMENT, DOPPLER_TOKEN) are present →
//     auto-detected, no config required. This is the
//     `doppler run -- nextdeploy ship` path.
//
//  2. The user explicitly opts in via nextdeploy.yml:
//
//     secrets:
//     doppler:
//     inject_env: true
//
// Filtering strategy — allowlist, not denylist:
//
// We shell out to `doppler secrets --json` (which inherits DOPPLER_*
// from our env) to get the canonical list of Doppler-managed key names
// for the active project/config, then keep only env vars whose names
// appear in that list. This guarantees the user's local shell
// environment (KITTY_PID, WAYLAND_DISPLAY, GNOME_DESKTOP_SESSION_ID, …)
// can never leak into a deploy, even if a future shell adds a brand-new
// SCREAMING_SNAKE_CASE variable that no denylist anticipates.
//
// Fallback path: when the doppler CLI is unavailable or the call fails
// (no network, expired token), we revert to the legacy denylist
// heuristic and emit a loud warning so the operator knows what's
// happening. Better to ship with the conservative filter than to fail
// the deploy outright.
//
// The second return value is a short label describing the activation
// source, used for logging. Empty string means "Doppler not active,
// nothing was harvested".
func harvestDopplerEnv(cfg *config.NextDeployConfig) (map[string]string, string) {
	dopplerProject := os.Getenv("DOPPLER_PROJECT")
	dopplerConfig := os.Getenv("DOPPLER_CONFIG")
	dopplerEnvName := os.Getenv("DOPPLER_ENVIRONMENT")
	dopplerToken := os.Getenv("DOPPLER_TOKEN")
	autoDetected := dopplerProject != "" || dopplerConfig != "" || dopplerEnvName != "" || dopplerToken != ""

	optIn := cfg != nil && cfg.Secrets.Doppler != nil && cfg.Secrets.Doppler.InjectEnv

	if !autoDetected && !optIn {
		return nil, ""
	}

	source := "config opt-in"
	switch {
	case autoDetected && dopplerProject != "" && dopplerConfig != "":
		source = fmt.Sprintf("doppler run: project=%s config=%s", dopplerProject, dopplerConfig)
	case autoDetected:
		source = "doppler run"
	}

	log := shared.PackageLogger("serverless", "🔐 SECRETS")

	allowlist, allowErr := dopplerKeyAllowlist()
	envMap := envSliceToMap(os.Environ())

	if allowErr != nil {
		// Do NOT fall back to harvesting the whole shell environment — that
		// leaks desktop/session vars (WAYLAND_DISPLAY, DBUS_SESSION_BUS_ADDRESS,
		// …) as Worker secret_text and churns the deploy hash every run. Ship
		// only explicitly-declared secrets instead.
		log.Warn("Could not query `doppler secrets --json` (%v). NOT harvesting the "+
			"shell environment (would leak desktop/session vars as Worker secrets). "+
			"Fix the doppler CLI, or declare secrets in secrets.files[].", allowErr)
		return nil, source
	}

	out := make(map[string]string, len(allowlist))
	for k := range allowlist {
		// Skip Doppler's own bookkeeping vars even if they appear in the
		// allowlist — DOPPLER_PROJECT/CONFIG/TOKEN must not be pushed to
		// the application runtime.
		if strings.HasPrefix(k, "DOPPLER_") {
			continue
		}
		if v, ok := envMap[k]; ok {
			out[k] = v
		}
	}
	return out, source
}

// dopplerKeyAllowlist queries the local doppler CLI for the set of
// secret names managed by the active project/config. The CLI inherits
// DOPPLER_PROJECT/DOPPLER_CONFIG/DOPPLER_TOKEN from our process env, so
// this matches whatever `doppler run` would inject.
//
// Returns (set, nil) on success, (nil, err) when the CLI is missing,
// the call fails, or the JSON shape is unexpected. The caller is
// expected to fall back to a heuristic on error rather than abort.
func dopplerKeyAllowlist() (map[string]struct{}, error) {
	bin, err := exec.LookPath("doppler")
	if err != nil {
		return nil, fmt.Errorf("doppler CLI not on PATH: %w", err)
	}
	// `doppler secrets --json` returns an object keyed by secret name.
	// Values contain {computed, computedValueType, computedVisibility,
	// note, ...}. We only need the keys.
	cmd := exec.Command(bin, "secrets", "--json", "--no-check-version") // #nosec G204 — bin is from LookPath, args are static
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("`doppler secrets --json`: %w", err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, fmt.Errorf("parse doppler json: %w", err)
	}
	if len(raw) == 0 {
		return nil, fmt.Errorf("doppler returned an empty key list (project/config not selected?)")
	}
	keys := make(map[string]struct{}, len(raw))
	for k := range raw {
		keys[k] = struct{}{}
	}
	return keys, nil
}

// envSliceToMap turns os.Environ() output into a name→value map.
func envSliceToMap(env []string) map[string]string {
	out := make(map[string]string, len(env))
	for _, kv := range env {
		i := strings.IndexByte(kv, '=')
		if i <= 0 {
			continue
		}
		out[kv[:i]] = kv[i+1:]
	}
	return out
}

// mergeInto copies src into dst, overwriting existing keys.
func mergeInto(dst, src map[string]string) {
	maps.Copy(dst, src)
}
