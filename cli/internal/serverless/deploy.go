package serverless

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Golangcodes/nextdeploy/internal/packaging"
	"github.com/Golangcodes/nextdeploy/shared"
	"github.com/Golangcodes/nextdeploy/shared/config"
	"github.com/Golangcodes/nextdeploy/shared/envstore"
	"github.com/Golangcodes/nextdeploy/shared/nextcore"
	"github.com/Golangcodes/nextdeploy/shared/secrets"
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

// Deploy orchestrates the full serverless deployment pipeline:
//  1. Discovers the build artifact (app.tar.gz)
//  2. Fetches local secrets via SecretManager and pushes them to the cloud secret store
//  3. Uploads static assets to CDN/Storage
//  4. Deploys the compute layer (Lambda, Workers, etc.)
//  5. Invalidates the CDN cache
func Deploy(ctx context.Context, cfg *config.NextDeployConfig, meta *nextcore.NextCorePayload, verbose bool) error {
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
			log.Warn("Failed to load local secrets (non-fatal): %v", err)
			appSecrets = map[string]string{}
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

	log.Info("Serverless deployment complete! Application is live.")

	// ── 6.5. Post-deploy smoke verify ───────────────────────────────────────
	// Non-fatal by default — CI callers can opt into FailOnError when they
	// want the deploy to gate on the smoke check. Domain-less deploys (no
	// custom domain, workers.dev only) skip automatically.
	if _, err := SmokeVerify(ctx, log, cfg, meta, SmokeOpts{}); err != nil {
		log.Warn("Smoke verify returned error (non-fatal): %v", err)
	}

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
//  2. The user explicitly opts in via nextdeploy.yml:
//
//       secrets:
//         doppler:
//           inject_env: true
//
//     This is useful when the env is populated by something other than
//     `doppler run` — for example a CI step that calls
//     `doppler secrets download --no-file --format=env > $GITHUB_ENV`.
//
// To avoid leaking the local shell into the cloud, we filter out variables
// that are clearly system, tooling, or nextdeploy-internal. Doppler-injected
// secrets are by convention SCREAMING_SNAKE_CASE app vars, so the heuristic
// is: keep upper-case identifiers that don't match any deny-list pattern.
//
// The second return value is a short label describing the activation source,
// used for logging. Empty string means "Doppler not active, nothing was
// harvested".
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

	out := map[string]string{}
	for _, kv := range os.Environ() {
		i := strings.IndexByte(kv, '=')
		if i <= 0 {
			continue
		}
		k, v := kv[:i], kv[i+1:]
		if !shouldHarvestEnvKey(k) {
			continue
		}
		out[k] = v
	}

	source := "config opt-in"
	switch {
	case autoDetected && dopplerProject != "" && dopplerConfig != "":
		source = fmt.Sprintf("doppler run: project=%s config=%s", dopplerProject, dopplerConfig)
	case autoDetected:
		source = "doppler run"
	}
	return out, source
}

// shouldHarvestEnvKey returns true when an environment variable looks like a
// user app secret rather than a system/tooling/nextdeploy variable.
//
// Rules:
//   - Must be SCREAMING_SNAKE_CASE (no lowercase letters).
//   - Must not match any tooling/credential/system prefix the harvester
//     should never push to the cloud (CLOUDFLARE_API_TOKEN, AWS keys, the
//     Doppler bookkeeping vars themselves, shell internals).
//
// This list is conservative — we'd rather drop a secret the user expected
// us to push than push a credential we shouldn't. Users who need to force
// a denied key into the deploy can declare it in `secrets.files[]` or in
// the managed store.
func shouldHarvestEnvKey(k string) bool {
	if k == "" {
		return false
	}
	hasLower := false
	for _, r := range k {
		if r >= 'a' && r <= 'z' {
			hasLower = true
			break
		}
	}
	if hasLower {
		return false
	}
	denyExact := map[string]struct{}{
		"PATH": {}, "HOME": {}, "USER": {}, "SHELL": {}, "PWD": {}, "OLDPWD": {},
		"LANG": {}, "LC_ALL": {}, "TERM": {}, "TMPDIR": {}, "LOGNAME": {},
		"HOSTNAME": {}, "DISPLAY": {}, "XAUTHORITY": {}, "MAIL": {},
		"SSH_AUTH_SOCK": {}, "SSH_AGENT_PID": {}, "SSH_CONNECTION": {}, "SSH_CLIENT": {}, "SSH_TTY": {},
		"GOPATH": {}, "GOROOT": {}, "GOCACHE": {}, "GOMODCACHE": {},
		"NODE_PATH": {}, "NPM_CONFIG_PREFIX": {},
		"_": {},
	}
	if _, bad := denyExact[k]; bad {
		return false
	}
	denyPrefixes := []string{
		"DOPPLER_",       // Doppler bookkeeping (we don't push our own auth)
		"CLOUDFLARE_",    // CF deploy creds, not app secrets
		"R2_",            // R2 deploy creds
		"AWS_",           // AWS deploy creds
		"GOOGLE_",        // GCP deploy creds
		"AZURE_",         // Azure deploy creds
		"NEXTDEPLOY_",    // our own internal flags
		"ND_",            // our own internal flags (short prefix)
		"BASH_",          // shell internals
		"XDG_",           // XDG base dirs
		"LC_",            // locale
		"GITHUB_",        // GitHub Actions runner internals
		"RUNNER_",        // GitHub Actions runner internals
		"CI_",            // generic CI internals
	}
	for _, p := range denyPrefixes {
		if strings.HasPrefix(k, p) {
			return false
		}
	}
	return true
}

// mergeInto copies src into dst, overwriting existing keys.
func mergeInto(dst, src map[string]string) {
	for k, v := range src {
		dst[k] = v
	}
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
