package cmd

import (
	"context"
	"fmt"
	"io"
	"nextdeploy/internal/failfast"
	"nextdeploy/internal/logger"
	"nextdeploy/internal/server"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var (
	PrepLogs = logger.PackageLogger("prepare", "🔧 PREPARE")

	// Command flags
	verbose    bool
	timeout    time.Duration
	streamMode bool
)

var prepareCmd = &cobra.Command{
	Use:   "prepare",
	Short: "Prepare target server with required tools",
	Long: `Installs Docker, Caddy, Go and other required tools on the target server.
This command will:
1. Verify server connectivity
2. Install required packages
3. Configure necessary services
4. Validate the installation`,
	Run: runPrepare,
}

func init() {
	rootCmd.AddCommand(prepareCmd)
}

func runPrepare(cmd *cobra.Command, args []string) {
	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		cancel()
		PrepLogs.Warn("Received interrupt signal, cancelling preparation...")
	}()

	// Get the appropriate output writer
	var stream io.Writer
	if streamMode {
		stream = cmd.OutOrStdout()
	}

	// Initialize server manager
	serverMgr, err := server.New(
		server.WithConfig(),
		server.WithSSH(),
	)
	failfast.Failfast(err, failfast.Error, "Error setting up server")

	defer func() {
		err := serverMgr.CloseSSHConnections()
		failfast.Failfast(err, failfast.Error, "Error closing SSH connections")

	}()

	// Determine target server
	serverName, err := selectTargetServer(ctx, serverMgr)
	failfast.Failfast(err, failfast.Error, "Failed to select target server")
	// Run preparation
	err = executePreparation(ctx, serverMgr, serverName, stream)
	failfast.Failfast(err, failfast.Error, "Preparation failed")

	PrepLogs.Success("✅ Server %s prepared successfully!", serverName)
}

func selectTargetServer(ctx context.Context, serverMgr *server.ServerStruct) (string, error) {
	// Check for deployment server first
	if depServer, err := serverMgr.GetDeploymentServer(); err == nil {
		PrepLogs.Debug("Using deployment server: %s", depServer)
		return depServer, nil
	}

	// Fall back to first available server
	servers := serverMgr.ListServers()
	if len(servers) == 0 {
		return "", fmt.Errorf("no servers configured")
	}

	PrepLogs.Warn("No deployment server configured, using first server: %s", servers[0])
	return servers[0], nil
}

func executePreparation(ctx context.Context, serverMgr *server.ServerStruct, serverName string, stream io.Writer) error {
	PrepLogs.Info("Starting preparation of server %s", serverName)

	// Phase 1: Pre-check
	err := verifyServerPrerequisites(ctx, serverMgr, serverName, stream)
	failfast.Failfast(err, failfast.Error, "Server prerequisites verification failed")
	// Phase 2: Installation
	err = installRequiredPackages(ctx, serverMgr, serverName, stream)
	failfast.Failfast(err, failfast.Error, "Package installation failed")
	// Phase 3: Verification
	err = verifyInstallation(ctx, serverMgr, serverName, stream)
	failfast.Failfast(err, failfast.Error, "Installation verification failed")
	return nil
}

func verifyServerPrerequisites(ctx context.Context, serverMgr *server.ServerStruct, serverName string, stream io.Writer) error {
	PrepLogs.Info("Verifying server prerequisites...")

	checks := []struct {
		command string
		message string
	}{
		{"uname -a", "Checking system information..."},
		{"df -h", "Checking disk space..."},
		{"free -m", "Checking memory..."},
	}

	for _, check := range checks {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			if stream != nil {
				PrepLogs.Debug("Running prerequisite check: %s", check.command)
			}

			output, err := serverMgr.ExecuteCommand(ctx, serverName, check.command, stream)
			failfast.Failfast(err, failfast.Error, fmt.Sprintf("Failed to run prerequisite check: %s", check.command))

			if verbose {
				PrepLogs.Debug("Check %q output:\n%s", check.command, output)
			}
		}
	}

	return nil
}

func installRequiredPackages(ctx context.Context, serverMgr *server.ServerStruct, serverName string, stream io.Writer) error {
	PrepLogs.Info("Installing required packages...")

	// First determine package manager - trim any whitespace from output
	pkgManager, err := serverMgr.ExecuteCommand(ctx, serverName, "command -v apt-get >/dev/null && echo apt || echo yum", stream)
	failfast.Failfast(err, failfast.Error, "Failed to determine package manager")

	// Clean up the package manager string
	pkgManager = strings.TrimSpace(pkgManager)
	PrepLogs.Debug("Detected package manager: %s", pkgManager)

	// Execute appropriate installation commands
	switch pkgManager {
	case "apt":
		PrepLogs.Info("Detected APT package manager")
		return installWithApt(ctx, serverMgr, serverName, stream)
	case "yum", "dnf":
		PrepLogs.Info("Detected YUM/DNF package manager")
		return installWithYum(ctx, serverMgr, serverName, stream)
	default:
		return fmt.Errorf("unsupported package manager: %s", pkgManager)
	}
}

