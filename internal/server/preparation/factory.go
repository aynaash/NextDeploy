package preparation

import (
	"context"
	"io"

	"github.com/gohugoio/hugo/output"
)

func PackageManagerFactory(
	ctx context.Context,
	serverMgr ServerManager,
	serverName string,
	stream io.Writer,
) (
	PackageManager, error,
) {
	pkgType, err := detectPackageManager(ctx, serverMgr, serverName, stream)
	if err != nil {
		PrepLogs.Error("failed to detect package manager: %v", err)
		return nil, err
	}

	switch pkgType {
	case Apt:
		return NewAptManager(serverMgr.Executor(), stream), nil
	case Yum:
		return NewYumManager(serverMgr.Executor(), stream, Yum), nil
	case Dnf:
		return NewYumManager(serverMgr.Executor(), stream, Dnf), nil
	case Zypp:
		return NewYumManager(serverMgr.Executor(), stream, Zypp), nil
	default:
		PrepLogs.Error("unsupported package manager type: %s", pkgType)
		return nil, err

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
		PrepLogs.Error("failed to execute package manager detection command: %v", err)
		return "", err
	}
	output = output.TrimSpace()
	switch output {
	case "apt":
		return Apt, nil
	case "dnf":
		return Dnf, nil
	case "yum":
		return Yum, nil
	case "zypp":
		return Zypp, nil
	default:
		PrepLogs.Error("unknown package manager detected: %s", output)
		return "", err
	}
}
