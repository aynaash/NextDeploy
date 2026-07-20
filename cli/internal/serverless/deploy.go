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

type resourceProvisioner interface {
	ProvisionResources(ctx context.Context, cfg *config.NextDeployConfig) error
}

func hintCloudflarePermissions(err error) error {
	s := err.Error()
	if strings.Contains(s, "403") || strings.Contains(s, "Authentication error") || strings.Contains(s, `"code":10000`) {
		return fmt.Errorf("%w\n  → the Cloudflare API token is likely missing a permission for the resource above "+
			"(e.g. Hyperdrive:Edit, Workers KV Storage:Edit, D1:Edit, Workers AI:Read). "+
			"Add it at https://dash.cloudflare.com/profile/api-tokens", err)
	}
	return err
}

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

func Deploy(ctx context.Context, cfg *config.NextDeployConfig, meta *nextcore.NextCorePayload, verbose, provision, verify bool) error {
	log := shared.PackageLogger("serverless", "☁️  SERVERLESS")

	if err := validateProviderConsistency(cfg, log); err != nil {
		return err
	}

	p, err := New(cfg.Serverless.Provider, verbose)
	if err != nil {
		return err
	}

	if err := p.Initialize(ctx, cfg); err != nil {
		return fmt.Errorf("provider initialization failed: %w", err)
	}

	if err := maybeProvisionResources(ctx, p, cfg, provision, log); err != nil {
		return err
	}

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

	pushSecrets := func() error {
		appSecrets, err := loadLocalSecrets(cfg)
		if err != nil {
			if secretsDeclared(cfg) {
				return fmt.Errorf("refusing to deploy: secrets are declared but failed to load "+
					"(would strip all live Worker secrets): %w", err)
			}
			log.Warn("No secrets loaded (none declared): %v", err)
			appSecrets = map[string]string{}
		}
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

	t0 := time.Now()
	if err := p.DeployStatic(ctx, pkgResult, cfg, meta); err != nil {
		return fmt.Errorf("failed to deploy static assets: %w", err)
	}
	if verbose {
		log.Info("  Static upload completed in %s", time.Since(t0).Round(time.Millisecond))
	}

	t0 = time.Now()
	if err := p.DeployCompute(ctx, pkgResult, cfg, meta); err != nil {
		return fmt.Errorf("failed to deploy compute layer: %w", err)
	}
	if verbose {
		log.Info("  Lambda deployment completed in %s", time.Since(t0).Round(time.Millisecond))
	}

	if cfg.Serverless.Provider != "aws" {
		t0 = time.Now()
		if err := p.InvalidateCache(ctx, cfg); err != nil {
			log.Error("Cache invalidation failed (non-fatal): %v", err)
		} else if verbose {
			log.Info("  CDN invalidation completed in %s", time.Since(t0).Round(time.Millisecond))
		}
	}

	log.Info("Serverless deployment complete — verifying...")

	if _, err := SmokeVerify(ctx, log, cfg, meta, SmokeOpts{FailOnError: verify}); err != nil {
		if verify {
			return fmt.Errorf("post-deploy smoke verify failed: %w", err)
		}
		log.Warn("Smoke verify returned error (non-fatal): %v", err)
	}

	log.Info("Application is live.")

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

func Rollback(ctx context.Context, cfg *config.NextDeployConfig, opts RollbackOptions) error {
	log := shared.PackageLogger("serverless", "☁️  SERVERLESS")

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

func LoadLocalSecrets(cfg *config.NextDeployConfig) (map[string]string, error) {
	return loadLocalSecrets(cfg)
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
	if _, err := os.Stat(filepath.Join(".nextdeploy", ".env")); err == nil {
		return true
	}
	return false
}

func loadLocalSecrets(cfg *config.NextDeployConfig) (map[string]string, error) {
	log := shared.PackageLogger("serverless", "🔐 SECRETS")
	merged := map[string]string{}

	if env, err := envstore.ReadEnvFile(".env"); err == nil {
		mergeInto(merged, env)
		log.Info("Loaded %d secrets from .env", len(env))
	} else if !os.IsNotExist(err) {
		log.Warn("Failed to read .env (non-fatal): %v", err)
	}

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

	if dopplerEnv, source := harvestDopplerEnv(cfg); source != "" {
		mergeInto(merged, dopplerEnv)
		log.Info("Loaded %d secrets from Doppler (%s)", len(dopplerEnv), source)
	}

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
		log.Warn("Could not query `doppler secrets --json` (%v). NOT harvesting the "+
			"shell environment (would leak desktop/session vars as Worker secrets). "+
			"Fix the doppler CLI, or declare secrets in secrets.files[].", allowErr)
		return nil, source
	}

	out := make(map[string]string, len(allowlist))
	for k := range allowlist {
		if strings.HasPrefix(k, "DOPPLER_") {
			continue
		}
		if v, ok := envMap[k]; ok {
			out[k] = v
		}
	}
	return out, source
}

func dopplerKeyAllowlist() (map[string]struct{}, error) {
	bin, err := exec.LookPath("doppler")
	if err != nil {
		return nil, fmt.Errorf("doppler CLI not on PATH: %w", err)
	}
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

func mergeInto(dst, src map[string]string) {
	maps.Copy(dst, src)
}

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
	if got == "aws" && want == "aws" {
		return nil
	}
	return fmt.Errorf(
		"provider mismatch: CloudProvider.name=%q but serverless.provider=%q — "+
			"set them to the same value, or remove CloudProvider entirely if you only deploy via serverless",
		got, want,
	)
}
