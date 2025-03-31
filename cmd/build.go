package cmd

import (
	"fmt"
	"NextOperations/utils"

	"github.com/spf13/cobra"
)

var imageName, registry string

var buildCmd = &cobra.Command{
	Use:   "build",
	Short: "Builds a Docker image using the latest Git commit hash",
	Run: func(cmd *cobra.Command, args []string) {
		if !utils.DockerfileExists() {
			fmt.Println("No Dockerfile found in the current working directory")
			return
		}

		if err := utils.BuildAndPushDockerImage(imageName, registry, false); err != nil {
			fmt.Println("Error:", err)
		}
	},
}

func init() {
	buildCmd.Flags().StringVarP(&imageName, "image", "i", "my-app", "Docker image name")
	// TODO: Change this to DigitalOcean registry
	buildCmd.Flags().StringVarP(&registry, "registry", "r", "docker.io/myuser", "Docker registry")

	rootCmd.AddCommand(buildCmd)
}
