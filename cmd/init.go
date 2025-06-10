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

var (
	forceOverwrite bool
	skipPrompts    bool
	defaultConfig  bool
	devConfig      bool
)

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
	initCmd.Flags().BoolVarP(&forceOverwrite, "force", "f", false,
		"Overwrite existing files without prompting")
	initCmd.Flags().BoolVarP(&skipPrompts, "yes", "y", false,
		"Skip all prompts and generate sample configuration file:Make sure to add the actual values")
	initCmd.Flags().BoolVar(&defaultConfig, "default-config", false,
		"Generate with default configuration only")
	initCmd.Flags().BoolVar(&devConfig, "y", true,
		"Generate with sample data and edit in the yaml file")

	rootCmd.AddCommand(initCmd)
}
