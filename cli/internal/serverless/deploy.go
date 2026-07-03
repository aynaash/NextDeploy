package serverless

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/aynaash/nextdeploy/internal/packaging"
	"github.com/aynaash/nextdeploy/shared"
	"github.com/aynaash/nextdeploy/shared/config"
	"github.com/aynaash/nextdeploy/shared/envstore"
	"github.com/aynaash/nextdeploy/shared/nextcore"
	"github.com/aynaash/nextdeploy/shared/secenv"
	"github.com/aynaash/nextdeploy/shared/secrets"
)

// New returns a new serverless provider based on the provider name.
func New(providerName string, verbose bool) (Provider, error) {
	switch providerName {
	case "aws":
		return NewAWSProvider(verbose), nil
	case "cloudflare":
		return NewCloudflareProvider(), nil
	default:
		return nil, fmt.Errorf("unsupported serverless provider: %s (supported: aws, cloudflare)", providerName)
	}
}

// resourceProvisioner is implemented by providers that can reconcile declared
// infra (KV, Hyperdrive, D1, …) before deploying. CloudflareProvider implements
// it; AWS doesn't (and doesn't need to).
type resourceProvisioner interface {
	ProvisionResources(ctx context.Context, cfg *config.NextDeployConfig) error
}

// hintCloudflarePermissions augments a raw Cloudflare auth failure with the
// missing-token-scope guidance users actually need.
func hintCloudflarePermissions(err error) error {
	s := err.Error()
	if strings.Contains(s, "403") || strings.Contains(s, "Authentication error") || strings.Contains(s, `"code":10000`) {
		return fmt.Errorf("%w\n  → the Cloudflare API token is likely missing a permission for the resource above "+
			"(e.g. Hyperdrive:Edit, Workers KV Storage:Edit, D1:Edit, Workers AI:Read). "+
			"Add it at https://dash.cloudflare.com/profile/api-tokens", err)
	}
	return err
}

// maybeProvisionResources reconciles declared infra before deploying, when the
// provider supports it and provisioning isn't disabled. Idempotent — existing
// resources are skipped — so it's safe to run on every ship.
func maybeProvisionResources(ctx context.Context, p Provider, cfg *config.NextDeployConfig, provision bool, log *shared.Logger) error {
	if !provision {
		return nil
	}
	rp, ok := p.(resourceProvisioner)
	if !ok {
		return nil
	}
	log.Info("Reconciling declared Cloudflare resources...")
	if err := rp.ProvisionResources(ctx, cfg); err != nil {
		return fmt.Errorf("provisioning declared resources failed "+
			"(pass --no-provision to skip, or `nextdeploy apply` to review the plan): %w",
			hintCloudflarePermissions(err))
	}
	return nil
}

