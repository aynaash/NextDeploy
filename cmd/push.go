package cmd

import (
	"github.com/spf13/cobra"
	"nextdeploy/internal/logger"
	"nextdeploy/internal/registry"
)

var (
	imagename    string
	registryName string
)
var reg *registry.RegistryValidator
var (
	plog = logger.PackageLogger("Pushcommand", "🅿️ PUSHCOMMAND")
)

func init() {
	var err error
	reg = registry.NewRegistryValidator()
	if err != nil {
		plog.Error("Failed to initialize registry validator: %v", err)
	}

	pushCmd := &cobra.Command{
		Use:   "push",
		Short: "Push the latest Docker image to the specified registry",
		RunE: func(cmd *cobra.Command, args []string) error {
			localImage, _ := cmd.Flags().GetString("image")
			tag, _ := cmd.Flags().GetString("tag")
			return reg.PushImageToRegistry(localImage, tag)
		},
	}
	pushCmd.Flags().StringVarP(&imagename, "image", "i", "", "Docker image name (required)")
	pushCmd.Flags().StringVarP(&registryName, "registry", "r", "", "Docker registry (e.g., docker.io/myuser/my-app) (required)")

	// Mark flags as required
	rootCmd.AddCommand(pushCmd)
}
