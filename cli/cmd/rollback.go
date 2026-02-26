package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var rollbackCmd = &cobra.Command{
	Use:   "rollback",
	Short: "Rollback to the previous deployment instantly",
	Long:  "Swaps the active symlink to the previous release and restarts the application via the daemon.",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("NextDeploy Rollback")
		fmt.Println("// TODO: Hook into the daemon's snapshot/symlink management to instantly revert process.")
	},
}

func init() {
	rootCmd.AddCommand(rollbackCmd)
}
