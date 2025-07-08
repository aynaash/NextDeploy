package preparation

import (
	"context"
	"io"
)

type Preparer interface {
	VerifyPreRequisites(ctx context.Context, serverName string, stream io.Writer) error
	InstallPackages(ctx context.Context, serverName string, stream io.Writer) error
	SetupDirectories(ctx context.Context, serverName string, stream io.Writer) error
	VerifyInstallation(ctx context.Context, serverName string, stream io.Writer) error
}
