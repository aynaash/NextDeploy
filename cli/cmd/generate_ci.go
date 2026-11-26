package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/aynaash/nextdeploy/cli/internal/cicd"
	"github.com/aynaash/nextdeploy/shared"
	"github.com/aynaash/nextdeploy/shared/config"

	"github.com/spf13/cobra"
)

var (
	generateCIForce    bool
	generateCIPreview  bool
	generateCIOutput   string
	generateCIProvider string
	generateCITarget   string
)

var generateCICmd = &cobra.Command{
	Use:     "generate-ci",
	Aliases: []string{"ci"},
	Short:   "Generate a GitHub Actions workflow for zero-touch deployment",
	Long: `Creates a .github/workflows/nextdeploy.yml file that automatically
builds your Next.js project and ships it using NextDeploy on every push to main.

The workflow detects the deployment type from nextdeploy.yml AT RUNTIME and
runs the matching deploy (cloudflare / aws / vps), so retargeting the app does
not require regenerating CI. The generated job relies on ` + "`nextdeploy ship`" + `
owning the Next build, so the workflow itself doesn't need to call
` + "`next build`" + ` separately.

This command still resolves your current target — but only to print the
GitHub secrets you need to configure for it.`,
	Run: func(cmd *cobra.Command, args []string) {
		log := shared.PackageLogger("generate-ci", " CI/CD")

		cfg, err := config.Load()
		if err != nil {
			log.Error("Failed to load nextdeploy.yml: %v", err)
			log.Info("Run `nextdeploy init` first.")
			os.Exit(1)
		}

		normProvider, err := normalizeCIProvider(generateCIProvider)
		if err != nil {
			log.Error("%v", err)
			os.Exit(1)
		}

		target, providerName := resolveCITarget(cfg)
		effectiveTarget := target
		if target == "serverless" && providerName != "" {
			effectiveTarget = providerName
		}
		if generateCITarget != "" {
			normTarget := strings.ToLower(strings.TrimSpace(generateCITarget))
			switch normTarget {
			case "cloudflare", "aws", "vps", "auto":
				effectiveTarget = normTarget
				if normTarget != "auto" {
					if normTarget == "vps" {
						target = "vps"
						providerName = ""
					} else {
						target = "serverless"
						providerName = normTarget
					}
				}
			default:
				log.Error("Unsupported target %q. Supported: cloudflare, vps, aws, auto", generateCITarget)
				os.Exit(1)
			}
		}
		log.Info("Resolved target=%s provider=%s deployment_type=%s ci_provider=%s", target, providerName, effectiveTarget, normProvider)

		var workflow string
		var defaultWorkflowPath string
		if normProvider == "gitlab-ci" {
			workflow = cicd.RenderGitLabCIWorkflow(effectiveTarget)
			defaultWorkflowPath = filepath.FromSlash(cicd.GitLabWorkflowPath)
		} else {
			workflow = cicd.RenderDeployWorkflow(effectiveTarget)
			defaultWorkflowPath = filepath.FromSlash(cicd.WorkflowPath)
		}

		workflowPath := defaultWorkflowPath
		if generateCIOutput != "" {
			workflowPath = filepath.FromSlash(generateCIOutput)
		}
		if err := os.MkdirAll(filepath.Dir(workflowPath), 0o750); err != nil {
			log.Error("Failed to create %s: %v", filepath.Dir(workflowPath), err)
			os.Exit(1)
		}

		if !generateCIForce {
			if _, err := os.Stat(workflowPath); err == nil {
				log.Error("Workflow already exists at %s — pass --force to overwrite", workflowPath)
				os.Exit(1)
			}
		}

		if err := os.WriteFile(workflowPath, []byte(workflow), 0o600); err != nil {
			log.Error("Failed to write workflow: %v", err)
			os.Exit(1)
		}

		log.Success("Wrote %s", workflowPath)

		if generateCIPreview {
			if normProvider == "gitlab-ci" {
				log.Warn("Per-PR preview workflows are currently available for GitHub Actions only")
			} else {
				previews := map[string]string{
					cicd.PreviewWorkflowPath:        cicd.RenderPreviewWorkflow(),
					cicd.PreviewCleanupWorkflowPath: cicd.RenderPreviewCleanupWorkflow(),
				}
				for rel, body := range previews {
					p := filepath.FromSlash(rel)
					if !generateCIForce {
						if _, err := os.Stat(p); err == nil {
							log.Error("Workflow already exists at %s — pass --force to overwrite", p)
							os.Exit(1)
						}
					}
					if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
						log.Error("Failed to write %s: %v", p, err)
						os.Exit(1)
					}
					log.Success("Wrote %s", p)
				}
			}
		}

		log.Info("")
		log.Info("Next steps:")
		if normProvider == "gitlab-ci" {
			log.Info("  1. Configure secrets in GitLab → Settings → CI/CD → Variables:")
		} else {
			log.Info("  1. Configure secrets in GitHub → Settings → Secrets and variables → Actions:")
		}
		log.Info("     🔐 Doppler-first (Recommended):")
		log.Info("        You ONLY need to configure ONE secret:")
		log.Info("        - DOPPLER_TOKEN (all deployment credentials and application secrets are streamed directly from Doppler)")
		log.Info("     🔑 Normal way (When not using Doppler):")
		log.Info("        Configure individual provider secrets:")
		for _, s := range secretsForTarget(target, providerName) {
			log.Info("        - %s", s)
		}
		if generateCIPreview && normProvider != "gitlab-ci" {
			log.Info("        - CLOUDFLARE_WORKERS_SUBDOMAIN (required for preview URLs if not stored in Doppler)")
		}
		log.Info("  2. Commit and push to `main` — the workflow runs automatically.")
		log.Info("")
		log.Info("Tip: re-deploys are cheap. NextDeploy short-circuits when nothing changed:")
		log.Info("  - R2/S3 assets are content-hash deduped (only changed files upload).")
		log.Info("  - Worker bundle has a deploy-hash tag — identical bundles skip re-upload.")
	},
}

