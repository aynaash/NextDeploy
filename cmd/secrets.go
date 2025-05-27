package cmd 

import (
	"github.com/spf13/cobra"
)

var secretsCmd = &cobra.Command{
	Use:   "secrets",
	Short: "Manage secrets for your NextDeploy projects",
	Long: `Command for adding, listing and managing secret provider like Doppler`,
}

func init() {
	rootCmd.AddCommand(secretsCmd)
}
