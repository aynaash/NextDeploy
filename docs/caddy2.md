# Enhanced Caddy SSL Management Package

Here's an improved version of your Caddy management functionality that integrates with the SSH server package we discussed earlier. This provides remote Caddy configuration capabilities with better error handling, logging, and SSH integration.

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
