package daemon

import (
	"context"
	"fmt"
	"nextdeploy/daemon/internal/types"
	"time"
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
	DesiredState  string // "running" or "stopped"
	RestartPolicy string // "always", "on-failure", "never"
	MaxRestarts   int
	RestartCount  int
	LastCheck     time.Time
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
	go hm.monitorLoop()
}

func (hm *HealthMonitor) monitorLoop() {
	ticker := time.NewTicker(30 * time.Second)
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
	status, err := hm.dockerClient.getContainerStatus(app.ContainerName)
	if err != nil {
		if app.DesiredState == "running" && app.RestartPolicy != "never" {
			hm.restartContainer(app)
		}
		return
	}
	// check if container is in desired state
	switch status {
	case "exited", "dead":
		if app.DesiredState == "running" && app.shouldRestart() {
			hm.restartContainer(app)
		}
	case "restarting":
		// container is already restarting, do nothing
	case "running":
		// container is running, reset restart count
		if !hm.isApplicationHealthy(app.ContainerName) {
			hm.restartContainer(app)
		}
	}
}
func (hm *HealthMonitor) isApplicationHealthy(containerName string) bool {
	// Placeholder for actual health check logic, e.g., HTTP check, TCP check, etc.
	// For now, we assume the application is healthy if the container is running.
	return true
}
func (hm *HealthMonitor) restartContainer(app *MonitoredApp) {
	app.RestartCount++
	// stop first if running
	hm.dockerClient.ExecuteCommand("docker", "stop", app.ContainerName)
	if err := hm.dockerClient.ExecuteCommand("docker", "start", app.ContainerName); err != nil {
		fmt.Printf("Failed to restart container %s: %v\n", app.ContainerName, err)
	} else {
		fmt.Printf("Restarted container %s\n", app.ContainerName)
	}
}
func (app *MonitoredApp) shouldRestart() bool {
	if app.RestartPolicy == "never" {
		return false
	}
	if app.MaxRestarts > 0 && app.RestartCount >= app.MaxRestarts {
		return false
	}
	return true
}
