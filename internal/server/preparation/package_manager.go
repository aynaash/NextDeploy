package preparation

import (
	"context"
	"io"
)

type PackageManager interface {
	Install(ctx context.Context, packages []string, stream io.Writer) error
	IsInstalled(ctx context.Context, packageName string) (bool, error)
	Update(ctx context.Context, stream io.Writer) error
}

type PackageManagerType string

const (
	Apt  PackageManagerType = "apt"
	Yum  PackageManagerType = "yum"
	Dnf  PackageManagerType = "dnf"
	Zypp PackageManagerType = "zypp"
)

type ServerManager interface {
	ExecuteCommand(ctx context.Context, serverName, command string, stream io.Writer) (string, error)
	CloseSSHConnection() error
	ListServer() []string
	GetDeploymentServer() (string, error)
	PackageManagerFactory(ctx context.Context, serverName string, stream io.Writer) (PackageManager, error)
	DetectPackageManager(ctx context.Context, serverName string, stream io.Writer) (PackageManagerType, error)
	VerifyPreRequisites(ctx context.Context, serverName string, stream io.Writer) error
	InstallPackages(ctx context.Context, serverName string, stream io.Writer) error
	SetupDirectories(ctx context.Context, serverName string, stream io.Writer) error
	VerifyInstallation(ctx context.Context, serverName string, stream io.Writer) error
	InstallNginx(ctx context.Context, serverName string, stream io.Writer) error
	CloseConnection(tx context.Context, serverName string, stream io.Writer) error
}
