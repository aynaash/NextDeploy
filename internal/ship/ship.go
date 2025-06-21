package ship

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"nextdeploy/internal/config"
	"nextdeploy/internal/git"
	"nextdeploy/internal/logger"
	"nextdeploy/internal/server"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fatih/color"
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

func VerifyServers(ctx context.Context, serverMgr *server.ServerStruct, servers []string, stream io.Writer) error {
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

func TransferRequiredFiles(ctx context.Context, serverMgr *server.ServerStruct, stream io.Writer, serverName string) error {
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

func DeployContainers(ctx context.Context, serverMgr *server.ServerStruct, serverName string, stream io.Writer) error {
	infoColor.Printf("Deploying containers on %s...\n", serverName)

	deployCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	// ===========Phase 1: Login to Docker registry===========
	ShipLogs.Info("Logging in to Docker registry...")
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load configuration to get password and user for logins : %w", err)
	}
	if cfg.Docker.Username == "" || cfg.Docker.Password == "" {
		return fmt.Errorf("Docker configuration is missing username or password")
	}
	loginCommand := fmt.Sprintf("docker login --username %s --password %s", cfg.Docker.Username, cfg.Docker.Password)
	if _, err := serverMgr.ExecuteCommand(deployCtx, serverName, loginCommand, stream); err != nil {
		warnColor.Printf("  Warning: Failed to login to Docker registry: %v\n", err)

	}
	//===== Determin curre deployment green =========
	currentColor := ""
	output, err := serverMgr.ExecuteCommand(deployCtx, serverName, "docker ps --format '{{.Names}}'", stream)
	if err != nil {
		return fmt.Errorf("failed to list running containers: %w", err)
	} else {
		if strings.Contains(output, "-blue") {
			currentColor = "blue"
		}
		if strings.Contains(output, "-green") {
			currentColor = "green"
		}
	}
	// ======= Prepare new deployment
	newColor := "blue" // default color for new deployment
	if currentColor == "blue" {
		newColor = "green" // switch to green if current is bluePort
	}
	newContainerName := fmt.Sprintf("%s-%s", serverName, newColor)
	newContainerPort := bluePort // default port for new deployment
	if newColor == "green" {
		newContainerPort = greenPort // switch to green port if new deployment is green
	}
	image := cfg.Docker.Image
	ShipLogs.Info("Pulling image: %s", image)
	tag, err := git.GetCommitHash()
	if err != nil {
		ShipLogs.Debug("Error getting tag for image")
	}
	if tag != "" {
		image = fmt.Sprintf("%s:%s", image, tag)
		infoColor.Printf("  Using image tag: %s\n", image)
	} else {
		infoColor.Printf("  Using image without tag: %s\n", image)
	}
	if image == "" && cfg.Docker.Image != "" {
		image = cfg.Docker.Image
		infoColor.Printf("  Using image from configuration: %s\n", image)
	} else if image == "" {
		return fmt.Errorf("docker image not specified in configuration or command line")
	} else {
		infoColor.Printf("  Using image: %s\n", image)
	}
	fullImageName := fmt.Sprintf("%s:%s", image, tag)

	ShipLogs.Debug("Full image name: %s", fullImageName)

	pullCommand := fmt.Sprintf("docker pull %s", image)
	if _, err := serverMgr.ExecuteCommand(deployCtx, serverName, pullCommand, stream); err != nil {
		return fmt.Errorf("failed to pull Docker image %s: %w", image, err)
	}
	// Check if image is specified
	if image == "" {
		return fmt.Errorf("docker image not specified in configuration")
	}
	//===========Phase 4: Deploy new containers ==============
	infoColor.Printf("  Deploying new %s container: %s on port %s\n", newColor, newContainerName, newContainerPort)

	if err != nil {
		return fmt.Errorf("failed to get commit hash for image tag: %w", err)
	}
	var envVars []string
	for key, value := range cfg.Environment {
		envVars = append(envVars, fmt.Sprintf("-e %s=%s", key, value))
	}
	if len(envVars) > 0 {
		infoColor.Printf("  Setting environment variables: %s\n", strings.Join(envVars, ", "))
	}

	if newColor == "green" {
		newContainerPort = "3002" // green
	}
	infoColor.Printf("  Deploying new container: %s on port %s\n", newContainerName, newContainerPort)
	runCommand := fmt.Sprintf("docker run -d --name %s --restart unless-stopped -p %s:3000 %s", newContainerName, newContainerPort, image)

	if _, err := serverMgr.ExecuteCommand(deployCtx, serverName, runCommand, stream); err != nil {
		ShipLogs.Error("Failed to start containers: %v", err)
		return fmt.Errorf("failed to start new %s container %w", newColor, err)
	}

	//===========Phase 5: Check contaier heath =============
	if err := VerifyContainerHealth(serverMgr, serverName, newContainerPort, stream); err != nil {
		stopCommand := fmt.Sprintf("docker stop %s && docker rm %s", newContainerName, newContainerName)
		serverMgr.ExecuteCommand(deployCtx, serverName, stopCommand, stream)
		return fmt.Errorf("container health check failed: %w", err)
	}
	out, err := serverMgr.ExecuteCommand(deployCtx, serverName, "docker ps --filter status=running --format '{{.Names}}'", stream)
	ShipLogs.Debug("Running containers:\n%s", out)
	if err != nil {
		return fmt.Errorf("failed to check running containers: %w", err)
	}

	//==============Phase 6: Update caddy via api ==============
	if err := UpdateCaddyConfig(ctx, serverMgr, serverName, newContainerName, newContainerPort, stream); err != nil {
		stopCommand := fmt.Sprintf("docker stop %s && docker rm %s", newContainerName, newContainerName)
		serverMgr.ExecuteCommand(deployCtx, serverName, stopCommand, stream)
		return fmt.Errorf("failed to update Caddy configuration: %w", err)
	}

	// ===========Phase 7: clean up old containers ==============
	if currentColor != "" {
		oldContainerName := fmt.Sprintf("%s-%s", serverName, currentColor)
		infoColor.Printf("  Cleaning up old container: %s\n", oldContainerName)
		cleanupCommand := fmt.Sprintf("docker stop %s && docker rm %s", oldContainerName, oldContainerName)
		if _, err := serverMgr.ExecuteCommand(deployCtx, serverName, cleanupCommand, stream); err != nil {
			warnColor.Printf("  Warning: Failed to clean up old container %s: %v\n", oldContainerName, err)
		}
	}

	successColor.Printf("✓ Containers running:\n%s\n", output)
	return nil
}
func VerifyContainerHealth(serverMgr *server.ServerStruct, serverName, port string, stream io.Writer) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// healthCheckURL := fmt.Sprintf("http://localhost:%s/health", port)
	// healthCheckCommand := fmt.Sprintf(
	// 	`for i in $(seq 1 30); do
	//            status=$(curl -s -o /dev/null -w '%%{http_code}' %s || echo "000")
	//            if [ "$status" -eq 200 ]; then
	//                echo "Health check passed"
	//                exit 0
	//            fi
	//            sleep 2
	//         done
	//         echo "Health check failed after 30 attempts"
	//         exit 1`, healthCheckURL)
	healthCheckCommand := "docker ps -a"

	_, err := serverMgr.ExecuteCommand(ctx, serverName, healthCheckCommand, stream)
	return err
}

