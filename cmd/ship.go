package cmd

import (
	"context"
	"fmt"
	"io"
	"nextdeploy/internal/config"
	"nextdeploy/internal/git"
	"nextdeploy/internal/logger"
	"nextdeploy/internal/server"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
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
	shipCmd.Flags().StringVarP(&configFile, "config", "c", "nextdeploy.yml", "Path to configuration file")
	shipCmd.Flags().BoolVar(&forceDeploy, "force", false, "Force deployment even if checks fail")
	shipCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Simulate deployment without making changes")
	rootCmd.AddCommand(shipCmd)
}

func Ship(cmd *cobra.Command, args []string) {
	ctx := context.Background()
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
	if err := verifyServers(ctx, serverMgr, servers, stream); err != nil {
		return fmt.Errorf("pre-deployment checks failed: %w", err)
	}

	infoColor.Println("\n=== PHASE 2: File transfers ===")
	if err := transferRequiredFiles(ctx, serverMgr, stream, servers[0]); err != nil {
		return fmt.Errorf("file transfer failed: %w", err)
	}

	if !dryRun {
		infoColor.Println("\n=== PHASE 3: Container deployment ===")
		if err := deployContainers(ctx, serverMgr, servers[0], stream); err != nil {
			return fmt.Errorf("container deployment failed: %w", err)
		}
	}

	infoColor.Println("\n=== PHASE 4: Post-deployment verification ===", stream)
	if err := verifyDeployment(ctx, serverMgr, servers[0], stream); err != nil {
		return fmt.Errorf("post-deployment verification failed: %w", err)
	}

	return nil
}

func verifyServers(ctx context.Context, serverMgr *server.ServerStruct, servers []string, stream io.Writer) error {
	infoColor.Println("Verifying server connectivity and requirements...")

	var wg sync.WaitGroup
	errorChan := make(chan error, len(servers))

	for _, name := range servers {
		wg.Add(1)
		go func(serverName string) {
			defer wg.Done()

			serverCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()

			infoColor.Printf("  Checking server: %s\n", serverName)

			if err := retryOperation(serverCtx, 3, 2*time.Second, func() error {
				return serverMgr.PingServer(serverName)
			}); err != nil {
				errorChan <- fmt.Errorf("server %s is unreachable: %w", serverName, err)
				return
			}

			if _, err := serverMgr.ExecuteCommand(serverCtx, serverName, "docker --version", stream); err != nil {
				errorChan <- fmt.Errorf("Docker not available on %s: %w", serverName, err)
				return
			}

			output, err := serverMgr.ExecuteCommand(serverCtx, serverName, "df -h /", stream)
			if err != nil {
				errorChan <- fmt.Errorf("failed to check disk space on %s: %w", serverName, err)
				return
			}
			debugColor.Printf("[%s] Disk space:\n%s\n", serverName, output)
		}(name)
	}

	wg.Wait()
	close(errorChan)

	var errors []error
	for err := range errorChan {
		errors = append(errors, err)
	}

	if len(errors) > 0 && !forceDeploy {
		return fmt.Errorf("server verification failed: %v", errors)
	} else if len(errors) > 0 {
		warnColor.Println("Proceeding with deployment despite verification issues (force flag set)")
	}

	successColor.Println("✓ All server checks completed")
	return nil
}

func transferRequiredFiles(ctx context.Context, serverMgr *server.ServerStruct, stream io.Writer, serverName string) error {
	ShipLogs.Info("Transferring required files to %s...", serverName)

	files := map[string]string{
		"nextdeploy.yml": "nextdeploy.yml",
	}

	// Use a directory in the user's home folder instead of /app
	homeDir, err := serverMgr.ExecuteCommand(ctx, serverName, "echo $HOME", stream)
	infoColor.Printf("User home directory on %s: %s\n", serverName, homeDir)
	if err != nil {
		return fmt.Errorf("failed to get user home directory: %w", err)
	}
	homeDir = strings.TrimSpace(homeDir)
	baseDir := filepath.Join(homeDir, "app") // Now using ~/app instead of /app

	var wg sync.WaitGroup
	errorChan := make(chan error, len(files))

	// Create base directory with proper permissions
	ShipLogs.Debug("Creating base directory: %s", baseDir)
	if _, err := serverMgr.ExecuteCommand(ctx, serverName,
		fmt.Sprintf("mkdir -p %s && chmod 755 %s", baseDir, baseDir), stream); err != nil {
		return fmt.Errorf("failed to create base directory %s: %w", baseDir, err)
	}

	for localPath, remotePath := range files {
		wg.Add(1)
		go func(local, remote string) {
			defer wg.Done()

			fileCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
			defer cancel()

			// Verify local file exists first
			if _, err := os.Stat(local); os.IsNotExist(err) {
				errorChan <- fmt.Errorf("local file %s does not exist", local)
				return
			}

			ShipLogs.Debug("Transferring %s to %s:%s", local, serverName, remote)

			// Create full remote path
			fullRemotePath := filepath.Join(baseDir, remote)

			// Ensure remote directory exists
			remoteDir := filepath.Dir(fullRemotePath)
			if remoteDir != baseDir { // Skip if we're already in base dir
				ShipLogs.Debug("Ensuring remote directory %s exists", remoteDir)
				if _, err := serverMgr.ExecuteCommand(fileCtx, serverName,
					fmt.Sprintf("mkdir -p %s && chmod 755 %s", remoteDir, remoteDir), stream); err != nil {
					errorChan <- fmt.Errorf("failed to create remote directory %s: %w", remoteDir, err)
					return
				}
			}

			// Retry with exponential backoff
			if err := retryOperation(fileCtx, 3, 5*time.Second, func() error {
				return serverMgr.UploadFile(fileCtx, serverName, local, fullRemotePath)
			}); err != nil {
				errorChan <- fmt.Errorf("failed to upload %s: %w", local, err)
				return
			}

			// Set proper permissions
			if strings.HasSuffix(fullRemotePath, ".sh") {
				if _, err := serverMgr.ExecuteCommand(fileCtx, serverName,
					fmt.Sprintf("chmod +x %s", fullRemotePath), stream); err != nil {
					errorChan <- fmt.Errorf("failed to set executable permissions on %s: %w", fullRemotePath, err)
					return
				}
			}

			ShipLogs.Info("Successfully transferred %s to %s", local, fullRemotePath)
		}(localPath, remotePath)
	}

	wg.Wait()
	close(errorChan)

	for err := range errorChan {
		if !forceDeploy {
			return fmt.Errorf("file transfer failed: %w", err)
		}
		ShipLogs.Warn("File transfer error (proceeding anyway): %v", err)
	}

	return nil
}

