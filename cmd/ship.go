package cmd

import (
	"context"
	"fmt"
	"io"
	"nextdeploy/internal/failfast"
	"nextdeploy/internal/logger"
	"nextdeploy/internal/server"
	"nextdeploy/internal/ship"
	"os"
	"os/signal"

	"github.com/spf13/cobra"
)

const (
	bluePort  = "3001" // Blue container port
	greenPort = "3002" // Green container port
)

var (
	ShipLogs = logger.PackageLogger("ship::", "🚢::")
	// Command flags
	dryRun bool
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

	if dryRun {
		ShipLogs.Warn("🚧 DRY RUN MODE: No changes will be made 🚧\n")
	}

	serverMgr, err := server.New(
		server.WithConfig(),
		server.WithSSH(),
	)
	failfast.Failfast(err, failfast.Error, "Failed to initialize server manager")
	defer func() {
		if err := serverMgr.CloseSSHConnections(); err != nil {
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
		failfast.Failfast(err, failfast.Error, "Deployment failed")
		os.Exit(1)
	}

	ShipLogs.Success("\n🎉 Deployment completed successfully! 🎉")
}

func runDeployment(ctx context.Context, serverMgr *server.ServerStruct, servers []string, stream io.Writer) error {
	ShipLogs.Info("=== PHASE 1: Pre-deployment checks ===")
	if err := ship.VerifyServers(ctx, serverMgr, servers, stream); err != nil {
		failfast.Failfast(err, failfast.Error, "Pre-deployment checks failed")
		return fmt.Errorf("pre-deployment checks failed: %w", err)
	}

	ShipLogs.Info("=== PHASE 2: File transfers ===")
	if err := ship.TransferRequiredFiles(ctx, serverMgr, stream, servers[0]); err != nil {
		return fmt.Errorf("file transfer failed: %w", err)
	}

	if !dryRun {
		ShipLogs.Info("=== PHASE 3: Container deployment ===")
		if err := ship.DeployContainers(ctx, serverMgr, servers[0], stream); err != nil {
			failfast.Failfast(err, failfast.Error, "Container deployment failed")
			return fmt.Errorf("container deployment failed: %w", err)
		}
	}

	ShipLogs.Info("=== PHASE 4: Post-deployment verification ===", stream)
	if err := ship.VerifyDeployment(ctx, serverMgr, servers[0], stream); err != nil {
		failfast.Failfast(err, failfast.Error, "Post-deployment verification failed")
		return fmt.Errorf("post-deployment verification failed: %w", err)
	}

	return nil
}
