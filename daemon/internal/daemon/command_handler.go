package daemon

import (
	"encoding/json"
	"fmt"
	"log"
	"os/exec"

	"os"
	"strings"
	"time"

	"nextdeploy/daemon/internal/registry"
	"nextdeploy/daemon/internal/types"
)

type CommandHandler struct {
	dockerClient  *DockerClient
	config        *types.DaemonConfig
	healthMonitor *HealthMonitor
}

func NewCommandHandler(dockerClient *DockerClient, config *types.DaemonConfig) *CommandHandler {
	return &CommandHandler{
		dockerClient: dockerClient,
		config:       config,
	}
}
func (ch *CommandHandler) startApp(args map[string]interface{}) types.Response {
	name, _ := args["name"].(string)

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
		app.RestartCount = 0
	}

	return ch.restartContainer(args)
}
func (ch *CommandHandler) getAppLogs(name string, lines int) ([]string, error) {
	cmd := exec.Command("docker", "logs", "--tail", fmt.Sprintf("%d", lines), name)
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	logLines := strings.Split(strings.TrimSpace(string(output)), "\n")
	return logLines, nil
}
func (ch *CommandHandler) removeApp(args map[string]interface{}) types.Response {
	name, _ := args["name"].(string)

	// Stop container and remove from monitoring
	if app, exists := ch.healthMonitor.monitoredApps[name]; exists {
		app.DesiredState = "removed"
		delete(ch.healthMonitor.monitoredApps, name)
	}

	return ch.removeContainer(args)
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

func (ch *CommandHandler) HandleCommand(cmd types.Command) types.Response {
	switch cmd.Type {
	case "swapcontainers":
		return ch.swapContainers(cmd.Args)
	case "listcontainers":
		return ch.listContainers(cmd.Args)
	case "deploy":
		return ch.deployContainer(cmd.Args)
	case "status":
		return ch.getStatus()
	case "restart":
		return ch.restartContainer(cmd.Args)
	case "logs":
		return ch.getContainerLogs(cmd.Args)
	case "stop":
		return ch.stopContainer(cmd.Args)
	case "start":
		return ch.startContainer(cmd.Args)
	case "remove":
		return ch.removeContainer(cmd.Args)
	case "pull":
		return ch.pullImage(cmd.Args)
	case "inspect":
		return ch.inspectContainer(cmd.Args)
	case "health":
		return ch.healthCheck(cmd.Args)
	case "rollback":
		return ch.rollbackContainer(cmd.Args)
	case "setupCaddy":
		return ch.setUpCaddy(cmd.Args)
	case "stopdaemon":
		return ch.stopDaemon(cmd.Args)
	case "restartDaemon":
		return ch.restartDaemon(cmd.Args)
	default:
		return types.Response{
			Success: false,
			Message: fmt.Sprintf("Unknown command: %s", cmd.Type),
		}
	}
}

func (ch *CommandHandler) stopDaemon(args map[string]interface{}) types.Response {
	fmt.Println("Stopping daemon...")
	ch.Shutdown()
	return types.Response{
		Success: true,
		Message: "Daemon stopped successfully",
	}
}
func (ch *CommandHandler) restartDaemon(args map[string]interface{}) types.Response {
	fmt.Println("Restarting daemon...")
	ch.Shutdown()
	time.Sleep(2 * time.Second) // wait for shutdown
	// restart the process
	execPath, err := os.Executable()
	if err != nil {
		return types.Response{
			Success: false,
			Message: fmt.Sprintf("Failed to get executable path: %v", err),
		}
	}
	cmd := exec.Command(execPath, "--foreground=true", "--config="+ch.config.ConfigPath)
	err = cmd.Start()
	if err != nil {
		return types.Response{
			Success: false,
			Message: fmt.Sprintf("Failed to restart daemon: %v", err),
		}
	}
	return types.Response{
		Success: true,
		Message: "Daemon restarted successfully",
	}
}
func (ch *CommandHandler) Shutdown() {
	// Perform any necessary cleanup here
	log.Println("Shutting down daemon...")
	cmd, err := os.FindProcess(os.Getpid())
	if err != nil {
		log.Printf("Error finding process: %v", err)
		return
	}
	err = cmd.Signal(os.Interrupt)
	if err != nil {
		log.Printf("Error sending interrupt signal: %v", err)
		return
	}
	log.Println("Daemon shutdown complete.")
}
func (ch *CommandHandler) ValidateCommand(cmd types.Command) error {
	allowedCommands := []string{
		"swapcontainers", "listcontainers", "deploy", "status",
		"restart", "logs", "stop", "start", "remove", "pull",
		"inspect", "health", "rollback",
		"setupCaddy",
	}

	for _, allowed := range allowedCommands {
		if cmd.Type == allowed {
			return nil
		}
	}

	return fmt.Errorf("command not allowed: %s", cmd.Type)
}

func (ch *CommandHandler) swapContainers(args map[string]interface{}) types.Response {
	fromContainer, ok1 := args["from"].(string)
	toContainer, ok2 := args["to"].(string)

	if !ok1 || !ok2 {
		return types.Response{
			Success: false,
			Message: "swapcontainers requires 'from' and 'to' arguments",
		}
	}

	// Get container details first
	fromInfo, err := ch.dockerClient.GetContainerInfo(fromContainer)
	if err != nil {
		return types.Response{
			Success: false,
			Message: fmt.Sprintf("Failed to get info for container %s: %v", fromContainer, err),
		}
	}

	toInfo, err := ch.dockerClient.GetContainerInfo(toContainer)
	if err != nil {
		return types.Response{
			Success: false,
			Message: fmt.Sprintf("Failed to get info for container %s: %v", toContainer, err),
		}
	}

	// Stop both containers
	if err := ch.dockerClient.ExecuteCommand("stop", fromContainer); err != nil {
		return types.Response{Success: false, Message: fmt.Sprintf("Failed to stop %s: %v", fromContainer, err)}
	}

	if err := ch.dockerClient.ExecuteCommand("stop", toContainer); err != nil {
		return types.Response{Success: false, Message: fmt.Sprintf("Failed to stop %s: %v", toContainer, err)}
	}

	// Rename containers (swap names)
	tempName := fromContainer + "-temp-" + fmt.Sprintf("%d", time.Now().Unix())

	if err := ch.dockerClient.ExecuteCommand("rename", fromContainer, tempName); err != nil {
		return types.Response{Success: false, Message: fmt.Sprintf("Failed to rename %s: %v", fromContainer, err)}
	}

	if err := ch.dockerClient.ExecuteCommand("rename", toContainer, fromContainer); err != nil {
		return types.Response{Success: false, Message: fmt.Sprintf("Failed to rename %s: %v", toContainer, err)}
	}

	if err := ch.dockerClient.ExecuteCommand("rename", tempName, toContainer); err != nil {
		return types.Response{Success: false, Message: fmt.Sprintf("Failed to rename %s: %v", tempName, err)}
	}

	// Start containers if they were running
	if fromInfo.Status == "running" {
		ch.dockerClient.ExecuteCommand("start", toContainer)
	}
	if toInfo.Status == "running" {
		ch.dockerClient.ExecuteCommand("start", fromContainer)
	}

	return types.Response{
		Success: true,
		Message: fmt.Sprintf("Successfully swapped containers %s and %s", fromContainer, toContainer),
	}
}

func (ch *CommandHandler) listContainers(args map[string]interface{}) types.Response {
	showAll := false
	if all, ok := args["all"].(bool); ok {
		showAll = all
	}

	var cmd *exec.Cmd

	if showAll {
		cmd = exec.Command("docker", "ps", "-a", "--format", "table {{.ID}}\t{{.Names}}\t{{.Image}}\t{{.Status}}\t{{.Ports}}\t{{.CreatedAt}}")
	} else {
		cmd = exec.Command("docker", "ps", "--format", "table {{.ID}}\t{{.Names}}\t{{.Image}}\t{{.Status}}\t{{.Ports}}\t{{.CreatedAt}}")
	}
	output, err := cmd.Output()
	if err != nil {
		return types.Response{
			Success: false,
			Message: fmt.Sprintf("Failed to list containers: %v", err),
		}
	}

	//parse container info
	containers := ch.dockerClient.ParseContainerList(string(output))
	return types.Response{
		Success: true,
		Message: "Container list retrieved successfully",
		Data:    containers,
	}
}

func (ch *CommandHandler) deployContainer(args map[string]interface{}) types.Response {
	image, ok1 := args["image"].(string)
	if !ok1 {
		return types.Response{
			Success: false,
			Message: "deploy requires 'image' argument",
		}
	}

	containerName, ok2 := args["name"].(string)
	if !ok2 {
		containerName = ch.config.ContainerPrefix + strings.ReplaceAll(image, ":", "-")
	}

	// Build docker run command
	dockerArgs := []string{"run", "-d", "--name", containerName}

	// Add ports if specified
	if ports, ok := args["ports"].([]interface{}); ok {
		for _, port := range ports {
			if portStr, ok := port.(string); ok {
				dockerArgs = append(dockerArgs, "-p", portStr)
			}
		}
	}

	// Add environment variables if specified
	if envVars, ok := args["env"].([]interface{}); ok {
		for _, env := range envVars {
			if envStr, ok := env.(string); ok {
				dockerArgs = append(dockerArgs, "-e", envStr)
			}
		}
	}

	// Add volumes if specified
	if volumes, ok := args["volumes"].([]interface{}); ok {
		for _, volume := range volumes {
			if volStr, ok := volume.(string); ok {
				dockerArgs = append(dockerArgs, "-v", volStr)
			}
		}
	}

	// Add restart policy
	restartPolicy := "unless-stopped"
	if restart, ok := args["restart"].(string); ok {
		restartPolicy = restart
	}
	dockerArgs = append(dockerArgs, "--restart", restartPolicy)

	// Add the image
	dockerArgs = append(dockerArgs, image)

	// Add command if specified
	if command, ok := args["command"].(string); ok {
		dockerArgs = append(dockerArgs, strings.Fields(command)...)
	}

	cmd := exec.Command("docker", dockerArgs...)
	output, err := cmd.Output()
	if err != nil {
		return types.Response{
			Success: false,
			Message: fmt.Sprintf("Failed to deploy container: %v", err),
		}
	}

	containerID := strings.TrimSpace(string(output))

	return types.Response{
		Success: true,
		Message: fmt.Sprintf("Successfully deployed container %s", containerName),
		Data: map[string]string{
			"container_id":   containerID,
			"container_name": containerName,
			"image":          image,
		},
	}
}

func (ch *CommandHandler) getStatus() types.Response {
	// get docker system info

	cmd := exec.Command("docker", "info", "system", "df")
	output, err := cmd.Output()

	if err != nil {
		return types.Response{
			Success: false,
			Message: fmt.Sprintf("Failed to get docker system info: %v", err),
		}
	}

	// get container counts
	runningCmd := exec.Command("docker", "ps", "-q")

	runningOutput, err := runningCmd.Output()

	if string(runningOutput) == "" {
		runningOutput = []byte("0")
	}

	allCmd := exec.Command("docker", "ps", "-a", "-q")
	allOutput, err := allCmd.Output()

	if string(allOutput) == "" {
		allOutput = []byte("0")
	}

	status := map[string]string{
		"daemon_status":      "healthy",
		"docker_accessible":  "yes",
		"containers_running": strings.TrimSpace(string(runningOutput)),
		"containers_total":   ch.config.SocketPath,
		"docker_system_info": strings.TrimSpace(string(output)),
	}

	return types.Response{
		Success: true,
		Message: "Daemon status retrieved successfully",
		Data:    status,
	}
}

func (ch *CommandHandler) restartContainer(args map[string]interface{}) types.Response {
	container, ok := args["container"].(string)
	if !ok {
		return types.Response{
			Success: false,
			Message: "restart requires 'container' argument",
		}
	}

	if err := ch.dockerClient.ExecuteCommand("restart", container); err != nil {
		return types.Response{
			Success: false,
			Message: fmt.Sprintf("Failed to restart container %s: %v", container, err),
		}
	}
	return types.Response{}
}

func (ch *CommandHandler) getContainerLogs(args map[string]interface{}) types.Response {
	container, ok1 := args["container"].(string)
	if !ok1 {
		return types.Response{
			Success: false,
			Message: "logs requires 'container' argument",
		}
	}

	lines := "100" // default
	if linesArg, ok := args["lines"]; ok {
		lines = fmt.Sprintf("%v", linesArg)
	}

	follow := false
	if followArg, ok := args["follow"].(bool); ok {
		follow = followArg
	}

	dockerArgs := []string{"logs", "--tail", lines}
	if follow {
		dockerArgs = append(dockerArgs, "-f")
	}
	dockerArgs = append(dockerArgs, container)

	cmd := exec.Command("docker", dockerArgs...)
	output, err := cmd.Output()
	if err != nil {
		return types.Response{
			Success: false,
			Message: fmt.Sprintf("Failed to get logs for %s: %v", container, err),
		}
	}

	logLines := strings.Split(strings.TrimSpace(string(output)), "\n")

	return types.Response{
		Success: true,
		Message: fmt.Sprintf("Retrieved %d log lines for %s", len(logLines), container),
		Data:    logLines,
	}
}

func (ch *CommandHandler) stopContainer(args map[string]interface{}) types.Response {
	container, ok := args["container"].(string)
	if !ok {
		return types.Response{
			Success: false,
			Message: "stop requires 'container' argument",
		}
	}
	if err := ch.dockerClient.ExecuteCommand("stop", container); err != nil {
		return types.Response{
			Success: false,
			Message: fmt.Sprintf("Failed to stop container %s: %v", container, err),
		}
	}
	return types.Response{
		Success: true,
		Message: fmt.Sprintf("Successfully stopped container %s", container),
	}
}

func (ch *CommandHandler) startContainer(args map[string]interface{}) types.Response {
	container, ok := args["container"].(string)
	if !ok {
		return types.Response{
			Success: false,
			Message: "start requires 'container' argument",
		}
	}
	if err := ch.dockerClient.ExecuteCommand("start", container); err != nil {
		return types.Response{
			Success: false,
			Message: fmt.Sprintf("Failed to start container %s: %v", container, err),
		}
	}
	return types.Response{
		Success: true,
		Message: fmt.Sprintf("Successfully started container %s", container),
	}
}

func (ch *CommandHandler) removeContainer(args map[string]interface{}) types.Response {
	container, ok := args["container"].(string)
	if !ok {
		return types.Response{
			Success: false,
			Message: "remove requires 'container' argument",
		}
	}

	force := false
	if forceArg, ok := args["force"].(bool); ok {
		force = forceArg
	}
	dockerArgs := []string{"rm"}
	if force {
		dockerArgs = append(dockerArgs, "-f")
	}
	dockerArgs = append(dockerArgs, container)
	if err := ch.dockerClient.ExecuteCommand(dockerArgs...); err != nil {
		return types.Response{
			Success: false,
			Message: fmt.Sprintf("Failed to remove container %s: %v", container, err),
		}
	}
	return types.Response{
		Success: true,
		Message: fmt.Sprintf("Successfully removed container %s", container),
	}
}

func (ch *CommandHandler) pullImage(args map[string]interface{}) types.Response {
	image, ok := args["image"].(string)
	if !ok {
		return types.Response{
			Success: false,
			Message: "pull requires 'image' argument",
		}
		newApp := false
		if newAppArg, ok := args["newapp"].(bool); ok {
			newApp = newAppArg
		}
		blueGreen := false
		if blueGreenArg, ok := args["bluegreen"].(bool); ok {
			blueGreen = blueGreenArg
		}
		log.Printf("Pulling image %s (newApp=%v, blueGreen=%v)", image, newApp, blueGreen)

		// handle authentication for registry
		// TODO: handle registry types better
		reg := registry.GetRegistryType()
		if reg == "digitalocean" {
			err := registry.HandleDigitalOceanRegistryAuth()
			if err != nil {

				return types.Response{
					Success: false,
					Message: fmt.Sprintf("Failed to authenticate with DigitalOcean registry: %v", err),
				}
			}
		}
		// execute docker pull
		cmd := exec.Command("docker", "pull", image)
		output, err := cmd.CombinedOutput()
		if err != nil {
			return types.Response{
				Success: false,
				Message: fmt.Sprintf("Failed to pull image %s: %v - %s", image, err, string(output)),
			}
		}
		// if this is a new app or blue-green deployment, tag the image with the container
		if newApp || blueGreen {
			newimagetag := generateNewImageTag(image, newApp, blueGreen)
			if err := ch.dockerClient.ExecuteCommand("tag", image, newimagetag); err != nil {
				return types.Response{
					Success: false,
					Message: fmt.Sprintf("Failed to tag image %s as %s: %v", image, newimagetag, err),
				}
			}
		}
		return types.Response{
			Success: true,
			Message: fmt.Sprintf("Successfully pulled image: %s", image),
			Data: map[string]interface{}{
				"image":       image,
				"newapp":      newApp,
				"bluegreen":   blueGreen,
				"pull_output": string(output),
			},
		}
	}
	return types.Response{
		Success: false,
		Message: "pull requires 'image' argument",
	}
}

func (ch *CommandHandler) switchContainers(args map[string]interface{}) types.Response {
	// Validate required arguments
	currentContainer, ok1 := args["current"].(string)
	newContainer, ok2 := args["new"].(string)
	if !ok1 || !ok2 {
		return types.Response{
			Success: false,
			Message: "switch requires 'current' and 'new' arguments",
		}
	}

	// Parse optional arguments with defaults
	newApp := false
	if newAppArg, ok := args["newapp"].(bool); ok {
		newApp = newAppArg
	}

	blueGreen := false
	if blueGreenArg, ok := args["bluegreen"].(bool); ok {
		blueGreen = blueGreenArg
	}

	log.Printf("Switching containers: current=%s, new=%s, newApp=%v, blueGreen=%v",
		currentContainer, newContainer, newApp, blueGreen)

	// Validate container existence
	if !ch.dockerClient.ContainerExists(newContainer) {
		return types.Response{
			Success: false,
			Message: fmt.Sprintf("New container %s does not exist", newContainer),
		}
	}

	// Validate current container exists (if not a new app)
	if !newApp && !ch.dockerClient.ContainerExists(currentContainer) {
		return types.Response{
			Success: false,
			Message: fmt.Sprintf("Current container %s does not exist", currentContainer),
		}
	}

	// Handle deployment strategy
	if blueGreen {
		return ch.handleBlueGreenSwitch(currentContainer, newContainer, newApp)
	}

	// Handle standard deployment
	if response := ch.handleStandardSwitch(currentContainer, newContainer, newApp); !response.Success {
		return response
	}

	return types.Response{
		Success: true,
		Message: fmt.Sprintf("Successfully switched from %s to %s", currentContainer, newContainer),
	}
}

func (ch *CommandHandler) setUpCaddy(args map[string]interface{}) types.Response {
	setup, ok := args["setup"].(bool)
	if !ok || !setup {
		return types.Response{
			Success: false,
			Message: "setupCaddy requires 'setup' argument set to true",
		}
	}

	log.Println("Setting up Caddy server...")
	// read cdaddy file from ~/app/.nextdeploy/caddy/Caddyfile
	caddyfilePath := "~/app/.nextdeploy/caddy/Caddyfile"
	caddyfileContent, err := os.ReadFile(caddyfilePath)
	if err != nil {
		return types.Response{
			Success: false,
			Message: fmt.Sprintf("Failed to read Caddyfile: %v", err),
		}
	}
	// write Caddyfile to /etc/caddy/Caddyfile
	err = os.WriteFile("/etc/caddy/Caddyfile", caddyfileContent, 0644)
	if err != nil {
		return types.Response{
			Success: false,
			Message: fmt.Sprintf("Failed to write Caddyfile to /etc/caddy: %v", err),
		}
	}
	// restart caddy service
	var output []byte
	cmd := exec.Command("caddy", "reload", "--config", "/etc/caddy/Caddyfile")
	output, err = cmd.CombinedOutput()
	if err != nil {
		return types.Response{
			Success: false,
			Message: fmt.Sprintf("Failed to reload Caddy: %v - %s", err, string(output)),
		}
	}
	log.Println("Caddy server setup and reloaded successfully.")
	// start the caddy service if not running
	cmd = exec.Command("systemctl", "start", "caddy")
	output, err = cmd.CombinedOutput()
	if err != nil {
		return types.Response{
			Success: false,
			Message: fmt.Sprintf("Failed to start Caddy service: %v - %s", err, string(output)),
		}
	}
	return types.Response{
		Success: true,
		Message: "Caddy server setup and started successfully",
	}
}
func generateNewImageTag(image string, newApp bool, blueGreen bool) string {
	timestamp := time.Now().Format("20060102-150405")
	if newApp {
		return fmt.Sprintf("%s:%s", image, timestamp)
	}
	if blueGreen {
		return fmt.Sprintf("%s:bluegreen-%s", image, timestamp)
	}
	return fmt.Sprintf("%s:updated-%s", image, timestamp)
}
func (ch *CommandHandler) inspectContainer(args map[string]interface{}) types.Response {
	container, ok := args["container"].(string)
	if !ok {
		return types.Response{
			Success: false,
			Message: "inspect requires 'container' argument",
		}
	}

	cmd := exec.Command("docker", "inspect", container)
	output, err := cmd.Output()
	if err != nil {
		return types.Response{
			Success: false,
			Message: fmt.Sprintf("Failed to inspect container %s: %v", container, err),
		}
	}

	var inspectData []map[string]interface{}
	if err := json.Unmarshal(output, &inspectData); err != nil {
		return types.Response{
			Success: false,
			Message: fmt.Sprintf("Failed to parse inspect data for %s: %v", container, err),
		}
	}
	return types.Response{
		Success: true,
		Message: fmt.Sprintf("Successfully inspected container %s", container),
		Data:    inspectData,
	}
}

func (ch *CommandHandler) healthCheck(args map[string]interface{}) types.Response {
	container, ok := args["container"].(string)
	if !ok {
		// system health check
		return ch.getStatus()
	}

	cmd := exec.Command("docker", "inspect", "--format", "{{.State.Health.Status}}", container)
	output, err := cmd.Output()
	if err != nil {
		return types.Response{
			Success: false,
			Message: fmt.Sprintf("Failed to get health status for container %s: %v", container, err),
		}
	}

	healthStatus := strings.TrimSpace(string(output))

	healthy := healthStatus == "healthy" || healthStatus == "none"
	return types.Response{
		Success: healthy,
		Message: fmt.Sprintf("Container %s health status: %s", container, healthStatus),
		Data: map[string]string{
			"container":     container,
			"health_status": healthStatus,
			"healthy":       fmt.Sprintf("%v", healthy),
		},
	}
}

func (ch *CommandHandler) rollbackContainer(args map[string]interface{}) types.Response {
	container, ok1 := args["container"].(string)
	if !ok1 {
		return types.Response{
			Success: false,
			Message: "rollback requires 'container' argument",
		}
	}

	// look for previous container
	previousContainer := container + "-previous"

	// Check if previous container exists
	cmd := exec.Command("docker", "ps", "-a", "--filter", "name="+previousContainer, "--format", "{{.Names}}")
	output, err := cmd.Output()
	if err != nil || strings.TrimSpace(string(output)) == "" {
		return types.Response{
			Success: false,
			Message: fmt.Sprintf("No previous container found for %s", container),
		}
	}
	// Stop current container
	ch.dockerClient.ExecuteCommand("stop", container)
	// Rename current container to temp
	backupname := container + "-backup-" + fmt.Sprintf("%d", time.Now().Unix())
	ch.dockerClient.ExecuteCommand("rename", container, backupname)

	// Rename previous container to current
	ch.dockerClient.ExecuteCommand("rename", previousContainer, container)
	// start the rolled back container
	ch.dockerClient.ExecuteCommand("start", container)
	return types.Response{
		Success: true,
		Message: fmt.Sprintf("Successfully rolled back container %s", container),
		Data: map[string]string{
			"rolled_back_to": container,
			"backup_of":      backupname,
		},
	}

}

func (ch *CommandHandler) handleBlueGreenSwitch(currentContainer string, newContainer string, newApp bool) types.Response {
	// Blue-green deployment strategy
	// 1. Deploy new container alongside current
	// 2. Switch traffic to new container (handled externally, e.g. via load balancer)
	// 3. Stop and remove old container (retain for rollback if needed)

	// Deploy new container with different ports if needed
	newContainerName := fmt.Sprintf("%s-bluegreen-%d", newContainer, time.Now().Unix())
	deployArgs := map[string]interface{}{
		"image": newContainer,
		"name":  newContainerName,
		"ports": []string{"3000:3001"},
	}

	if newApp {
		deployArgs["env"] = []string{"NEW_APP=true"}
	}
	deployReponse := ch.deployContainer(deployArgs)
	if !deployReponse.Success {
		return deployReponse
	}

	// wait for new container to be healthy
	time.Sleep(10 * time.Second)
	healthResponse := ch.healthCheck(map[string]interface{}{"container": newContainerName})
	if !healthResponse.Success {
		return types.Response{
			Success: false,
			Message: fmt.Sprintf("New container %s is not healthy: %s", newContainerName, healthResponse.Message),
		}
	}
	// stop old container
	if err := ch.dockerClient.ExecuteCommand("stop", currentContainer); err != nil {
		return types.Response{
			Success: false,
			Message: fmt.Sprintf("Failed to stop current container %s: %v", currentContainer, err),
		}
	}
	// rename old container to previous for rollback
	previousContainer := currentContainer + "-previous"
	if err := ch.dockerClient.ExecuteCommand("rename", currentContainer, previousContainer); err != nil {
		return types.Response{
			Success: false,
			Message: fmt.Sprintf("Failed to rename current container %s to %s: %v", currentContainer, previousContainer, err),
		}
	}
	return types.Response{
		Success: true,
		Message: fmt.Sprintf("Successfully switched to new container %s using blue-green deployment", newContainerName),
		Data: map[string]string{
			"old_container":     previousContainer,
			"new_container":     newContainerName,
			"deployment_method": "blue-green",
		},
	}
}

func (ch *CommandHandler) handleStandardSwitch(currentContainer, newContainer string, newApp bool) types.Response {
	// Standard deployment strategy
	// 1. Stop current container
	// 2. Start new container

	// Stop current container
	if err := ch.dockerClient.ExecuteCommand("stop", currentContainer); err != nil {
		return types.Response{
			Success: false,
			Message: fmt.Sprintf("Failed to stop current container %s: %v", currentContainer, err),
		}
	}

	// Start new container
	deployArgs := map[string]interface{}{
		"image": newContainer,
		"name":  currentContainer, // Reuse the same name
	}

	if newApp {
		deployArgs["env"] = []string{"NEW_APP=true"}
	}

	deployResponse := ch.deployContainer(deployArgs)
	if !deployResponse.Success {
		// Try to restart old container if new deployment fails
		ch.dockerClient.ExecuteCommand("start", currentContainer)
		return deployResponse
	}

	return types.Response{
		Success: true,
		Message: fmt.Sprintf("Standard switch completed: %s -> %s", currentContainer, newContainer),
		Data: map[string]interface{}{
			"old_container": currentContainer,
			"new_container": newContainer,
			"strategy":      "standard",
		},
	}
}
