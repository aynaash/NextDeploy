package cmd

import (
	"context"
	"fmt"
	"io"
	"nextdeploy/cli/internal/server"
	"nextdeploy/shared"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

// StreamWriter handles streaming output to different writers with color support
type StreamWriter struct {
	writers []io.Writer
}

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
	prepareCmd.Flags().BoolVar(&verbose, "verbose", false, "enable verbose output")
	prepareCmd.Flags().DurationVar(&timeout, "timeout", 3*time.Minute, "timeout for preparation")
	prepareCmd.Flags().BoolVar(&streamMode, "stream", false, "stream output to stdout")

	rootCmd.AddCommand(prepareCmd)
}

// StreamWriter methods
func NewStreamWriter(writers ...io.Writer) *StreamWriter {
	return &StreamWriter{
		writers: writers,
	}
}

func (s *StreamWriter) Write(p []byte) (n int, err error) {
	for _, w := range s.writers {
		n, err = w.Write(p)
		if err != nil {
			return n, err
		}
	}
	return len(p), nil
}

func (s *StreamWriter) Println(attrs ...color.Attribute) func(w io.Writer, format string, a ...interface{}) {
	c := color.New(attrs...)
	return func(w io.Writer, format string, a ...interface{}) {
		if s.hasWriter(w) {
			c.Fprintf(w, format+"\n", a...)
		}
	}
}

func (s *StreamWriter) Printf(w io.Writer, attrs []color.Attribute, format string, a ...interface{}) {
	if s.hasWriter(w) {
		c := color.New(attrs...)
		c.Fprintf(w, format, a...)
	}
}

func (s *StreamWriter) hasWriter(target io.Writer) bool {
	for _, w := range s.writers {
		if w == target {
			return true
		}
	}
	return false
}

func (s *StreamWriter) AddWriter(w io.Writer) {
	s.writers = append(s.writers, w)
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

	// Initialize stream writer
	stream := NewStreamWriter()
	if streamMode {
		stream.AddWriter(cmd.OutOrStdout())
	}
	if verbose {
		// In verbose mode, also write to stderr
		stream.AddWriter(cmd.ErrOrStderr())
	}

	// Initialize server manager
	serverMgr, err := server.New(
		server.WithConfig(),
		server.WithSSH(),
	)
	if err != nil {
		PrepLogs.Error("Failed to initialize server manager: %v", err)
		return
	}

	// Ensure SSH connections are closed
	defer func() {
		if err := serverMgr.CloseSSHConnection(); err != nil {
			PrepLogs.Error("Failed to close SSH connections: %v", err)
		} else {
			PrepLogs.Debug("SSH connections closed successfully")
		}
	}()

	stream.Printf(cmd.OutOrStdout(), []color.Attribute{color.FgCyan}, "Starting server preparation\n")

	// Determine target server
	serverName, err := selectTargetServer(ctx, serverMgr, cmd, stream)
	if err != nil {
		PrepLogs.Error("Failed to select target server: %v", err)
		return
	}

	// Run preparation
	if err := executePreparation(ctx, serverMgr, serverName, cmd, stream); err != nil {
		PrepLogs.Error("Preparation failed: %v", err)
		return
	}

	PrepLogs.Success("Server %s prepared successfully!", serverName)
}

func selectTargetServer(ctx context.Context, serverMgr *server.ServerStruct, cmd *cobra.Command, stream *StreamWriter) (string, error) {
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
		// Check for deployment server first
		if depServer, err := serverMgr.GetDeploymentServer(); err == nil {
			PrepLogs.Debug("Using deployment server: %s", depServer)
			stream.Printf(cmd.OutOrStdout(), []color.Attribute{color.FgGreen}, "✓ Using deployment server: %s\n", depServer)
			return depServer, nil
		}

		// Fall back to first available server
		servers := serverMgr.ListServers()
		if len(servers) == 0 {
			return "", fmt.Errorf("no servers configured")
		}

		PrepLogs.Warn("No deployment server configured, using first server: %s", servers[0])
		stream.Printf(cmd.OutOrStdout(), []color.Attribute{color.FgYellow}, "⚠ Using first available server: %s\n", servers[0])
		return servers[0], nil
	}
}

