package cmd

import (
	"context"
	"fmt"
	"io"
	"nextdeploy/internal/logger"
	"nextdeploy/internal/server"
	"nextdeploy/internal/server/preparation"
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
	verbose      bool
	timeout      time.Duration
	streamMode   bool
	installAWS   bool
	forceInstall bool
	skipVerify   bool
	installNginx bool
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
	prepareCmd.Flags().BoolVarP(&installNginx, "nginx", "n", false, "Install Nginx instead of Caddy")
	prepareCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output")
	prepareCmd.Flags().DurationVar(&timeout, "timeout", 5*time.Minute, "Timeout for the preparation process")
	prepareCmd.Flags().BoolVar(&streamMode, "stream", true, "Stream command output in real-time")
	prepareCmd.Flags().BoolVar(&installAWS, "aws", false, "Install AWS CLI tools")
	prepareCmd.Flags().BoolVar(&forceInstall, "force", false, "Force reinstall of all packages")
	prepareCmd.Flags().BoolVar(&skipVerify, "skip-verify", false, "Skip installation verification")
	rootCmd.AddCommand(prepareCmd)
}

func runPrepare(cmd *cobra.Command, args []string) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer func() {
		cancel()
	}()
	// setup signal handling
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-signalChan
		PrepLogs.Warn("Preparation interrupted by user")
		cancel()

	}()

	var outputStream io.Writer = io.Discard
	if streamMode {
		outputStream = newBufferedStreamWriter(os.Stdout)
		defer outputStream.(*bufferedStreamWriter).Flush()
	} else {
		outputStream = os.Stdout
	}

	serverMgr, err := server.New(
		server.WithConfig(),
		server.WithSSH(),
		server.WithDaemon(),
	)
	if err != nil {
		PrepLogs.Error("Failed to initialize server manager: %v", err)
		return
	}
	PrepLogs.Info("Preparing server with configuration")
	defer func() {
		serverMgr.CloseSSHConnection()
	}()

	serverName, err := selectTargetServer(ctx, serverMgr)
	if err != nil {
		PrepLogs.Error("Failed to select target server: %v", err)
		return
	}
	// package manager initialization
	pkgManager, err := preparation.PackageManagerFactory(
		ctx,
		serverMgr,
		serverName,
		outputStream,
	)
	if err != nil {
		PrepLogs.Error("Failed to initialize package manager: %v", err)
		return
	}
	PrepLogs.Info("Using package manager: %s", pkgManager)
	_ = preparation.NewPreparationManager(
		serverMgr,
		pkgManager,
		installAWS,
		forceInstall,
		installNginx,
		verbose,
	)
	if err := executePreparation(ctx, serverMgr, serverName, outputStream, skipVerify); err != nil {
		PrepLogs.Error("Preparation failed: %v", err)
		return
	}
	PrepLogs.Success("Preparation completed successfully for server: %s", serverName)
}

func selectTargetServer(ctx context.Context, serverMgr *server.ServerStruct) (string, error) {
	if depServer, err := serverMgr.GetDeploymentServer(); err == nil {
		PrepLogs.Info("Using deployment server: %s", depServer)
		return depServer, nil
	}

	servers := serverMgr.ListServers()
	if len(servers) == 0 {
		return "", fmt.Errorf("no servers configured")
	}

	PrepLogs.Warn("No deployment server configured, using first server: %s", servers[0])
	return servers[0], nil
}

func executePreparation(
	ctx context.Context,
	prepManager preparation.Preparer,
	serverName string,
	stream io.Writer,
	skipVerify bool,
) error {
	PrepLogs.Info("🚀 Starting preparation of server %s", serverName)

	steps := []struct {
		name    string
		execute func() error
	}{
		{
			"Prerequisite Verification",
			func() error { return prepManager.VerifyPreRequisites(ctx, serverName, stream) },
		},
		{
			"Package Installation",
			func() error { return prepManager.InstallPackages(ctx, serverName, stream) },
		},
		{
			"Directory Setup",
			func() error { return prepManager.SetupDirectories(ctx, serverName, stream) },
		},
	}

	if !skipVerify {
		steps = append(steps, struct {
			name    string
			execute func() error
		}{
			"Installation Verification",
			func() error { return prepManager.VerifyInstallation(ctx, serverName, stream) },
		})
	}

	for _, step := range steps {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			PrepLogs.Info("=== %s ===", step.name)
			if err := step.execute(); err != nil {
				return fmt.Errorf("%s failed: %w", step.name, err)
			}
		}
	}

	return nil
}

