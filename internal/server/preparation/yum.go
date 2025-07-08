package preparation

import (
	"context"
	"fmt"
	"io"
	"strings"
)

// YumManager implements PackageManager for YUM/DNF systems
type YumManager struct {
	serverMgr  ServerManager
	serverName string
}

func NewYumManager(serverMgr ServerManager, serverName string) *YumManager {
	return &YumManager{
		serverMgr:  serverMgr,
		serverName: serverName,
	}
}

func (ym *YumManager) Update(ctx context.Context, stream io.Writer) error {
	// Try DNF first (newer systems), fall back to YUM
	cmd := "if command -v dnf >/dev/null; then sudo dnf upgrade -y; else sudo yum upgrade -y; fi"
	output, err := ym.serverMgr.ExecuteCommand(ctx, ym.serverName, cmd, stream)
	if err != nil {
		return fmt.Errorf("failed to update packages: %w", err)
	}
	if strings.Contains(output, "No packages marked for update") {
		output = "No packages to update"
		return nil
	}

	return nil
}

func (ym *YumManager) Install(ctx context.Context, packages []string, stream io.Writer) error {
	// Use DNF if available, otherwise YUM
	cmd := fmt.Sprintf(
		"if command -v dnf >/dev/null; then sudo dnf install -y %s; else sudo yum install -y %s; fi",
		strings.Join(packages, " "),
		strings.Join(packages, " "),
	)
	output, err := ym.serverMgr.ExecuteCommand(ctx, ym.serverName, cmd, stream)
	if err != nil {
		return fmt.Errorf("failed to install packages: %w", err)
	}
	if strings.Contains(output, "No match for argument") {
		return fmt.Errorf("no packages found to install: %s", strings.Join(packages, ", "))
	}
	return nil
}

func (ym *YumManager) IsInstalled(ctx context.Context, packageName string) (bool, error) {
	cmd := fmt.Sprintf(
		"if command -v dnf >/dev/null; then "+
			"dnf list installed %[1]s >/dev/null 2>&1 && echo 'installed' || echo 'not installed'; "+
			"else "+
			"yum list installed %[1]s >/dev/null 2>&1 && echo 'installed' || echo 'not installed'; "+
			"fi",
		packageName,
	)
	output, err := ym.serverMgr.ExecuteCommand(ctx, ym.serverName, cmd, nil)
	if err != nil {
		return false, fmt.Errorf("failed to check package installation: %w", err)
	}
	return strings.Contains(output, "installed"), nil
}
