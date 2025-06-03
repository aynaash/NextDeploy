/*
* I want this command to push the  latest image to given registry
* - login to registry
* - push latest image to BuildAndPushDockerImage
* - hanlde errors 
*  use doppler to get secrets 
*
*
*
*/
package cmd

import (
	"fmt"
	"nextdeploy/internal/utils"

	"github.com/spf13/cobra"
)

var pushCmd = &cobra.Command{
	Use:   "push",
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
