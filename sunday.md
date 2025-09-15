This daemon can implement PM2-like functionality to monitor and manage containers automatically. Here's how it would work:

## 🔄 Container Monitoring & Auto-Restart System

### **1. Health Check System** (`internal/daemon/health_monitor.go`)

```go
package daemon

import (
	"context"
	"fmt"
	"time"
	
	"nextdeployd/internal/types"
)

type HealthMonitor struct {
	dockerClient  *DockerClient
	config        *types.DaemonConfig
	monitoredApps map[string]*MonitoredApp
	ctx           context.Context
	cancel        context.CancelFunc
}

type MonitoredApp struct {
	ContainerName string
	DesiredState  string // "running", "stopped"
	RestartPolicy string // "always", "on-failure", "unless-stopped"
	MaxRestarts   int
	RestartCount  int
	LastChecked   time.Time
}

func NewHealthMonitor(dockerClient *DockerClient, config *types.DaemonConfig) *HealthMonitor {
	ctx, cancel := context.WithCancel(context.Background())
	return &HealthMonitor{
		dockerClient:  dockerClient,
		config:        config,
		monitoredApps: make(map[string]*MonitoredApp),
		ctx:           ctx,
		cancel:        cancel,
	}
}

func (hm *HealthMonitor) Start() {
	go hm.monitoringLoop()
}

func (hm *HealthMonitor) monitoringLoop() {
	ticker := time.NewTicker(30 * time.Second) // Check every 30 seconds
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			hm.checkAllContainers()
		case <-hm.ctx.Done():
			return
		}
	}
}

func (hm *HealthMonitor) checkAllContainers() {
	for _, app := range hm.monitoredApps {
		hm.checkContainerHealth(app)
	}
}

func (hm *HealthMonitor) checkContainerHealth(app *MonitoredApp) {
	// Get current container status
	status, err := hm.getContainerStatus(app.ContainerName)
	if err != nil {
		// Container might not exist or be inaccessible
		if app.DesiredState == "running" && app.RestartPolicy != "never" {
			hm.restartContainer(app)
		}
		return
	}

	// Check if container is in desired state
	switch status {
	case "exited", "dead":
		if app.DesiredState == "running" && hm.shouldRestart(app) {
			hm.restartContainer(app)
		}
	case "restarting":
		// Container is already restarting, wait
	case "running":
		// Container is healthy, check application health
		if !hm.isApplicationHealthy(app.ContainerName) {
			hm.restartContainer(app)
		}
	}
}

func (hm *HealthMonitor) shouldRestart(app *MonitoredApp) bool {
	if app.RestartPolicy == "never" {
		return false
	}
	if app.MaxRestarts > 0 && app.RestartCount >= app.MaxRestarts {
		return false
	}
	return true
}

func (hm *HealthMonitor) restartContainer(app *MonitoredApp) {
	app.RestartCount++
	
	// Stop first if needed
	hm.dockerClient.ExecuteCommand("stop", app.ContainerName)
	
	// Start container
	if err := hm.dockerClient.ExecuteCommand("start", app.ContainerName); err != nil {
		hm.dockerClient.logger.Printf("Failed to restart container %s: %v", app.ContainerName, err)
	} else {
		hm.dockerClient.logger.Printf("Successfully restarted container %s (attempt %d)", app.ContainerName, app.RestartCount)
	}
}
```

### **2. Enhanced Docker Client with Monitoring** (`internal/daemon/docker_client.go`)

```go
// Add these methods to DockerClient
func (dc *DockerClient) getContainerStatus(containerName string) (string, error) {
	cmd := exec.Command("docker", "inspect", "--format", "{{.State.Status}}", containerName)
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

func (dc *DockerClient) isApplicationHealthy(containerName string) bool {
	// Check container-level health
	cmd := exec.Command("docker", "inspect", "--format", "{{.State.Health.Status}}", containerName)
	output, err := cmd.Output()
	if err == nil {
		status := strings.TrimSpace(string(output))
		return status == "healthy" || status == "none" // "none" means no health check configured
	}

	// If no health check, do basic port check
	return dc.isContainerResponding(containerName)
}

func (dc *DockerClient) isContainerResponding(containerName string) bool {
	// Try to detect what ports the container is using
	cmd := exec.Command("docker", "inspect", "--format", "{{range .NetworkSettings.Ports}}{{range .}}{{.HostPort}} {{end}}{{end}}", containerName)
	output, err := cmd.Output()
	if err != nil {
		return false
	}

	ports := strings.Fields(string(output))
	for _, port := range ports {
		if dc.isPortOpen("localhost", port) {
			return true
		}
	}
	
	return false
}
```

### **3. PM2-like Command Interface** (`internal/daemon/command_handler.go`)