// Deploy orchestrates the full serverless deployment pipeline:
//  1. Discovers the build artifact (app.tar.gz)
//  2. Fetches local secrets via SecretManager and pushes them to the cloud secret store
//  3. Uploads static assets to CDN/Storage
//  4. Deploys the compute layer (Lambda, Workers, etc.)
//  5. Invalidates the CDN cache
//
//nolint:gocognit,gocyclo,cyclop,funlen // top-level orchestrator with pre-existing complexity; this change only delegates provisioning to a helper.
func Deploy(ctx context.Context, cfg *config.NextDeployConfig, meta *nextcore.NextCorePayload, verbose, provision, verify bool) error {
	log := shared.PackageLogger("serverless", "☁️  SERVERLESS")

	if err := validateProviderConsistency(cfg, log); err != nil {
		return err
	}

	// ── 1. Resolve provider ──────────────────────────────────────────────────
	p, err := New(cfg.Serverless.Provider, verbose)
	if err != nil {
		return err
	}

	if err := p.Initialize(ctx, cfg); err != nil {
		return fmt.Errorf("provider initialization failed: %w", err)
	}

	// ── 1b. Provision declared resources (idempotent; --no-provision opts out) ─
	if err := maybeProvisionResources(ctx, p, cfg, provision, log); err != nil {
		return err
	}

	// ── 2. Run Packaging ───────────────────────────────────────────────────
	projectDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get project root: %w", err)
	}

	packager, err := packaging.NewPackager(projectDir, meta)
	if err != nil {
		return fmt.Errorf("failed to initialize packager: %w", err)
	}
	defer packager.Cleanup()

	log.Info("Splitting assets for optimized Serverless deployment...")
	pkgResult, err := packager.Package()
	if err != nil {
		return fmt.Errorf("packaging failed: %w", err)
	}

	if pkgResult.SizeWarning != "" {
		log.Warn("%s", pkgResult.SizeWarning)
	}
	log.Info("Package split: %dMB Lambda zip, %d S3 assets", pkgResult.LambdaZipSize/(1024*1024), len(pkgResult.S3Assets))

	// ── 3. Push secrets (always before compute) ──────────────────────────────
	// AWS: secrets must land in Secrets Manager BEFORE DeployCompute because
	//      the allow_secrets_in_env fallback reads them at deploy time to
	//      bake into Lambda env vars.
	// Cloudflare: UpdateSecrets stashes the set onto the provider; the
	//      stash is then folded into the script upload as secret_text
	//      bindings during DeployCompute. This avoids the per-secret PUT
	//      rate limit (CF error 10013).
	pushSecrets := func() error {
		appSecrets, err := loadLocalSecrets(cfg)
		if err != nil {
			// Only tolerable when nothing was declared. If the project references
			// secret sources, a load failure must abort — shipping an empty set
			// strips every live secret (CF upload is replace-not-merge).
			if secretsDeclared(cfg) {
				return fmt.Errorf("refusing to deploy: secrets are declared but failed to load "+
					"(would strip all live Worker secrets): %w", err)
			}
			log.Warn("No secrets loaded (none declared): %v", err)
			appSecrets = map[string]string{}
		}
		// Register every secret value so it is scrubbed from any subsequent log
		// output (deploy summaries, errors, smoke probes). Must happen before
		// anything touches the values.
		secenv.RegisterSecrets(appSecrets)
		for _, warn := range secretsPreflight(cfg) {
			log.Warn("secret hygiene: %s", warn)
		}
		if err := p.UpdateSecrets(ctx, cfg.App.Name, appSecrets); err != nil {
			return fmt.Errorf("failed to push secrets to cloud provider: %w", err)
		}
		return nil
	}

	if err := pushSecrets(); err != nil {
		return err
	}

	// ── 4. Deploy static assets ──────────────────────────────────────────────
	t0 := time.Now()
	if err := p.DeployStatic(ctx, pkgResult, cfg, meta); err != nil {
		return fmt.Errorf("failed to deploy static assets: %w", err)
	}
	if verbose {
		log.Info("  Static upload completed in %s", time.Since(t0).Round(time.Millisecond))
	}

	// ── 5. Deploy compute layer ──────────────────────────────────────────────
	t0 = time.Now()
	if err := p.DeployCompute(ctx, pkgResult, cfg, meta); err != nil {
		return fmt.Errorf("failed to deploy compute layer: %w", err)
	}
	if verbose {
		log.Info("  Lambda deployment completed in %s", time.Since(t0).Round(time.Millisecond))
	}

	// ── 6. Invalidate CDN cache ──────────────────────────────────────────────
	// AWS DeployCompute already triggers an invalidation immediately after the
	// distribution is created/updated. Skip the redundant orchestration-level
	// call for AWS to avoid double-billing and double latency. Other providers
	// (Cloudflare) still need this hop because their compute deploy doesn't
	// touch the CDN cache.
	if cfg.Serverless.Provider != "aws" {
		t0 = time.Now()
		if err := p.InvalidateCache(ctx, cfg); err != nil {
			log.Error("Cache invalidation failed (non-fatal): %v", err)
		} else if verbose {
			log.Info("  CDN invalidation completed in %s", time.Since(t0).Round(time.Millisecond))
		}
	}

	log.Info("Serverless deployment complete — verifying...")

	// ── 6.5. Post-deploy smoke verify ───────────────────────────────────────
	// Non-fatal by default; `--verify` (verify=true) gates the deploy on the
	// smoke check for CI. Domain-less deploys (no custom domain, workers.dev
	// only) skip automatically.
	if _, err := SmokeVerify(ctx, log, cfg, meta, SmokeOpts{FailOnError: verify}); err != nil {
		if verify {
			return fmt.Errorf("post-deploy smoke verify failed: %w", err)
		}
		log.Warn("Smoke verify returned error (non-fatal): %v", err)
	}

	log.Info("Application is live.")

	// ── 7. Generate Visual Report ───────────────────────────────────────────
	resMap, err := p.GetResourceMap(ctx, cfg)
	if err == nil {
		reportPath, err := GenerateResourceView(&cfg.App, resMap)
		if err == nil {
			absPath, _ := filepath.Abs(reportPath)
			log.Info("┌────────────────────────────────────────────────────────────┐")
			log.Success("│  🚀 DEPLOYMENT REPORT READY                                │")
			log.Info("├────────────────────────────────────────────────────────────┤")
			log.Info("│  Report: file://%s", absPath)
			log.Info("│                                                            │")
			log.Info("│  ⚠️  DNS GUIDANCE: Open this report immediately to see     │")
			log.Info("│     the exact DNS records needed for your custom domain.   │")
			log.Info("└────────────────────────────────────────────────────────────┘")
		} else {
			log.Warn("Failed to generate visual report: %v", err)
		}
	} else {
		log.Warn("Failed to fetch resource map for report: %v", err)
	}

	return nil
}

