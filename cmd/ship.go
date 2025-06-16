package cmd

import (
	"context"
	"fmt"
	"nextdeploy/internal/logger"
	"nextdeploy/internal/server"
	"os"
	"sync"
	"time"

	"github.com/spf13/cobra"
)

var (
	ShipLogs = logger.PackageLogger("ship", "🚢")

	// Command flags
	forceDeploy bool
	dryRun      bool
)

// shipCmd represents the ship command
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
	// Add flags to the ship command
	shipCmd.Flags().StringVarP(&configFile, "config", "c", "nextdeploy.yml", "Path to configuration file")
	shipCmd.Flags().BoolVar(&forceDeploy, "force", false, "Force deployment even if checks fail")
	shipCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Simulate deployment without making changes")

	// Add ship command to root
	rootCmd.AddCommand(shipCmd)
}

// Ship is the main deployment function
func Ship(cmd *cobra.Command, args []string) {
	ShipLogs.Info("Starting deployment process...")

	if dryRun {
		ShipLogs.Warn("DRY RUN MODE: No changes will be made")
	}

	// Initialize server manager with config and SSH connections
	serverMgr, err := server.New(
		server.WithConfig(configFile),
		server.WithSSH(),
	)
	if err != nil {
		ShipLogs.Error("Failed to initialize server manager: %v", err)
		os.Exit(1)
	}
	defer func() {
		if err := serverMgr.CloseSSHConnections(); err != nil {
			ShipLogs.Error("Error closing connections: %v", err)
		}
	}()

	// Get list of target servers
	servers := serverMgr.ListServers()
	if len(servers) == 0 {
		ShipLogs.Error("No servers configured for deployment")
		os.Exit(1)
	}

	// Execute deployment steps
	if err := runDeployment(serverMgr, servers); err != nil {
		ShipLogs.Error("Deployment failed: %v", err)
		os.Exit(1)
	}

	ShipLogs.Success("Deployment completed successfully!")
}

// runDeployment orchestrates the deployment steps
func runDeployment(serverMgr *server.ServerStruct, servers []string) error {
	// Phase 1: Pre-deployment checks
	if err := verifyServers(serverMgr, servers); err != nil {
		return fmt.Errorf("pre-deployment checks failed: %w", err)
	}

	// Phase 2: File transfers
	if err := transferRequiredFiles(serverMgr, servers[0]); err != nil {
		return fmt.Errorf("file transfer failed: %w", err)
	}

	// Phase 3: Container deployment
	if !dryRun {
		if err := deployContainers(serverMgr, servers[0]); err != nil {
			return fmt.Errorf("container deployment failed: %w", err)
		}
	}

	// Phase 4: Post-deployment verification
	if err := verifyDeployment(serverMgr, servers[0]); err != nil {
		return fmt.Errorf("post-deployment verification failed: %w", err)
	}

	return nil
}

// verifyServers checks all target servers are reachable and meet requirements
func verifyServers(serverMgr *server.ServerStruct, servers []string) error {
	ShipLogs.Info("Verifying server connectivity and requirements...")

	var wg sync.WaitGroup
	errorChan := make(chan error, len(servers))

	for _, name := range servers {
		wg.Add(1)
		go func(serverName string) {
			defer wg.Done()

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			// Check basic connectivity
			if err := retryOperation(ctx, 3, 2*time.Second, func() error {
				return serverMgr.PingServer(serverName)
			}); err != nil {
				errorChan <- fmt.Errorf("server %s is unreachable: %w", serverName, err)
				return
			}

			// Check Docker availability
			if _, err := serverMgr.ExecuteCommand(ctx, serverName, "docker --version"); err != nil {
				errorChan <- fmt.Errorf("Docker not available on %s: %w", serverName, err)
				return
			}

			// Check disk space (example check)
			output, err := serverMgr.ExecuteCommand(ctx, serverName, "df -h /")
			if err != nil {
				errorChan <- fmt.Errorf("failed to check disk space on %s: %w", serverName, err)
				return
			}
			ShipLogs.Debug("[%s] Disk space:\n%s", serverName, output)
		}(name)
	}

	wg.Wait()
	close(errorChan)

	// Collect all errors
	var errors []error
	for err := range errorChan {
		errors = append(errors, err)
	}

	if len(errors) > 0 && !forceDeploy {
		return fmt.Errorf("server verification failed: %v", errors)
	} else if len(errors) > 0 {
		ShipLogs.Warn("Proceeding with deployment despite verification issues (force flag set)")
	}

	return nil
}