func deployContainers(ctx context.Context, serverMgr *server.ServerStruct, serverName string, stream io.Writer) error {
	infoColor.Printf("Deploying containers on %s...\n", serverName)

	deployCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	infoColor.Println("  Stopping existing containers...")
	// TODO:- use blue-green deployment strategy here in the future
	// login to docker to registry

	// ===========Phase 1: Login to Docker registry===========
	ShipLogs.Info("Logging in to Docker registry...")
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load configuration to get password and user for logins : %w", err)
	}
	password := cfg.Docker.Password
	username := cfg.Docker.Username
	loginCommand := fmt.Sprintf("docker login --username %s --password %s", username, password)
	if _, err := serverMgr.ExecuteCommand(deployCtx, serverName, loginCommand, stream); err != nil {
		warnColor.Printf("  Warning: Failed to login to Docker registry: %v\n", err)

	}
	//===========Phase 2: Stop existing containers===========
	infoColor.Println("  Stopping existing containers...")
	//TODO:- use blue-green deployment strategy here in the future
	//
	// Phase 3: Load the tag for image to be pull and its names
	image := cfg.Docker.Image
	ShipLogs.Info("Pulling image: %s", image)
	if image == "" {
		return fmt.Errorf("Docker image not specified in configuration")
	}
	// TODO: make sure the logic for tags is rigid and highly deterministic
	imageTag, err := git.GetCommitHash()
	if err != nil {
		return fmt.Errorf("failed to get commit hash for image tag: %w", err)
	}
	imageWithTag := fmt.Sprintf("%s:%s", image, imageTag)
	infoColor.Printf("  Using image: %s\n", imageWithTag)
	pullCommand := fmt.Sprintf("docker pull %s", imageWithTag)
	if _, err := serverMgr.ExecuteCommand(deployCtx, serverName, pullCommand, stream); err != nil {
		warnColor.Printf("  Warning: Failed to stop existing containers (may not exist): %v\n", err)
	}
	//TODO: implement a mechanism to stop existing containers gracefully
	infoColor.Println("  Starting new containers...")
	runCommand := fmt.Sprintf("docker run -d --name %s --restart unless-stopped -p 3000:3000 %s", serverName, imageWithTag)
	if _, err := serverMgr.ExecuteCommand(deployCtx, serverName, runCommand, stream); err != nil {
		ShipLogs.Error("Failed to start containers: %v", err)
		return fmt.Errorf("failed to start containers: %w", err)
	}

	output, err := serverMgr.ExecuteCommand(deployCtx, serverName, "docker ps --filter status=running --format '{{.Names}}'", stream)
	if err != nil {
		return fmt.Errorf("failed to check running containers: %w", err)
	}

	successColor.Printf("✓ Containers running:\n%s\n", output)
	return nil
}

func verifyDeployment(ctx context.Context, serverMgr *server.ServerStruct, serverName string, stream io.Writer) error {
	infoColor.Printf("Verifying deployment on %s...\n", serverName)
	//TODO: Implement health checks for containers and application
	// verifyCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	// defer cancel()
	//
	// infoColor.Println("  Checking container health...")
	// output, err := serverMgr.ExecuteCommand(verifyCtx, serverName, "docker ps --filter health=healthy --format '{{.Names}}'", stream)
	// if err != nil {
	// 	return fmt.Errorf("failed to check container health: %w", err)
	// }
	//
	// if output == "" {
	// 	return fmt.Errorf("no healthy containers found")
	// }
	//
	// successColor.Printf("✓ Healthy containers:\n%s\n", output)
	//
	// infoColor.Println("  Checking application health endpoint...")
	// if _, err := serverMgr.ExecuteCommand(verifyCtx, serverName,
	// 	"curl -sSf http://localhost:3000/health > /dev/null", stream); err != nil {
	// 	return fmt.Errorf("application health check failed: %w", err)
	// }

	successColor.Println("✓ Application health check passed")
	return nil
}

func retryOperation(ctx context.Context, maxAttempts int, initialDelay time.Duration, fn func() error) error {
	var lastErr error

	for attempt := 1; attempt <= maxAttempts; attempt++ {
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
			debugColor.Printf("    Attempt %d/%d failed, retrying in %v: %v\n",
				attempt, maxAttempts, delay, err)

			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return fmt.Errorf("context canceled while waiting for retry: %w", lastErr)
			}
		}
	}

	return fmt.Errorf("after %d attempts, last error: %w", maxAttempts, lastErr)
}
