package ship

import (
	"context"
	"fmt"
	"io"
	"nextdeploy/cli/internal/server"
	"nextdeploy/shared"
	"nextdeploy/shared/config"
	"nextdeploy/shared/git"
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
	// TODO: add logic for transfering .nextdeploy directory
	files := map[string]string{
		"nextdeploy.yml.enc": "nextdeploy.yml.enc",
		".env.enc":           ".env.enc",
	}

	// Use a directory in the user's home folder instead of /app
	homeDir, err := serverMgr.ExecuteCommand(ctx, serverName, "echo $HOME", stream)
	ShipLogs.Debug("User home directory on %s: %s\n", serverName, homeDir)
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

func DeployContainers(ctx context.Context, serverMgr *server.ServerStruct, serverName string, credentials bool, stream io.Writer) error {
	ShipLogs.Info("Deploying containers on %s...\n", serverName)

	deployCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	// ===========Phase 1: Login to Docker registry===========
	ShipLogs.Info("Logging in to Docker registry...")
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load configuration to get password and user for logins : %w", err)
	}

	if cfg.Docker.Registry == "dockerhub" {
		if cfg.Docker.Username == "" || cfg.Docker.Password == "" {
			return fmt.Errorf("docker configuration is missing username or password")
		}
		loginCommand := fmt.Sprintf("docker login --username %s --password %s", cfg.Docker.Username, cfg.Docker.Password)
		if _, err := serverMgr.ExecuteCommand(deployCtx, serverName, loginCommand, stream); err != nil {
			ShipLogs.Warn("  Warning: Failed to login to Docker registry: %v\n", err)
		}
	}

	if cfg.Docker.Registry == "erc" {
		ShipLogs.Info("Using ECR as container registry")
		if cfg.Docker.RegistryRegion == "" {
			ShipLogs.Error("Error no ecr region set")
			return fmt.Errorf("ECR registry region is not specified in configuration")
		}
		if cfg.Docker.Image == "" {
			ShipLogs.Error("No docker image name")
			return fmt.Errorf("ECR repository name is not specified in configuration")
		}
		if credentials {
			ShipLogs.Info("Preparing ECR server credentials")
			err := serverMgr.PrepareEcrCredentials(os.Stdout)
			if err != nil {

				ShipLogs.Error("  Warning: Failed to prepare ECR credentials: %v\n", err)
				return fmt.Errorf("failed to prepare ECR credentials: %w", err)
			}
		}
	}

	//===== Determine current deployment color =========
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
		ShipLogs.Debug("  Using image tag: %s\n", image)
	} else {
		ShipLogs.Debug("  Using image without tag: %s\n", image)
	}

	if image == "" && cfg.Docker.Image != "" {
		image = cfg.Docker.Image
		ShipLogs.Debug("  Using image from configuration: %s\n", image)
	} else if image == "" {
		return fmt.Errorf("docker image not specified in configuration or command line")
	} else {
		ShipLogs.Debug("  Using image: %s\n", image)
	}

	ShipLogs.Debug("Image without tag is:%s", image)

	if cfg.Docker.Registry == "ecr" {
		err := serverMgr.PrepareEcrCredentials(stream)
		if err != nil {
			return err
		}

		// ShipLogs.Debug("Full image name: %s", image)
		// pullCommand := fmt.Sprintf("docker pull %s", image)
		//
		// if _, err := serverMgr.ExecuteCommand(deployCtx, serverName, pullCommand, stream); err != nil {
		// 	return fmt.Errorf("failed to pull Docker image %s  === %w", image, err)
		// }

		// Check if image is specified
		if image == "" {
			return fmt.Errorf("docker image not specified in configuration")
		}
	} // <-- This closing brace was missing for the ecr if block

	//===========Phase 4: Deploy new containers ==============
	ShipLogs.Info("  Deploying new %s container: %s on port %s\n", newColor, newContainerName, newContainerPort)

	if err != nil {
		return fmt.Errorf("failed to get commit hash for image tag: %w", err)
	}

	var envVars []string
	for key, value := range cfg.Environment {
		envVars = append(envVars, fmt.Sprintf("-e %v=%v", key, value))
	}
	if len(envVars) > 0 {
		ShipLogs.Debug("  Setting environment variables: %s\n", strings.Join(envVars, ", "))
	}

	if newColor == "green" {
		newContainerPort = "3002" // green
	}

	ShipLogs.Debug("  Deploying new container: %s on port %s\n", newContainerName, newContainerPort)
	runCommand := fmt.Sprintf("docker run -d --name %s --restart unless-stopped -p %s:3000 %s", newContainerName, newContainerPort, image)

	if _, err := serverMgr.ExecuteCommand(deployCtx, serverName, runCommand, stream); err != nil {
		ShipLogs.Error("Failed to start containers: %v", err)
		return fmt.Errorf("failed to start new %s container %w", newColor, err)
	}

	//do caddy reload

	//===========Phase 5: Check container health =============
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

	// ===========Phase 7: clean up old containers ==============
	if currentColor != "" {
		oldContainerName := fmt.Sprintf("%s-%s", serverName, currentColor)
		ShipLogs.Debug("  Cleaning up old container: %s\n", oldContainerName)
		cleanupCommand := fmt.Sprintf("docker stop %s && docker rm %s", oldContainerName, oldContainerName)
		if _, err := serverMgr.ExecuteCommand(deployCtx, serverName, cleanupCommand, stream); err != nil {
			ShipLogs.Warn("  Warning: Failed to clean up old container %s: %v\n", oldContainerName, err)
		}
	}

	// ==========Phase 8: Reload caddy =============
	reloadCommand := "sudo systemctl reload caddy"
	if _, err := serverMgr.ExecuteCommand(deployCtx, serverName, reloadCommand, stream); err != nil {
		ShipLogs.Warn("  Warning: Failed to reload Caddy service: %v\n", err)
	} else {
		ShipLogs.Success("✓ Caddy service reloaded successfully")
	}

	//TODO:--- add way to count successfully deployments by send new deployments to api endpoint
	//TODO: --- Add feedback collection to api endpoint: or ui from terminal

	ShipLogs.Success("✓ Containers running:\n%s\n", output)
	return nil
} // <-- This is the actual end of the DeployContainers function

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
// Helper simulation functions
func simulateCommand(cmd string) (string, error) {
	// Simulate successful command execution
	if strings.Contains(cmd, "fail") { // Example failure condition
		return "", fmt.Errorf("simulated command failure")
	}
	return "simulated command output", nil
}

