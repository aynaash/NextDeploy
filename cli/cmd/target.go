package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/aynaash/nextdeploy/shared/config"
	"github.com/spf13/cobra"
)

var (
	targetJSON bool
)

var targetCmd = &cobra.Command{
	Use:     "target",
	Aliases: []string{"target-type", "deployment-type"},
	Short:   "Print the resolved deployment target and provider for this project",
	Long: `Reads nextdeploy.yml and prints the effective deployment type (cloudflare, aws, or vps).
Useful in CI/CD pipelines, automation scripts, and local debugging.`,
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := config.Load()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading configuration: %v\n", err)
			os.Exit(1)
		}

		target, provider := resolveCITarget(cfg)
		deployType := target
		if target == "serverless" && provider != "" {
			deployType = provider
		}

		if targetJSON {
			out := map[string]string{
				"target":          target,
				"provider":        provider,
				"deployment_type": deployType,
			}
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			_ = enc.Encode(out)
			return
		}

		fmt.Println(deployType)
	},
}

func init() {
	targetCmd.Flags().BoolVar(&targetJSON, "json", false, "Output resolved target as JSON")
	rootCmd.AddCommand(targetCmd)
}
