package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

var (
	logger      *log.Logger
	logFile     *os.File
	logFilePath string
	logConfig   LoggerConfig
)

func init() {
	fmt.Println("Initializing NextDeploy Daemon...")

	// default config
	logConfig = LoggerConfig{
		LogDir:      "/var/log/nextdeployd",
		LogFileName: "nextdeployd.log",
		MaxFileSize: 10 * 1024 * 1024, // 10 MB
		MaxBackups:  5,
	}
	// create log directory if not exists
	if err := os.MkdirAll(logConfig.LogDir, 0755); err != nil {
		fmt.Printf("Failed to create log directory: %v\n", err)
		os.Exit(1)
	}
}

// init for file handling

func init() {
	logFilePath = filepath.Join(logConfig.LogDir, logConfig.LogFileName)
	// open log file(append mode, create if not exists)
	var err error
	logFile, err = os.OpenFile(logFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Printf("Failed to open log file: %v\n", err)
		os.Exit(1)
	}
	// create multi-writer for both file and std out
	multiWriter := io.MultiWriter(os.Stdout, logFile)

	// initialize logger with timestamp and file/line info
	logger = log.New(multiWriter, "NEXTDEPLOY: ", log.LstdFlags|log.Lshortfile)

	logger.Println("Logger initialized")
}

// Command types that the daemon can handle
func NewNextDeployDaemon(configPath string) (*NextDeployDaemon, error) {
	config := &DaemonConfig{
		SocketPath:      "/var/run/nextdeployd.sock",
		SocketMode:      "0660",
		DockerSocket:    "/var/run/docker.sock",
		ContainerPrefix: "nextdeploy-",
		LogLevel:        "info",
		LogDir:          "/var/log/nextdeployd", // Default
		LogMaxSize:      10,                     // Default 10MB
		LogMaxBackups:   5,                      // Default 5 backups

	}

	// Load config if exists
	if configPath != "" {
		if err := loadConfig(configPath, config); err != nil {
			log.Printf("Warning: Could not load config file: %v", err)
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	return &NextDeployDaemon{
		ctx:        ctx,
		cancel:     cancel,
		socketPath: config.SocketPath,
		config:     config,
	}, nil
}

func (d *NextDeployDaemon) Start() error {
	// Check if we can access Docker
	if err := d.checkDockerAccess(); err != nil {
		LogError(logger, logConfig, "Docker access check failed")
		return fmt.Errorf("docker access check failed: %w", err)
	}

	// Remove existing socket file if it exists
	os.Remove(d.socketPath)

	// Create Unix domain socket
	listener, err := net.Listen("unix", d.socketPath)
	if err != nil {
		return fmt.Errorf("failed to create socket: %w", err)
	}
	d.listener = listener

	// Set socket permissions (only local access)
	if err := d.setSocketPermissions(); err != nil {
		return fmt.Errorf("failed to set socket permissions: %w", err)
	}

	log.Printf("NextDeploy daemon started, listening on %s", d.socketPath)
	log.Printf("Docker socket: %s", d.config.DockerSocket)

	// Start accepting connections
	go d.acceptConnections()

	// Handle signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	for {
		select {
		case sig := <-sigCh:
			switch sig {
			case syscall.SIGHUP:
				log.Println("Received SIGHUP, reloading config...")
				// Reload config logic here
			case syscall.SIGINT, syscall.SIGTERM:
				log.Printf("Received signal: %v", sig)
				d.Shutdown()
				return nil
			}
		case <-d.ctx.Done():
			return nil
		}
	}
}

func (d *NextDeployDaemon) checkDockerAccess() error {
	cmd := exec.Command("docker", "version")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker not accessible: %w", err)
	}
	return nil
}

func (d *NextDeployDaemon) setSocketPermissions() error {
	// Set restrictive permissions - only root and specified users/groups
	if err := os.Chmod(d.socketPath, 0660); err != nil {
		return err
	}

	// Get the socket directory and ensure it's secure
	socketDir := filepath.Dir(d.socketPath)
	if err := os.Chmod(socketDir, 0755); err != nil {
		log.Printf("Warning: Could not set directory permissions: %v", err)
	}

	log.Printf("Socket permissions set to 0660 on %s", d.socketPath)
	return nil
}