// transferRequiredFiles uploads all necessary files to the server
func transferRequiredFiles(serverMgr *server.ServerStruct, serverName string) error {
	ShipLogs.Info("Transferring required files to %s...", serverName)

	files := map[string]string{
		".env.production":    "/app/.env",
		"docker-compose.yml": "/app/docker-compose.yml",
		"scripts/start.sh":   "/app/start.sh",
	}

	var wg sync.WaitGroup
	errorChan := make(chan error, len(files))

	for local, remote := range files {
		wg.Add(1)
		go func(localPath, remotePath string) {
			defer wg.Done()

			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			if dryRun {
				ShipLogs.Info("[DRY RUN] Would upload %s to %s:%s", localPath, serverName, remotePath)
				return
			}

			if err := retryOperation(ctx, 3, 3*time.Second, func() error {
				return serverMgr.UploadFile(ctx, serverName, localPath, remotePath)
			}); err != nil {
				errorChan <- fmt.Errorf("failed to upload %s: %w", localPath, err)
				return
			}

			ShipLogs.Info("Successfully uploaded %s to %s", localPath, remotePath)
		}(local, remote)
	}

	wg.Wait()
	close(errorChan)

	// Check for errors
	for err := range errorChan {
		if !forceDeploy {
			return fmt.Errorf("file transfer failed: %w", err)
		}
		ShipLogs.Warn("File transfer error (proceeding anyway): %v", err)
	}

	return nil
}

// deployContainers handles the container deployment process
func deployContainers(serverMgr *server.ServerStruct, serverName string) error {
	ShipLogs.Info("Starting container deployment on %s...", serverName)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Step 1: Stop existing container if running
	if _, err := serverMgr.ExecuteCommand(ctx, serverName, "docker compose down"); err != nil {
		ShipLogs.Warn("Failed to stop existing containers (may not exist): %v", err)
	}

	// Step 2: Pull new image
	if err := retryOperation(ctx, 3, 10*time.Second, func() error {
		_, err := serverMgr.ExecuteCommand(ctx, serverName, "docker compose pull")
		return err
	}); err != nil {
		return fmt.Errorf("failed to pull Docker image: %w", err)
	}

	// Step 3: Start new containers
	if _, err := serverMgr.ExecuteCommand(ctx, serverName, "docker compose up -d"); err != nil {
		return fmt.Errorf("failed to start containers: %w", err)
	}

	// Step 4: Verify container is running
	output, err := serverMgr.ExecuteCommand(ctx, serverName, "docker ps --filter status=running --format '{{.Names}}'")
	if err != nil {
		return fmt.Errorf("failed to check running containers: %w", err)
	}

	ShipLogs.Info("Running containers:\n%s", output)
	return nil
}

// verifyDeployment checks if deployment was successful
func verifyDeployment(serverMgr *server.ServerStruct, serverName string) error {
	ShipLogs.Info("Verifying deployment on %s...", serverName)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Check container status
	output, err := serverMgr.ExecuteCommand(ctx, serverName, "docker ps --filter health=healthy --format '{{.Names}}'")
	if err != nil {
		return fmt.Errorf("failed to check container health: %w", err)
	}

	if output == "" {
		return fmt.Errorf("no healthy containers found")
	}

	ShipLogs.Info("Healthy containers:\n%s", output)

	// Optional: Check application health endpoint
	// This would depend on your application's health check endpoint
	if _, err := serverMgr.ExecuteCommand(ctx, serverName,
		"curl -sSf http://localhost:3000/health > /dev/null"); err != nil {
		return fmt.Errorf("application health check failed: %w", err)
	}

	return nil
}

// retryOperation retries a function with exponential backoff
func retryOperation(ctx context.Context, maxAttempts int, initialDelay time.Duration, fn func() error) error {
	var lastErr error

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		// Check if context is done before each attempt
		select {
		case <-ctx.Done():
			if lastErr != nil {
				return fmt.Errorf("context canceled after %d attempts, last error: %w", attempt-1, lastErr)
			}
			return ctx.Err()
		default:
		}

		err := fn()
		if err == nil {
			return nil
		}

		lastErr = err
		if attempt < maxAttempts {
			delay := time.Duration(attempt) * initialDelay
			ShipLogs.Debug("Attempt %d/%d failed, retrying in %v: %v",
				attempt, maxAttempts, delay, err)

			select {
			case <-time.After(delay):
				// Continue with next attempt
			case <-ctx.Done():
				return fmt.Errorf("context canceled while waiting for retry: %w", lastErr)
			}
		}
	}

	return fmt.Errorf("after %d attempts, last error: %w", maxAttempts, lastErr)
}
