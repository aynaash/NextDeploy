package cmd

import (
	"context"
	"fmt"
	"io"
	"nextdeploy/internal/logger"
	"nextdeploy/internal/server"
	"nextdeploy/internal/ship"
	"os"
	"os/signal"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

const (
	bluePort  = "3001" // Blue container port
	greenPort = "3002" // Green container port
)

var (
	ShipLogs = logger.PackageLogger("ship", "🚢")

	// Color definitions
	successColor = color.New(color.FgGreen, color.Bold)
	errorColor   = color.New(color.FgRed, color.Bold)
	warnColor    = color.New(color.FgYellow, color.Bold)
	infoColor    = color.New(color.FgCyan, color.Bold)
	debugColor   = color.New(color.FgMagenta)

	// Command flags
	forceDeploy bool
	dryRun      bool
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
	out := cmd.OutOrStdout()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	go func() {
		<-c
		cancel()
		fmt.Fprintln(out, "\n🚨 Deployment interrupted by user! 🚨")
		os.Exit(1)
	}()

	infoColor.Printf("Starting deployment process at %s\n\n", time.Now().Format(time.RFC1123))

	if dryRun {
		warnColor.Println("🚧 DRY RUN MODE: No changes will be made 🚧\n")
	}

	serverMgr, err := server.New(
		server.WithConfig(),
		server.WithSSH(),
	)
	if err != nil {
		errorColor.Printf("Failed to initialize server manager: %v\n", err)
		os.Exit(1)
	}
	defer func() {
		if err := serverMgr.CloseSSHConnections(); err != nil {
			errorColor.Printf("Error closing connections: %v\n", err)
		}
	}()

	servers := serverMgr.ListServers()
	if len(servers) == 0 {
		errorColor.Println("No servers configured for deployment")
		os.Exit(1)
	}
	var stream = io.Discard

	if err := runDeployment(ctx, serverMgr, servers, stream); err != nil {
		errorColor.Printf("Deployment failed: %v\n", err)
		os.Exit(1)
	}

	successColor.Println("\n🎉 Deployment completed successfully! 🎉")
}

func runDeployment(ctx context.Context, serverMgr *server.ServerStruct, servers []string, stream io.Writer) error {
	infoColor.Println("\n=== PHASE 1: Pre-deployment checks ===")
	if err := ship.VerifyServers(ctx, serverMgr, servers, stream); err != nil {
		return fmt.Errorf("pre-deployment checks failed: %w", err)
	}

	infoColor.Println("\n=== PHASE 2: File transfers ===")
	if err := ship.TransferRequiredFiles(ctx, serverMgr, stream, servers[0]); err != nil {
		return fmt.Errorf("file transfer failed: %w", err)
	}

	if !dryRun {
		infoColor.Println("\n=== PHASE 3: Container deployment ===")
		if err := ship.DeployContainers(ctx, serverMgr, servers[0], stream); err != nil {
			return fmt.Errorf("container deployment failed: %w", err)
		}
	}

	infoColor.Println("\n=== PHASE 4: Post-deployment verification ===", stream)
	if err := ship.VerifyDeployment(ctx, serverMgr, servers[0], stream); err != nil {
		return fmt.Errorf("post-deployment verification failed: %w", err)
	}

	return nil
}
