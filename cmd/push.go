
package cmd

import (
	"fmt"
	"nextdeploy/utils"

	"github.com/spf13/cobra"
)

var pushCmd = &cobra.Command{
	Use:   "push",
	Short: "Pushes the latest Docker image",
	Run: func(cmd *cobra.Command, args []string) {
		if err := utils.BuildAndPushDockerImage(imageName, registry, true); err != nil {
			fmt.Println("Error:", err)
		}
	},
}

func init() {
	pushCmd.Flags().StringVarP(&imageName, "image", "i", "my-app", "Docker image name")
	pushCmd.Flags().StringVarP(&registry, "registry", "r", "docker.io/myuser", "Docker registry")
	rootCmd.AddCommand(pushCmd)
}
