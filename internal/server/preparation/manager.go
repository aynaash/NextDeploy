package preparation

import (
	"context"
	"fmt"
	"io"
	"strings"

	"nextdeploy/internal/logger"
)

var (
	PrepLogs = logger.PackageLogger("preparation", "🔧")
)

type PreparationManager struct {
	serverMgr      ServerManager
	packageManager PackageManager
	installAWS     bool
	forceInstall   bool
	installNginx   bool
	verbose        bool
}

func NewPreparationManager(serverMgr ServerManager, pm PackageManager, installAWS, forceInstall, installNginx, verbose bool) *PreparationManager {
	return &PreparationManager{
		serverMgr:      serverMgr,
		packageManager: pm,
		installAWS:     installAWS,
		forceInstall:   forceInstall,
		installNginx:   installNginx,
		verbose:        verbose,
	}
}

func (pm *PreparationManager) VerifyPrerequisites(ctx context.Context, serverName string, stream io.Writer) error {
	PrepLogs.Info("Verifying prerequisites for server %s", serverName)

	checks := []struct {
		command string
		message string
	}{
		{"uname -a", "System information"},
		{"df -h --output=size,used,avail,pcent", "Disk space"},
		{"free -h", "Memory usage"},
		{"lscpu", "CPU information"},
		{"nproc", "Processor count"},
	}

	for _, check := range checks {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			PrepLogs.Debug("Checking %s...", check.message)
			output, err := pm.serverMgr.ExecuteCommand(ctx, serverName, check.command, stream)
			if err != nil {
				PrepLogs.Error("Prerequisite check failed for %s: %v", check.message, err)
				return fmt.Errorf("%s check failed: %w", check.message, err)
			}

			if pm.verbose {
				PrepLogs.Debug("%s output:\n%s", check.message, strings.TrimSpace(output))
			}
		}
	}

	PrepLogs.Success("All prerequisites verified")
	return nil
}

