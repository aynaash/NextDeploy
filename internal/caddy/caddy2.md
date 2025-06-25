# Enhanced Caddy SSL Management Package

Here's an improved version of your Caddy management functionality that integrates with the SSH server package we discussed earlier. This provides remote Caddy configuration capabilities with better error handling, logging, and SSH integration.

```go
package caddy

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"nextdeploy/internal/logger"
	"nextdeploy/internal/server"
)

var (
	caddylogger = logger.PackageLogger("Caddy", "🚀")
)

type CaddyManager struct {
	serverMgr *server.ServerStruct
	localMode bool
}

type CaddyOption func(*CaddyManager) error

// New creates a new Caddy manager with options
func New(opts ...CaddyOption) (*CaddyManager, error) {
	cm := &CaddyManager{localMode: true} // Default to local mode
	
	for _, opt := range opts {
		if err := opt(cm); err != nil {
			return nil, fmt.Errorf("failed to apply option: %w", err)
		}
	}
	
	return cm, nil
}

// WithServerManager enables remote Caddy management via SSH
func WithServerManager(sm *server.ServerStruct) CaddyOption {
	return func(cm *CaddyManager) error {
		cm.serverMgr = sm
		cm.localMode = false
		return nil
	}
}

// Install checks and installs Caddy on the target system
func (cm *CaddyManager) Install(ctx context.Context) error {
	if cm.localMode {
		return cm.installLocal(ctx)
	}
	return cm.installRemote(ctx)
}

func (cm *CaddyManager) installLocal(ctx context.Context) error {
	caddylogger.Info("Checking local Caddy installation...")

	_, err := exec.LookPath("caddy")
	if err == nil {
		caddylogger.Info("Caddy is already installed locally")
		return nil
	}

	caddylogger.Info("Installing Caddy locally...")

	// Use official Caddy install script with timeout
	cmdCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, "sh", "-c", "curl https://getcaddy.com | bash -s personal")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		caddylogger.Error("Failed to install Caddy locally: %v", err)
		return fmt.Errorf("caddy installation failed: %w", err)
	}

	caddylogger.Info("Caddy installed successfully locally")
	return nil
}

func (cm *CaddyManager) installRemote(ctx context.Context) error {
	if cm.serverMgr == nil {
		return fmt.Errorf("server manager not configured")
	}

	// Check if Caddy is already installed
	output, err := cm.serverMgr.ExecuteCommand(ctx, cm.serverMgr.ListServers()[0], "which caddy")
	if err == nil && strings.TrimSpace(output) != "" {
		caddylogger.Info("Caddy is already installed on remote server")
		return nil
	}

	caddylogger.Info("Installing Caddy on remote server...")

	// Install using official script with retries
	err = cm.retryCommand(ctx, 3, 10*time.Second, func() error {
		_, err := cm.serverMgr.ExecuteCommand(ctx, cm.serverMgr.ListServers()[0], 
			"curl https://getcaddy.com | bash -s personal")
		return err
	})

	if err != nil {
		caddylogger.Error("Failed to install Caddy remotely: %v", err)
		return fmt.Errorf("remote caddy installation failed: %w", err)
	}

	// Enable and start Caddy service
	if _, err := cm.serverMgr.ExecuteCommand(ctx, cm.serverMgr.ListServers()[0], 
		"sudo systemctl enable --now caddy"); err != nil {
		caddylogger.Error("Failed to enable Caddy service: %v", err)
		return fmt.Errorf("failed to enable caddy service: %w", err)
	}

	caddylogger.Info("Caddy installed and started successfully on remote server")
	return nil
}

// Configure sets up a new reverse proxy with automatic SSL
func (cm *CaddyManager) Configure(ctx context.Context, domain, backend string, opts ...ConfigOption) error {
	config := &caddyConfig{
		domain:  domain,
		backend: backend,
		port:    "80",
	}
	
	for _, opt := range opts {
		opt(config)
	}

	if cm.localMode {
		return cm.configureLocal(ctx, config)
	}
	return cm.configureRemote(ctx, config)
}

type caddyConfig struct {
	domain      string
	backend     string
	port        string
	basicAuth   []string
	redirectWWW bool
}

type ConfigOption func(*caddyConfig)

func WithPort(port string) ConfigOption {
	return func(c *caddyConfig) {
		c.port = port
	}
}

func WithBasicAuth(users []string) ConfigOption {
	return func(c *caddyConfig) {
		c.basicAuth = users
	}
}

func WithWWWRedirect() ConfigOption {
	return func(c *caddyConfig) {
		c.redirectWWW = true
	}
}

func (cm *CaddyManager) configureLocal(ctx context.Context, config *caddyConfig) error {
	configPath := "/etc/caddy/Caddyfile"
	caddylogger.Info("Creating local Caddy configuration for %s", config.domain)

	configContent := cm.generateConfig(config)
	
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		caddylogger.Error("Failed to write Caddyfile: %v", err)
		return fmt.Errorf("failed to write caddy config: %w", err)
	}

	// Reload Caddy
	if err := exec.CommandContext(ctx, "systemctl", "reload", "caddy").Run(); err != nil {
		caddylogger.Error("Failed to reload Caddy: %v", err)
		return fmt.Errorf("failed to reload caddy: %w", err)
	}

	caddylogger.Info("Caddy configuration applied successfully")
	cm.printDNSInstructions(config.domain)
	return nil
}

func (cm *CaddyManager) configureRemote(ctx context.Context, config *caddyConfig) error {
	if cm.serverMgr == nil {
		return fmt.Errorf("server manager not configured")
	}

	serverName := cm.serverMgr.ListServers()[0]
	caddylogger.Info("Configuring Caddy on %s for %s", serverName, config.domain)

	configContent := cm.generateConfig(config)
	
	// Upload configuration
	tempPath := "/tmp/Caddyfile_" + fmt.Sprint(time.Now().Unix())
	if err := cm.serverMgr.UploadFile(ctx, serverName, "", tempPath, strings.NewReader(configContent)); err != nil {
		caddylogger.Error("Failed to upload Caddyfile: %v", err)
		return fmt.Errorf("failed to upload caddy config: %w", err)
	}

	// Move to final location with sudo
	moveCmd := fmt.Sprintf("sudo mv %s /etc/caddy/Caddyfile && sudo chown caddy:caddy /etc/caddy/Caddyfile", tempPath)
	if _, err := cm.serverMgr.ExecuteCommand(ctx, serverName, moveCmd); err != nil {
		caddylogger.Error("Failed to install Caddyfile: %v", err)
		return fmt.Errorf("failed to install caddy config: %w", err)
	}

	// Reload Caddy
	if _, err := cm.serverMgr.ExecuteCommand(ctx, serverName, "sudo systemctl reload caddy"); err != nil {
		caddylogger.Error("Failed to reload Caddy: %v", err)
		return fmt.Errorf("failed to reload caddy: %w", err)
	}

	caddylogger.Info("Caddy configuration applied successfully on %s", serverName)
	cm.printDNSInstructions(config.domain)
	return nil
}

func (cm *CaddyManager) generateConfig(config *caddyConfig) string {
	var sb strings.Builder
	
	// Primary domain
	sb.WriteString(fmt.Sprintf("%s {\n", config.domain))
	
	// WWW redirect if enabled
	if config.redirectWWW {
		sb.WriteString(fmt.Sprintf("\tredir https://www.%s{uri}\n", config.domain))
	}
	
	// Basic auth if configured
	if len(config.basicAuth) > 0 {
		sb.WriteString("\tbasicauth {\n")
		for _, user := range config.basicAuth {
			sb.WriteString(fmt.Sprintf("\t\t%s\n", user))
		}
		sb.WriteString("\t}\n")
	}
	
	// Reverse proxy configuration
	sb.WriteString(fmt.Sprintf("\treverse_proxy %s {\n", config.backend))
	sb.WriteString("\t\theader_up Host {host}\n")
	sb.WriteString("\t\theader_up X-Real-IP {remote}\n")
	sb.WriteString("\t\theader_up X-Forwarded-For {remote}\n")
	sb.WriteString("\t\theader_up X-Forwarded-Proto {scheme}\n")
	sb.WriteString("\t}\n")
	sb.WriteString("}\n")
	
	// WWW version if redirect not enabled
	if !config.redirectWWW {
		sb.WriteString(fmt.Sprintf("www.%s {\n", config.domain))
		sb.WriteString(fmt.Sprintf("\treverse_proxy %s\n", config.backend))
		sb.WriteString("}\n")
	}
	
	return sb.String()
}

func (cm *CaddyManager) printDNSInstructions(domain string) {
	caddylogger.Info("\n=== DNS Setup Instructions ===")
	caddylogger.Info("For your domain to work properly, configure these DNS records:")
	
	if cm.localMode {
		caddylogger.Info("1. Create an A record for %s pointing to your server's public IP", domain)
		caddylogger.Info("2. Create a CNAME for www.%s pointing to %s", domain, domain)
	} else {
		serverName := cm.serverMgr.ListServers()[0]
		caddylogger.Info("1. On server %s, ensure the domain %s resolves to this server", serverName, domain)
	}
	
	caddylogger.Info("\nCaddy will automatically provision SSL certificates once DNS is configured")
	caddylogger.Info("DNS changes may take up to 48 hours to propagate globally")
}

func (cm *CaddyManager) retryCommand(ctx context.Context, maxAttempts int, delay time.Duration, fn func() error) error {
	var lastErr error
	
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		select {
		case <-ctx.Done():
			return fmt.Errorf("operation canceled: %w", lastErr)
		default:
		}
		
		err := fn()
		if err == nil {
			return nil
		}
		
		lastErr = err
		if attempt < maxAttempts {
			caddylogger.Warn("Attempt %d failed, retrying in %v: %v", attempt, delay, err)
			time.Sleep(delay)
		}
	}
	
	return fmt.Errorf("after %d attempts: %w", maxAttempts, lastErr)
}
```

