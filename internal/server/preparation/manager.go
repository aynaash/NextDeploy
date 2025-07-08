package preparation

import (
	"context"
	"io"
	"nextdeploy/internal/logger"
	"fmt"
	"strings"
)

var (
	PrepLogs = logger.PackageLogger("preparation", "PreparationLogs")
)

type PreparationManager struct {
	serverMgr      ServerManager
	packageManager PackageManager
	InstallAWS     bool
	forceInstall   bool
	verbose        bool
}

func NewPreparationManager(serverMgr ServerManager, pm PackageManager, installAWS, forceInstall, verbose bool) *PreparationManager {
	return &PreparationManager{
		serverMgr:      serverMgr,
		packageManager: pm,
		InstallAWS:     installAWS,
		forceInstall:   forceInstall,
		verbose:        verbose,
	}
}

func (pm *PreparationManager) VerifyPrerequisites(ctx context.Context, serverName string, stream io.Write) error {

	PrepLogs.Info("Verifying prerequisites for server")
	checks := []struct {
		command string
		message string
	}{
		{"uname -a", "Checking system information"},
		{"df -h --output=size, used, avail, pcent", "Checking disk space"},
		{"free -h", "Checking memory usage"},
		{"lscpu", "Checking CPU information"},
		{"cat /etc/os-release", "Checking OS release information"},
		{"cat /proc/cpuinfo", "Checking CPU details"},
		{"cat /proc/meminfo", "Checking memory details"},
		{"cat /proc/version", "Checking kernel version"},
		{"ip a", "Checking network interfaces"},
		{"hostname", "Checking hostname"},
		{"nproc", "Checking number of processors"},
		{"uptime", "Checking system uptime"},
	}

	for _, check := range checks {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			PrepLogs.Info("Checking %s:", check.message)
			output, err := pm.serverMgr.ExecuteCommand(ctx, serverName, check.command, stream)
			if err != nil {
				PrepLogs.Error("Failed to execute command '%s': %v", check.command, err)
				return err
			}
			if pm.verbose {
				PrepLogs.Info("Output of '%s':\n%s", check.command, strings.TrimSpace(output))
			} else {
				PrepLogs.Info("Command '%s' executed successfully", check.command)
			}

		}
	}
	PrepLogs.Success("All prerequisite checks completed successfully")

	return nil
}

// internal/server/preparation/manager.go (additional methods)

func (pm *PreparationManager) InstallPackages(ctx context.Context, serverName string, stream io.Writer) error {
	// Update package lists first
	if err := pm.pkgManager.Update(ctx, stream); err != nil {
		logToStream(stream, "Failed to update package lists", color.FgRed)
		return fmt.Errorf("package update failed: %w", err)
	}

	// Install base packages
	basePkgs := []string{
		"curl", "git", "make", "gcc",
		"ca-certificates", "software-properties-common",
	}

	if err := pm.installBasePackages(ctx, serverName, basePkgs, stream); err != nil {
		return err
	}

	// Install Docker
	if err := pm.installDocker(ctx, serverName, stream); err != nil {
		return err
	}

	// Install Caddy
	if err := pm.installCaddy(ctx, serverName, stream); err != nil {
		return err
	}

	// Install AWS CLI if requested
	if pm.installAWS {
		if err := pm.installAWScli(ctx, serverName, stream); err != nil {
			return err
		}
	}

	return nil
}

func (pm *PreparationManager) installBasePackages(ctx context.Context, serverName string, packages []string, stream io.Writer) error {
	logToStream(stream, "Installing base packages...", color.FgYellow)
	if err := pm.pkgManager.Install(ctx, packages, stream); err != nil {
		logToStream(stream, "Failed to install base packages", color.FgRed)
		return fmt.Errorf("base package installation failed: %w", err)
	}
	logToStream(stream, "✓ Base packages installed", color.FgGreen)
	return nil
}

func (pm *PreparationManager) installDocker(ctx context.Context, serverName string, stream io.Writer) error {
	logToStream(stream, "Checking Docker installation...", color.FgCyan)

	installed, err := pm.pkgManager.IsInstalled(ctx, "docker")
	if err != nil {
		return fmt.Errorf("failed to check Docker installation: %w", err)
	}

	if !installed || pm.forceInstall {
		logToStream(stream, "Installing Docker...", color.FgYellow)
		cmds := []string{
			"curl -fsSL https://get.docker.com | sudo sh",
			"sudo usermod -aG docker $USER",
			"sudo systemctl enable docker",
			"sudo systemctl start docker",
		}

		for _, cmd := range cmds {
			if err := pm.serverMgr.ExecuteCommand(ctx, serverName, cmd, stream); err != nil {
				return fmt.Errorf("docker installation failed: %w", err)
			}
		}
		logToStream(stream, "✓ Docker installed", color.FgGreen)
	} else {
		logToStream(stream, "✓ Docker already installed", color.FgGreen)
	}
	return nil
}