func (d *NextDeployDaemon) acceptConnections() {
	for {
		conn, err := d.listener.Accept()
		if err != nil {
			select {
			case <-d.ctx.Done():
				return
			default:
				log.Printf("Error accepting connection: %v", err)
				continue
			}
		}

		go d.handleConnection(conn)
	}
}

func (d *NextDeployDaemon) handleConnection(conn net.Conn) {
	defer conn.Close()

	// Set connection timeout
	conn.SetDeadline(time.Now().Add(30 * time.Second))

	decoder := json.NewDecoder(conn)
	encoder := json.NewEncoder(conn)

	var cmd Command
	if err := decoder.Decode(&cmd); err != nil {
		log.Printf("Error decoding command: %v", err)
		return
	}

	log.Printf("Received command: %s with args: %v", cmd.Type, cmd.Args)

	// Validate command
	if err := d.validateCommand(cmd); err != nil {
		response := Response{
			Success: false,
			Message: fmt.Sprintf("Command validation failed: %v", err),
		}
		encoder.Encode(response)
		return
	}

	response := d.executeCommand(cmd)

	if err := encoder.Encode(response); err != nil {
		log.Printf("Error sending response: %v", err)
	}
}

func (d *NextDeployDaemon) validateCommand(cmd Command) error {
	// Basic command validation
	allowedCommands := []string{
		"swapcontainers", "listcontainers", "deploy", "status",
		"restart", "logs", "stop", "start", "remove", "pull",
		"inspect", "health", "rollback",
	}

	for _, allowed := range allowedCommands {
		if cmd.Type == allowed {
			return nil
		}
	}

	return fmt.Errorf("command not allowed: %s", cmd.Type)
}

func (d *NextDeployDaemon) executeCommand(cmd Command) Response {
	switch cmd.Type {
	case "swapcontainers":
		return d.swapContainers(cmd.Args)
	case "listcontainers":
		return d.listContainers(cmd.Args)
	case "deploy":
		return d.deployContainer(cmd.Args)
	case "status":
		return d.getStatus()
	case "restart":
		return d.restartContainer(cmd.Args)
	case "logs":
		return d.getContainerLogs(cmd.Args)
	case "stop":
		return d.stopContainer(cmd.Args)
	case "start":
		return d.startContainer(cmd.Args)
	case "remove":
		return d.removeContainer(cmd.Args)
	case "pull":
		return d.pullImage(cmd.Args)
	case "inspect":
		return d.inspectContainer(cmd.Args)
	case "health":
		return d.healthCheck(cmd.Args)
	case "rollback":
		return d.rollbackContainer(cmd.Args)
	default:
		return Response{
			Success: false,
			Message: fmt.Sprintf("Unknown command: %s", cmd.Type),
		}
	}
}

// Docker command implementations
func (d *NextDeployDaemon) swapContainers(args map[string]interface{}) Response {
	fromContainer, ok1 := args["from"].(string)
	toContainer, ok2 := args["to"].(string)

	if !ok1 || !ok2 {
		return Response{
			Success: false,
			Message: "swapcontainers requires 'from' and 'to' arguments",
		}
	}

	log.Printf("Swapping containers: %s <-> %s", fromContainer, toContainer)

	// Get container details first
	fromInfo, err := d.getContainerInfo(fromContainer)
	if err != nil {
		return Response{
			Success: false,
			Message: fmt.Sprintf("Failed to get info for container %s: %v", fromContainer, err),
		}
	}

	toInfo, err := d.getContainerInfo(toContainer)
	if err != nil {
		return Response{
			Success: false,
			Message: fmt.Sprintf("Failed to get info for container %s: %v", toContainer, err),
		}
	}

	// Stop both containers
	if err := d.dockerCommand("stop", fromContainer); err != nil {
		return Response{Success: false, Message: fmt.Sprintf("Failed to stop %s: %v", fromContainer, err)}
	}

	if err := d.dockerCommand("stop", toContainer); err != nil {
		return Response{Success: false, Message: fmt.Sprintf("Failed to stop %s: %v", toContainer, err)}
	}

	// Rename containers (swap names)
	tempName := fromContainer + "-temp-" + fmt.Sprintf("%d", time.Now().Unix())

	if err := d.dockerCommand("rename", fromContainer, tempName); err != nil {
		return Response{Success: false, Message: fmt.Sprintf("Failed to rename %s: %v", fromContainer, err)}
	}

	if err := d.dockerCommand("rename", toContainer, fromContainer); err != nil {
		return Response{Success: false, Message: fmt.Sprintf("Failed to rename %s: %v", toContainer, err)}
	}

	if err := d.dockerCommand("rename", tempName, toContainer); err != nil {
		return Response{Success: false, Message: fmt.Sprintf("Failed to rename %s: %v", tempName, err)}
	}

	// Start containers if they were running
	if fromInfo.Status == "running" {
		d.dockerCommand("start", toContainer)
	}
	if toInfo.Status == "running" {
		d.dockerCommand("start", fromContainer)
	}

	return Response{
		Success: true,
		Message: fmt.Sprintf("Successfully swapped containers %s and %s", fromContainer, toContainer),
	}
}

