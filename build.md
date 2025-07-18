# NextDeploy Build & Deployment Guide

## Table of Contents
1. [Build Script Usage](#build-script-usage)
2. [Daemon Deployment](#daemon-deployment)
3. [CLI Setup](#cli-setup)
4. [File Structure & Permissions](#file-structure--permissions)
5. [Troubleshooting](#troubleshooting)

## Build Script Usage

### Basic Commands
```bash
# Build both daemon and CLI
go run build.go

# Build only daemon (for servers)
go run build.go -target daemon

# Build only CLI (for development)
go run build.go -target cli
```

### Build Flags
| Flag | Description | Default |
|------|-------------|---------|
| `-target` | Specify target to build (`daemon`, `cli`, or `all`) | `all` |
| `-output` | Custom output directory | `./bin` |
| `-version` | Override version number | Reads from version.txt |
| `-static` | Force static linking (for daemon) | `true` for daemon |

## Daemon Deployment

### System Requirements
- Linux server (x86_64 recommended)
- Docker installed and running
- Root/sudo access for service management

### Installation Steps
```bash
# 1. Copy binary to system location
sudo cp bin/nextdeploy-daemon /usr/local/bin/

# 2. Create required directories
sudo mkdir -p /var/lib/nextdeploy/{keys,logs}
sudo mkdir -p /etc/nextdeploy

# 3. Set permissions
sudo chown -R root:root /var/lib/nextdeploy
sudo chmod 700 /var/lib/nextdeploy/keys
sudo chmod 600 /var/lib/nextdeploy/logs

# 4. Create systemd service
sudo tee /etc/systemd/system/nextdeploy.service <<EOF
[Unit]
Description=NextDeploy Daemon
After=network.target docker.service

[Service]
User=root
ExecStart=/usr/local/bin/nextdeploy-daemon \
    --daemon \
    --key-dir /var/lib/nextdeploy/keys \
    --log-file /var/lib/nextdeploy/logs/daemon.log \
    --config /etc/nextdeploy/config.yaml \
    --host 0.0.0.0 \
    --port 8080
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

# 5. Enable and start
sudo systemctl daemon-reload
sudo systemctl enable --now nextdeploy
```

### Daemon Flags
| Flag | Description | Default | Required |
|------|-------------|---------|----------|
| `--daemon` | Run in daemon mode | `false` | Yes (for production) |
| `--key-dir` | Directory for encryption keys | `/var/lib/nextdeploy/keys` | Yes |
| `--log-file` | Path to log file | `/var/log/nextdeploy.log` | No |
| `--log-format` | Log format (`text` or `json`) | `json` | No |
| `--host` | Interface to bind to | `0.0.0.0` | No |
| `--port` | Port to listen on | `8080` | No |
| `--pid-file` | PID file location | `/var/run/nextdeploy.pid` | No |
| `--config` | Config file path | `/etc/nextdeploy/config.yaml` | No |
| `--debug` | Enable debug logging | `false` | No |

## CLI Setup

### Installation
```bash
# Copy to local bin directory
cp bin/nextdeploy ~/.local/bin/

# Or install system-wide
sudo cp bin/nextdeploy /usr/local/bin/
```

### Configuration
```bash
# Set daemon endpoint
export NEXTDEPLOY_DAEMON_URL="http://your-server:8080"

# For persistent configuration, add to ~/.bashrc
echo 'export NEXTDEPLOY_DAEMON_URL="http://your-server:8080"' >> ~/.bashrc
```

### CLI Flags
| Flag | Description | Example |
|------|-------------|---------|
| `--daemon-url` | Override daemon URL | `--daemon-url http://localhost:8080` |
| `--verbose` | Show detailed output | `--verbose` |
| `--config` | Use alternative config | `--config ~/.nextdeploy.yaml` |

## File Structure & Permissions

### Server Files
| Path | Permission | Owner | Purpose |
|------|-----------|-------|---------|
| `/usr/local/bin/nextdeploy-daemon` | `755` | `root:root` | Daemon binary |
| `/var/lib/nextdeploy/keys/` | `700` | `root:root` | Encryption keys |
| `/var/lib/nextdeploy/logs/` | `755` | `root:root` | Log files |
| `/etc/nextdeploy/config.yaml` | `600` | `root:root` | Configuration |
| `/var/run/nextdeploy.pid` | `644` | `root:root` | PID file |

### Developer Files
| Path | Permission | Owner | Purpose |
|------|-----------|-------|---------|
| `~/.local/bin/nextdeploy` | `755` | User | CLI binary |
| `~/.nextdeploy.yaml` | `600` | User | Local configuration |
| `~/.cache/nextdeploy/` | `700` | User | Cache files |

## Troubleshooting

### Common Issues
1. **Permission Denied Errors**
   ```bash
   sudo chown -R root:root /var/lib/nextdeploy
   sudo chmod 700 /var/lib/nextdeploy/keys
   ```

2. **Daemon Not Starting**
   ```bash
   journalctl -u nextdeploy -f
   ```

3. **CLI Connection Issues**
   ```bash
   curl -v http://your-server:8080/health
   ```

4. **Docker Permission Problems**
   ```bash
   sudo usermod -aG docker nextdeploy
   ```

### Log Locations
- Daemon logs: `/var/lib/nextdeploy/logs/daemon.log`
- System logs: `journalctl -u nextdeploy`
- CLI debug: `nextdeploy --verbose [command]`

## Security Recommendations
1. Always run the daemon as root in production
2. Restrict key directory to root-only access
3. Use HTTPS for remote connections
4. Regularly rotate encryption keys
5. Monitor `/var/lib/nextdeploy/logs/` for suspicious activity

For production deployments, consider adding:
- TLS certificates
- Firewall rules limiting access to the daemon port
-# **NextDeploy Build System Guide**

## **Clear Naming Convention**
To avoid confusion between components, we'll use these names consistently:
- **`nextdeployd`**: The daemon (server component)
- **`ndctl`**: The CLI tool (developer component)

## **Revised Build Script**
```go
package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func main() {
	// Configuration
	projectRoot := "."
	daemonPath := filepath.Join(projectRoot, "daemon", "main.go")
	cliPath := filepath.Join(projectRoot, "cli", "main.go")
	outputDir := filepath.Join(projectRoot, "bin")

	// Ensure bin directory exists
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		fail("Error creating bin directory: %v", err)
	}

	// Get version info
	version := getVersion()
	commit := getGitCommit()
	buildTime := time.Now().Format(time.RFC3339)

	// Build targets
	targets := []struct {
		name        string
		source      string
		output      string
		environment []string
		ldflags     string
	}{
		{
			name:   "nextdeployd (server daemon)",
			source: daemonPath,
			output: filepath.Join(outputDir, "nextdeployd"),
			environment: []string{
				"CGO_ENABLED=0",  // Static linking
				"GOOS=linux",     // Target OS
				"GOARCH=amd64",   // Target architecture
			},
			ldflags: fmt.Sprintf(`-s -w -X main.Version=%s -X main.Commit=%s -X main.BuildTime=%s`,
				version, commit, buildTime),
		},
		{
			name:   "ndctl (CLI tool)",
			source: cliPath,
			output: filepath.Join(outputDir, "ndctl"),
			environment: []string{
				fmt.Sprintf("GOOS=%s", getLocalOS()),
				fmt.Sprintf("GOARCH=%s", getLocalArch()),
			},
			ldflags: fmt.Sprintf(`-X main.Version=%s -X main.Commit=%s`, version, commit),
		},
	}

	// Execute builds
	for _, target := range targets {
		fmt.Printf("\n🚀 Building %s...\n", target.name)
		fmt.Printf("   Source: %s\n", target.source)
		fmt.Printf("   Output: %s\n", target.output)

		cmd := exec.Command("go", "build",
			"-ldflags", target.ldflags,
			"-o", target.output,
			target.source,
		)
		cmd.Env = append(os.Environ(), target.environment...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			fail("Error building %s: %v", target.name, err)
		}

		fmt.Printf("✅ Successfully built %s\n", target.output)
	}

	printPostBuildInstructions()
}

// ... [keep existing helper functions unchanged] ...

func printPostBuildInstructions() {
	fmt.Println(`
📝 Post-Build Instructions:

SERVER COMPONENT (nextdeployd):
1. Copy to server: scp bin/nextdeployd user@server:/usr/local/bin/
2. Create system directories:
   sudo mkdir -p /var/lib/nextdeploy/{keys,logs}
   sudo chmod 700 /var/lib/nextdeploy/keys
3. Set up systemd service:
   sudo cp examples/nextdeployd.service /etc/systemd/system/
   sudo systemctl enable --now nextdeployd

DEVELOPER TOOL (ndctl):
1. Install locally:
   mv bin/ndctl ~/.local/bin/
2. Configure daemon endpoint:
   echo 'export NDCTL_ENDPOINT="http://your-server:8080"' >> ~/.bashrc
   source ~/.bashrc
3. Verify connection:
   ndctl ping

💡 Production Tip: Build nextdeployd directly on target server for compatibility.
`)
}
```

## **Key Improvements**

1. **Clear Naming Scheme**:
   - Daemon: `nextdeployd` (standard *nix daemon naming)
   - CLI: `ndctl` (NextDeploy Control)

2. **Separate Build Configurations**:
   - Daemon gets full optimization flags (`-s -w`)
   - CLI keeps debug symbols for development

3. **Enhanced Instructions**:
   - Separate sections for server vs developer setup
   - Clear environment variable naming (`NDCTL_ENDPOINT`)

4. **Production Readiness**:
   - Explicit directory permission guidance
   - Systemd service example reference

## **Implementation Guide**

### **1. Building the Components**
```bash
# Build both components
go run build.go

# Build just the daemon
go run build.go -target daemon

# Build just the CLI
go run build.go -target cli
```

### **2. Server Deployment**
```bash
# On the server
sudo install -m 755 bin/nextdeployd /usr/local/bin/

# Create secure directories
sudo mkdir -p /var/lib/nextdeploy/{keys,logs}
sudo chown -R root:root /var/lib/nextdeploy
sudo chmod 700 /var/lib/nextdeploy/keys

# Systemd service (save as /etc/systemd/system/nextdeployd.service)
[Unit]
Description=NextDeploy Daemon
After=network.target

[Service]
ExecStart=/usr/local/bin/nextdeployd \
    --key-dir /var/lib/nextdeploy/keys \
    --log-file /var/lib/nextdeploy/logs/daemon.log
Restart=always

[Install]
WantedBy=multi-user.target
```

### **3. Developer Setup**
```bash
# Install CLI
mv bin/ndctl ~/.local/bin/

# Configure
echo 'export NDCTL_ENDPOINT="http://your-server:8080"' >> ~/.bashrc
source ~/.bashrc

# Test
ndctl version
ndctl containers list
```

## **Why These Changes Matter**

1. **Avoids Naming Collisions**:
   - Clear distinction between server (`nextdeployd`) and client (`ndctl`)

2. **Production-Grade Defaults**:
   - Daemon builds as static binary
   - Secure default permissions

3. **Developer Experience**:
   - Intuitive CLI name (`ndctl`)
   - Clear environment variable naming

4. **Maintainability**:
   - Separate build configurations
   - Explicit documentation

This revised structure provides a clear separation of concerns while maintaining all the functionality of the original system. Regular automated backups of `/var/lib/nextdeploy/keys/`
