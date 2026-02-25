package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var logsCmd = &cobra.Command{
	Use:   "logs",
	Short: "Stream application logs natively from the daemon",
	Long:  "Streams systemd journal logs natively with capabilities to filter by specific Next.js routes.",
	Run: func(cmd *cobra.Command, args []string) {
		routeFilter, _ := cmd.Flags().GetString("route")

		fmt.Printf("🚀 NextDeploy Logs (Route Filter: %s)\n", routeFilter)
		fmt.Println("// TODO: Yusuf - Hook into the daemon's log streaming websocket/SSE to tail logs accurately.")
	},
}

func init() {
	logsCmd.Flags().String("route", "", "Filter logs by a specific route (e.g. /api/upload)")
	rootCmd.AddCommand(logsCmd)
}