// normalizeCIProvider standardizes user-provided CI platform strings.
func normalizeCIProvider(raw string) (string, error) {
	norm := strings.ToLower(strings.TrimSpace(raw))
	if norm == "" {
		return "github-actions", nil
	}
	switch norm {
	case "github", "github-actions":
		return "github-actions", nil
	case "gitlab", "gitlab-ci":
		return "gitlab-ci", nil
	default:
		return "", fmt.Errorf("unsupported CI provider %q. Currently supported: github-actions, gitlab-ci", raw)
	}
}

// resolveCITarget collapses cfg into the (target, provider) pair the
// generated workflow needs. Returns ("vps", "") for VPS, or
// ("serverless", "<provider>") for AWS/Cloudflare.
func resolveCITarget(cfg *config.NextDeployConfig) (target, provider string) {
	target = strings.ToLower(cfg.ResolveTargetType(""))
	if target == "serverless" && cfg.Serverless != nil {
		provider = strings.ToLower(cfg.Serverless.Provider)
	}
	return target, provider
}

// secretsForTarget lists the GitHub secrets the workflow expects for the
// chosen deploy target. Order matters — CI logs show this list verbatim.
//
// These are DEPLOY credentials only. App secrets (AUTH_SECRET, database URLs,
// …) belong in nextdeploy.yml so `secenv` folds them into the deploy; putting
// them in the workflow means they only exist in CI, not in local ships.
func secretsForTarget(target, provider string) []string {
	switch target {
	case "serverless":
		switch provider {
		case "cloudflare":
			return []string{
				"CLOUDFLARE_API_TOKEN  (required)",
				"CLOUDFLARE_ACCOUNT_ID (optional; auto-discovered when unset)",
				"NEXTDEPLOY_CF_STATE_KEY (optional; same value across team/CI for shared resource state)",
				"R2_ACCESS_KEY_ID      (optional; API token can provide R2 access)",
				"R2_SECRET_ACCESS_KEY  (optional; API token can provide R2 access)",
			}
		case "aws":
			return []string{
				"AWS_ACCESS_KEY_ID     (required)",
				"AWS_SECRET_ACCESS_KEY (required)",
				"AWS_REGION            (optional — defaults to cfg.cloud_provider.region)",
			}
		}
	case "vps":
		return []string{
			"VPS_HOST              (optional — required when nextdeploy.yml uses host: ${VPS_HOST} or omits host)",
			"SSH_PRIVATE_KEY       (required — private key configured in nextdeploy.yml)",
			"SSH_KNOWN_HOSTS       (optional — pins server host key in CI to prevent MITM)",
			"SSH_KEY_PASSPHRASE    (optional — if private key is encrypted with a passphrase)",
			"NEXTDEPLOY_STRICT_HOST_KEY (optional — set to 1 to fail closed if host key is unpinned)",
		}
	}
	return []string{"(no deploy credentials required for this target)"}
}

func init() {
	generateCICmd.Flags().BoolVar(&generateCIForce, "force", false, "Overwrite existing workflow files")
	generateCICmd.Flags().BoolVar(&generateCIPreview, "preview", false,
		"Also emit per-PR preview workflows (preview.yml + preview-cleanup.yml, Cloudflare only)")
	generateCICmd.Flags().StringVar(&generateCIOutput, "output", "", "Target workflow file destination (default: .github/workflows/nextdeploy.yml)")
	generateCICmd.Flags().StringVar(&generateCIProvider, "provider", "github-actions", "CI provider to target (github-actions, gitlab-ci)")
	generateCICmd.Flags().StringVar(&generateCITarget, "target", "", "Target deployment type override (cloudflare, vps, aws, auto)")
	rootCmd.AddCommand(generateCICmd)
}