func (pm *PreparationManager) installCaddy(ctx context.Context, serverName string, stream io.Writer) error {
	logToStream(stream, "Checking Caddy installation...", color.FgCyan)

	installed, err := pm.pkgManager.IsInstalled(ctx, "caddy")
	if err != nil {
		return fmt.Errorf("failed to check Caddy installation: %w", err)
	}

	if !installed || pm.forceInstall {
		logToStream(stream, "Installing Caddy...", color.FgYellow)

		var cmds []string
		if _, ok := pm.pkgManager.(*AptManager); ok {
			cmds = []string{
				"sudo apt-get install -y debian-keyring debian-archive-keyring apt-transport-https",
				"curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' | sudo gpg --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg",
				"curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' | sudo tee /etc/apt/sources.list.d/caddy-stable.list",
				"sudo apt-get update",
				"sudo apt-get install -y caddy",
				"sudo systemctl enable caddy",
				"sudo systemctl start caddy",
			}
		} else {
			cmds = []string{
				"sudo yum install -y yum-plugin-copr",
				"sudo yum copr enable -y @caddy/caddy",
				"sudo yum install -y caddy",
				"sudo systemctl enable caddy",
				"sudo systemctl start caddy",
			}
		}

		for _, cmd := range cmds {
			if err := pm.serverMgr.ExecuteCommand(ctx, serverName, cmd, stream); err != nil {
				return fmt.Errorf("caddy installation failed: %w", err)
			}
		}
		logToStream(stream, "✓ Caddy installed", color.FgGreen)
	} else {
		logToStream(stream, "✓ Caddy already installed", color.FgGreen)
	}
	return nil
}

func (pm *PreparationManager) installAWScli(ctx context.Context, serverName string, stream io.Writer) error {
	logToStream(stream, "Checking AWS CLI installation...", color.FgCyan)

	installed, err := pm.pkgManager.IsInstalled(ctx, "aws")
	if err != nil {
		return fmt.Errorf("failed to check AWS CLI installation: %w", err)
	}

	if !installed || pm.forceInstall {
		logToStream(stream, "Installing AWS CLI...", color.FgYellow)
		cmds := []string{
			"curl -sL https://awscli.amazonaws.com/awscli-exe-linux-x86_64.zip -o awscliv2.zip",
			"unzip -q awscliv2.zip",
			"sudo ./aws/install --bin-dir /usr/local/bin --install-dir /usr/local/aws-cli --update",
			"rm -rf awscliv2.zip aws",
		}

		for _, cmd := range cmds {
			if err := pm.serverMgr.ExecuteCommand(ctx, serverName, cmd, stream); err != nil {
				return fmt.Errorf("aws cli installation failed: %w", err)
			}
		}
		logToStream(stream, "✓ AWS CLI installed", color.FgGreen)
	} else {
		logToStream(stream, "✓ AWS CLI already installed", color.FgGreen)
	}
	return nil
}

func (pm *PreparationManager) SetupDirectories(ctx context.Context, serverName string, stream io.Writer) error {
	const appDir = "/opt/nextjs-app"
	logToStream(stream, fmt.Sprintf("Creating application directory at %s", appDir), color.FgBlue)

	cmds := []string{
		fmt.Sprintf("sudo mkdir -p %s", appDir),
		fmt.Sprintf("sudo chown -R $(whoami): %s", appDir),
		fmt.Sprintf("sudo chmod 755 %s", appDir),
	}

	for _, cmd := range cmds {
		if err := pm.serverMgr.ExecuteCommand(ctx, serverName, cmd, stream); err != nil {
			return fmt.Errorf("failed to create application directory: %w", err)
		}
	}

	logToStream(stream, fmt.Sprintf("✓ Application directory ready at %s", appDir), color.FgGreen)
	return nil
}

func (pm *PreparationManager) VerifyInstallation(ctx context.Context, serverName string, stream io.Writer) error {
	logToStream(stream, "🔍 Verifying installations...", color.FgBlue)

	tools := []struct {
		name    string
		command string
	}{
		{"Docker", "docker --version"},
		{"Caddy", "caddy version"},
	}

	if pm.installAWS {
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

			output, err := pm.serverMgr.ExecuteCommand(ctx, serverName, tool.command, stream)
			if err != nil {
				logToStream(stream, fmt.Sprintf("%s verification failed", tool.name), color.FgRed)
				return fmt.Errorf("%s verification failed: %w", tool.name, err)
			}

			if pm.verbose {
				logToStream(stream, fmt.Sprintf("%s version: %s", tool.name, strings.TrimSpace(output)), color.FgCyan)
			}
		}
	}

	logToStream(stream, "✓ All tools verified successfully", color.FgGreen)
	return nil
}
