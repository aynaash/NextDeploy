package cmd

import (
	"context"
	"github.com/spf13/cobra"
	"nextdeploy/internal/logger"
	"nextdeploy/internal/server"
	"os"
)

var (
	PrepLogs = logger.PackageLogger("prepare", "🔧 PREPARE")
)

// Add to cmd package
var prepareCmd = &cobra.Command{
	Use:   "prepare",
	Short: "Prepare target server with required tools",
	Long:  "Installs Docker, Caddy, Go and other required tools on the target server",
	Run: func(cmd *cobra.Command, args []string) {
		serverMgr, err := server.New(
			server.WithConfig(configFile),
			server.WithSSH(),
		)
		if err != nil {
			PrepLogs.Error("Failed to initialize server manager: %v", err)
			os.Exit(1)
		}
		defer serverMgr.CloseSSHConnections()

		servers := serverMgr.ListServers()
		if len(servers) == 0 {
			PrepLogs.Error("No servers configured")
			os.Exit(1)
		}

		if err := serverMgr.PrepareServer(context.Background(), servers[0]); err != nil {
			PrepLogs.Error("Preparation failed: %v", err)
			os.Exit(1)
		}

		PrepLogs.Success("Server preparation completed!")
	},
}

// Add to init()
func init() {
	prepareCmd.Flags().StringVarP(&configFile, "config", "c", "nextdeploy.yml", "Path to configuration file")
	rootCmd.AddCommand(prepareCmd)
}
