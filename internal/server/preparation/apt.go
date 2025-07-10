package preparation

import (
	"context"
	"fmt"
	"io"
	"strings"
)

// AptManager implements PackageManager for APT systems
type AptManager struct {
	serverName string
	servermgr  ServerManager
}

func NewAptManager(serverMgr ServerManager, serverName string) *AptManager {
	return &AptManager{
		serverName: serverName,
	}
}

func (am *AptManager) Update(ctx context.Context, stream io.Writer) error {
	_, err := am.servermgr.ExecuteCommand(ctx, am.serverName, "sudo apt-get update -y", stream)
	return err
}

func (am *AptManager) Install(ctx context.Context, packages []string, stream io.Writer) error {
	cmd := fmt.Sprintf("sudo DEBIAN_FRONTEND=noninteractive apt-get install -y %s", strings.Join(packages, " "))
	output, err := am.servermgr.ExecuteCommand(ctx, am.serverName, cmd, stream)
	if err != nil {
		return fmt.Errorf("failed to install packages: %w", err)
	}
	if strings.Contains(output, "No packages found") {
		return fmt.Errorf("no packages found to install: %s", strings.Join(packages, ", "))
	}
	return nil
}

func (am *AptManager) IsInstalled(ctx context.Context, packageName string) (bool, error) {
	cmd := fmt.Sprintf("dpkg -l %s | grep -q ^ii && echo 'installed' || echo 'not installed'", packageName)
	output, err := am.servermgr.ExecuteCommand(ctx, am.serverName, cmd, nil)
	if err != nil {
		return false, fmt.Errorf("failed to check package installation: %w", err)
	}
	return strings.Contains(output, "installed"), nil
}
