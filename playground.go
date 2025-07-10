//go:build ignore
// +build ignore

// internal/server/preparation/manager.go
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
	installNginx   bool // Changed from field to method to avoid name collision
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

func (pm *PreparationManager) installNginx(ctx context.Context, serverName string, stream io.Writer) error {
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

func (pm *PreparationManager) InstallPackages(ctx context.Context, serverName string, stream io.Writer) error {
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

	// Install Nginx if requested - fixed the boolean condition
	if pm.installNginx {
		if err := pm.installNginx(ctx, serverName, stream); err != nil {
			return err
		}
	}

	PrepLogs.Success("All packages installed successfully")
	return nil
}

// ... rest of the methods remain unchanged ...