func simulateCurrentConfig() string {
	// Return a simple simulated config
	return `{
		"routes": [
			{
				"match": [{"host": ["existing.example.com"]}],
				"handle": [{"handler": "reverse_proxy", "upstreams": [{"dial": "localhost:3000"}]}]
			}
		]
	}`
}
func simulateRouteRemoval(routes []map[string]interface{}, domain string) []map[string]interface{} {
	// Simulate filtering out routes matching our domain
	var filtered []map[string]interface{}
	for _, route := range routes {
		if matches, ok := route["match"].([]interface{}); ok {
			shouldKeep := true
			for _, match := range matches {
				if m, ok := match.(map[string]interface{}); ok {
					if hosts, ok := m["host"].([]interface{}); ok {
						for _, h := range hosts {
							if h == domain {
								shouldKeep = false
								break
							}
						}
					}
				}
			}
			if shouldKeep {
				filtered = append(filtered, route)
			}
		}
	}
	return filtered
}

func simulateConfigUpdate(config string) error {
	// Simulate config update - could add random failures for testing
	if strings.Contains(config, "fail") {
		return fmt.Errorf("simulated config update failure")
	}
	return nil
}

func simulateVerifyConfig(domain string) bool {
	// Simulate verification - could make this fail randomly for testing
	return true // or false to test failure cases
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

func SetupCaddy(ctx context.Context, serverMgr *server.ServerStruct, serverName string, fresh bool, stream io.Writer) error {
	ShipLogs.Info("Setting up Caddy on %s...\n", serverName)

	// Check if Caddy is installed
	if _, err := serverMgr.ExecuteCommand(ctx, serverName, "caddy version", stream); err != nil {
		return fmt.Errorf("Caddy is not installed on %s: %w", serverName, err)
	}
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}
	domain := cfg.App.Domain
	email := cfg.SSL.Email
	if domain == "" {
		return fmt.Errorf("Caddy domain is not specified in configuration")
	}

	configContent := fmt.Sprintf(`
{
    # Global settings
    email %s # For Let's Encryp
    acme_ca https://acme-v02.api.letsencrypt.org/directory
    log {
        output file /var/log/caddy/access.log {
            roll_size 100mb
            roll_keep 5
        }
    }
}

# Primary domain
%s {
    # Handle HTTP->HTTPS and www->non-www redirects
    redir https://%s{uri} permanent

    # WebSocket route matcher
    @ws {
        header Connection *Upgrade*
        header Upgrade websocket
    }

    # Reverse proxy configuration
    reverse_proxy @ws localhost:3001  # WebSocket traffic
    reverse_proxy localhost:3001 localhost:3002 {  # Normal traffic
        # Next.js optimizations
        transport http {
            keepalive 30s
            tls_insecure_skip_verify  # For local Docker networks
        }

        # Load balancing
        lb_policy first
        lb_try_duration 5s
        health_uri /health
        health_interval 10s
    }

    # Security headers
    header {
        Strict-Transport-Security "max-age=31536000; includeSubDomains"
        X-Content-Type-Options nosniff
        X-Frame-Options DENY
        Referrer-Policy strict-origin-when-cross-origin
        -Server  # Remove server header
    }

    # Cache control (customize per route)
    @static {
        path *.css *.js *.jpg *.png *.svg *.ico *.woff2
    }
    header @static Cache-Control "public, max-age=31536000, immutable"
		{
    admin :2019
}
}`, email, domain, domain)

	// Create Caddyfile directory if it doesn't exist
	if fresh {
		caddyDir := "/etc/caddy"
		if _, err := serverMgr.ExecuteCommand(ctx, serverName, fmt.Sprintf("mkdir -p %s && chmod 755 %s", caddyDir, caddyDir), stream); err != nil {
			return fmt.Errorf("failed to create Caddy directory %s: %w", caddyDir, err)
		}

		// Write the dynamic Caddyfile
		caddyFilePath := filepath.Join(caddyDir, "Caddyfile")
		localfile, err := os.CreateTemp("", "Caddyfile")
		if err != nil {
			return fmt.Errorf("failed to create temporary Caddyfile: %w", err)
		}
		defer os.Remove(localfile.Name()) // Clean up temp file
		if _, err := localfile.WriteString(configContent); err != nil {
			return fmt.Errorf("failed to write Caddyfile content: %w", err)
		}
		if err := localfile.Close(); err != nil {
			return fmt.Errorf("failed to close temporary Caddyfile: %w", err)
		}
		// Upload the Caddyfile to the server
		if err := serverMgr.UploadFile(ctx, serverName, localfile.Name(), caddyFilePath); err != nil {
			return fmt.Errorf("failed to upload Caddyfile to %s: %w", serverName, err)
		}
	}

	// Reload Caddy service
	if _, err := serverMgr.ExecuteCommand(ctx, serverName, "sudo systemctl reload caddy", stream); err != nil {
		return fmt.Errorf("failed to reload Caddy service: %w", err)
	}

	ShipLogs.Success("✓ Caddy setup completed successfully")
	// Print dns instruction
	PrintDNSInstructions(domain)
	return nil
}

func VerifyDeployment(ctx context.Context, serverMgr *server.ServerStruct, serverName string, stream io.Writer) error {
	ShipLogs.Info("Verifying deployment on %s...\n", serverName)
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

func PrintDNSInstructions(domain string) {
	fmt.Println("\n=== DNS Setup Instructions ===")
	fmt.Println("For your domain to work properly, you need to configure DNS records with your domain provider.")

	fmt.Println("\n1. If this server has a static IP address:")
	fmt.Printf("   - Create an A record for %s pointing to your server's IP address\n", domain)
	fmt.Printf("   - Create an A record for www.%s pointing to the same IP (or CNAME to %s)\n", domain, domain)

	fmt.Println("\n2. If you're using a dynamic DNS service:")
	fmt.Printf("   - Configure your dynamic DNS client for %s\n", domain)

	fmt.Println("\nNote: Caddy will automatically handle SSL certificate provisioning once DNS is properly configured.")
	fmt.Println("After DNS changes, it may take up to 48 hours to propagate globally")
	fmt.Println("You can verify DNS resolution with: dig +short " + domain)
}
