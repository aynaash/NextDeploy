package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var initialize = &cobra.Command{
	Use:   "Initilization command",
	Short: "Initiates the project files and config for docker file and NextDeploy.yml file",
	Long:  `This command initiates the NextDeploy by generating a docker file template for nextjs image and secrets file that it needs for other commands`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Next deploy init command")

	},
}

func init() {
	rootCmd.AddCommand(initialize)
}
