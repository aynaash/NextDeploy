package cmd

import (
	"github.com/aynaash/nextdeploy/cli/internal/initialcommand"
	"github.com/spf13/cobra"
)

type PackageManager string

func (pm PackageManager) String() string {
	return string(pm)
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize Next.js deployment configuration or scaffold a new fullstack project",
	Long: `Scaffolds deployment configuration for Next.js applications or generates a complete
production-ready fullstack starter (App Router, React 19, TypeScript, styling, and CI).

When run in an empty directory or non-Next.js project, NextDeploy prompts to create
the fullstack starter for your target platform (Cloudflare Workers, VPS, or AWS).`,
	RunE: initialcommand.RunInitCommand,
}

func init() {
	initCmd.Flags().String("target", "", "Deploy target: vps | aws | cloudflare (skip the interactive prompt)")
	initCmd.Flags().String("platform", "", "Alias for --target")
	initCmd.Flags().String("mode", "", "Cloudflare only: scaffold | sniff (skip the interactive prompt)")
	initCmd.Flags().String("db", "", "Cloudflare scaffold only: d1 | byo (skip the interactive prompt)")
	rootCmd.AddCommand(initCmd)
}
