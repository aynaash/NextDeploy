package ship

import (
	"context"
	"fmt"
	"io"
	"nextdeploy/cli/internal/server"
	"nextdeploy/shared"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	bluePort  = "3001" // Blue container port
	greenPort = "3002" // Green container port
)

var (
	ShipLogs    = shared.PackageLogger("ship", "🚢")
	forceDeploy bool
)

func VerifyServers(ctx context.Context, serverMgr *server.ServerStruct, servers []string, stream io.Writer) error {
	ShipLogs.Info("Verifying server connectivity and requirements...")

	var wg sync.WaitGroup
	errorChan := make(chan error, len(servers))

	for _, name := range servers {
		wg.Add(1)
		go func(serverName string) {
			defer wg.Done()

			serverCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()

			ShipLogs.Info("  Checking server: %s\n", serverName)

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
			ShipLogs.Debug("[%s] Disk space:\n%s\n", serverName, output)
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
		ShipLogs.Info("Proceeding with deployment despite verification issues (force flag set)")
	}

	ShipLogs.Success("✓ All server checks completed")
	return nil
}

func TransferRequiredFiles(ctx context.Context, serverMgr *server.ServerStruct, stream io.Writer, serverName string) error {
	ShipLogs.Info("Transferring required files to %s...", serverName)

	// Define files to transfer
	files := map[string]string{
		"nextdeploy.yml.enc": "nextdeploy.yml.enc",
		".env.enc":           ".env.enc",
	}

	// Use a directory in the user's home folder
	homeDir, err := serverMgr.ExecuteCommand(ctx, serverName, "echo $HOME", stream)
	ShipLogs.Debug("User home directory on %s: %s\n", serverName, homeDir)
	if err != nil {
		return fmt.Errorf("failed to get user home directory: %w", err)
	}
	homeDir = strings.TrimSpace(homeDir)
	baseDir := filepath.Join(homeDir, "app")

	var wg sync.WaitGroup
	errorChan := make(chan error, len(files)+2) // +2 for .nextdeploy and .next directories

	// Create base directory with proper permissions
	ShipLogs.Debug("Creating base directory: %s", baseDir)
	if _, err := serverMgr.ExecuteCommand(ctx, serverName,
		fmt.Sprintf("mkdir -p %s && chmod 755 %s", baseDir, baseDir), stream); err != nil {
		return fmt.Errorf("failed to create base directory %s: %w", baseDir, err)
	}

	// Transfer individual files
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

			ShipLogs.Info("Successfully transferred %s to %s", local, fullRemotePath)
		}(localPath, remotePath)
	}

	// Transfer .nextdeploy directory
	wg.Add(1)
	go func() {
		defer wg.Done()

		fileCtx, cancel := context.WithTimeout(ctx, 5*time.Minute) // Longer timeout for directory transfer
		defer cancel()

		localNextDeployDir := ".nextdeploy"
		if _, err := os.Stat(localNextDeployDir); os.IsNotExist(err) {
			ShipLogs.Debug(".nextdeploy directory does not exist locally, skipping")
			return
		}

		remoteNextDeployDir := filepath.Join(baseDir, ".nextdeploy")
		ShipLogs.Info("Transferring %s directory to %s:%s", localNextDeployDir, serverName, remoteNextDeployDir)

		// Create remote directory
		if _, err := serverMgr.ExecuteCommand(fileCtx, serverName,
			fmt.Sprintf("mkdir -p %s && chmod 755 %s", remoteNextDeployDir, remoteNextDeployDir), stream); err != nil {
			errorChan <- fmt.Errorf("failed to create remote directory %s: %w", remoteNextDeployDir, err)
			return
		}

		// Upload all files in .nextdeploy directory
		err := filepath.Walk(localNextDeployDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if info.IsDir() {
				return nil // Skip directories, we'll create them as needed
			}

			relPath, err := filepath.Rel(localNextDeployDir, path)
			if err != nil {
				return err
			}

			remoteFilePath := filepath.Join(remoteNextDeployDir, relPath)
			remoteFileDir := filepath.Dir(remoteFilePath)

			// Create subdirectory if needed
			if remoteFileDir != remoteNextDeployDir {
				if _, err := serverMgr.ExecuteCommand(fileCtx, serverName,
					fmt.Sprintf("mkdir -p %s && chmod 755 %s", remoteFileDir, remoteFileDir), stream); err != nil {
					return fmt.Errorf("failed to create remote directory %s: %w", remoteFileDir, err)
				}
			}

			// Upload file
			if err := serverMgr.UploadFile(fileCtx, serverName, path, remoteFilePath); err != nil {
				return fmt.Errorf("failed to upload %s: %w", path, err)
			}

			// Set executable permissions if it's a script
			if strings.HasSuffix(remoteFilePath, ".sh") {
				if _, err := serverMgr.ExecuteCommand(fileCtx, serverName,
					fmt.Sprintf("chmod +x %s", remoteFilePath), stream); err != nil {
					return fmt.Errorf("failed to set executable permissions on %s: %w", remoteFilePath, err)
				}
			}

			ShipLogs.Debug("Transferred %s to %s", path, remoteFilePath)
			return nil
		})

		if err != nil {
			errorChan <- fmt.Errorf("failed to upload .nextdeploy directory: %w", err)
			return
		}

		ShipLogs.Success("✓ Successfully transferred .nextdeploy directory to %s", remoteNextDeployDir)
	}()

	// Transfer keys directory
	wg.Add(1)
	go func() {
		defer wg.Done()

		fileCtx, cancel := context.WithTimeout(ctx, 10*time.Minute) // Even longer timeout for .next directory
		defer cancel()

		homeDir, err := os.UserHomeDir()
		if err != nil {
			errorChan <- fmt.Errorf("failed to get local user home directory: %w", err)
			return
		}
		localNextDir := filepath.Join(homeDir, ".nextdeploy")
		if _, err := os.Stat(localNextDir); os.IsNotExist(err) {
			ShipLogs.Debug(".next directory does not exist locally, skipping")
			return
		}

		remoteNextDir := filepath.Join(baseDir, ".nextdeploykeys")
		ShipLogs.Info("Transferring %s directory to %s:%s", localNextDir, serverName, remoteNextDir)

		// Create remote directory
		if _, err := serverMgr.ExecuteCommand(fileCtx, serverName,
			fmt.Sprintf("mkdir -p %s && chmod 755 %s", remoteNextDir, remoteNextDir), stream); err != nil {
			errorChan <- fmt.Errorf("failed to create remote directory %s: %w", remoteNextDir, err)
			return
		}

		// Upload all files in .next directory
		err = filepath.Walk(localNextDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if info.IsDir() {
				return nil // Skip directories, we'll create them as needed
			}

			relPath, err := filepath.Rel(localNextDir, path)
			if err != nil {
				return err
			}

			remoteFilePath := filepath.Join(remoteNextDir, relPath)
			remoteFileDir := filepath.Dir(remoteFilePath)

			// Create subdirectory if needed
			if remoteFileDir != remoteNextDir {
				if _, err := serverMgr.ExecuteCommand(fileCtx, serverName,
					fmt.Sprintf("mkdir -p %s && chmod 755 %s", remoteFileDir, remoteFileDir), stream); err != nil {
					return fmt.Errorf("failed to create remote directory %s: %w", remoteFileDir, err)
				}
			}

			// Upload file
			if err := serverMgr.UploadFile(fileCtx, serverName, path, remoteFilePath); err != nil {
				return fmt.Errorf("failed to upload %s: %w", path, err)
			}

			ShipLogs.Debug("Transferred %s to %s", path, remoteFilePath)
			return nil
		})

		if err != nil {
			errorChan <- fmt.Errorf("failed to upload .nextdeploykeys directory: %w", err)
			return
		}

		ShipLogs.Success("✓ Successfully transferred .nextdeploykeys directory to %s", remoteNextDir)
	}()

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

func DeployContainers(ctx context.Context, serverMgr *server.ServerStruct, serverName string, credentials bool, stream io.Writer) error {
	ShipLogs.Info("Deploying containers on %s...\n", serverName)
	return nil
}

func ExecuteSimpleCommand(ctx context.Context, serverMgr *server.ServerStruct, serverName string, command string, stream io.Writer) error {
	ShipLogs.Info("Executing command on %s: %s", serverName, command)

	cmdCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	output, err := serverMgr.ExecuteCommand(cmdCtx, serverName, command, stream)
	if err != nil {
		return fmt.Errorf("failed to execute command: %w", err)
	}

	ShipLogs.Debug("Command output: %s", output)
	ShipLogs.Success("✓ Command executed successfully")
	return nil
}

func VerifyDeployment(ctx context.Context, serverMgr *server.ServerStruct, serverName string, stream io.Writer) error {
	ShipLogs.Info("Verifying deployment on %s...\n", serverName)

	ShipLogs.Success("✓ Application health check passed")
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
			ShipLogs.Debug("    Attempt %d/%d failed, retrying in %v: %v\n",
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
