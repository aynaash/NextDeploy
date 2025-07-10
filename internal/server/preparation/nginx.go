package preparation

import (
	"context"
	"fmt"
	"io"

	"github.com/fatih/color"
)

type NginxConfigurator struct {
	serverMgr  ServerManager
	serverName string
}

func NewNginxConfigurator(serverMgr ServerManager, serverName string) *NginxConfigurator {
	return &NginxConfigurator{
		serverMgr:  serverMgr,
		serverName: serverName,
	}
}

func (nc *NginxConfigurator) SetupBasicConfig(ctx context.Context, stream io.Writer) error {
	logToStream(stream, "Configuring Nginx...", color.FgYellow)

	cmds := []string{
		// Backup default config
		"sudo cp /etc/nginx/nginx.conf /etc/nginx/nginx.conf.bak",
		// Set worker processes to auto
		`sudo sed -i 's/worker_processes.*/worker_processes auto;/' /etc/nginx/nginx.conf`,
		// Reload Nginx
		"sudo systemctl reload nginx",
	}

	for _, cmd := range cmds {
		_, err := nc.serverMgr.ExecuteCommand(ctx, nc.serverName, cmd, stream)
		if err != nil {
			return fmt.Errorf("nginx configuration failed: %w", err)
		}
	}

	logToStream(stream, "✓ Nginx configured", color.FgGreen)
	return nil
}
