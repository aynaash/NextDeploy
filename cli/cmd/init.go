package cmd

import (
	"github.com/aynaash/nextdeploy/cli/internal/initialcommand"
	"github.com/aynaash/nextdeploy/shared/nextcore"
	"github.com/spf13/cobra"
)

type PackageManager string

func (pm PackageManager) String() string {
	return string(pm)
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize Next.js deployment configuration",
	Long: `Scaffolds deployment configuration for Next.js applications including:
- Dockerfile for containerization
- nextdeploy.yml configuration
- Optional sample files and gitignore updates`,
	PreRunE: nextcore.ValidateNextJSProject,
	RunE:    initialcommand.RunInitCommand,
}

func init() {
	initCmd.Flags().String("target", "", "Deploy target: vps | aws | cloudflare (skip the interactive prompt)")
	initCmd.Flags().String("mode", "", "Cloudflare only: scaffold | sniff (skip the interactive prompt)")
	initCmd.Flags().String("db", "", "Cloudflare scaffold only: d1 | byo (skip the interactive prompt)")
	rootCmd.AddCommand(initCmd)
}
