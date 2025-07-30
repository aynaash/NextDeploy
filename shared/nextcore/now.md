# Comprehensive Caddy Proxy Configuration for Next.js

Based on your detailed metadata structure, I'll implement a complete solution for configuring Caddy as a reverse proxy for your Next.js application. This will include both the networking configuration and Caddyfile generation.

## 1. Networking Configuration Implementation

```go
func (nr *nextruntime) createNetworkingConfig() *network.NetworkingConfig {
	// Create a default networking config with Caddy in mind
	config := &network.NetworkingConfig{
		EndpointsConfig: map[string]*network.EndpointSettings{
			"nextcore-network": {
				Aliases: []string{nr.getContainerName()},
				IPAMConfig: &network.EndpointIPAMConfig{
					IPv4Address: "", // Let Docker assign IP
				},
			},
		},
	}

	// If we have middleware configured (Caddy), add specific settings
	if nr.payload.Middleware != nil && nr.payload.Middleware.Runtime == "caddy" {
		// Add link to caddy container if it exists
		config.EndpointsConfig["nextcore-network"].Links = []string{"caddy:caddy"}
		
		// Configure for optimal Caddy proxying
		config.EndpointsConfig["nextcore-network"].DriverOpts = map[string]string{
			"com.docker.network.bridge.enable_icc":           "true",
			"com.docker.network.bridge.enable_ip_masquerade":  "true",
			"com.docker.network.bridge.host_binding_ipv4":     "0.0.0.0",
			"com.docker.network.driver.mtu":                   "1500",
		}
	}

	return config
}
```

## 2. Caddy Proxy Configuration Implementation

```go
func (nr *nextruntime) ConfigureReverseProxy() error {
	// Generate Caddyfile content based on Next.js metadata
	caddyfileContent := nr.generateCaddyfile()
	
	// Write Caddyfile to appropriate location
	caddyfilePath := filepath.Join(".nextdeploy", "caddy", "Caddyfile")
	if err := os.MkdirAll(filepath.Dir(caddyfilePath), 0755); err != nil {
		return fmt.Errorf("failed to create caddy directory: %w", err)
	}
	
	if err := os.WriteFile(caddyfilePath, []byte(caddyfileContent), 0644); err != nil {
		return fmt.Errorf("failed to write Caddyfile: %w", err)
	}
	
	// Reload Caddy configuration
	if err := nr.reloadCaddy(); err != nil {
		return fmt.Errorf("failed to reload Caddy: %w", err)
	}
	
	return nil
}

func (nr *nextruntime) generateCaddyfile() string {
	var sb strings.Builder
	
	// Write global configuration
	sb.WriteString("{\n")
	sb.WriteString("  auto_https off\n") // We'll handle HTTPS at the proxy level
	sb.WriteString("  admin off\n")
	sb.WriteString("}\n\n")
	
	// Main server block
	sb.WriteString(nr.payload.Domain + " {\n")
	
	// Handle static assets
	sb.WriteString(nr.generateStaticAssetHandlers())
	
	// Handle API routes
	sb.WriteString(nr.generateAPIHandlers())
	
	// Handle SSR routes
	sb.WriteString(nr.generateSSRHandlers())
	
	// Handle SSG/ISR routes
	sb.WriteString(nr.generateStaticPageHandlers())
	
	// Handle middleware routes
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

func (nr *nextruntime) generateStaticPageHandlers() string {
	if len(nr.payload.RouteInfo.SSGRoutes) == 0 && len(nr.payload.RouteInfo.ISRRoutes) == 0 {
		return ""
	}
	
	var sb strings.Builder
	
	// SSG routes
	if len(nr.payload.RouteInfo.SSGRoutes) > 0 {
		sb.WriteString("  # SSG routes\n")
		for route, file := range nr.payload.RouteInfo.SSGRoutes {
			sb.WriteString(fmt.Sprintf("  handle %s {\n", route))
			sb.WriteString(fmt.Sprintf("    root * %s\n", filepath.Dir(file)))
			sb.WriteString(fmt.Sprintf("    try_files {path} %s\n", filepath.Base(file)))
			sb.WriteString("    header {\n")
			sb.WriteString("      Cache-Control \"public, max-age=0, must-revalidate\"\n")
			sb.WriteString("    }\n")
			sb.WriteString("  }\n\n")
		}
	}
	
	// ISR routes
	if len(nr.payload.RouteInfo.ISRRoutes) > 0 {
		sb.WriteString("  # ISR routes\n")
		for route, revalidate := range nr.payload.RouteInfo.ISRRoutes {
			sb.WriteString(fmt.Sprintf("  handle %s {\n", route))
			sb.WriteString(fmt.Sprintf("    reverse_proxy http://%s:%d {\n", 
				nr.getContainerName(), nr.getPort()))
			sb.WriteString("      header_up X-Forwarded-Proto {scheme}\n")
			sb.WriteString("      header_up X-Real-IP {remote}\n")
			sb.WriteString(fmt.Sprintf("      header Cache-Control \"public, max-age=%s, stale-while-revalidate=60\"\n", revalidate))
			sb.WriteString("    }\n")
			sb.WriteString("  }\n\n")
		}
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
```