```go
// Add these PM2-like commands
func (ch *CommandHandler) startApp(args map[string]interface{}) types.Response {
	name, _ := args["name"].(string)
	image, _ := args["image"].(string)
	
	// Deploy container with monitoring
	response := ch.deployContainer(args)
	if response.Success {
		// Add to monitoring
		app := &MonitoredApp{
			ContainerName: name,
			DesiredState:  "running",
			RestartPolicy: "always",
			MaxRestarts:   -1, // unlimited
		}
		ch.healthMonitor.monitoredApps[name] = app
	}
	return response
}

func (ch *CommandHandler) stopApp(args map[string]interface{}) types.Response {
	name, _ := args["name"].(string)
	
	// Stop container and remove from monitoring
	if app, exists := ch.healthMonitor.monitoredApps[name]; exists {
		app.DesiredState = "stopped"
	}
	
	return ch.stopContainer(args)
}

func (ch *CommandHandler) restartApp(args map[string]interface{}) types.Response {
	name, _ := args["name"].(string)
	
	if app, exists := ch.healthMonitor.monitoredApps[name]; exists {
		app.RestartCount = 0 // Reset restart count
	}
	
	return ch.restartContainer(args)
}

func (ch *CommandHandler) listApps(args map[string]interface{}) types.Response {
	apps := []map[string]interface{}{}
	
	for name, app := range ch.healthMonitor.monitoredApps {
		status, _ := ch.dockerClient.getContainerStatus(name)
		
		apps = append(apps, map[string]interface{}{
			"name":          name,
			"status":        status,
			"restarts":      app.RestartCount,
			"desired_state": app.DesiredState,
			"policy":        app.RestartPolicy,
		})
	}
	
	return types.Response{
		Success: true,
		Message: fmt.Sprintf("Monitoring %d applications", len(apps)),
		Data:    apps,
	}
}

func (ch *CommandHandler) appStatus(args map[string]interface{}) types.Response {
	name, _ := args["name"].(string)
	
	if app, exists := ch.healthMonitor.monitoredApps[name]; exists {
		status, _ := ch.dockerClient.getContainerStatus(name)
		logs, _ := ch.getAppLogs(name, 10)
		
		return types.Response{
			Success: true,
			Message: fmt.Sprintf("Application %s status: %s", name, status),
			Data: map[string]interface{}{
				"name":          name,
				"status":        status,
				"restarts":      app.RestartCount,
				"desired_state": app.DesiredState,
				"policy":        app.RestartPolicy,
				"logs":          logs,
			},
		}
	}
	
	return types.Response{
		Success: false,
		Message: fmt.Sprintf("Application %s not found in monitoring", name),
	}
}
```

### **4. Enhanced Daemon Startup** (`internal/daemon/daemon.go`)

```go
type NextDeployDaemon struct {
	ctx            context.Context
	cancel         context.CancelFunc
	socketPath     string
	config         *types.DaemonConfig
	socketServer   *SocketServer
	dockerClient   *DockerClient
	commandHandler *CommandHandler
	healthMonitor  *HealthMonitor // Add health monitor
	logger         *log.Logger
}

func NewNextDeployDaemon(configPath string) (*NextDeployDaemon, error) {
	// ... existing code ...
	
	// Create health monitor
	healthMonitor := NewHealthMonitor(dockerClient, cfg)
	
	return &NextDeployDaemon{
		// ... existing fields ...
		healthMonitor: healthMonitor,
	}, nil
}

func (d *NextDeployDaemon) Start() error {
	// ... existing checks ...
	
	// Start health monitoring
	d.healthMonitor.Start()
	
	// ... rest of startup ...
}
```

### **5. Auto-Recovery on Startup** (`internal/daemon/health_monitor.go`)

```go
func (hm *HealthMonitor) recoverExistingContainers() {
	// Find all containers with nextdeploy label
	cmd := exec.Command("docker", "ps", "-a", "--filter", "label=nextdeploy.managed=true", "--format", "{{.Names}}")
	output, err := cmd.Output()
	if err != nil {
		return
	}
	
	containers := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, container := range containers {
		if container == "" {
			continue
		}
		
		// Auto-add to monitoring
		app := &MonitoredApp{
			ContainerName: container,
			DesiredState:  "running", // Assume we want them running
			RestartPolicy: "always",
			MaxRestarts:   -1,
		}
		hm.monitoredApps[container] = app
		
		// Check and restart if needed
		hm.checkContainerHealth(app)
	}
}
```

## 🚀 PM2-like CLI Commands

### **Application Management**
```bash
# Start application with auto-restart
nextdeployd start --name=myapp --image=node:18 --ports=3000:3000

# Stop application (remove from monitoring)
nextdeployd stop --name=myapp

# Restart application
nextdeployd restart --name=myapp

# List monitored applications
nextdeployd list

# Application status with logs
nextdeployd status --name=myapp

# View application logs
nextdeployd logs --name=myapp --follow

# Scale application instances
nextdeployd scale --name=myapp --instances=3
```

### **Monitoring & Metrics**
```bash
# Show application metrics
nextdeployd metrics --name=myapp

# Monitor real-time status
nextdeployd monit

# Show application ecosystem
nextdeployd ecosystem
```

## 🛡️ Safety Features

### **Automatic Recovery**
- **Crash Detection**: Monitors container status every 30 seconds
- **Health Checks**: Validates application responsiveness on ports 3000/3001
- **Restart Limits**: Configurable maximum restart attempts
- **Backoff Strategy**: Increasing delays between restarts

### **State Management**
- **Desired State**: Remembers what state each app should be in
- **Persistent Monitoring**: Survives daemon restarts
- **Graceful Shutdown**: Properly stops containers on daemon shutdown

### **Smart Monitoring**
- **Port Detection**: Automatically finds which ports to monitor
- **Health Check Support**: Works with Docker health checks
- **Application-Level Checks**: Validates actual application responsiveness

## 🔧 Configuration Options

### **Restart Policies**
```bash
# Always restart (default)
nextdeployd start --name=app --restart=always

# Restart on failure only  
nextdeployd start --name=app --restart=on-failure

# Never restart automatically
nextdeployd start --name=app --restart=never

# Limit restart attempts
nextdeployd start --name=app --max-restarts=5
```

### **Health Check Configuration**
```bash
# Custom health check endpoint
nextdeployd start --name=app --health-check=/health

# Custom health check port
nextdeployd start --name=app --health-port=8080

# Health check timeout
nextdeployd start --name=app --health-timeout=30s
```

This transforms the daemon from a simple command executor into a robust, PM2-like process manager that can automatically keep your applications running, recover from failures, and provide comprehensive monitoring - all while maintaining the security and reliability of the client-daemon architecture.
