package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/aynaash/nextdeploy/cli/internal/serverless"
	"github.com/aynaash/nextdeploy/shared"
	"github.com/aynaash/nextdeploy/shared/config"
	"github.com/spf13/cobra"
)

var applyYes bool

var applyCmd = &cobra.Command{
	Use:   "apply",
	Short: "Reconcile: create every declared Cloudflare resource that's missing",
	Long: `Computes the plan (like 'nextdeploy plan'), then provisions everything
declared under cloudflare.resources.* that doesn't already exist — D1 databases
(with migrations), KV namespaces, Hyperdrive configs, queues, Vectorize indexes,
AI gateways, and DNS records.

Idempotent and safe to re-run: existing resources are left untouched, and a
resource deleted out-of-band is recreated. Stops without changes if immutable
drift is detected (e.g. a Vectorize dimension mismatch).

Exit codes:
  0  reconcile applied cleanly
  1  unable to apply (auth, network, etc.)
  2  immutable drift detected — nothing applied`,
	Run: func(cmd *cobra.Command, args []string) {
		log := shared.PackageLogger("apply", "🛠️  APPLY")

		cfg, err := config.Load()
		if err != nil {
			log.Error("Failed to load config: %v", err)
			os.Exit(1)
		}
		if cfg.Serverless == nil || cfg.Serverless.Cloudflare == nil {
			log.Error("apply only supports Cloudflare deployments — no serverless.cloudflare block found")
			os.Exit(1)
		}

		ctx := context.Background()
		provider := serverless.NewCloudflareProvider()
		if err := provider.Initialize(ctx, cfg); err != nil {
			log.Error("Cloudflare initialize failed: %v", err)
			os.Exit(1)
		}

		plan, err := provider.Plan(ctx, cfg)
		if err != nil {
			log.Error("Plan failed: %v", err)
			os.Exit(1)
		}
		renderPlan(plan)
		if plan.HasDrift() {
			os.Exit(2)
		}
		if pendingChanges(plan) == 0 {
			fmt.Println("Nothing to apply — all declared resources already exist.")
			return
		}
		if !applyYes && !confirm("Apply these changes to Cloudflare?") {
			fmt.Println("Aborted.")
			return
		}

		result, err := provider.Apply(ctx, cfg)
		if err != nil {
			log.Error("Apply failed: %v", err)
			if result != nil && result.HasDrift() {
				os.Exit(2)
			}
			os.Exit(1)
		}
		fmt.Println()
		fmt.Println(success("✓ Reconcile complete — declared resources are in place."))
	},
}

func pendingChanges(r *serverless.PlanResult) int {
	n := 0
	for _, it := range r.Items {
		if it.Action == serverless.PlanCreate || it.Action == serverless.PlanUpdate {
			n++
		}
	}
	return n
}

func confirm(prompt string) bool {
	fmt.Printf("%s [y/N]: ", prompt)
	var answer string
	if _, err := fmt.Scanln(&answer); err != nil {
		return false
	}
	return answer == "y" || answer == "Y" || answer == "yes"
}

func init() {
	applyCmd.Flags().BoolVar(&applyYes, "yes", false, "Skip the confirmation prompt (non-interactive)")
	rootCmd.AddCommand(applyCmd)
}
