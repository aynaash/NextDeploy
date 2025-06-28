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

var (
	ShipLogs = logger.PackageLogger("ship::", "🚢::")
	// Command flags
	dryRun bool
	fresh  bool // fresh flag for caddy setup
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
	// add fresh boolean flag for dry run
	shipCmd.Flags().BoolVarP(&dryRun, "fresh ", "f", false, "Perform caddy setup")
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

	ShipLogs.Info("=== PHASE 4: Post-deployment verification ===")
	if err := ship.VerifyDeployment(ctx, serverMgr, servers[0], stream); err != nil {
		failfast.Failfast(err, failfast.Error, "Post-deployment verification failed")
		return fmt.Errorf("post-deployment verification failed: %w", err)
	}

	ShipLogs.Info(" ==== PHASE 4: Setting up caddy ====")
	if err := ship.SetupCaddy(ctx, serverMgr, servers[0], fresh, stream); err != nil {
		failfast.Failfast(err, failfast.Error, "Caddy setup failed")
		return fmt.Errorf("caddy setup failed: %w", err)
	}

	return nil
}