// Rollback orchestrates the serverless rollback process. Opts forwards
// --steps / --to <commit> from the CLI down to the provider implementation.
func Rollback(ctx context.Context, cfg *config.NextDeployConfig, opts RollbackOptions) error {
	log := shared.PackageLogger("serverless", "☁️  SERVERLESS")

	// ── 1. Resolve provider ──────────────────────────────────────────────────
	p, err := New(cfg.Serverless.Provider, false)
	if err != nil {
		return err
	}

	if err := p.Initialize(ctx, cfg); err != nil {
		return fmt.Errorf("provider initialization failed: %w", err)
	}

	// ── 2. Trigger Rollback ──────────────────────────────────────────────────
	if err := p.Rollback(ctx, cfg, opts); err != nil {
		return fmt.Errorf("serverless rollback failed: %w", err)
	}

	log.Info(" Serverless rollback complete!")
	return nil
}

// LoadLocalSecrets is the exported alias for loadLocalSecrets, used by
// CLI commands outside this package (e.g. `nextdeploy secrets prune`)
// that need to compute the canonical local secret set.
func LoadLocalSecrets(cfg *config.NextDeployConfig) (map[string]string, error) {
	return loadLocalSecrets(cfg)
}

// loadLocalSecrets merges secrets from every supported source for the current
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
// secretsDeclared reports whether the project references any secret source.
// When true, a loadLocalSecrets failure must abort the deploy rather than ship
// an empty set — CF Worker uploads are replace-not-merge, so an empty set
// strips every live secret. When false (no declared sources) an empty set is
// legitimately the intended state.
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

// validateProviderConsistency rejects configurations where the legacy
// CloudProvider.name disagrees with serverless.provider. It's an easy
// foot-gun: an old AWS yaml that grew a new `serverless: { provider:
// cloudflare }` block would silently load AWS creds from CloudProvider
// and try to ship them to Cloudflare. We'd rather fail loud than have
// access keys leak across providers.
func validateProviderConsistency(cfg *config.NextDeployConfig, log *shared.Logger) error {
	if cfg == nil || cfg.Serverless == nil {
		return nil
	}
	want := cfg.Serverless.Provider
	if want == "" {
		return fmt.Errorf("serverless.provider is required for target_type: serverless (one of: aws, cloudflare)")
	}
	if cfg.CloudProvider == nil {
		return nil
	}
	got := cfg.CloudProvider.Name
	if got == "" || got == want {
		return nil
	}
	// Allow harmless aliases. CloudProvider was originally an AWS-only
	// concept; treat empty/aws as compatible with serverless.provider:aws.
	if got == "aws" && want == "aws" {
		return nil
	}
	return fmt.Errorf(
		"provider mismatch: CloudProvider.name=%q but serverless.provider=%q — "+
			"set them to the same value, or remove CloudProvider entirely if you only deploy via serverless",
		got, want,
	)
}
