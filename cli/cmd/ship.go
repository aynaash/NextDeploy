package cmd

import (
	"context"
	"fmt"
	"io"
	"nextdeploy/cli/internal/server"
	"nextdeploy/cli/internal/ship"
	"nextdeploy/shared"
	"nextdeploy/shared/registry"
	"os"
	"os/signal"

	"github.com/spf13/cobra"
)

var (
	ShipLogs    = shared.PackageLogger("ship::", "🚢::")
	dryRun      bool
	credentials bool
	newapp      bool
	bluegreen   bool
	serve       bool
)

var shipCmd = &cobra.Command{
	Use:   "ship",
	Short: "Deploy a containerized application to target VPS",
	Long: `The ship command handles the complete deployment lifecycle:
- Verifies server connectivity
- Transfers necessary files
- Pulls the specified Docker image
- Deploys containers with proper configuration
- Verifies deployment success`,
	Run: Ship,
}

func init() {
	shipCmd.Flags().BoolVarP(&serve, "serve", "s", false, "Perform new caddy setup")
	shipCmd.Flags().BoolVarP(&credentials, "credentials", "c", false, "Use credentials for deployment")
	shipCmd.Flags().BoolVarP(&dryRun, "dry-run", "d", false, "Simulate deployment without making changes")
	shipCmd.Flags().BoolVarP(&newapp, "new", "n", false, "Indicate this is a new application deployment")
	shipCmd.Flags().BoolVarP(&bluegreen, "bluegreen", "b", false, "Use blue-green deployment strategy")

	rootCmd.AddCommand(shipCmd)
}

func Ship(cmd *cobra.Command, args []string) {

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	go func() {
		<-c
		cancel()
		ShipLogs.Debug("Received interrupt signal, cleaning up...")
		os.Exit(1)
	}()
	ShipLogs.Debug("Starting deployment process...")

	serverMgr, err := server.New(
		server.WithConfig(),
		server.WithSSH(),
	)
	if err != nil {
		return
	}
	defer func() {
		if err := serverMgr.CloseSSHConnection(); err != nil {
			ShipLogs.Error("Error closing connections: %v\n", err)
		}
	}()

	servers := serverMgr.ListServers()
	if len(servers) == 0 {
		ShipLogs.Debug("No servers configured for deployment")
		os.Exit(1)
	}
	var stream = io.Discard

	if err := runDeployment(ctx, serverMgr, servers, stream); err != nil {
		ShipLogs.Error("Deployment failed: %v", err)
		return
	}

	ShipLogs.Success("\n🎉 Deployment completed successfully! 🎉")
}

func runDeployment(ctx context.Context, serverMgr *server.ServerStruct, servers []string, stream io.Writer) error {
	ShipLogs.Info("=== PHASE 1: Pre-deployment checks ===")
	if err := ship.VerifyServers(ctx, serverMgr, servers, stream); err != nil {
		return fmt.Errorf("pre-deployment checks failed: %w", err)
	}

	ShipLogs.Info("=== PHASE 2: File transfers ===")
	if err := ship.TransferRequiredFiles(ctx, serverMgr, stream, servers[0]); err != nil {
		return fmt.Errorf("file transfer failed: %w", err)
	}

	if !dryRun {
		ShipLogs.Info("=== PHASE 3: Container deployment ===")
		if err := ship.DeployContainers(ctx, serverMgr, servers[0], credentials, stream); err != nil {
			return fmt.Errorf("container deployment failed: %w", err)
		}
	}

	ShipLogs.Info("=== PHASE 4: Post-deployment verification ===")
	if err := ship.VerifyDeployment(ctx, serverMgr, servers[0], stream); err != nil {
		return fmt.Errorf("post-deployment verification failed: %w", err)
	}
	// serverMgr execture nextdeployd pull --image=image --newapp --bluegreen
	latestImageName := registry.GetLatestImageName()

	ShipLogs.Info("Pulling latest image: %s", latestImageName)

	output, err := serverMgr.ExecuteCommand(ctx, servers[0], fmt.Sprintf("nextdeployd pull --image=%s", latestImageName), stream)
	if err != nil {
		ShipLogs.Error("Error pulling latest image: %v", err)
		return fmt.Errorf("failed to pull latest image: %w", err)
	}
	ShipLogs.Info("Pull output: %s", output)

	currentContainer := registry.GetLatestImageName()
	ShipLogs.Info("Current running container: %s", currentContainer)

	// switch to new container
	ShipLogs.Info("Switching to new container...")

	switchResult, err := serverMgr.ExecuteCommand(ctx, servers[0], fmt.Sprintf("nextdeployd switch --current=%s --new=%s --newapp=%t --bluegreen=%t", currentContainer, latestImageName, newapp, bluegreen), stream)
	if err != nil {
		ShipLogs.Error("Error switching containers: %v", err)
		return fmt.Errorf("failed to switch containers: %w", err)
	}
	ShipLogs.Info("Switch output: %s", switchResult)

	if serve {
		ShipLogs.Info("Setting up Caddy server...")
		caddyOutput, err := serverMgr.ExecuteCommand(ctx, servers[0], "nextdeployd caddy --setup", stream)
		if err != nil {
			ShipLogs.Error("Error setting up Caddy: %v", err)
			return fmt.Errorf("failed to setup Caddy: %w", err)
		}
		ShipLogs.Info("Caddy setup output: %s", caddyOutput)
	}

	return nil
}
