package cmd

// FEATURE: GIVE USER  ABILITY TO START FROM ZERO USING OUR OWN NEXTJS TEMPLATES

import (
	"github.com/spf13/cobra"
	"nextdeploy/internal/detect"
	"nextdeploy/internal/initialcommand"
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
	PreRunE: detect.ValidateNextJSProject,
	RunE:    initialcommand.RunInitCommand,
}

func init() {
	rootCmd.AddCommand(initCmd)
}
