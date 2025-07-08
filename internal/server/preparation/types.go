package preparation

import (
	"context"
	"io"
)

type ServerManager interface {
	ExecuteCommand(ctx context.Context, serverName, command string, stream io.Writer) (string, error)
	CloseConnectionS() error
	ListServer() []string
	GetDeploymentServer() (string, error)
}
