package nextcore

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
)

type nextruntime struct {
	dockerclient *client.client
	payload      *nextcorepayload
}

func NewNextRuntime(payload *NextCorePayload) (*nextruntime, error) {
	cli, err := client.NewClientWithOpts(
		client.FromEnv,
		client.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %w", err)
	}

	return &nextruntime{
		dockerclient: cli,
		payload:      payload,
	}
}

func (nr *nextruntime) CreateContainer(ctx context.Context) (string, error) {
	// configure container based on metadata
	containerConfig := nr.createContainerConfig()
	hostConfig := nr.createHostConfig()
	neworkingConfig := nr.createNetworkingConfig()
	// create the container
	resp, err := nr.dockerclient.ContainerCreate(
		ctx,
		containerConfig,
		hostConfig,
		neworkingConfig,
		nil,
		nr.getContainerName(),
	)

	if err != nil {
		return "", fmt.Errorf("failed to create container:%w", err)
	}
	// start the container
	if err := nr.dockerclient.ContainerStart(
		ctx,
		resp.ID,
		container.StartOptions{},
	); err != nil {
		return "", fmt.Errorf("failed to start container")
	}

	return resp.ID, nil
}

func (nr *nextruntime) createContainerConfig() *container.Config {
	config := &container.Config{
		Image:        nr.getImageName(),
		ExposedPorts: nr.getExposedPorts(),
		Env:          nr.getEnvironmentVariables(),
		Cmd:          nr.getStartCommand(),
		Labels:       nr.getLabels(),
	}

	// Configure health check if needed
	if nr.payload.NextConfig.Output == "standalone" {
		config.Healthcheck = &container.HealthConfig{
			Test:     []string{"CMD-SHELL", "curl -f http://localhost:3000/api/health || exit 1"},
			Interval: 30 * time.Second,
			Timeout:  10 * time.Second,
			Retries:  3,
		}
	}

	return config

}

func (nr *nextruntime) createHostConfig() *container.HealthConfig {
	hostConfig := &container.HostConfig{
		PortBindings: nr.getPortBindings(),
		RestartPolicy: container.RestartPolicy{
			Name: "unless-stopped",
		},
		Mounts:      nr.getMounts(),
		Resources:   container.Resources{},
		SecurityOpt: []string{"no-new-privileges"},
	}

	// Configure memory limits based on routes
	if len(nr.payload.RouteInfo.SSRRoutes) > 0 {
		hostConfig.Resources.Memory = 512 * 1024 * 1024 // 512MB
	} else {
		hostConfig.Resources.Memory = 256 * 1024 * 1024 // 256MB
	}

	return hostConfig
}

func (nr *nextruntime) getContainerName() string {
	return fmt.Sprintf("%s-%s", strings.ToLower(nr.payload.config.AppName), &nr.payload.GitCommit[:7])
}

func (nr *nextruntime) getImageName() string {
	return fmt.Sprintf("%s:%s", strings.ToLower(nr.payload.config.AppName), &nr.payload.GitCommit[:7])
}

func (nr *nextruntime) getExposedPorts() map[nat.Port]struct{} {
	ports := make(map[nat.Port]struct{})
	ports["3000/tcp"] = struct{}{}

	// Add additional ports if specified in config
	if nr.payload.Config != nil && nr.payload.Config.App.Port != "" {
		ports[nat.Port(nr.payload.Config.App.Port+"/tcp")] = struct{}{}
	}
	return ports

}
