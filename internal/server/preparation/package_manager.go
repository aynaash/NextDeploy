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