func (d *NextDeployDaemon) listContainers(args map[string]interface{}) Response {
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
		return Response{
			Success: false,
			Message: fmt.Sprintf("Failed to list containers: %v", err),
		}
	}

	// Parse container info
	containers := d.parseContainerList(string(output))

	return Response{
		Success: true,
		Message: fmt.Sprintf("Found %d containers", len(containers)),
		Data:    containers,
	}
}

func (d *NextDeployDaemon) deployContainer(args map[string]interface{}) Response {
	image, ok1 := args["image"].(string)
	if !ok1 {
		return Response{
			Success: false,
			Message: "deploy requires 'image' argument",
		}
	}

	containerName, ok2 := args["name"].(string)
	if !ok2 {
		containerName = d.config.ContainerPrefix + strings.ReplaceAll(image, ":", "-")
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

	log.Printf("Deploying container with: docker %v", dockerArgs)

	cmd := exec.Command("docker", dockerArgs...)
	output, err := cmd.Output()
	if err != nil {
		return Response{
			Success: false,
			Message: fmt.Sprintf("Failed to deploy container: %v", err),
		}
	}

	containerID := strings.TrimSpace(string(output))

	return Response{
		Success: true,
		Message: fmt.Sprintf("Successfully deployed container %s", containerName),
		Data: map[string]string{
			"container_id":   containerID,
			"container_name": containerName,
			"image":          image,
		},
	}
}

func (d *NextDeployDaemon) getStatus() Response {
	// Get Docker system info
	cmd := exec.Command("docker", "system", "df")
	output, _ := cmd.Output()

	// Get container counts
	runningCmd := exec.Command("docker", "ps", "-q")
	runningOutput, _ := runningCmd.Output()
	runningCount := len(strings.Split(strings.TrimSpace(string(runningOutput)), "\n"))
	if string(runningOutput) == "" {
		runningCount = 0
	}

	allCmd := exec.Command("docker", "ps", "-aq")
	allOutput, _ := allCmd.Output()
	totalCount := len(strings.Split(strings.TrimSpace(string(allOutput)), "\n"))
	if string(allOutput) == "" {
		totalCount = 0
	}

	status := map[string]interface{}{
		"daemon_status":      "healthy",
		"docker_accessible":  true,
		"containers_running": runningCount,
		"containers_total":   totalCount,
		"socket_path":        d.socketPath,
		"docker_system_info": strings.TrimSpace(string(output)),
	}

	return Response{
		Success: true,
		Message: "Daemon status retrieved",
		Data:    status,
	}
}

func (d *NextDeployDaemon) restartContainer(args map[string]interface{}) Response {
	container, ok := args["container"].(string)
	if !ok {
		return Response{
			Success: false,
			Message: "restart requires 'container' argument",
		}
	}

	if err := d.dockerCommand("restart", container); err != nil {
		return Response{
			Success: false,
			Message: fmt.Sprintf("Failed to restart container %s: %v", container, err),
		}
	}

	return Response{
		Success: true,
		Message: fmt.Sprintf("Successfully restarted container: %s", container),
	}
}

func (d *NextDeployDaemon) getContainerLogs(args map[string]interface{}) Response {
	container, ok1 := args["container"].(string)
	if !ok1 {
		return Response{
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
		return Response{
			Success: false,
			Message: fmt.Sprintf("Failed to get logs for %s: %v", container, err),
		}
	}

	logLines := strings.Split(strings.TrimSpace(string(output)), "\n")

	return Response{
		Success: true,
		Message: fmt.Sprintf("Retrieved %d log lines for %s", len(logLines), container),
		Data:    logLines,
	}
}

func (d *NextDeployDaemon) stopContainer(args map[string]interface{}) Response {
	container, ok := args["container"].(string)
	if !ok {
		return Response{
			Success: false,
			Message: "stop requires 'container' argument",
		}
	}

	if err := d.dockerCommand("stop", container); err != nil {
		return Response{
			Success: false,
			Message: fmt.Sprintf("Failed to stop container %s: %v", container, err),
		}
	}

	return Response{
		Success: true,
		Message: fmt.Sprintf("Successfully stopped container: %s", container),
	}
}

func (d *NextDeployDaemon) startContainer(args map[string]interface{}) Response {
	container, ok := args["container"].(string)
	if !ok {
		return Response{
			Success: false,
			Message: "start requires 'container' argument",
		}
	}

	if err := d.dockerCommand("start", container); err != nil {
		return Response{
			Success: false,
			Message: fmt.Sprintf("Failed to start container %s: %v", container, err),
		}
	}

	return Response{
		Success: true,
		Message: fmt.Sprintf("Successfully started container: %s", container),
	}
}

func (d *NextDeployDaemon) removeContainer(args map[string]interface{}) Response {
	container, ok := args["container"].(string)
	if !ok {
		return Response{
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

	cmd := exec.Command("docker", dockerArgs...)
	if err := cmd.Run(); err != nil {
		return Response{
			Success: false,
			Message: fmt.Sprintf("Failed to remove container %s: %v", container, err),
		}
	}

	return Response{
		Success: true,
		Message: fmt.Sprintf("Successfully removed container: %s", container),
	}
}

func (d *NextDeployDaemon) pullImage(args map[string]interface{}) Response {
	image, ok := args["image"].(string)
	if !ok {
		return Response{
			Success: false,
			Message: "pull requires 'image' argument",
		}
	}

	log.Printf("Pulling image: %s", image)

	// Handle authentication for registries if needed
	err := handleRegistryAuth()
	if err != nil {
		LogError(logger, logConfig, fmt.Sprintf("Registry auth failed: %v", err))
		os.Exit(1)
	}

	cmd := exec.Command("docker", "pull", image)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return Response{
			Success: false,
			Message: fmt.Sprintf("Failed to pull image %s: %v\nOutput: %s", image, err, string(output)),
		}
	}

	return Response{
		Success: true,
		Message: fmt.Sprintf("Successfully pulled image: %s", image),
		Data:    strings.TrimSpace(string(output)),
	}
}

func (d *NextDeployDaemon) inspectContainer(args map[string]interface{}) Response {
	container, ok := args["container"].(string)
	if !ok {
		return Response{
			Success: false,
			Message: "inspect requires 'container' argument",
		}
	}

	cmd := exec.Command("docker", "inspect", container)
	output, err := cmd.Output()
	if err != nil {
		return Response{
			Success: false,
			Message: fmt.Sprintf("Failed to inspect container %s: %v", container, err),
		}
	}

	var inspectData interface{}
	if err := json.Unmarshal(output, &inspectData); err != nil {
		return Response{
			Success: false,
			Message: fmt.Sprintf("Failed to parse inspect output: %v", err),
		}
	}

	return Response{
		Success: true,
		Message: fmt.Sprintf("Container %s inspection data", container),
		Data:    inspectData,
	}
}

func (d *NextDeployDaemon) healthCheck(args map[string]interface{}) Response {
	container, ok := args["container"].(string)
	if !ok {
		// System health check
		return d.getStatus()
	}

	// Container health check
	cmd := exec.Command("docker", "inspect", "--format", "{{.State.Health.Status}}", container)
	output, err := cmd.Output()
	if err != nil {
		return Response{
			Success: false,
			Message: fmt.Sprintf("Failed to check health for %s: %v", container, err),
		}
	}

	healthStatus := strings.TrimSpace(string(output))
	healthy := healthStatus == "healthy" || healthStatus == "none"

	return Response{
		Success: healthy,
		Message: fmt.Sprintf("Container %s health status: %s", container, healthStatus),
		Data: map[string]interface{}{
			"container":     container,
			"health_status": healthStatus,
			"healthy":       healthy,
		},
	}
}

func (d *NextDeployDaemon) rollbackContainer(args map[string]interface{}) Response {
	container, ok1 := args["container"].(string)
	if !ok1 {
		return Response{
			Success: false,
			Message: "rollback requires 'container' argument",
		}
	}

	// Look for previous version container
	previousContainer := container + "-previous"

	// Check if previous version exists
	cmd := exec.Command("docker", "ps", "-a", "--filter", "name="+previousContainer, "--format", "{{.Names}}")
	output, err := cmd.Output()
	if err != nil || strings.TrimSpace(string(output)) == "" {
		return Response{
			Success: false,
			Message: fmt.Sprintf("No previous version found for container %s", container),
		}
	}

	// Stop current container
	d.dockerCommand("stop", container)

	// Rename current to backup
	backupName := container + "-backup-" + fmt.Sprintf("%d", time.Now().Unix())
	d.dockerCommand("rename", container, backupName)

	// Rename previous to current
	d.dockerCommand("rename", previousContainer, container)

	// Start the rolled back container
	if err := d.dockerCommand("start", container); err != nil {
		return Response{
			Success: false,
			Message: fmt.Sprintf("Failed to start rolled back container: %v", err),
		}
	}

	return Response{
		Success: true,
		Message: fmt.Sprintf("Successfully rolled back container %s", container),
		Data: map[string]string{
			"container":      container,
			"backup_created": backupName,
		},
	}
}

// Helper functions
func (d *NextDeployDaemon) dockerCommand(args ...string) error {
	cmd := exec.Command("docker", args...)
	return cmd.Run()
}

func (d *NextDeployDaemon) getContainerInfo(containerName string) (*ContainerInfo, error) {
	cmd := exec.Command("docker", "inspect", "--format", "{{.State.Status}}", containerName)
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	return &ContainerInfo{
		Name:   containerName,
		Status: strings.TrimSpace(string(output)),
	}, nil
}

func (d *NextDeployDaemon) parseContainerList(output string) []map[string]string {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	var containers []map[string]string

	if len(lines) <= 1 { // Only header or empty
		return containers
	}

	for _, line := range lines[1:] { // Skip header
		if strings.TrimSpace(line) == "" {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) >= 6 {
			containers = append(containers, map[string]string{
				"id":      fields[0],
				"name":    fields[1],
				"image":   fields[2],
				"status":  fields[3],
				"ports":   fields[4],
				"created": strings.Join(fields[5:], " "),
			})
		}
	}

	return containers
}

func (d *NextDeployDaemon) Shutdown() {
	log.Println("Shutting down daemon...")
	d.cancel()

	if d.listener != nil {
		d.listener.Close()
	}

	// Clean up socket file
	os.Remove(d.socketPath)
	log.Println("Daemon stopped")
}

// Configuration loading
func loadConfig(configPath string, config *DaemonConfig) error {
	file, err := os.Open(configPath)
	if err != nil {
		return err
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	return decoder.Decode(config)
}

// Client functions for sending commands
func sendCommand(socketPath string, cmd Command) (*Response, error) {
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to daemon: %w", err)
	}
	defer conn.Close()

	// Set timeout
	conn.SetDeadline(time.Now().Add(30 * time.Second))

	encoder := json.NewEncoder(conn)
	decoder := json.NewDecoder(conn)

	// Send command
	if err := encoder.Encode(cmd); err != nil {
		return nil, fmt.Errorf("failed to send command: %w", err)
	}

	// Receive response
	var response Response
	if err := decoder.Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to receive response: %w", err)
	}

	return &response, nil
}

// Command-line interface
func main() {
	config := LoggerConfig{
		LogDir:      "/var/log/nextdeployd",
		LogFileName: "nextdeployd.log",
		MaxFileSize: 10 * 1024 * 1024, // 10 MB
		MaxBackups:  5,
	}
	logger := SetupLogger(config)
	LogInfo(logger, config, "Starting NextDeploy daemon...")
	socketPath := "/var/run/nextdeployd.sock"
	configPath := "/etc/nextdeployd/config.json"

	if len(os.Args) < 2 {
		fmt.Println("NextDeploy Daemon - Docker Container Management")
		fmt.Println("\nUsage:")
		fmt.Println("  Start daemon: nextdeployd daemon [--config=/path/to/config.json]")
		fmt.Println("\nContainer Commands:")
		fmt.Println("  nextdeployd listcontainers [--all=true]")
		fmt.Println("  nextdeployd deploy --image=nginx:latest --name=web-server --ports=80:8080")
		fmt.Println("  nextdeployd start --container=web-server")
		fmt.Println("  nextdeployd stop --container=web-server")
		fmt.Println("  nextdeployd restart --container=web-server")
		fmt.Println("  nextdeployd remove --container=web-server [--force=true]")
		fmt.Println("  nextdeployd logs --container=web-server [--lines=50]")
		fmt.Println("  nextdeployd inspect --container=web-server")
		fmt.Println("\nDeployment Commands:")
		fmt.Println("  nextdeployd swapcontainers --from=app-v1 --to=app-v2")
		fmt.Println("  nextdeployd rollback --container=web-server")
		fmt.Println("  nextdeployd pull --image=nginx:latest")
		fmt.Println("\nHealth & Status:")
		fmt.Println("  nextdeployd status")
		fmt.Println("  nextdeployd health [--container=web-server]")
		os.Exit(1)
	}

	command := os.Args[1]

	if command == "daemon" {
		// Check for config flag
		for _, arg := range os.Args[2:] {
			if strings.HasPrefix(arg, "--config=") {
				configPath = strings.TrimPrefix(arg, "--config=")
				break
			}
		}

		// Run as daemon
		daemon, err := NewNextDeployDaemon(configPath)
		if err != nil {
			log.Fatalf("Failed to create daemon: %v", err)
		}

		if err := daemon.Start(); err != nil {
			log.Fatalf("Daemon failed: %v", err)
		}
		return
	}

	// Client mode - send command to running daemon
	var cmd Command
	args := make(map[string]interface{})

	// Parse command-line arguments
	for i := 2; i < len(os.Args); i++ {
		arg := os.Args[i]
		if strings.HasPrefix(arg, "--") {
			parts := strings.SplitN(arg[2:], "=", 2)
			if len(parts) == 2 {
				// Try to parse as bool or number, otherwise keep as string
				value := parts[1]
				if value == "true" {
					args[parts[0]] = true
				} else if value == "false" {
					args[parts[0]] = false
				} else {
					args[parts[0]] = value
				}
			}
		}
	}

	cmd.Type = command
	cmd.Args = args

	// Send command to daemon
	response, err := sendCommand(socketPath, cmd)
	if err != nil {
		fmt.Printf("Error: %v\n", err)

		// Check if daemon is running
		if strings.Contains(err.Error(), "connect") || strings.Contains(err.Error(), "no such file") {
			fmt.Println("Hint: Is the daemon running? Start it with: nextdeployd daemon")
			fmt.Printf("Socket path: %s\n", socketPath)
		}
		os.Exit(1)
	}

	// Display response
	if response.Success {
		fmt.Printf("✅ %s\n", response.Message)
		if response.Data != nil {
			switch data := response.Data.(type) {
			case []interface{}:
				for _, item := range data {
					if itemMap, ok := item.(map[string]interface{}); ok {
						// Format container list nicely
						for key, value := range itemMap {
							fmt.Printf("  %s: %v\n", key, value)
						}
						fmt.Println()
					} else {
						fmt.Printf("  %v\n", item)
					}
				}
			case map[string]interface{}:
				for key, value := range data {
					fmt.Printf("  %s: %v\n", key, value)
				}
			case []map[string]string:
				// Handle container list format
				if len(data) > 0 {
					fmt.Printf("\n%-12s %-20s %-30s %-15s %-20s\n", "ID", "NAME", "IMAGE", "STATUS", "PORTS")
					fmt.Println(strings.Repeat("-", 100))
					for _, container := range data {
						fmt.Printf("%-12s %-20s %-30s %-15s %-20s\n",
							truncate(container["id"], 12),
							truncate(container["name"], 20),
							truncate(container["image"], 30),
							container["status"],
							truncate(container["ports"], 20))
					}
				}
			case string:
				fmt.Printf("  %s\n", data)
			default:
				if jsonData, err := json.MarshalIndent(data, "  ", "  "); err == nil {
					fmt.Printf("  %s\n", string(jsonData))
				} else {
					fmt.Printf("  %v\n", data)
				}
			}
		}
	} else {
		fmt.Printf("❌ %s\n", response.Message)
		os.Exit(1)
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
