// Package serverless orchestrates serverless deployments across cloud providers.
package serverless

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aynaash/nextdeploy/cli/internal/serverless/core"
	"github.com/aynaash/nextdeploy/internal/packaging"
	"github.com/aynaash/nextdeploy/shared"
	"github.com/aynaash/nextdeploy/shared/config"
	"github.com/aynaash/nextdeploy/shared/nextcore"
	"github.com/aynaash/nextdeploy/shared/secenv"
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

	if err := validateProviderConsistency(cfg); err != nil {
		return err
	}

	// ── 1. Resolve provider ──────────────────────────────────────────────────
	p, err := New(cfg.Serverless.Provider, verbose)
	if err != nil {
		return err
	}
	if err = p.Initialize(ctx, cfg); err != nil {
		return fmt.Errorf("provider initialization failed: %w", err)
	}

	if syncer, ok := p.(interface {
		SyncResourceState(context.Context, *config.NextDeployConfig) error
	}); ok {
		if err = syncer.SyncResourceState(ctx, cfg); err != nil {
			return fmt.Errorf("synchronize provider state: %w", err)
		}
	}

	// ── 1b. Provision declared resources (idempotent; --no-provision opts out) ─
	if err = maybeProvisionResources(ctx, p, cfg, provision, log); err != nil {
		return err
	}

	// ── 2. Run Packaging ───────────────────────────────────────────────────
	var projectDir string
	projectDir, err = os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get project root: %w", err)
	}

	packager, err := packaging.NewPackager(projectDir, meta)
	if err != nil {
		return fmt.Errorf("failed to initialize packager: %w", err)
	}
	defer packager.Cleanup()

	log.Info("Splitting assets for optimized Serverless deployment...")
	// find deployment target and skip size checks for Cloudflare (no Lambda ZIP, no size limits)

	target := cfg.Serverless.Provider
	if target == "" {
		log.Warn("serverless.provider is required for target_type: serverless (one of: aws, cloudflare)")
	}
	pkgResult, err := packager.Package(target)
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
		var appSecrets map[string]string
		appSecrets, err = core.LoadLocalSecrets(cfg)
		if err != nil {
			// Only tolerable when nothing was declared. If the project references
			// secret sources, a load failure must abort — shipping an empty set
			// strips every live secret (CF upload is replace-not-merge).
			if core.SecretsDeclared(cfg) {
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
		if err = p.UpdateSecrets(ctx, cfg.App.Name, appSecrets); err != nil {
			return fmt.Errorf("failed to push secrets to cloud provider: %w", err)
		}
		return nil
	}

	if err = pushSecrets(); err != nil {
		return err
	}

	// ── 4. Deploy static assets ──────────────────────────────────────────────
	t0 := time.Now()
	if err = p.DeployStatic(ctx, pkgResult, cfg, meta); err != nil {
		return fmt.Errorf("failed to deploy static assets: %w", err)
	}
	if verbose {
		log.Info("  Static upload completed in %s", time.Since(t0).Round(time.Millisecond))
	}

	// ── 5. Deploy compute layer ──────────────────────────────────────────────
	t0 = time.Now()
	if err = p.DeployCompute(ctx, pkgResult, cfg, meta); err != nil {
		return fmt.Errorf("failed to deploy compute layer: %w", err)
	}
	if verbose {
		log.Info("  deployment completed in %s", time.Since(t0).Round(time.Millisecond))
	}

	// ── 6. Invalidate CDN cache ──────────────────────────────────────────────
	// AWS DeployCompute already triggers an invalidation immediately after the
	// distribution is created/updated. Skip the redundant orchestration-level
	// call for AWS to avoid double-billing and double latency. Other providers
	// (Cloudflare) still need this hop because their compute deploy doesn't
	// touch the CDN cache.
	if cfg.Serverless.Provider != "aws" {
		t0 = time.Now()
		if err = p.InvalidateCache(ctx, cfg); err != nil {
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
	if _, err = SmokeVerify(ctx, log, cfg, meta, SmokeOpts{FailOnError: verify}); err != nil {
		if verify {
			return fmt.Errorf("post-deploy smoke verify failed: %w", err)
		}
		log.Warn("Smoke verify returned error (non-fatal): %v", err)
	}

	log.Info("Application is live.")

	// ── 7. Generate Visual Report ───────────────────────────────────────────
	resMap, err := p.GetResourceMap(ctx, cfg)
	if err == nil {
		reportPath, reportErr := GenerateResourceView(&cfg.App, resMap)
		if reportErr == nil {
			absPath, _ := filepath.Abs(reportPath)
			log.Info("┌────────────────────────────────────────────────────────────┐")
			log.Success("│  🚀 DEPLOYMENT REPORT READY                                │")
			log.Info("├────────────────────────────────────────────────────────────┤")
			log.Info("│  Report: file://%s", absPath)
			log.Info("│                                                            │")
			log.Info("│   DNS GUIDANCE: Open this report immediately to see     │")
			log.Info("│     the exact DNS records needed for your custom domain.   │")
			log.Info("└────────────────────────────────────────────────────────────┘")
		} else {
			log.Warn("Failed to generate visual report: %v", reportErr)
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

	if err = p.Initialize(ctx, cfg); err != nil {
		return fmt.Errorf("provider initialization failed: %w", err)
	}

	// ── 2. Trigger Rollback ──────────────────────────────────────────────────
	if err = p.Rollback(ctx, cfg, opts); err != nil {
		return fmt.Errorf("serverless rollback failed: %w", err)
	}

	log.Info(" Serverless rollback complete!")
	return nil
}

// LoadLocalSecrets is the exported alias for core.LoadLocalSecrets, used by
// CLI commands outside this package (e.g. `nextdeploy secrets prune`)
// that need to compute the canonical local secret set.
func LoadLocalSecrets(cfg *config.NextDeployConfig) (map[string]string, error) {
	return core.LoadLocalSecrets(cfg)
}

// validateProviderConsistency rejects configurations where the legacy
// CloudProvider.name disagrees with serverless.provider. It's an easy
// foot-gun: an old AWS yaml that grew a new `serverless: { provider:
// cloudflare }` block would silently load AWS creds from CloudProvider
// and try to ship them to Cloudflare. We'd rather fail loud than have
// access keys leak across providers.
func validateProviderConsistency(cfg *config.NextDeployConfig) error {
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
