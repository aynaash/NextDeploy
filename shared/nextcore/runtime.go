package nextcore

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
)

type nextruntime struct {
	dockerclient *client.Client
	payload      *NextCorePayload
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
	}, nil
}

func (nr *nextruntime) createNetworkingConfig() *network.NetworkingConfig {
	// FIX: create right newwork config with caddy and Nginx in mind
	return nil
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

func (nr *nextruntime) createHostConfig() *container.HostConfig {
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
	return fmt.Sprintf("%s-%s", strings.ToLower(nr.payload.Config.App.Name), &nr.payload.GitCommit)
}

func (nr *nextruntime) getImageName() string {
	return fmt.Sprintf("%s:%s", strings.ToLower(nr.payload.Config.App.Name), &nr.payload.GitCommit)
}

func (nr *nextruntime) getExposedPorts() map[nat.Port]struct{} {
	ports := make(map[nat.Port]struct{})
	ports["3000/tcp"] = struct{}{}

	// Add additional ports if specified in config
	// TODO: This type check is bad find a better way to do it
	if nr.payload.Config != nil && nr.payload.Config.App.Port != 0 {
		ports[nat.Port(string(nr.payload.Config.App.Port)+"/tcp")] = struct{}{}
	}
	return ports

}

func (nr *nextruntime) getEnvironmentVariables() []string {
	// TODO: Implement environment variable handling
	var envVars []string
	return envVars
}

func (nr *nextruntime) getPort() int {
	if nr.payload.Config != nil && nr.payload.Config.App.Port != 0 {
		return nr.payload.Config.App.Port
	}
	return 3000 // Default value
}

// FIX: we need this to be more eloborate
func (nr *nextruntime) getStartCommand() []string {
	if nr.payload.StartCommand != "" {
		switch nr.payload.NextConfig.Output {
		case "standalone":
			return []string{"node", "server.js"}
		case "export":
			return []string{"npm", "run", "start"}

		}
	}
	return []string{"npm", "start"}
}

func (nr *nextruntime) getLabels() map[string]string {
	lables := map[string]string{
		"appname":     nr.payload.AppName,
		"nextversion": nr.payload.NextVersion,
		"gitcommit":   nr.payload.GitCommit,
		"buildtime":   nr.payload.GeneratedAt,
	}
	// add middleware if info present
	if nr.payload.Middleware != nil {
		lables["middlewareruntime"] = nr.payload.Middleware.Runtime
	}

	return lables
}

func (nr *nextruntime) getPortBindings() map[nat.Port][]nat.PortBinding {
	bindings := make(map[nat.Port][]nat.PortBinding)
	port := nr.getPort()
	bindings[nat.Port(string(port)+"/tcp")] = []nat.PortBinding{
		{
			HostIP:   "0.0.0.0",
			HostPort: string(port),
		},
	}

	return bindings
}

func (nr *nextruntime) getMounts() []mount.Mount {
	var mounts []mount.Mount

	// Mount static assets if CDN is disabled
	if !nr.payload.CDNEnabled && nr.payload.AssetsOutputDir != "" {
		mounts = append(mounts, mount.Mount{
			Type:   mount.TypeBind,
			Source: nr.payload.AssetsOutputDir,
			Target: "/app/public",
		})
	}

	// Mount image cache if using image optimization
	if nr.payload.HasImageAssets {
		cacheDir := filepath.Join(os.TempDir(), "next-image-cache")
		os.MkdirAll(cacheDir, 0755)
		mounts = append(mounts, mount.Mount{
			Type:   mount.TypeBind,
			Source: cacheDir,
			Target: "/tmp/cache",
		})
	}

	return mounts
}

func (nr *nextruntime) ConfigureReverseProxy() error {
	// TODO: integrate the caddy proxy logic here
	// based on the routes and middleware in the payload
	// implementation ---
	//  ---- Genearete Caddyfile based on routes and middleware
	//  ---- Reload caddy configuration

	return nil
}

func (nr *nextruntime) GetRuntimeStats(ctx context.Context) (container.StatsResponseReader, error) {
	containerName := nr.getContainerName()
	return nr.dockerclient.ContainerStats(ctx, containerName, false)
}
