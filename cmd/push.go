package cmd

import (
	"github.com/spf13/cobra"
	"log"
	"nextdeploy/internal/registry"
)

var (
	imagename    string
	registryName string
)
var reg *registry.RegistryValidator

func init() {
	var err error
	reg, err = registry.NewRegistryValidator()
	if err != nil {
		log.Fatalf("Failed to initialize registry validator: %v", err)
	}

	pushCmd := &cobra.Command{
		Use:   "push",
		Short: "Push the latest Docker image to the specified registry",
		RunE:  reg.PushImageToRegistry, // Handles errors
	}

	pushCmd.Flags().StringVarP(&imagename, "image", "i", "", "Docker image name (required)")
	pushCmd.Flags().StringVarP(&registryName, "registry", "r", "", "Docker registry (e.g., docker.io/myuser/my-app) (required)")

	// Mark flags as required
	rootCmd.AddCommand(pushCmd)
}
