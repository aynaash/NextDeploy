package cmd

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/aynaash/nextdeploy/cli/internal/cicd"
	"github.com/aynaash/nextdeploy/shared"
	"github.com/aynaash/nextdeploy/shared/config"

	"github.com/spf13/cobra"
)

var (
	generateCIForce   bool
	generateCIPreview bool
)

var generateCICmd = &cobra.Command{
	Use:     "generate-ci",
	Aliases: []string{"ci"},
	Short:   "Generate a GitHub Actions workflow for zero-touch deployment",
	Long: `Creates a .github/workflows/nextdeploy.yml file that automatically
builds your Next.js project and ships it using NextDeploy on every push to main.

The workflow detects the deployment type from nextdeploy.yml AT RUNTIME and
runs the matching deploy (cloudflare / vps), so retargeting the app does
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

		target, providerName := resolveCITarget(cfg)
		log.Info("Resolved target=%s provider=%s", target, providerName)

		// The workflow itself is target-agnostic — it detects the type on the
		// runner. The resolved target is only used to tell the user which
		// secrets to configure.
		workflow := cicd.RenderDeployWorkflow()

		workflowPath := filepath.FromSlash(cicd.WorkflowPath)
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

		log.Info("")
		log.Info("Next steps:")
		log.Info("  1. Add the secrets listed below in GitHub → Settings → Secrets and variables → Actions:")
		for _, s := range secretsForTarget(target, providerName) {
			log.Info("       - %s", s)
		}
		if generateCIPreview {
			log.Info("       - CLOUDFLARE_WORKERS_SUBDOMAIN (required for preview URLs — your")
			log.Info("         account's workers.dev subdomain, from the Cloudflare dashboard)")
		}
		log.Info("  2. Commit and push to `main` — the workflow runs automatically.")
		log.Info("")
		log.Info("Tip: re-deploys are cheap. NextDeploy short-circuits when nothing changed:")
		log.Info("  - R2 assets are content-hash deduped (only changed files upload).")
		log.Info("  - Worker bundle has a deploy-hash tag — identical bundles skip re-upload.")
	},
}

// resolveCITarget collapses cfg into the (target, provider) pair the
// generated workflow needs. Returns ("vps", "") for VPS, or
// ("serverless", "<provider>") for Cloudflare.
func resolveCITarget(cfg *config.NextDeployConfig) (target, provider string) {
	target = strings.ToLower(cfg.TargetType)
	if target == "" {
		target = "vps"
	}
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
				"CLOUDFLARE_ACCOUNT_ID (required)",
				"R2_ACCESS_KEY_ID      (required if you upload assets to R2)",
				"R2_SECRET_ACCESS_KEY  (required if you upload assets to R2)",
			}
		}
	case "vps":
		return []string{
			"SSH_PRIVATE_KEY (required — same key your local nextdeploy uses)",
		}
	}
	return []string{"(no deploy credentials required for this target)"}
}

func init() {
	generateCICmd.Flags().BoolVar(&generateCIForce, "force", false, "Overwrite existing workflow files")
	generateCICmd.Flags().BoolVar(&generateCIPreview, "preview", false,
		"Also emit per-PR preview workflows (preview.yml + preview-cleanup.yml, Cloudflare only)")
	rootCmd.AddCommand(generateCICmd)
}
