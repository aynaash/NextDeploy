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
	NextCoreLogger.Debug("the container config is:%v", containerConfig)
	hostConfig := nr.createHostConfig()
	NextCoreLogger.Debug("the host config is:%v", hostConfig)
	neworkingConfig := nr.createNetworkingConfig()
	NextCoreLogger.Debug("the networking config is:%v", neworkingConfig)
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
	return fmt.Sprintf("%s-%s", strings.ToLower(nr.payload.Config.App.Name), nr.payload.GitCommit)
}

func (nr *nextruntime) getImageName() string {
	tag := nr.payload.GitCommit
	if tag == "" {
		tag = "latest"
	}
	return fmt.Sprintf("%s:%s", strings.ToLower(nr.payload.Config.App.Name), tag)
}

func (nr *nextruntime) getExposedPorts() map[nat.Port]struct{} {
	ports := make(map[nat.Port]struct{})
	ports["3000/tcp"] = struct{}{}

	// Add additional ports if specified in config
	// TODO: This type check is bad find a better way to do it
	if nr.payload.Config != nil && nr.payload.Config.App.Port != 0 {
		ports[nat.Port(string(nr.getPort())+"/tcp")] = struct{}{}
	}
	return ports

}

func (nr *nextruntime) getEnvironmentVariables() []string {
	// TODO: Implement environment variable handling
	var envVars []string
	return envVars
}

