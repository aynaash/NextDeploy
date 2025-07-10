// internal/server/preparation/factory.go
package preparation

import (
	"context"
	"fmt"
	"io"
	"strings"
)

// Then wrap your server struct when passing to functions:
// PackageManagerFactory creates the appropriate package manager
func PackageManagerFactory(
	ctx context.Context,
	serverMgr ServerManager,
	serverName string,
	stream io.Writer,
) (PackageManager, error) {
	// Detect package manager
	pkgType, err := detectPackageManager(ctx, serverMgr, serverName, stream)
	//FIX: The package manager is not returned correctly
	PrepLogs.Debug("Detected package manager:%s", pkgType)
	if err != nil {
		PrepLogs.Error("Error detecting package manager:", err)
		return nil, err
	}

	switch pkgType {
	case Apt:
		return NewAptManager(serverMgr, serverName), nil
	case Yum, Dnf:
		return NewYumManager(serverMgr, serverName), nil
	default:
		return nil, fmt.Errorf("unsupported package manager: %s", pkgType)
	}
}

func detectPackageManager(ctx context.Context, serverMgr ServerManager, serverName string, stream io.Writer) (PackageManagerType, error) {
	detectCmd := `
		if command -v apt-get >/dev/null 2>&1; then echo "apt";
		elif command -v dnf >/dev/null 2>&1; then echo "dnf";
		elif command -v yum >/dev/null 2>&1; then echo "yum";
		else echo "unknown"; fi
	`

	output, err := serverMgr.ExecuteCommand(ctx, serverName, detectCmd, stream)
	if err != nil {
		PrepLogs.Error("Failed to execute command to detect package manager:", err)
		return "", fmt.Errorf("failed to detect package manager: %w", err)
	}

	return PackageManagerType(strings.TrimSpace(output)), nil
}
