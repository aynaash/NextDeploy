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
	// Create a default network confi with caddy in mind
	config := &network.NetworkingConfig{
		EndpointsConfig: map[string]*network.EndpointSettings{
			"nextcore-network": {
				Aliases: []string{nr.getContainerName()},
				IPAMConfig: &network.EndpointIPAMConfig{
					IPv4Address: fmt.Sprintf("172.18.0.%d", nr.payload.Config.App.Port%254+1), // Simple IP allocation
				},
			},
		},
	}

	// if we have middleware configured caddy , add specific network settings
	if nr.payload.Middleware != nil && nr.payload.Middleware.Runtime == "caddy" {
		// add link to caddy container if it exists
		config.EndpointsConfig["nextcore-network"].Links = []string{
			"caddy:nextcore-caddy",
		}

		// configure for optimal caddy proxying
		config.EndpointsConfig["nextcore-network"].DriverOpts = map[string]string{
			"com.docker.network.bridge.enable_icc":           "true",
			"com.docker.network.bridge.enable_ip_masquerade": "true",
			"com.docker.network.bridge.host_binding_ipv4":    "0.0.0.0",
			"com.docker.network.driver.mu":                   "1500",
		}
	}
	return config
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
		NextCoreLogger.Error("Error creating container:%s", err)
		return "", fmt.Errorf("failed to create container:%w", err)
	}
	// start the container
	if err := nr.dockerclient.ContainerStart(
		ctx,
		resp.ID,
		container.StartOptions{},
	); err != nil {
		NextCoreLogger.Error("Error starting container:%s", err)
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
	if nr.payload.Output == "standalone" {
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
	return fmt.Sprintf("%s-%v", strings.ToLower(nr.payload.Config.App.Name), &nr.payload.GitCommit)
}

func (nr *nextruntime) getImageName() string {
	return fmt.Sprintf("%s:%v", strings.ToLower(nr.payload.Config.App.Name), &nr.payload.GitCommit)
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

// BUG:getPort returns the port to be used for the Next.js application.
func (nr *nextruntime) getPort() int {
	if nr.payload.Config != nil && nr.payload.Config.App.Port != 0 {
		return nr.payload.Config.App.Port
	}
	return 3000 // Default value
}

// FIX: we need this to be more eloborate
func (nr *nextruntime) getStartCommand() []string {
	if nr.payload.StartCommand != "" {
		switch nr.payload.Output {
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
	// Generate caddy file content based on nextjs metadata
	caddyfileConfig := nr.generateCaddyfile()

	// write caddy file to appropriate location
	caddyfilePath := filepath.Join(".nextdeploy", "caddy", "Caddyfile")
	if err := os.MkdirAll(filepath.Dir(caddyfilePath), 0755); err != nil {
		NextCoreLogger.Error("failed to crate caddy directory:%s", err)
		return fmt.Errorf("failed to create caddy directory:%w", err)

	}

	if err := os.WriteFile(caddyfilePath, []byte(caddyfileConfig), 0644); err != nil {
		NextCoreLogger.Error("failed to write caddy file :%w", err)
		return fmt.Errorf("failed to wrote Caddyfile:%s", err)
	}

	// reload caddy configuration
	if err := nr.reloadCaddy(); err != nil {
		NextCoreLogger.Error("error reloading config:%w", err)
		return fmt.Errorf("failedd to reload Caddy")
	}

	return nil

}

func (nr *nextruntime) generateCaddyfile() string {
	var sb strings.Builder

	// write global configs
	sb.WriteString("{\n")
	// we will have to handle https at proxy level
	sb.WriteString("  auto_https off\n")
	sb.WriteString("  admin off\n")
	sb.WriteString("}\n\n")

	// main server block
	sb.WriteString(nr.payload.Domain + " {\n")
	// handle static assets
	sb.WriteString(nr.generateStaticAssetHandlers())
	// handle api routes
	sb.WriteString(nr.generateAPIHandlers())
	// handle ssr routes
	sb.WriteString(nr.generateSSRHandlers())
	// handle ssg/isr routes
	sb.WriteString(nr.generateStaticPageHandlers())
	// handle middlware routes
	sb.WriteString(nr.generateMiddlewareHandlers())
	// Default reverse proxy to Next.js app
	sb.WriteString(fmt.Sprintf("  reverse_proxy http://%s:%d {\n", nr.getContainerName(), nr.getPort()))
	sb.WriteString("    header_up X-Forwarded-Proto {scheme}\n")
	sb.WriteString("    header_up X-Real-IP {remote}\n")
	sb.WriteString("    transport http {\n")
	sb.WriteString("      keepalive 32\n")
	sb.WriteString("      keepalive_interval 30s\n")
	sb.WriteString("    }\n")
	sb.WriteString("  }\n")

	sb.WriteString("}\n")

	return sb.String()
}

func (nr *nextruntime) generateStaticAssetHandlers() string {
	var sb strings.Builder

	if len(nr.payload.StaticAssets.PublicDir) > 0 {
		sb.WriteString("  # Public directory assets\n")
		sb.WriteString("  handle /public/* {\n")
		sb.WriteString("    root * .nextdeploy/static\n")
		sb.WriteString("    file_server\n")

		// Add caching headers for static assets
		sb.WriteString("    header {\n")
		sb.WriteString("      Cache-Control \"public, max-age=31536000, immutable\"\n")
		sb.WriteString("    }\n")
		sb.WriteString("  }\n\n")
	}

	if len(nr.payload.StaticAssets.NextStatic) > 0 {
		sb.WriteString("  # Next.js static assets\n")
		sb.WriteString("  handle /_next/static/* {\n")
		sb.WriteString("    root * .next/static\n")
		sb.WriteString("    file_server\n")
		sb.WriteString("    header {\n")
		sb.WriteString("      Cache-Control \"public, max-age=31536000, immutable\"\n")
		sb.WriteString("    }\n")
		sb.WriteString("  }\n\n")
	}

	return sb.String()
}

func (nr *nextruntime) generateSSRHandlers() string {
	if len(nr.payload.RouteInfo.SSRRoutes) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("  # SSR routes\n")

	for _, route := range nr.payload.RouteInfo.SSRRoutes {
		sb.WriteString(fmt.Sprintf("  handle %s {\n", route))
		sb.WriteString(fmt.Sprintf("    reverse_proxy http://%s:%d {\n",
			nr.getContainerName(), nr.getPort()))
		sb.WriteString("      header_up X-Forwarded-Proto {scheme}\n")
		sb.WriteString("      header_up X-Real-IP {remote}\n")
		sb.WriteString("    }\n")
		sb.WriteString("  }\n\n")
	}

	return sb.String()
}

func (nr *nextruntime) generateAPIHandlers() string {

}
func (nr *nextruntime) GetRuntimeStats(ctx context.Context) (container.StatsResponseReader, error) {
	containerName := nr.getContainerName()
	return nr.dockerclient.ContainerStats(ctx, containerName, false)
}