func installWithApt(ctx context.Context, serverMgr *server.ServerStruct, serverName string, stream io.Writer) error {
	// Check and install base packages only if missing
	basePkgs := []string{"curl", "git", "make", "gcc", "build-essential"}
	err := installIfMissing(ctx, serverMgr, serverName, basePkgs, stream)
	failfast.Failfast(err, failfast.Error, "Failed to install base packages")

	// Docker installation check
	if !isInstalled(ctx, serverMgr, serverName, "docker", stream) {
		dockerCmds := []string{
			"curl -fsSL https://get.docker.com | sudo sh",
			"sudo usermod -aG docker $USER",
			"sudo systemctl enable docker",
			"sudo systemctl start docker",
		}
		if err := executeCommands(ctx, serverMgr, serverName, dockerCmds, stream); err != nil {
			return fmt.Errorf("docker installation failed: %w", err)
		}
	} else {
		logToStream(stream, "✓ Docker already installed", color.FgGreen)
	}
	//TODO: 	//sudo apt update
	// sudo apt install amazon-ecr-credential-helper

	// Caddy installation check
	if !isInstalled(ctx, serverMgr, serverName, "caddy", stream) {
		caddyCmds := []string{
			"sudo apt install -y debian-keyring debian-archive-keyring apt-transport-https",
			"curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' | sudo gpg --batch --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg",
			"curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' | sudo tee /etc/apt/sources.list.d/caddy-stable.list",
			"sudo apt update",
			"sudo DEBIAN_FRONTEND=noninteractive apt install -y caddy",
			"sudo systemctl enable caddy",
			"sudo systemctl start caddy",
		}
		err := executeCommands(ctx, serverMgr, serverName, caddyCmds, stream)
		failfast.Failfast(err, failfast.Error, "Caddy installation failed")
	} else {
		logToStream(stream, "✓ Caddy already installed", color.FgGreen)
	}

	// Go installation check
	if !isInstalled(ctx, serverMgr, serverName, "/usr/local/go/bin/go", stream) {
		goCmds := []string{
			"export GOLANG_VERSION=1.21.0",
			"export ARCH=amd64",
			"curl -OL https://golang.org/dl/go${GOLANG_VERSION}.linux-${ARCH}.tar.gz",
			"sudo rm -rf /usr/local/go",
			"sudo tar -C /usr/local -xzf go${GOLANG_VERSION}.linux-${ARCH}.tar.gz",
			// Add to both profile files
			`echo 'export PATH=$PATH:/usr/local/go/bin' | sudo tee -a /etc/profile.d/go.sh >/dev/null`,
			`echo 'export PATH=$PATH:/usr/local/go/bin' >> $HOME/.bashrc`,
			// Set permissions
			"sudo chmod 644 /etc/profile.d/go.sh",
			// Cleanup
			"rm go${GOLANG_VERSION}.linux-${ARCH}.tar.gz",
			// Verify installation using full path
			"/usr/local/go/bin/go version"}
		if err := executeCommands(ctx, serverMgr, serverName, goCmds, stream); err != nil {
			return fmt.Errorf("go installation failed: %w", err)
		}
	} else {
		logToStream(stream, "✓ Go already installed", color.FgGreen)
		error := ensureGoPath(ctx, serverMgr, serverName, stream)
		if error != nil {
			PrepLogs.Warn("Failed to ensure Go PATH: %v", error)
		}
	}

	return nil
}
func ensureGoPath(ctx context.Context, serverMgr *server.ServerStruct, serverName string, stream io.Writer) error {
	cmds := []string{
		// Check if PATH is already configured
		`grep -q '/usr/local/go/bin' /etc/profile.d/go.sh || echo 'export PATH=$PATH:/usr/local/go/bin' | sudo tee -a /etc/profile.d/go.sh >/dev/null`,
		`grep -q '/usr/local/go/bin' $HOME/.bashrc || echo 'export PATH=$PATH:/usr/local/go/bin' >> $HOME/.bashrc`,
		"sudo chmod 644 /etc/profile.d/go.sh",
	}
	return executeCommands(ctx, serverMgr, serverName, cmds, stream)
}

// Helper functions
func isInstalled(ctx context.Context, serverMgr *server.ServerStruct, serverName, command string, stream io.Writer) bool {
	checkCmd := fmt.Sprintf("command -v %s >/dev/null 2>&1", command)
	_, err := serverMgr.ExecuteCommand(ctx, serverName, checkCmd, stream)
	return err == nil
}