// Improved updateCaddyConfig with better error handling
func UpdateCaddyConfig(ctx context.Context, serverMgr *server.ServerStruct, serverName, domain, port string, stream io.Writer) error {
	const maxRetries = 3
	const retryDelay = 500 * time.Millisecond

	ShipLogs.Info("Starting Caddy config update",
		"domain", domain,
		"port", port,
		"server", serverName)

	// 1. Verify Caddy is running
	ShipLogs.Debug("Checking Caddy availability")
	if _, err := serverMgr.ExecuteCommand(ctx, serverName, "caddy -v", stream); err != nil {
		ShipLogs.Error("Caddy not available", "error", err)
		return fmt.Errorf("caddy container not found: %w", err)
	}

	// 2. Get current config to modify
	getConfigCmd := `curl -sS "http://localhost:2019/config/apps/http/servers/srv0"`
	ShipLogs.Debug("Fetching current Caddy config")
	currentConfig, err := serverMgr.ExecuteCommand(ctx, serverName, getConfigCmd, stream)
	if err != nil {
		ShipLogs.Error("Failed to get current config", "error", err)
		return fmt.Errorf("failed to get current config: %w", err)
	}

	// 3. Parse and modify config
	var config struct {
		Routes []map[string]interface{} `json:"routes"`
	}
	if err := json.Unmarshal([]byte(currentConfig), &config); err != nil {
		ShipLogs.Error("Failed to parse config", "error", err)
		return fmt.Errorf("failed to parse config: %w", err)
	}

	// Remove existing route if it exists
	ShipLogs.Debug("Removing existing route if present")
	filteredRoutes := make([]map[string]interface{}, 0)
	for _, route := range config.Routes {
		if route["match"] != nil {
			if matches, ok := route["match"].([]interface{}); ok {
				for _, match := range matches {
					if hostMatch, ok := match.(map[string]interface{}); ok {
						if hosts, ok := hostMatch["host"].([]interface{}); ok {
							for _, h := range hosts {
								if h == domain {
									continue // Skip this route
								}
							}
						}
					}
				}
			}
		}
		filteredRoutes = append(filteredRoutes, route)
	}

	// Add new route
	newRoute := map[string]interface{}{
		"match": []map[string]interface{}{
			{
				"host": []string{domain},
			},
		},
		"handle": []map[string]interface{}{
			{
				"handler": "reverse_proxy",
				"upstreams": []map[string]interface{}{
					{
						"dial": fmt.Sprintf("localhost:%s", port),
					},
				},
			},
		},
	}
	config.Routes = append(filteredRoutes, newRoute)

	updatedConfig, err := json.Marshal(config)
	if err != nil {
		ShipLogs.Error("Failed to marshal updated config", "error", err)
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// 4. Apply updated config
	updateCmd := fmt.Sprintf(
		`curl -sS -X POST "http://localhost:2019/load" \
        -H "Content-Type: application/json" \
        -d '%s'`,
		strings.ReplaceAll(string(updatedConfig), "'", "'\\''"))

	ShipLogs.Debug("Applying updated config", "config", string(updatedConfig))
	if _, err := serverMgr.ExecuteCommand(ctx, serverName, updateCmd, stream); err != nil {
		ShipLogs.Error("Failed to update config", "error", err)
		return fmt.Errorf("failed to update config: %w", err)
	}

	// 5. Verify update
	verifyCmd := `curl -sS "http://localhost:2019/config/apps/http/servers/srv0"`
	for i := 0; i < maxRetries; i++ {
		ShipLogs.Debug("Verifying config update", "attempt", i+1)
		verifyOutput, err := serverMgr.ExecuteCommand(ctx, serverName, verifyCmd, stream)
		if err == nil && strings.Contains(verifyOutput, domain) {
			ShipLogs.Info("Caddy config updated successfully")
			return nil
		}
		time.Sleep(retryDelay)
	}

	ShipLogs.Error("Failed to verify config update")
	return fmt.Errorf("failed to verify config update after %d attempts", maxRetries)
}
func RollbackDeployment(serverMgr *server.ServerStruct, serverName, domain, previousPort string, stream io.Writer) error {
	// Get current active port
	getConfig := `curl -s "http://localhost:2019/config/apps/http/servers/srv0/routes/` + domain + `"`
	// printout the config
	ShipLogs.Debug("Current Caddy config for %s: %s", domain, getConfig)

	// Revert to previous port
	configJSON := fmt.Sprintf(`{
        "@id": "%s",
        "handle": [{
            "handler": "reverse_proxy",
            "upstreams": [{"dial": "localhost:%s"}]
        }]
    }`, domain, previousPort)

	updateCommand := fmt.Sprintf(
		`curl -X POST "http://localhost:2019/config/apps/http/servers/srv0/routes" \
        -H "Content-Type: application/json" \
        -d '%s'`, configJSON)

	_, err := serverMgr.ExecuteCommand(context.Background(), serverName, updateCommand, stream)
	return err
}

func VerifyDeployment(ctx context.Context, serverMgr *server.ServerStruct, serverName string, stream io.Writer) error {
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
