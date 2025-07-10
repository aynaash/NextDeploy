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
	cmd := "if command -v dnf >/dev/null; then sudo dnf upgrade -y; else sudo yum upgrade -y; fi"
	_, err := ym.serverMgr.ExecuteCommand(ctx, ym.serverName, cmd, stream)
	return err
}

func (ym *YumManager) Install(ctx context.Context, packages []string, stream io.Writer) error {
	cmd := fmt.Sprintf(
		"if command -v dnf >/dev/null; then sudo dnf install -y %s; else sudo yum install -y %s; fi",
		strings.Join(packages, " "),
		strings.Join(packages, " "),
	)
	_, err := ym.serverMgr.ExecuteCommand(ctx, ym.serverName, cmd, stream)
	return err
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