func verifyServerPrerequisites(ctx context.Context, serverMgr *server.ServerStruct, serverName string, stream io.Writer) error {
	PrepLogs.Info("🔍 Verifying server prerequisites...")

	checks := []struct {
		command string
		message string
	}{
		{"uname -a", "System Information"},
		{"df -h --output=size,used,avail,pcent", "Disk Space"},
		{"free -m", "Memory"},
		{"nproc", "CPU Cores"},
		{"lscpu", "CPU Architecture"},
	}

	for _, check := range checks {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			logToStream(stream, fmt.Sprintf("Checking %s...", check.message), color.FgBlue)

			output, err := serverMgr.ExecuteCommand(ctx, serverName, check.command, stream)
			if err != nil {
				return fmt.Errorf("prerequisite check failed for %s: %w", check.message, err)
			}

			if verbose {
				PrepLogs.Debug("%s check output:\n%s", check.message, output)
			}
		}
	}

	logToStream(stream, "✓ All prerequisites verified", color.FgGreen)
	return nil
}

func installRequiredPackages(ctx context.Context, serverMgr *server.ServerStruct, serverName string, stream io.Writer) error {
	pkgManager, err := detectPackageManager(ctx, serverMgr, serverName, stream)
	if err != nil {
		return fmt.Errorf("failed to detect package manager: %w", err)
	}

	switch pkgManager {
	case "apt":
		return installWithApt(ctx, serverMgr, serverName, stream)
	case "yum", "dnf":
		return installWithYum(ctx, serverMgr, serverName, stream)
	default:
		return fmt.Errorf("unsupported package manager: %s", pkgManager)
	}
}

func detectPackageManager(ctx context.Context, serverMgr *server.ServerStruct, serverName string, stream io.Writer) (string, error) {
	// Try to detect package manager with fallbacks
	detectCmd := `
		if command -v apt-get >/dev/null 2>&1; then echo "apt";
		elif command -v dnf >/dev/null 2>&1; then echo "dnf";
		elif command -v yum >/dev/null 2>&1; then echo "yum";
		else echo "unknown"; fi
	`

	pkgManager, err := serverMgr.ExecuteCommand(ctx, serverName, detectCmd, stream)
	if err != nil {
		PrepLogs.Error("Failed to detect package manager: %v", err)
		return "", err
	}

	return strings.TrimSpace(pkgManager), nil
}

func installWithApt(ctx context.Context, serverMgr *server.ServerStruct, serverName string, stream io.Writer) error {
	logToStream(stream, "Detected APT package manager", color.FgYellow)

	// Update package lists first
	if err := runCommand(ctx, serverMgr, serverName, "sudo apt-get update -y", "Updating package lists", stream); err != nil {
		PrepLogs.Error("Failed to update package lists: %v", err)
		return err
	}

	// Install base packages
	basePkgs := []string{
		"curl", "git", "make", "gcc", "build-essential",
		"ca-certificates", "software-properties-common",
	}

	if err := installPackages(ctx, serverMgr, serverName, basePkgs, "Base packages", stream); err != nil {
		PrepLogs.Error("Failed to install base packages: %v", err)
		return err
	}

	// Install Docker
	if forceInstall || !isInstalled(ctx, serverMgr, serverName, "docker", stream) {
		dockerCmds := []string{
			"sudo apt-get remove -y docker docker-engine docker.io containerd runc",
			"curl -fsSL https://get.docker.com | sudo sh",
			"sudo usermod -aG docker $USER",
			"sudo systemctl enable docker",
			"sudo systemctl start docker",
		}
		if err := runCommands(ctx, serverMgr, serverName, dockerCmds, "Installing Docker", stream); err != nil {
			PrepLogs.Error("Failed to install Docker: %v", err)
			return err
		}
	} else {
		logToStream(stream, "✓ Docker already installed", color.FgGreen)
	}

	// Install Caddy
	if forceInstall || !isInstalled(ctx, serverMgr, serverName, "caddy", stream) {
		caddyCmds := []string{
			"sudo apt-get install -y debian-keyring debian-archive-keyring apt-transport-https",
			"curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' | sudo gpg --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg",
			"curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' | sudo tee /etc/apt/sources.list.d/caddy-stable.list",
			"sudo apt-get update",
			"sudo apt-get install -y caddy",
			"sudo systemctl enable caddy",
			"sudo systemctl start caddy",
		}
		if err := runCommands(ctx, serverMgr, serverName, caddyCmds, "Installing Caddy", stream); err != nil {
			PrepLogs.Error("Failed to install Caddy: %v", err)
			return err
		}
	} else {
		logToStream(stream, "✓ Caddy already installed", color.FgGreen)
	}

	// Install AWS CLI if requested
	if installAWS && (forceInstall || !isInstalled(ctx, serverMgr, serverName, "aws", stream)) {
		awsCmds := []string{
			"curl -sL https://awscli.amazonaws.com/awscli-exe-linux-x86_64.zip -o awscliv2.zip",
			"unzip -q awscliv2.zip",
			"sudo ./aws/install --bin-dir /usr/local/bin --install-dir /usr/local/aws-cli --update",
			"rm -rf awscliv2.zip aws",
		}
		if err := runCommands(ctx, serverMgr, serverName, awsCmds, "Installing AWS CLI", stream); err != nil {
			PrepLogs.Error("Failed to install AWS CLI: %v", err)
			return err
		}
	} else if installAWS {
		logToStream(stream, "✓ AWS CLI already installed", color.FgGreen)
	}

	return nil
}