// BUG:getPort returns the port to be used for the Next.js application.
func (nr *nextruntime) getPort() string {
	// if nr.payload.Config != nil && nr.payload.Config.App.Port != 0 {
	// 	return nr.payload.Config.App.Port
	// }
	return "3000" // Default value
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
		NextCoreLogger.Error("failed to write caddy file :%s", err)
		return fmt.Errorf("failed to wrote Caddyfile:%s", err)
	}

	// reload caddy configuration
	if err := nr.reloadCaddy(); err != nil {
		NextCoreLogger.Error("error reloading config:%s", err)
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
func convertRegexToCaddyMatcher(regex string) string {
	// Simple conversion - for more complex cases you might need a better approach
	return strings.TrimPrefix(strings.TrimSuffix(regex, "$"), "^")
}
func (nr *nextruntime) reloadCaddy() error {
	ctx := context.Background()

	// Send SIGHUP to Caddy container to reload config
	err := nr.dockerclient.ContainerKill(ctx, "caddy", "SIGHUP")
	if err != nil {
		return fmt.Errorf("failed to reload Caddy: %w", err)
	}

	return nil
}

func (nr *nextruntime) generateAPIHandlers() string {
	if len(nr.payload.RouteInfo.APIRoutes) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("  # API routes\n")

	for _, route := range nr.payload.RouteInfo.APIRoutes {
		sb.WriteString(fmt.Sprintf("  handle %s {\n", route))
		sb.WriteString(fmt.Sprintf("    reverse_proxy http://%s:%d%s {\n",
			nr.getContainerName(), nr.getPort(), route))
		sb.WriteString("      header_up X-Forwarded-Proto {scheme}\n")
		sb.WriteString("      header_up X-Real-IP {remote}\n")
		sb.WriteString("    }\n")
		sb.WriteString("  }\n\n")
	}

	return sb.String()
}
func (nr *nextruntime) generateMiddlewareHandlers() string {
	if nr.payload.Middleware == nil || len(nr.payload.Middleware.Matchers) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("  # Middleware routes\n")

	for _, matcher := range nr.payload.Middleware.Matchers {
		// Convert Next.js middleware matcher to Caddy syntax
		path := matcher.Pathname
		if path == "" && matcher.Pattern != "" {
			path = convertRegexToCaddyMatcher(matcher.Pattern)
		}

		sb.WriteString(fmt.Sprintf("  @%s path %s\n", matcher.Type, path))

		// Add conditions
		for _, condition := range matcher.Has {
			sb.WriteString(fmt.Sprintf("    %s %s %s\n",
				condition.Type, condition.Key, condition.Value))
		}

		sb.WriteString(fmt.Sprintf("  handle @%s {\n", matcher.Type))
		sb.WriteString(fmt.Sprintf("    reverse_proxy http://%s:%d {\n",
			nr.getContainerName(), nr.getPort()))
		sb.WriteString("      header_up X-Forwarded-Proto {scheme}\n")
		sb.WriteString("      header_up X-Real-IP {remote}\n")
		sb.WriteString("    }\n")
		sb.WriteString("  }\n\n")
	}

	return sb.String()
}
func (nr *nextruntime) generateStaticPageHandlers() string {
	if len(nr.payload.RouteInfo.SSRRoutes) == 0 && len(nr.payload.RouteInfo.ISRRoutes) == 0 {
		return ""
	}
	var sb strings.Builder

	// ssg routes
	if len(nr.payload.RouteInfo.SSRRoutes) > 0 {
		sb.WriteString(" # SSG routes\n")
		for route, file := range nr.payload.RouteInfo.SSGRoutes {
			sb.WriteString(fmt.Sprintf("  handle %s {\n", route))
			sb.WriteString(fmt.Sprintf("   root * %s\n", filepath.Dir(file)))
			sb.WriteString("     try_files {path} %s\n, filepath.Base(file))")
			sb.WriteString("     header {\n")
			sb.WriteString("       Cache-Control \"public, max-age=0 , must-revalidate\"\n")
			sb.WriteString("    }\n")
			sb.WriteString("  }\n\n")

		}
	}
	// isr routes
	if len(nr.payload.RouteInfo.ISRRoutes) > 0 {
		sb.WriteString(" # ISR routes\n")
		for route, revalidate := range nr.payload.RouteInfo.ISRRoutes {
			sb.WriteString(fmt.Sprintf("  handle %s {\n", route))
			sb.WriteString(fmt.Sprintf("    reverse_proxy http://%s:%d {\n", nr.getContainerName(), nr.getPort()))
			sb.WriteString("      header_up X-Forwarded-Proto {scheme}\n")
			sb.WriteString("      header_up X-Real-IP {remote}\n")
			sb.WriteString(fmt.Sprintf("      header Cache-Control \"public, max-age=%s, stale-while-revalidate=60\"\n", revalidate))
			sb.WriteString("    }\n")
			sb.WriteString("  }\n\n")
		}
	}
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

//	func (nr *nextruntime) EnableMonitoring() error {
//		// Add Prometheus endpoint if enabled in config
//		if nr.payload.Config.Monitoring.Prometheus {
//			nr.containerConfig.Env = append(nr.containerConfig.Env,
//				"NEXT_PUBLIC_PROMETHEUS_ENABLED=true",
//				"PROMETHEUS_PORT=9090",
//			)
//			nr.hostConfig.PortBindings["9090/tcp"] = []nat.PortBinding{
//				{HostIP: "0.0.0.0", HostPort: "9090"},
//			}
//		}
//		return nil
//
// //	}  ```go
//
//	   func (nr *NextRuntime) AddLifecycleHooks() {
//	       if nr.payload.NextBuild.HasAppRouter {
//	           nr.containerConfig.Labels["nextdeploy.lifecycle.preStop"] =
//	               "/bin/sh -c 'npm run poststop'"
//	       }
//	   }
//
//		//	  func (nr *NextRuntime) ConfigureLogging() {
//		       nr.hostConfig.LogConfig = &container.LogConfig{
//		           Type: "json-file",
//		           Config: map[string]string{
//		               "max-size": "10m",
//		               "max-file": "3",
//		           },
//		       }
//		   }
func (nr *nextruntime) GetRuntimeStats(ctx context.Context) (container.StatsResponseReader, error) {
	containerName := nr.getContainerName()
	return nr.dockerclient.ContainerStats(ctx, containerName, false)
}