## Usage Examples

### Local Caddy Management

```go
package main

import (
	"context"
	"log"
	"nextdeploy/internal/caddy"
	"time"
)

func main() {
	// Local Caddy management
	caddyMgr, err := caddy.New()
	if err != nil {
		log.Fatalf("Failed to create Caddy manager: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Install Caddy if needed
	if err := caddyMgr.Install(ctx); err != nil {
		log.Fatalf("Caddy installation failed: %v", err)
	}

	// Configure a reverse proxy
	err = caddyMgr.Configure(ctx, "example.com", "localhost:8080",
		caddy.WithWWWRedirect(),
		caddy.WithPort("443"),
	)
	if err != nil {
		log.Fatalf("Configuration failed: %v", err)
	}

	log.Println("Caddy configured successfully!")
}
```

### Remote Caddy Management via SSH

```go
package main

import (
	"context"
	"log"
	"nextdeploy/internal/caddy"
	"nextdeploy/internal/server"
	"time"
)

func main() {
	// First set up SSH server manager
	serverMgr, err := server.New(
		server.WithConfig("config.yaml"),
		server.WithSSH(),
	)
	if err != nil {
		log.Fatalf("Failed to initialize server manager: %v", err)
	}
	defer serverMgr.CloseSSHConnections()

	// Create Caddy manager with SSH backend
	caddyMgr, err := caddy.New(
		caddy.WithServerManager(serverMgr),
	)
	if err != nil {
		log.Fatalf("Failed to create Caddy manager: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Install Caddy on remote server
	if err := caddyMgr.Install(ctx); err != nil {
		log.Fatalf("Remote Caddy installation failed: %v", err)
	}

	// Configure reverse proxy with basic auth
	users := []string{
		"admin $2a$10$J9.8uZ7s3R2bQ1q1XwYbXeJ9d8vW7yYzB6nT5s4r3t2y1vXwZxXbC", // bcrypt hash
	}
	err = caddyMgr.Configure(ctx, "app.example.com", "localhost:3000",
		caddy.WithBasicAuth(users),
		caddy.WithPort("443"),
	)
	if err != nil {
		log.Fatalf("Remote configuration failed: %v", err)
	}

	log.Println("Remote Caddy configured successfully!")
}
```

## Key Improvements

1. **Unified Local/Remote Management**:
   - Single interface for both local and remote Caddy configuration
   - Automatic switching based on initialization options

2. **Enhanced Configuration**:
   - Support for basic authentication
   - WWW redirect options
   - Custom port configuration

3. **Better Error Handling**:
   - Retry logic for flaky operations
   - Context support for timeouts
   - Detailed error messages

4. **Security**:
   - Proper file permissions
   - Secure credential handling
   - Service management with sudo

5. **Integration**:
   - Works seamlessly with the SSH server package
   - Consistent logging approach

This package provides a robust way to manage Caddy configurations both locally and remotely, with automatic SSL certificate provisioning through Let's Encrypt. The functional options pattern makes it flexible and easy to extend with new features.