func executePreparation(ctx context.Context, serverMgr *server.ServerStruct, serverName string, cmd *cobra.Command, stream *StreamWriter) error {
	PrepLogs.Info("Starting preparation of server %s", serverName)
	stream.Printf(cmd.OutOrStdout(), []color.Attribute{color.FgCyan}, "\n📦 Preparing server: %s\n", serverName)

	// Phase 1: Verify prerequisites
	stream.Printf(cmd.OutOrStdout(), []color.Attribute{color.FgBlue}, "\n🔍 Phase 1: Verifying server prerequisites...\n")
	if err := verifyServerPrerequisites(ctx, serverMgr, serverName, cmd, stream); err != nil {
		return fmt.Errorf("server prerequisites verification failed: %w", err)
	}

	// Phase 2: Install required packages
	stream.Printf(cmd.OutOrStdout(), []color.Attribute{color.FgBlue}, "\n📥 Phase 2: Installing required packages...\n")
	if err := installRequiredPackages(ctx, serverMgr, serverName, cmd, stream); err != nil {
		return fmt.Errorf("failed to install required packages: %w", err)
	}

	// Phase 3: Install daemon control plane
	stream.Printf(cmd.OutOrStdout(), []color.Attribute{color.FgBlue}, "\n🤖 Phase 3: Installing daemon control plane...\n")
	if err := installDaemonControlPlane(ctx, serverMgr, serverName, cmd, stream); err != nil {
		return fmt.Errorf("failed to install daemon control plane: %w", err)
	}

	// Phase 4: Verification
	stream.Printf(cmd.OutOrStdout(), []color.Attribute{color.FgBlue}, "\n✅ Phase 4: Verifying installation...\n")
	if err := verifyInstallation(ctx, serverMgr, serverName, cmd, stream); err != nil {
		return fmt.Errorf("installation verification failed: %w", err)
	}

	stream.Printf(cmd.OutOrStdout(), []color.Attribute{color.FgGreen, color.Bold}, "\n✨ Server %s prepared successfully!\n", serverName)
	return nil
}

func verifyServerPrerequisites(ctx context.Context, serverMgr *server.ServerStruct, serverName string, cmd *cobra.Command, stream *StreamWriter) error {
	checks := []struct {
		command string
		message string
		timeout time.Duration
	}{
		{"uname -a", "System information", 5 * time.Second},
		{"df -h /", "Disk space", 10 * time.Second},
		{"free -m", "Memory", 5 * time.Second},
		{"uptime", "System uptime", 5 * time.Second},
	}

	for _, check := range checks {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			stream.Printf(cmd.OutOrStdout(), []color.Attribute{color.FgYellow}, "  • %s... ", check.message)

			checkCtx, cancel := context.WithTimeout(ctx, check.timeout)
			defer cancel()

			output, err := serverMgr.ExecuteCommand(checkCtx, serverName, check.command, stream)
			if err != nil {
				stream.Printf(cmd.OutOrStdout(), []color.Attribute{color.FgRed}, "❌ failed\n")
				PrepLogs.Error("Failed prerequisite check %q: %v", check.command, err)
				return fmt.Errorf("failed prerequisite check %q: %w", check.command, err)
			}

			stream.Printf(cmd.OutOrStdout(), []color.Attribute{color.FgGreen}, "✓\n")

			if verbose {
				PrepLogs.Debug("Check %q output:\n%s", check.command, output)
				stream.Printf(cmd.ErrOrStderr(), []color.Attribute{color.FgHiBlack}, "     %s\n", strings.Split(strings.TrimSpace(output), "\n")[0])
			}
		}
	}

	return nil
}

func installRequiredPackages(ctx context.Context, serverMgr *server.ServerStruct, serverName string, cmd *cobra.Command, stream *StreamWriter) error {
	stream.Printf(cmd.OutOrStdout(), []color.Attribute{color.FgYellow}, "  • Detecting package manager... ")

	// Determine package manager
	pkgManager, err := serverMgr.ExecuteCommand(ctx, serverName, "command -v apt-get >/dev/null && echo apt || (command -v yum >/dev/null && echo yum || echo unknown)", stream)
	if err != nil {
		stream.Printf(cmd.OutOrStdout(), []color.Attribute{color.FgRed}, "❌ failed\n")
		return fmt.Errorf("failed to determine package manager: %w", err)
	}

	pkgManager = strings.TrimSpace(pkgManager)
	stream.Printf(cmd.OutOrStdout(), []color.Attribute{color.FgGreen}, "✓ (%s)\n", pkgManager)

	// Execute appropriate installation
	switch pkgManager {
	case "apt":
		return installWithApt(ctx, serverMgr, serverName, cmd, stream)
	case "yum":
		return installWithYum(ctx, serverMgr, serverName, cmd, stream)
	default:
		return fmt.Errorf("unsupported package manager: %s", pkgManager)
	}
}

