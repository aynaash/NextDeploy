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
	reg, err = registry.New()
	if err != nil {
		plog.Error("Failed to initialize registry validator: %v", err)
	}

	pushCmd := &cobra.Command{
		Use:   "push",
		Short: "Push the latest Docker image to the specified registry",
		RunE: func(cmd *cobra.Command, args []string) error {
			//	localImage, _ := cmd.Flags().GetString("image")
			// tag, _ := cmd.Flags().GetString("tag")
			// if tag == "" {
			// 	plog.Error("Tag is required for pushing the image")
			// }
			// TODO: use tag if provided
			//return reg.PushImageToRegistry()
			if imagename == "" || registryName == "" {
				plog.Error("Both --image and --registry flags are required")
				return nil
			}
			plog.Info("Pushing image '%s' to registry '%s'", imagename, registryName)
			return reg.PushImage(imagename)
		},
	}
	pushCmd.Flags().StringVarP(&imagename, "image", "i", "", "Docker image name (required)")
	pushCmd.Flags().StringVarP(&registryName, "registry", "r", "", "Docker registry (e.g., docker.io/myuser/my-app) (required)")

	// Mark flags as required
	rootCmd.AddCommand(pushCmd)
}