func installWithYum(ctx context.Context, serverMgr *server.ServerStruct, serverName string, stream io.Writer) error {
	logToStream(stream, "Detected YUM/DNF package manager", color.FgYellow)

	// Install base packages
	basePkgs := []string{
		"curl", "git", "make", "gcc", "glibc-static",
		"ca-certificates", "yum-utils", "device-mapper-persistent-data", "lvm2",
	}

	if err := installPackages(ctx, serverMgr, serverName, basePkgs, "Base packages", stream); err != nil {
		return err
	}

	// Install Docker
	if forceInstall || !isInstalled(ctx, serverMgr, serverName, "docker", stream) {
		dockerCmds := []string{
			"sudo yum remove -y docker docker-client docker-client-latest docker-common docker-latest docker-latest-logrotate docker-logrotate docker-engine",
			"curl -fsSL https://get.docker.com | sudo sh",
			"sudo usermod -aG docker $USER",
			"sudo systemctl enable docker",
			"sudo systemctl start docker",
		}
		if err := runCommands(ctx, serverMgr, serverName, dockerCmds, "Installing Docker", stream); err != nil {
			PrepLogs.Error("Failed to install Docker: %v", err)
			return err
		}
	} else {
		logToStream(stream, "✓ Docker already installed", color.FgGreen)
	}

	// Install Caddy
	if forceInstall || !isInstalled(ctx, serverMgr, serverName, "caddy", stream) {
		caddyCmds := []string{
			"sudo yum install -y yum-plugin-copr",
			"sudo yum copr enable -y @caddy/caddy",
			"sudo yum install -y caddy",
			"sudo systemctl enable caddy",
			"sudo systemctl start caddy",
		}
		if err := runCommands(ctx, serverMgr, serverName, caddyCmds, "Installing Caddy", stream); err != nil {
			PrepLogs.Error("Failed to install Caddy: %v", err)
			return err
		}
	} else {
		logToStream(stream, "✓ Caddy already installed", color.FgGreen)
	}

	// Install AWS CLI if requested
	if installAWS && (forceInstall || !isInstalled(ctx, serverMgr, serverName, "aws", stream)) {
		awsCmds := []string{
			"curl -sL https://awscli.amazonaws.com/awscli-exe-linux-x86_64.zip -o awscliv2.zip",
			"unzip -q awscliv2.zip",
			"sudo ./aws/install --bin-dir /usr/local/bin --install-dir /usr/local/aws-cli --update",
			"rm -rf awscliv2.zip aws",
		}
		if err := runCommands(ctx, serverMgr, serverName, awsCmds, "Installing AWS CLI", stream); err != nil {
			PrepLogs.Error("Failed to install AWS CLI: %v", err)
			return err
		}
	} else if installAWS {
		logToStream(stream, "✓ AWS CLI already installed", color.FgGreen)
	}

	return nil
}

func installPackages(ctx context.Context, serverMgr *server.ServerStruct, serverName string, packages []string, name string, stream io.Writer) error {
	pkgManager, err := detectPackageManager(ctx, serverMgr, serverName, stream)
	PrepLogs.Error("Failed to detect package manager: %v", err)
	if err != nil {
		return err
	}

	var installCmd string
	switch pkgManager {
	case "apt":
		installCmd = fmt.Sprintf("sudo DEBIAN_FRONTEND=noninteractive apt-get install -y %s", strings.Join(packages, " "))
	case "yum", "dnf":
		installCmd = fmt.Sprintf("sudo yum install -y %s", strings.Join(packages, " "))
	default:
		return fmt.Errorf("unsupported package manager: %s", pkgManager)
	}

	return runCommand(ctx, serverMgr, serverName, installCmd, fmt.Sprintf("Installing %s", name), stream)
}