// install the daemon version nextdeployd
func installDaemonControlPlane(ctx context.Context, serverMgr *server.ServerStruct, serverName string, cmd *cobra.Command, stream *StreamWriter) error {
	stream.Printf(cmd.OutOrStdout(), []color.Attribute{color.FgYellow}, "  • Installing daemon control plane...\n")

	daemonUrl := "https://raw.githubusercontent.com/aynaash/NextDeploy/main/scripts/install-daemon.sh"
	installCmd := fmt.Sprintf("curl -sL %s | sudo bash", daemonUrl)

	if _, err := serverMgr.ExecuteCommand(ctx, serverName, installCmd, stream); err != nil {
		stream.Printf(cmd.OutOrStdout(), []color.Attribute{color.FgRed}, "❌ failed\n")
		PrepLogs.Warn("Failed to install daemon control plane: %v", err)
		return nil // Non-fatal for now while developing
	}

	stream.Printf(cmd.OutOrStdout(), []color.Attribute{color.FgGreen}, "  ✓ Daemon installed and running\n")
	return nil
}

func installWithApt(ctx context.Context, serverMgr *server.ServerStruct, serverName string, cmd *cobra.Command, stream *StreamWriter) error {
	stream.Printf(cmd.OutOrStdout(), []color.Attribute{color.FgYellow}, "  • Updating package lists... ")

	// Update package lists
	if _, err := serverMgr.ExecuteCommand(ctx, serverName, "sudo apt update -qq", stream); err != nil {
		stream.Printf(cmd.OutOrStdout(), []color.Attribute{color.FgRed}, "❌ failed\n")
		PrepLogs.Warn("Failed to update package lists: %v", err)
	} else {
		stream.Printf(cmd.OutOrStdout(), []color.Attribute{color.FgGreen}, "✓\n")
	}

	// Install base packages
	stream.Printf(cmd.OutOrStdout(), []color.Attribute{color.FgYellow}, "  • Installing base packages... ")
	basePkgs := []string{"curl", "git", "make", "gcc", "build-essential"}
	installCmd := fmt.Sprintf("sudo DEBIAN_FRONTEND=noninteractive apt install -y %s", strings.Join(basePkgs, " "))

	if _, err := serverMgr.ExecuteCommand(ctx, serverName, installCmd, stream); err != nil {
		stream.Printf(cmd.OutOrStdout(), []color.Attribute{color.FgRed}, "❌ failed\n")
		return fmt.Errorf("failed to install base packages: %w", err)
	}
	stream.Printf(cmd.OutOrStdout(), []color.Attribute{color.FgGreen}, "✓\n")

	// Install Node.js
	stream.Printf(cmd.OutOrStdout(), []color.Attribute{color.FgYellow}, "  • Installing Node.js 20... ")
	nodeCmds := []string{
		"curl -fsSL https://deb.nodesource.com/setup_20.x | sudo -E bash -",
		"sudo DEBIAN_FRONTEND=noninteractive apt-get install -y nodejs",
	}
	for _, cmdStr := range nodeCmds {
		if _, err := serverMgr.ExecuteCommand(ctx, serverName, cmdStr, stream); err != nil {
			return fmt.Errorf("Node.js installation failed at step %q: %w", cmdStr, err)
		}
	}
	stream.Printf(cmd.OutOrStdout(), []color.Attribute{color.FgGreen}, "✓\n")

	// Install Doppler CLI
	stream.Printf(cmd.OutOrStdout(), []color.Attribute{color.FgYellow}, "  • Installing Doppler CLI... ")
	dopplerCmd := "(curl -Ls --tlsv1.2 --proto \"=https\" --retry 3 https://cli.doppler.com/install.sh || wget -t 3 -qO- https://cli.doppler.com/install.sh) | sudo sh"
	if _, err := serverMgr.ExecuteCommand(ctx, serverName, dopplerCmd, stream); err != nil {
		return fmt.Errorf("Doppler installation failed: %w", err)
	}
	stream.Printf(cmd.OutOrStdout(), []color.Attribute{color.FgGreen}, "✓\n")

	// Install Caddy if missing
	stream.Printf(cmd.OutOrStdout(), []color.Attribute{color.FgYellow}, "  • Checking Caddy... ")
	if !isInstalled(ctx, serverMgr, serverName, "caddy", stream) {
		stream.Printf(cmd.OutOrStdout(), []color.Attribute{color.FgYellow}, "not found, installing...\n")

		caddyCmds := []string{
			"sudo apt install -y debian-keyring debian-archive-keyring apt-transport-https",
			"curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' | sudo gpg --batch --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg",
			"curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' | sudo tee /etc/apt/sources.list.d/caddy-stable.list",
			"sudo apt update",
			"sudo DEBIAN_FRONTEND=noninteractive apt install -y caddy",
			"sudo systemctl enable caddy",
			"sudo systemctl start caddy",
		}

		for _, cmdStr := range caddyCmds {
			if _, err := serverMgr.ExecuteCommand(ctx, serverName, cmdStr, stream); err != nil {
				return fmt.Errorf("caddy installation failed at step %q: %w", cmdStr, err)
			}
		}
		stream.Printf(cmd.OutOrStdout(), []color.Attribute{color.FgGreen}, "  ✓ Caddy installed\n")
	} else {
		stream.Printf(cmd.OutOrStdout(), []color.Attribute{color.FgGreen}, "✓ already installed\n")
	}

	return nil
}

