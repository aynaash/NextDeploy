package cmd

import (
	"context"
	"fmt"
	"io"
	"nextdeploy/cli/internal/server"
	"nextdeploy/shared"
	"nextdeploy/shared/failfast"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var (
	PrepLogs = shared.PackageLogger("prepare", "🔧 PREPARE")

	// Command flags
	verbose    bool
	timeout    time.Duration
	streamMode bool
)

var prepareCmd = &cobra.Command{
	Use:   "prepare",
	Short: "Prepare target server with required tools",
	Long: `Installs Docker, Caddy and other required tools on the target server.
This command will:
1. Verify server connectivity
2. Install required packages
3. Configure necessary services
4. Validate the installation`,
	Run: runPrepare,
}

func init() {
	// Add flags for the prepare command
	rootCmd.AddCommand(prepareCmd)
}

func runPrepare(cmd *cobra.Command, args []string) {
	// Create context with timeout
	if timeout == 0 {
		timeout = 3 * time.Minute // Default timeout
	}
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
		err := serverMgr.CloseSSHConnection()
		if err != nil {
			PrepLogs.Error("Failed to close SSH connections: %v", err)
		} else {
			PrepLogs.Debug("SSH connections closed successfully")
		}
	}()

	// Determine target server
	serverName, err := selectTargetServer(ctx, serverMgr)
	if err != nil {
		PrepLogs.Error("Failed to select target server: %v", err)
		return
	}
	// Run preparation
	err = executePreparation(ctx, serverMgr, serverName, stream)
	if err != nil {
		PrepLogs.Error("Preparation failed: %v", err)
		return
	}

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
	if err != nil {
		PrepLogs.Error("Server prerequisites verification failed: %v", err)
		return fmt.Errorf("server prerequisites verification failed: %w", err)
	}
	err = installRequiredPackages(ctx, serverMgr, serverName, stream)
	if err != nil {
		PrepLogs.Error("Failed to install required packages: %v", err)
		return fmt.Errorf("failed to install required packages: %w", err)
	}
	// Phase 3: Verification
	err = verifyInstallation(ctx, serverMgr, serverName, stream)
	if err != nil {
		PrepLogs.Error("Installation verification failed: %v", err)
		return fmt.Errorf("installation verification failed: %w", err)
	}
	return nil
}

func verifyServerPrerequisites(ctx context.Context, serverMgr *server.ServerStruct, serverName string, stream io.Writer) error {
	PrepLogs.Info("Verifying server prerequisites...")

	checks := []struct {
		command string
		message string
		timeout time.Duration // Individual timeout for each check
	}{
		{"uname -a", "Checking system information...", 5 * time.Second},
		{"df -h", "Checking disk space...", 10 * time.Second},
		{"free -m", "Checking memory...", 5 * time.Second},
	}

	for _, check := range checks {
		select {
		case <-ctx.Done():
			PrepLogs.Error("Prerequisite checks cancelled: %v", ctx.Err())
			return fmt.Errorf("prerequisite checks cancelled: %w", ctx.Err())
		default:
			if stream != nil {
				PrepLogs.Debug("Running prerequisite check: %s", check.command)
			}

			// Create a sub-context with individual timeout
			checkCtx, cancel := context.WithTimeout(ctx, check.timeout)
			defer cancel()

			output, err := serverMgr.ExecuteCommand(checkCtx, serverName, check.command, stream)
			if err != nil {
				fmt.Sprintf("Prerequisite check failed",
					"command", check.command,
					"error", err,
					"timeout", check.timeout)
				return fmt.Errorf("failed prerequisite check %q: %w", check.command, err)
			}
			verbose = true

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
	if err != nil {
		PrepLogs.Error("Failed to determine package manager: %v", err)
		return fmt.Errorf("failed to determine package manager: %w", err)
	}

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
	if err != nil {
		return fmt.Errorf("failed to install base packages: %w", err)
	}

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
		if err != nil {
			return fmt.Errorf("caddy installation failed: %w", err)
		}
	} else {
		logToStream(stream, "✓ Caddy already installed", color.FgGreen)
	}

	return nil
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

	if len(missingPkgs) == 0 {
		logToStream(stream, "✓ All base packages already installed", color.FgGreen)
		return nil
	}

	// Update package lists first
	updateCmd := "sudo apt update"
	if _, err := serverMgr.ExecuteCommand(ctx, serverName, updateCmd, stream); err != nil {
		PrepLogs.Warn("Failed to update package lists: %v", err)
	}

	// Install with retries
	installCmd := fmt.Sprintf("sudo DEBIAN_FRONTEND=noninteractive apt install -y %s", strings.Join(missingPkgs, " "))

	maxRetries := 3
	for attempt := 1; attempt <= maxRetries; attempt++ {
		if attempt > 1 {
			PrepLogs.Info("Retry attempt %d/%d for package installation", attempt, maxRetries)
			time.Sleep(2 * time.Second) // Wait before retry
		}

		output, err := serverMgr.ExecuteCommand(ctx, serverName, installCmd, stream)
		if err == nil {
			logToStream(stream, fmt.Sprintf("✓ Installed missing packages: %s", strings.Join(missingPkgs, ", ")), color.FgGreen)
			return nil
		}

		if attempt == maxRetries {
			PrepLogs.Debug("Final installation attempt failed. Output: %s", output)
			return fmt.Errorf("failed to install packages %s after %d attempts: %w",
				strings.Join(missingPkgs, ", "), maxRetries, err)
		}

		PrepLogs.Warn("Installation attempt %d failed: %v", attempt, err)
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

			output, err := serverMgr.ExecuteCommand(ctx, serverName, cmd, stream)
			if err != nil {
				PrepLogs.Debug("Command failed: %s, Output: %s", cmd, output)
				return fmt.Errorf("command failed: %s: %w", cmd, err)
			}
		}
	}
	return nil
}
func verifySudoAccess(ctx context.Context, serverMgr *server.ServerStruct, serverName string, stream io.Writer) error {
	PrepLogs.Info("Verifying sudo access...")

	// Test if we can run sudo without password
	testCmd := "sudo -n true 2>/dev/null && echo 'SUDO_OK' || echo 'SUDO_FAILED'"

	output, err := serverMgr.ExecuteCommand(ctx, serverName, testCmd, stream)
	if err != nil {
		return fmt.Errorf("sudo test failed: %w", err)
	}

	if strings.Contains(output, "SUDO_FAILED") {
		return fmt.Errorf("passwordless sudo not working. Output: %s", output)
	}

	PrepLogs.Debug("Sudo access verified: %s", strings.TrimSpace(output))
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
		{"docker --version", "Docker"},
		{"caddy version", "Caddy"},
	}

	for _, check := range checks {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			if stream != nil {
				color.New(color.FgBlue).Fprintf(stream, "🔍 Checking %s installation...\n", check.tool)
				PrepLogs.Debug("Running command: %s", check.command)

			}

			output, err := serverMgr.ExecuteCommand(ctx, serverName, check.command, stream)
			if err != nil {
				PrepLogs.Error("Failed to verify %s installation: %v", check.tool, err)
				return fmt.Errorf("failed to verify %s installation: %w", check.tool, err)
			}

			if verbose {
				PrepLogs.Debug("%s version: %s", check.tool, output)
			}
		}
	}

	return nil
}