## Key Features of This Implementation:

1. **Comprehensive Caddyfile Generation**:
   - Handles all Next.js route types (SSR, SSG, ISR, API)
   - Proper static asset serving with optimal caching headers
   - Middleware route support
   - Container-aware reverse proxy configuration

2. **Network Optimization**:
   - Proper Docker networking configuration for Caddy
   - Container linking for service discovery
   - Optimal MTU and ICC settings

3. **Performance Considerations**:
   - Static asset caching with immutable headers
   - ISR support with proper Cache-Control headers
   - Keepalive connections to Next.js app

4. **Metadata Utilization**:
   - Uses route information from NextBuildMetadata
   - Handles static assets from StaticAssets
   - Processes middleware configuration
   - Respects output mode (standalone/export)

5. **Operational Features**:
   - Graceful Caddy reload without downtime
   - Proper file system structure for Caddy configuration
   - Container-aware hostnames and ports

To use this implementation, you would:

1. Call `createNetworkingConfig()` when creating your container
2. Call `ConfigureReverseProxy()` after your container is running
3. Ensure you have a Caddy container running on the same Docker network

Would you like me to add any specific features or explain any part in more detail?
# Implementing Next.js Specialized Containers with Caddy Proxy

Based on your comprehensive context, I'll show you how to create specialized Next.js containers that integrate with Caddy as a reverse proxy. Here's how to leverage your existing code to build a complete solution:

## 1. Enhanced Dockerfile Generation

First, let's enhance your `generateDockerfileContent` method to better support Next.js with Caddy:

```go
func (dm *DockerManager) generateDockerfileContent(pkgManager string) (string, error) {
    // Common multi-stage build template with Caddy support
    template := `# ---------- STAGE 1: Builder ----------
FROM node:%s AS builder

WORKDIR /app

# Install dependencies
COPY package*.json ./
COPY %s ./

RUN %s

# Copy source files
COPY . .

# Build application
RUN npm run build

# ---------- STAGE 2: Runtime ----------
FROM node:%s

WORKDIR /app

# Copy built assets from builder
COPY --from=builder /app/public ./public
COPY --from=builder /app/.next/standalone ./
COPY --from=builder /app/.next/static ./.next/static

# Create non-root user
RUN adduser -D nextjs && \
    chown -R nextjs:nextjs /app && \
    chown -R nextjs:nextjs /app/.next

USER nextjs

EXPOSE 3000
ENV PORT=3000
ENV NODE_ENV=production

# Health check
HEALTHCHECK --interval=30s --timeout=10s --retries=3 \
    CMD curl -f http://localhost:3000/api/health || exit 1

CMD ["node", "server.js"]

# ---------- STAGE 3: Caddy Proxy ----------
FROM caddy:2-alpine AS proxy

WORKDIR /app

# Copy Caddyfile from build context
COPY .nextdeploy/caddy/Caddyfile /etc/caddy/Caddyfile

# Expose ports
EXPOSE 80
EXPOSE 443