func installWithYum(ctx context.Context, serverMgr *server.ServerStruct, serverName string, cmd *cobra.Command, stream *StreamWriter) error {
	stream.Printf(cmd.OutOrStdout(), []color.Attribute{color.FgYellow}, "  • Installing base packages... ")

	// Base packages for RHEL/CentOS/Amazon Linux
	basePkgs := []string{"curl", "git", "make", "gcc", "glibc-static", "ca-certificates", "yum-utils"}
	installCmd := fmt.Sprintf("sudo yum install -y %s", strings.Join(basePkgs, " "))

	if _, err := serverMgr.ExecuteCommand(ctx, serverName, installCmd, stream); err != nil {
		stream.Printf(cmd.OutOrStdout(), []color.Attribute{color.FgRed}, "❌ failed\n")
		return fmt.Errorf("failed to install base packages: %w", err)
	}
	stream.Printf(cmd.OutOrStdout(), []color.Attribute{color.FgGreen}, "✓\n")

	// Install Node.js
	stream.Printf(cmd.OutOrStdout(), []color.Attribute{color.FgYellow}, "  • Installing Node.js 20... ")
	nodeYumCmds := []string{
		"curl -fsSL https://rpm.nodesource.com/setup_20.x | sudo bash -",
		"sudo yum install -y nodejs",
	}
	for _, cmdStr := range nodeYumCmds {
		if _, err := serverMgr.ExecuteCommand(ctx, serverName, cmdStr, stream); err != nil {
			return fmt.Errorf("Node.js installation failed at step %q: %w", cmdStr, err)
		}
	}
	stream.Printf(cmd.OutOrStdout(), []color.Attribute{color.FgGreen}, "✓\n")

	// Install Doppler CLI
	stream.Printf(cmd.OutOrStdout(), []color.Attribute{color.FgYellow}, "  • Installing Doppler CLI... ")
	dopplerCmd := "(curl -Ls --tlsv1.2 --proto \"=https\" --retry 3 https://cli.doppler.com/install.sh || wget -t 3 -qO- https://cli.doppler.com/install.sh) | sudo sh"
	if _, err := serverMgr.ExecuteCommand(ctx, serverName, dopplerCmd, stream); err != nil {
		return fmt.Errorf("Doppler installation failed: %w", err)
	}
	stream.Printf(cmd.OutOrStdout(), []color.Attribute{color.FgGreen}, "✓\n")

	// Install Caddy if missing
	stream.Printf(cmd.OutOrStdout(), []color.Attribute{color.FgYellow}, "  • Checking Caddy... ")
	if !isInstalled(ctx, serverMgr, serverName, "caddy", stream) {
		stream.Printf(cmd.OutOrStdout(), []color.Attribute{color.FgYellow}, "not found, installing...\n")

		caddyCmds := []string{
			"sudo yum install -y yum-plugin-copr",
			"sudo yum copr enable -y @caddy/caddy",
			"sudo yum install -y caddy",
			"sudo systemctl enable caddy",
			"sudo systemctl start caddy",
		}

		for _, cmdStr := range caddyCmds {
			if _, err := serverMgr.ExecuteCommand(ctx, serverName, cmdStr, stream); err != nil {
				return fmt.Errorf("caddy installation failed at step %q: %w", cmdStr, err)
			}
		}
		stream.Printf(cmd.OutOrStdout(), []color.Attribute{color.FgGreen}, "  ✓ Caddy installed\n")
	} else {
		stream.Printf(cmd.OutOrStdout(), []color.Attribute{color.FgGreen}, "✓ already installed\n")
	}

	return nil
}