func isInstalled(ctx context.Context, serverMgr *server.ServerStruct, serverName, command string, stream io.Writer) bool {
	checkCmd := fmt.Sprintf("command -v %s >/dev/null 2>&1 && echo 'installed' || echo 'not installed'", command)
	output, err := serverMgr.ExecuteCommand(ctx, serverName, checkCmd, stream)
	if err != nil {
		PrepLogs.Error("Failed to check %s installation: %v", command, err)
		return false
	}
	return err == nil && strings.Contains(output, "installed")
}

func createAppDirectory(ctx context.Context, serverMgr *server.ServerStruct, serverName string, stream io.Writer) error {
	const appDir = "/opt/nextjs-app"

	logToStream(stream, fmt.Sprintf("Creating application directory at %s", appDir), color.FgBlue)

	cmds := []string{
		fmt.Sprintf("sudo mkdir -p %s", appDir),
		fmt.Sprintf("sudo chown -R $(whoami): %s", appDir),
		fmt.Sprintf("sudo chmod 755 %s", appDir),
		fmt.Sprintf("test -d %s && echo 'exists' || echo 'missing'", appDir),
	}

	output, err := serverMgr.ExecuteCommand(ctx, serverName, strings.Join(cmds, " && "), stream)
	if err != nil || !strings.Contains(output, "exists") {
		PrepLogs.Error("Failed to create application directory: %v", err)
		return fmt.Errorf("failed to create application directory: %w (output: %s)", err, output)
	}

	logToStream(stream, fmt.Sprintf("✓ Application directory ready at %s", appDir), color.FgGreen)
	return nil
}

func verifyInstallation(ctx context.Context, serverMgr *server.ServerStruct, serverName string, stream io.Writer) error {
	logToStream(stream, "🔍 Verifying installations...", color.FgBlue)

	tools := []struct {
		name    string
		command string
	}{
		{"Docker", "docker --version"},
		{"Caddy", "caddy version"},
	}

	if installAWS {
		tools = append(tools, struct {
			name    string
			command string
		}{"AWS CLI", "aws --version"})
	}

	for _, tool := range tools {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			logToStream(stream, fmt.Sprintf("Checking %s installation...", tool.name), color.FgCyan)

			output, err := serverMgr.ExecuteCommand(ctx, serverName, tool.command, stream)
			if err != nil {
				PrepLogs.Error("%s verification failed: %v", tool.name, err)
				return fmt.Errorf("%s verification failed: %w", tool.name, err)
			}

			if verbose {
				PrepLogs.Debug("%s version: %s", tool.name, strings.TrimSpace(output))
			}
		}
	}

	logToStream(stream, "✓ All tools verified successfully", color.FgGreen)
	return nil
}

func runCommand(ctx context.Context, serverMgr *server.ServerStruct, serverName, command, description string, stream io.Writer) error {
	logToStream(stream, fmt.Sprintf("▶ %s: %s", description, command), color.FgYellow)
	_, err := serverMgr.ExecuteCommand(ctx, serverName, command, stream)
	if err != nil {
		PrepLogs.Error("Failed to %s: %v", strings.ToLower(description), err)
		return fmt.Errorf("failed to %s: %w", strings.ToLower(description), err)
	}
	return nil
}

func runCommands(ctx context.Context, serverMgr *server.ServerStruct, serverName string, commands []string, description string, stream io.Writer) error {
	logToStream(stream, fmt.Sprintf("⚙️ %s...", description), color.FgHiBlue)
	for _, cmd := range commands {
		if err := runCommand(ctx, serverMgr, serverName, cmd, "", stream); err != nil {
			PrepLogs.Error("%s failed: %v", description, err)
			return fmt.Errorf("%s failed: %w", description, err)
		}
	}
	return nil
}

func logToStream(stream io.Writer, message string, colorAttr color.Attribute) {
	if stream != nil {
		c := color.New(colorAttr)
		c.Fprintln(stream, message)
	}
}

// bufferedStreamWriter provides buffered streaming with periodic flushes
type bufferedStreamWriter struct {
	writer io.Writer
	buffer []byte
}

func newBufferedStreamWriter(writer io.Writer) *bufferedStreamWriter {
	return &bufferedStreamWriter{writer: writer}
}

func (b *bufferedStreamWriter) Write(p []byte) (n int, err error) {
	b.buffer = append(b.buffer, p...)
	if len(b.buffer) > 4096 { // Flush every 4KB
		b.Flush()
	}
	return len(p), nil
}

func (b *bufferedStreamWriter) Flush() error {
	if len(b.buffer) > 0 {
		_, err := b.writer.Write(b.buffer)
		b.buffer = b.buffer[:0]
		return err
	}
	return nil
}
