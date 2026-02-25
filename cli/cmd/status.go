package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show the current status of your deployed application",
	Long:  "Queries the daemon for active PID, uptime, memory, routes, and ISR cache warmth.",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("🚀 NextDeploy Status")
		fmt.Println("// TODO: Yusuf - Query the daemon via API to populate dynamic PID, memory, uptime, and route metrics.")
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
