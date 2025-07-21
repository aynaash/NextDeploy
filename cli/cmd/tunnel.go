package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"nextdeploy/daemon/core"
)

var tunnelCmd = &cobra.Command{
	Use:   "tunnel",
	Short: "Manage reverse tunnels for agents behind firewalls",
}

var startTunnelCmd = &cobra.Command{
	Use:   "start",
	Short: "Start a reverse tunnel",
	Run: func(cmd *cobra.Command, args []string) {
		sshHost, _ := cmd.Flags().GetString("ssh-host")
		sshUser, _ := cmd.Flags().GetString("ssh-user")
		keyPath, _ := cmd.Flags().GetString("key-path")
		localPort, _ := cmd.Flags().GetInt("local-port")
		remotePort, _ := cmd.Flags().GetInt("remote-port")

		tunnel := core.NewReverseTunnel(sshHost, sshUser, keyPath, localPort, remotePort)
		if err := tunnel.Start(); err != nil {
			fmt.Printf("Failed to start tunnel: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("Reverse tunnel started successfully")
		fmt.Printf("Local port %d is available remotely on port %d\n", localPort, remotePort)
	},
}

func init() {
	tunnelCmd.AddCommand(startTunnelCmd)
	rootCmd.AddCommand(tunnelCmd)

	startTunnelCmd.Flags().String("ssh-host", "", "SSH server host (e.g., tunnel.nextdeploy.com:22)")
	startTunnelCmd.Flags().String("ssh-user", "nextdeploy", "SSH username")
	startTunnelCmd.Flags().String("key-path", "~/.ssh/id_rsa", "Path to SSH private key")
	startTunnelCmd.Flags().Int("local-port", 8443, "Local port to expose")
	startTunnelCmd.Flags().Int("remote-port", 0, "Remote port to bind (0 for random)")
}