func verifyInstallation(ctx context.Context, serverMgr *server.ServerStruct, serverName string, cmd *cobra.Command, stream *StreamWriter) error {
	checks := []struct {
		command string
		tool    string
	}{
		{"caddy version | head -1", "Caddy"},
		{"node --version", "Node.js"},
		{"doppler --version", "Doppler CLI"},
	}

	allGood := true
	for _, check := range checks {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			stream.Printf(cmd.OutOrStdout(), []color.Attribute{color.FgYellow}, "  • Verifying %s... ", check.tool)

			output, err := serverMgr.ExecuteCommand(ctx, serverName, check.command, stream)
			if err != nil {
				stream.Printf(cmd.OutOrStdout(), []color.Attribute{color.FgRed}, "❌ not found\n")
				PrepLogs.Error("Failed to verify %s installation: %v", check.tool, err)
				allGood = false
				continue
			}

			stream.Printf(cmd.OutOrStdout(), []color.Attribute{color.FgGreen}, "✓\n")
			if verbose {
				stream.Printf(cmd.ErrOrStderr(), []color.Attribute{color.FgHiBlack}, "     %s\n", strings.TrimSpace(output))
			}
		}
	}

	if !allGood {
		return fmt.Errorf("some installations failed verification")
	}

	return nil
}

// Helper functions
func isInstalled(ctx context.Context, serverMgr *server.ServerStruct, serverName, command string, stream *StreamWriter) bool {
	checkCmd := fmt.Sprintf("command -v %s >/dev/null 2>&1 && echo 'installed' || echo 'missing'", command)
	output, err := serverMgr.ExecuteCommand(ctx, serverName, checkCmd, stream)
	if err != nil {
		return false
	}
	return strings.Contains(output, "installed")
}

// CreateAppDirectory creates the minimal directory structure for containerized Next.js app
func CreateAppDirectory(ctx context.Context, serverMgr *server.ServerStruct, serverName string, cmd *cobra.Command, stream *StreamWriter) error {
	const appDir = "/opt/nextjs-app"

	stream.Printf(cmd.OutOrStdout(), []color.Attribute{color.FgYellow}, "  • Creating application directory... ")

	createCmd := fmt.Sprintf(
		"sudo mkdir -p %s && sudo chown -R $(whoami): %s && sudo chmod 755 %s",
		appDir, appDir, appDir,
	)

	if _, err := serverMgr.ExecuteCommand(ctx, serverName, createCmd, stream); err != nil {
		stream.Printf(cmd.OutOrStdout(), []color.Attribute{color.FgRed}, "❌ failed\n")
		return fmt.Errorf("failed to create application directory: %w", err)
	}

	// Verify directory was created
	verifyCmd := fmt.Sprintf("test -d %s && echo 'exists' || echo 'missing'", appDir)
	output, err := serverMgr.ExecuteCommand(ctx, serverName, verifyCmd, stream)
	if err != nil || !strings.Contains(output, "exists") {
		stream.Printf(cmd.OutOrStdout(), []color.Attribute{color.FgRed}, "❌ verification failed\n")
		return fmt.Errorf("directory verification failed: %w", err)
	}

	stream.Printf(cmd.OutOrStdout(), []color.Attribute{color.FgGreen}, "✓ (%s)\n", appDir)
	PrepLogs.Success("Application directory ready at %s", appDir)
	return nil
}