func (pm *PreparationManager) InstallPackages(ctx context.Context, serverName string, stream io.Writer) error {
	PrepLogs.Info("Starting package installation on server %s", serverName)

	// Update package lists first
	if err := pm.packageManager.Update(ctx, stream); err != nil {
		PrepLogs.Error("Failed to update package lists: %v", err)
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

	// Install Nginx if requested
	if pm.installNginx {
		if err := pm.InstallNginx(ctx, serverName, stream); err != nil {
			return err
		}
	}

	PrepLogs.Success("All packages installed successfully")
	return nil
}

func (pm *PreparationManager) installBasePackages(ctx context.Context, serverName string, packages []string, stream io.Writer) error {
	PrepLogs.Info("Installing base packages: %v", packages)
	if err := pm.packageManager.Install(ctx, packages, stream); err != nil {
		PrepLogs.Error("Base package installation failed: %v", err)
		return fmt.Errorf("base package installation failed: %w", err)
	}
	PrepLogs.Success("Base packages installed")
	return nil
}

func (pm *PreparationManager) installDocker(ctx context.Context, serverName string, stream io.Writer) error {
	PrepLogs.Info("Checking Docker installation...")

	installed, err := pm.packageManager.IsInstalled(ctx, "docker")
	if err != nil {
		PrepLogs.Error("Failed to check Docker installation: %v", err)
		return fmt.Errorf("docker check failed: %w", err)
	}

	if !installed || pm.forceInstall {
		PrepLogs.Info("Installing Docker...")
		cmds := []string{
			"curl -fsSL https://get.docker.com | sudo sh",
			"sudo usermod -aG docker $USER",
			"sudo systemctl enable docker",
			"sudo systemctl start docker",
		}

		for _, cmd := range cmds {
			if _, err := pm.serverMgr.ExecuteCommand(ctx, serverName, cmd, stream); err != nil {
				PrepLogs.Error("Docker installation failed: %v", err)
				return fmt.Errorf("docker installation failed: %w", err)
			}
		}
		PrepLogs.Success("Docker installed")
	} else {
		PrepLogs.Info("Docker already installed")
	}
	return nil
}

func (pm *PreparationManager) installCaddy(ctx context.Context, serverName string, stream io.Writer) error {
	PrepLogs.Info("Checking Caddy installation...")
	installed, err := pm.packageManager.IsInstalled(ctx, "caddy")
	if err != nil {
		PrepLogs.Error("Failed to check Caddy installation: %v", err)
		return fmt.Errorf("caddy check failed: %w", err)
	}

	if !installed || pm.forceInstall {
		PrepLogs.Info("Installing Caddy...")
		var cmds []string

		// Determine package manager type
		switch pm.packageManager.(type) {
		case *AptManager:
			cmds = []string{
				"sudo apt-get install -y debian-keyring debian-archive-keyring apt-transport-https",
				"curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' | sudo gpg --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg",
				"curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' | sudo tee /etc/apt/sources.list.d/caddy-stable.list",
				"sudo apt-get update",
				"sudo apt-get install -y caddy",
				"sudo systemctl enable caddy",
				"sudo systemctl start caddy",
			}
		default: // Assume yum/dnf
			cmds = []string{
				"sudo yum install -y yum-plugin-copr",
				"sudo yum copr enable -y @caddy/caddy",
				"sudo yum install -y caddy",
				"sudo systemctl enable caddy",
				"sudo systemctl start caddy",
			}
		}

		for _, cmd := range cmds {
			if _, err := pm.serverMgr.ExecuteCommand(ctx, serverName, cmd, stream); err != nil {
				PrepLogs.Error("Caddy installation failed: %v", err)
				return fmt.Errorf("caddy installation failed: %w", err)
			}
		}
		PrepLogs.Success("Caddy installed")
	} else {
		PrepLogs.Info("Caddy already installed")
	}
	return nil
}

func (pm *PreparationManager) installAWScli(ctx context.Context, serverName string, stream io.Writer) error {
	PrepLogs.Info("Checking AWS CLI installation...")

	// Check both possible locations
	checkCmd := "[ -x /usr/local/bin/aws ] || [ -x /usr/bin/aws ]"
	if _, err := pm.serverMgr.ExecuteCommand(ctx, serverName, checkCmd, stream); err == nil && !pm.forceInstall {
		PrepLogs.Success("AWS CLI already installed")
		return nil
	}

	PrepLogs.Info("Installing AWS CLI...")

	// First try official Ubuntu package (simpler)
	// TODO: we would need to add the actual intsll logic
	if err := pm.packageManager.Install(ctx, []string{"awscli"}, stream); err == nil {
		PrepLogs.Success("Installed via apt")
		return nil
	}

	PrepLogs.Info("Falling back to manual installation...")
	// we need to check if unzip and curl are installed
	unzipInstalled, err := pm.packageManager.IsInstalled(ctx, "unzip")
	if err != nil {
		PrepLogs.Error("Failed to check unzip installation: %v", err)
		return fmt.Errorf("unzip check failed: %w", err)
	}
	if !unzipInstalled {
		PrepLogs.Info("Installing unzip...")
		if err := pm.packageManager.Install(ctx, []string{"unzip"}, stream); err != nil {
			PrepLogs.Error("Unzip installation failed: %v", err)
			return fmt.Errorf("unzip installation failed: %w", err)
		}
	}
	curlInstalled, err := pm.packageManager.IsInstalled(ctx, "curl")
	if err != nil {
		PrepLogs.Error("Failed to check curl installation: %v", err)
		return fmt.Errorf("curl check failed: %w", err)
	}
	if !curlInstalled {
		PrepLogs.Info("Installing curl...")
		if err := pm.packageManager.Install(ctx, []string{"curl"}, stream); err != nil {
			PrepLogs.Error("Curl installation failed: %v", err)
			return fmt.Errorf("curl installation failed: %w", err)
		}
	}

	cmds := []string{
		"sudo rm -rf /tmp/awscli*", // Cleanup
		"cd /tmp",
		"curl -L https://awscli.amazonaws.com/awscli-exe-linux-x86_64.zip -o awscliv2.zip",
		"unzip awscliv2.zip",
		"sudo ./aws/install --bin-dir /usr/local/bin --install-dir /usr/local/aws-cli --update",
		"sudo ln -sf /usr/local/bin/aws /usr/bin/aws", // Ensure system-wide access
		"rm -rf awscliv2.zip aws",
		"aws --version", // Verify
	}

	for _, cmd := range cmds {
		output, err := pm.serverMgr.ExecuteCommand(ctx, serverName, cmd, stream)
		if err != nil {
			PrepLogs.Error("AWS CLI installation failed at command '%s': %v\nOutput: %s", cmd, err, output)
			return fmt.Errorf("aws cli installation failed at '%s': %w", cmd, err)
		}
	}

	PrepLogs.Success("AWS CLI installed successfully")
	return nil
}
func (pm *PreparationManager) InstallNginx(ctx context.Context, serverName string, stream io.Writer) error {
	PrepLogs.Info("Checking Nginx installation...")
	installed, err := pm.packageManager.IsInstalled(ctx, "nginx")
	if err != nil {
		PrepLogs.Error("Failed to check Nginx installation: %v", err)
		return fmt.Errorf("nginx check failed: %w", err)
	}

	if !installed || pm.forceInstall {
		PrepLogs.Info("Installing Nginx...")
		if err := pm.packageManager.Install(ctx, []string{"nginx"}, stream); err != nil {
			PrepLogs.Error("Nginx installation failed: %v", err)
			return fmt.Errorf("nginx installation failed: %w", err)
		}

		cmds := []string{
			"sudo systemctl enable nginx",
			"sudo systemctl start nginx",
		}

		for _, cmd := range cmds {
			if _, err := pm.serverMgr.ExecuteCommand(ctx, serverName, cmd, stream); err != nil {
				PrepLogs.Error("Nginx service setup failed: %v", err)
				return fmt.Errorf("nginx service setup failed: %w", err)
			}
		}
		PrepLogs.Success("Nginx installed and started")
	} else {
		PrepLogs.Info("Nginx already installed")
	}
	return nil
}

func (pm *PreparationManager) SetupDirectories(ctx context.Context, serverName string, stream io.Writer) error {
	const appDir = "/opt/nextjs-app"
	PrepLogs.Info("Setting up application directory at %s", appDir)

	cmds := []string{
		fmt.Sprintf("sudo mkdir -p %s", appDir),
		fmt.Sprintf("sudo chown -R $(whoami): %s", appDir),
		fmt.Sprintf("sudo chmod 755 %s", appDir),
	}

	for _, cmd := range cmds {
		if _, err := pm.serverMgr.ExecuteCommand(ctx, serverName, cmd, stream); err != nil {
			PrepLogs.Error("Directory setup failed: %v", err)
			return fmt.Errorf("directory setup failed: %w", err)
		}
	}

	PrepLogs.Success("Application directory setup complete")
	return nil
}

func (pm *PreparationManager) VerifyInstallation(ctx context.Context, serverName string, stream io.Writer) error {
	PrepLogs.Info("Verifying installations...")

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

	if pm.installNginx {
		tools = append(tools, struct {
			name    string
			command string
		}{"Nginx", "nginx -v"})
	}

	for _, tool := range tools {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			PrepLogs.Debug("Verifying %s installation...", tool.name)
			output, err := pm.serverMgr.ExecuteCommand(ctx, serverName, tool.command, stream)
			if err != nil {
				PrepLogs.Error("%s verification failed: %v", tool.name, err)
				return fmt.Errorf("%s verification failed: %w", tool.name, err)
			}

			if pm.verbose {
				PrepLogs.Info("%s version: %s", tool.name, strings.TrimSpace(output))
			}
		}
	}

	PrepLogs.Success("All installations verified")
	return nil
}
