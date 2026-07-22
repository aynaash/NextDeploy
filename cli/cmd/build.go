package cmd

import (
	"context"
	"os"

	"github.com/aynaash/nextdeploy/cli/internal/buildflow"
	"github.com/aynaash/nextdeploy/shared"
	"github.com/aynaash/nextdeploy/shared/config"

	"github.com/spf13/cobra"
)

var forceBuild bool

var buildCmd = &cobra.Command{
	Use:   "build",
	Short: "Validate, run `next build`, and prepare a deployable artifact",
	Run: func(cmd *cobra.Command, args []string) {
		log := shared.PackageLogger("build", "BUILD")
		log.Info("Starting NextDeploy build process...")

		cfg, err := config.Load()
		if err != nil {
			log.Error("Failed to load config: %v", err)
			os.Exit(1)
		}

		result, err := buildflow.Run(context.Background(), buildflow.Opts{
			ProjectDir: ".",
			Cfg:        cfg,
			Force:      forceBuild,
			Log:        log,
		})
		if err != nil {
			log.Error("Build failed: %v", err)
			os.Exit(1)
		}
		if result.Skipped {
			os.Exit(0)
		}

		if result.TarballPath != "" {
			log.Info("Build complete. Artifact: %s", result.TarballPath)
		} else {
			log.Info("Build complete. Standalone tree at %s", result.StandaloneDir)
		}
	},
}

func init() {
	buildCmd.Flags().BoolVarP(&forceBuild, "force", "f", false, "Force a full build even if git commit is unchanged")
	rootCmd.AddCommand(buildCmd)
}