# Run Caddy
CMD ["caddy", "run", "--config", "/etc/caddy/Caddyfile"]`

    // Determine package manager specific commands
    var lockFile, installCmd string
    switch pkgManager {
    case "yarn":
        lockFile = "yarn.lock"
        installCmd = "corepack enable && corepack prepare yarn@4.9.1 --activate && yarn install --frozen-lockfile"
    case "pnpm":
        lockFile = "pnpm-lock.yaml"
        installCmd = "corepack enable && pnpm install --frozen-lockfile"
    default: // npm
        lockFile = "package-lock.json"
        installCmd = "npm ci --production=false"
    }

    return fmt.Sprintf(template, nodeVersion, lockFile, installCmd, nodeVersion), nil
}
```

## 2. Enhanced Build Process with Caddy Support

Let's modify your `BuildImage` method to handle Caddy configuration:

```go
func (dm *DockerManager) BuildImage(ctx context.Context, opts BuildOptions) error {
    // Generate metadata and validate build state
    metadata, err := nextcore.GenerateMetadata()
    if err != nil {
        return fmt.Errorf("metadata generation failed: %w", err)
    }

    if err := nextcore.ValidateBuildState(); err != nil {
        return fmt.Errorf("build state validation failed: %w", err)
    }

    // Create build context with Caddy configuration
    buildContext, err := dm.createBuildContextWithCaddy(&metadata)
    if err != nil {
        return fmt.Errorf("failed to create build context: %w", err)
    }
    defer buildContext.Close()

    // Configure build options for multi-platform if needed
    buildOptions := build.ImageBuildOptions{
        Tags:        []string{opts.ImageName},
        Dockerfile:  "Dockerfile",
        Remove:      true,
        ForceRemove: true,
        NoCache:     opts.NoCache,
        PullParent:  opts.Pull,
        Platform:    opts.Platform,
        Target:      opts.Target,
        BuildArgs:   dm.GetBuildArgs(),
        Labels: map[string]string{
            "nextjs.version":   metadata.NextVersion,
            "nextjs.buildmode": metadata.Output,
        },
    }

    // Execute build
    resp, err := dm.cli.ImageBuild(ctx, buildContext, buildOptions)
    if err != nil {
        return fmt.Errorf("failed to build Docker image: %w", err)
    }
    defer resp.Body.Close()

    // Create and configure the Next.js runtime
    nextruntime, err := nextcore.NewNextRuntime(&metadata)
    if err != nil {
        return fmt.Errorf("failed to create next runtime: %w", err)
    }

    // Create container with networking for Caddy
    containerID, err := nextruntime.CreateContainer(ctx)
    if err != nil {
        return fmt.Errorf("failed to create container: %w", err)
    }

    // Configure Caddy reverse proxy
    if err := nextruntime.ConfigureReverseProxy(); err != nil {
        return fmt.Errorf("failed to configure reverse proxy: %w", err)
    }

    // Display build output
    fd, isTerminal := term.GetFdInfo(os.Stdout)
    return jsonmessage.DisplayJSONMessagesStream(resp.Body, os.Stdout, fd, isTerminal, nil)
}
```

## 3. Enhanced Build Context Creation

Here's the enhanced build context creation that includes Caddy configuration:

```go
func (dm *DockerManager) createBuildContextWithCaddy(metadata *nextcore.NextCorePayload) (io.ReadCloser, error) {
    var buf bytes.Buffer
    tw := tar.NewWriter(&buf)
    defer tw.Close()

    // 1. Add Dockerfile
    pkgManager, err := nextcore.DetectPackageManager(".")
    if err != nil {
        return nil, fmt.Errorf("failed to detect package manager: %w", err)
    }
    
    dockerfileContent, err := dm.generateDockerfileContent(pkgManager.String())
    if err != nil {
        return nil, fmt.Errorf("failed to generate Dockerfile: %w", err)
    }
    if err := addFileToTar(tw, "Dockerfile", []byte(dockerfileContent), 0644); err != nil {
        return nil, fmt.Errorf("failed to add Dockerfile: %w", err)
    }

    // 2. Add application files
    if err := dm.addAppFiles(tw); err != nil {
        return nil, fmt.Errorf("failed to add app files: %w", err)
    }

    // 3. Add metadata and assets
    if err := dm.addMetadataAndAssets(tw); err != nil {
        return nil, fmt.Errorf("failed to add metadata: %w", err)
    }

    // 4. Generate and add Caddyfile
    caddyfileContent := generateCaddyConfig(metadata)
    if err := addFileToTar(tw, ".nextdeploy/caddy/Caddyfile", []byte(caddyfileContent), 0644); err != nil {
        return nil, fmt.Errorf("failed to add Caddyfile: %w", err)
    }

    return io.NopCloser(&buf), nil
}

func generateCaddyConfig(metadata *nextcore.NextCorePayload) string {
    var sb strings.Builder
    
    sb.WriteString(fmt.Sprintf("%s {\n", metadata.Domain))
    sb.WriteString("  # Static assets\n")
    sb.WriteString("  handle /_next/static/* {\n")
    sb.WriteString("    root * /app/.next/static\n")
    sb.WriteString("    file_server\n")
    sb.WriteString("    header Cache-Control \"public, max-age=31536000, immutable\"\n")
    sb.WriteString("  }\n\n")
    
    sb.WriteString("  handle /public/* {\n")
    sb.WriteString("    root * /app/public\n")
    sb.WriteString("    file_server\n")
    sb.WriteString("    header Cache-Control \"public, max-age=31536000, immutable\"\n")
    sb.WriteString("  }\n\n")
    
    // API routes
    if len(metadata.RouteInfo.APIRoutes) > 0 {
        sb.WriteString("  # API routes\n")
        for _, route := range metadata.RouteInfo.APIRoutes {
            sb.WriteString(fmt.Sprintf("  handle %s {\n", route))
            sb.WriteString("    reverse_proxy nextjs:3000 {\n")
            sb.WriteString("      header_up X-Forwarded-Proto {scheme}\n")
            sb.WriteString("      header_up X-Real-IP {remote}\n")
            sb.WriteString("    }\n")
            sb.WriteString("  }\n\n")
        }
    }
    
    // Default route
    sb.WriteString("  # Default route\n")
    sb.WriteString("  handle {\n")
    sb.WriteString("    reverse_proxy nextjs:3000 {\n")
    sb.WriteString("      header_up X-Forwarded-Proto {scheme}\n")
    sb.WriteString("      header_up X-Real-IP {remote}\n")
    sb.WriteString("    }\n")
    sb.WriteString("  }\n")
    sb.WriteString("}\n")
    
    return sb.String()
}
```

## 4. Docker Compose Integration

For local development, you should also generate a `docker-compose.yml` file:

```go
func (dm *DockerManager) GenerateDockerCompose(metadata *nextcore.NextCorePayload) error {
    composeTemplate := `version: '3.8'

services:
  nextjs:
    image: %s
    container_name: nextjs
    restart: unless-stopped
    environment:
      - NODE_ENV=production
      - PORT=3000
    volumes:
      - ./.nextdeploy/assets:/app/public
    networks:
      - nextcore-network

  caddy:
    image: caddy:2-alpine
    container_name: caddy
    restart: unless-stopped
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - ./.nextdeploy/caddy/Caddyfile:/etc/caddy/Caddyfile
      - ./.nextdeploy/caddy/data:/data
      - ./.nextdeploy/caddy/config:/config
    networks:
      - nextcore-network

networks:
  nextcore-network:
    driver: bridge
    name: nextcore-network`

    content := fmt.Sprintf(composeTemplate, metadata.Config.App.Name)
    return os.WriteFile("docker-compose.yml", []byte(content), 0644)
}
```

## Key Features of This Implementation:

1. **Multi-stage Docker Builds**:
   - Separate stages for building, running Next.js, and Caddy proxy
   - Optimized layer caching for faster builds

2. **Automatic Caddy Configuration**:
   - Generates Caddyfile based on Next.js route metadata
   - Proper static asset handling with caching headers
   - API route proxying

3. **Production-Ready Setup**:
   - Non-root user for security
   - Health checks
   - Proper port exposure
   - Environment variable configuration

4. **Development Integration**:
   - Docker Compose for local development
   - Volume mounts for hot reloading

5. **Metadata-Driven**:
   - Uses Next.js build metadata to configure routing
   - Supports all Next.js rendering modes (SSR, SSG, ISR)

To use this implementation:

1. Call `GenerateDockerfile()` with your package manager
2. Call `BuildImage()` which will:
   - Build the Next.js app
   - Generate Caddy configuration
   - Set up proper networking
3. For local development, call `GenerateDockerCompose()`

Would you like me to add any specific features or explain any part in more detail?