func installIfMissing(ctx context.Context, serverMgr *server.ServerStruct, serverName string, packages []string, stream io.Writer) error {
	var missingPkgs []string
	for _, pkg := range packages {
		if !isInstalled(ctx, serverMgr, serverName, pkg, stream) {
			missingPkgs = append(missingPkgs, pkg)
		}
	}

	if len(missingPkgs) > 0 {
		installCmd := fmt.Sprintf("sudo apt install -y %s", strings.Join(missingPkgs, " "))
		if _, err := serverMgr.ExecuteCommand(ctx, serverName, installCmd, stream); err != nil {
			return err
		}
		logToStream(stream, fmt.Sprintf("✓ Installed missing packages: %s", strings.Join(missingPkgs, ", ")), color.FgGreen)
	} else {
		logToStream(stream, "✓ All base packages already installed", color.FgGreen)
	}
	return nil
}

func logToStream(stream io.Writer, message string, colorAttr color.Attribute) {
	if stream != nil {
		c := color.New(colorAttr)
		c.Fprintln(stream, message)
	}
}

func installWithYum(ctx context.Context, serverMgr *server.ServerStruct, serverName string, stream io.Writer) error {
	commands := []string{
		"sudo yum install -y curl git make gcc glibc-static ca-certificates yum-utils device-mapper-persistent-data lvm2",
		"curl -fsSL https://get.docker.com | sudo sh",
		"sudo usermod -aG docker $USER",
		"sudo systemctl enable docker",
		"sudo systemctl start docker",
		`sudo yum install -y yum-plugin-copr &&
		 sudo yum copr enable -y @caddy/caddy &&
		 sudo yum install -y caddy`,
		`curl -OL https://golang.org/dl/go1.21.0.linux-amd64.tar.gz &&
		 sudo rm -rf /usr/local/go &&
		 sudo tar -C /usr/local -xzf go1.21.0.linux-amd64.tar.gz &&
		 echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc &&
		 source ~/.bashrc &&
		 rm go1.21.0.linux-amd64.tar.gz`,
	}

	return executeCommands(ctx, serverMgr, serverName, commands, stream)
}

func executeCommands(ctx context.Context, serverMgr *server.ServerStruct, serverName string, commands []string, stream io.Writer) error {
	for _, cmd := range commands {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			if stream != nil {
				color.New(color.FgYellow).Fprintf(stream, "▶ %s\n", cmd)
			}

			_, err := serverMgr.ExecuteCommand(ctx, serverName, cmd, stream)
			failfast.Failfast(err, failfast.Error, fmt.Sprintf("Command failed: %s", cmd))
		}
	}
	return nil
}

// CreateAppDirectory creates the minimal directory structure for containerized Next.js app
func CreateAppDirectory(ctx context.Context, serverMgr *server.ServerStruct, serverName string) error {
	const appDir = "/opt/nextjs-app" // Standard location for containerized apps

	PrepLogs.Info("Creating application directory on %s at %s", serverName, appDir)

	// Create the base directory with proper permissions
	createCmd := fmt.Sprintf(
		"sudo mkdir -p %s && sudo chown -R $(whoami): %s && sudo chmod 755 %s",
		appDir,
		appDir,
		appDir,
	)

	_, err := serverMgr.ExecuteCommand(ctx, serverName, createCmd, os.Stdout)
	failfast.Failfast(err, failfast.Error, fmt.Sprintf("Failed to create application directory: %s", appDir))
	// Verify directory was created
	verifyCmd := fmt.Sprintf("test -d %s && echo 'exists' || echo 'missing'", appDir)
	output, err := serverMgr.ExecuteCommand(ctx, serverName, verifyCmd, nil)
	if err != nil || !strings.Contains(output, "exists") {
		return fmt.Errorf("directory verification failed: %w (output: %s)", err, output)
	}

	PrepLogs.Success("✅ Application directory ready at %s", appDir)
	return nil
}
func verifyInstallation(ctx context.Context, serverMgr *server.ServerStruct, serverName string, stream io.Writer) error {
	PrepLogs.Info("Verifying installations...")

	checks := []struct {
		command string
		tool    string
	}{
		// Source profile before checking go to ensure PATH is set
		{`source /etc/profile.d/go.sh >/dev/null 2>&1 || true; go version`, "Go"},
		{"docker --version", "Docker"},
		{"caddy version", "Caddy"}}

	for _, check := range checks {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			if stream != nil {
				color.New(color.FgBlue).Fprintf(stream, "🔍 Checking %s installation...\n", check.tool)
			}

			output, err := serverMgr.ExecuteCommand(ctx, serverName, check.command, stream)
			failfast.Failfast(err, failfast.Error, fmt.Sprintf("Failed to check %s installation", check.tool))

			if verbose {
				PrepLogs.Debug("%s version: %s", check.tool, output)
			}
		}
	}

	return nil
}
