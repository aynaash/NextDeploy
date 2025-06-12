package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"nextdeploy/internal/docker"
	"nextdeploy/internal/registry"
	"nextdeploy/internal/utils"
)

var pushCmd = &cobra.Command{
	Use:   "push",
	Short: "Push the latest Docker image to the specified registry",
	Run:   docker.PushImageToRegistry,
}

func init() {
	pushCmd.Flags().StringVarP(&imageName, "image", "i", "my-app", "Docker image name")
	pushCmd.Flags().StringVarP(&registry, "registry", "r", "docker.io/myuser", "Docker registry")
	rootCmd.AddCommand(pushCmd)
}
