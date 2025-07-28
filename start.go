// /*
// NextDeploy Master Control Script
//
// This script provides unified control for:
// 1. Building components (daemon and CLI)
// 2. Running the daemon with proper configuration
// 3. Development environment setup
// 4. Production deployment
//
// USAGE:
//
//	Build: go run start.go build [flags]
//	Run:   go run start.go run [flags]
//	Dev:   go run start.go dev
//	Clean: go run start.go clean
//
// BUILD FLAGS:
//
//	-target  string  Build target: 'daemon', 'cli', or 'all' (default "all")
//	-output  string  Output directory (default "./bin")
//	-version string  Override version number
//
// RUN FLAGS:
//
//	-config string   Config file path
//	-port   string   Server port (default "8080")
//	-host   string   Server host (default "0.0.0.0")
//	-debug           Enable debug mode
//
// ENVIRONMENT VARIABLES:
//
//	NEXTDEPLOY_VERSION  Set build version
//	BUILD_STATIC        Force static linking ("true" or "false")
//
// */
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Constants
const (
	defaultPort        = "8080"
	defaultHost        = "0.0.0.0"
	defaultConfigFile  = "config.yaml"
	defaultOutputDir   = "./bin"
	defaultTarget      = "all"
	defaultVersionFile = "VERSION"
)

// Project paths
var (
	projectRoot = "."
	daemonPath  = filepath.Join(projectRoot, "daemon", "main.go")
	cliPath     = filepath.Join(projectRoot, "cli", "main.go")
	homeDir     = getHomeDir()
)

// Config holds runtime configuration
type Config struct {
	Port    string
	Host    string
	Debug   bool
	LogFile string
	KeyDir  string
	PidFile string
}

// isDaemonRunning checks if the NextDeploy daemon is currently running
func isDaemonRunning() (bool, int, error) {
	// Try systemctl first (Linux systems with systemd)
	if _, err := exec.LookPath("systemctl"); err == nil {
		cmd := exec.Command("systemctl", "is-active", "nextdeploy.service")
		output, err := cmd.CombinedOutput()
		if err == nil && strings.TrimSpace(string(output)) == "active" {
			// Get PID if running
			pidCmd := exec.Command("systemctl", "show", "--property=MainPID", "nextdeploy.service")
			pidOutput, err := pidCmd.Output()
			if err == nil {
				pidStr := strings.TrimPrefix(strings.TrimSpace(string(pidOutput)), "MainPID=")
				if pid, err := strconv.Atoi(pidStr); err == nil && pid > 0 {
					return true, pid, nil
				}
			}
			return true, 0, nil
		}
	}

	// Fallback to pgrep for non-systemd systems
	if _, err := exec.LookPath("pgrep"); err == nil {
		cmd := exec.Command("pgrep", "-f", "nextdeployd")
		output, err := cmd.CombinedOutput()
		if err == nil {
			pids := strings.Fields(string(output))
			if len(pids) > 0 {
				if pid, err := strconv.Atoi(pids[0]); err == nil {
					return true, pid, nil
				}
			}
		}
	}

	// Final fallback - check process list
	cmd := exec.Command("ps", "aux")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return false, 0, fmt.Errorf("failed to check processes: %v", err)
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, "nextdeployd") && !strings.Contains(line, "start.go") {
			parts := strings.Fields(line)
			if len(parts) > 1 {
				if pid, err := strconv.Atoi(parts[1]); err == nil {
					return true, pid, nil
				}
			}
		}
	}

	return false, 0, nil
}

// daemonStatusCmd adds a new command to check daemon status
func daemonStatusCmd() {
	log.Println("🔍 Checking NextDeploy daemon status...")

	running, pid, err := isDaemonRunning()
	if err != nil {
		log.Printf("⚠️ Error checking daemon status: %v", err)
		return
	}

	if running {
		log.Printf("✅ Daemon is running (PID: %d)", pid)

		// Additional info for systemd systems
		if _, err := exec.LookPath("systemctl"); err == nil {
			cmd := exec.Command("systemctl", "status", "nextdeploy.service", "--no-pager")
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			_ = cmd.Run()
		}
	} else {
		log.Println("❌ Daemon is not running")
	}
}

// Update main() to include the status command
func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "build":
		buildCmd()
	case "run":
		runCmd()
	case "dev":
		devCmd()
	case "clean":
		cleanCmd()
	case "status": // New status command
		daemonStatusCmd()
	case "purge":
		purgeCmd()
	default:
		printUsage()
		os.Exit(1)
	}
}

// Update usage information
func printUsage() {
	fmt.Println(`NextDeploy Master Control

Commands:
  build    Build components (daemon and CLI)
  run      Run the daemon with configuration
  dev      Set up development environment
  clean    Clean build artifacts
  status   Check daemon status

Flags:
  Use 'go run start.go <command> -help' for command-specific flags`)
}

// ---------------------- BUILD COMMAND ----------------------
func buildCmd() {
	log.Println("🚀 Starting build process...")

	buildFlags := flag.NewFlagSet("build", flag.ExitOnError)
	target := buildFlags.String("target", defaultTarget, "Build target")
	outputDir := buildFlags.String("output", defaultOutputDir, "Output directory")
	versionOverride := buildFlags.String("version", "", "Version override")
	buildFlags.Parse(os.Args[2:])

	// Verify output directory exists
	if err := os.MkdirAll(*outputDir, 0755); err != nil {
		log.Fatalf("❌ Failed to create output directory: %v", err)
	}

	version := getVersion(*versionOverride)
	commit := getGitCommit()
	buildTime := time.Now().Format(time.RFC3339)

	targets := []struct {
		name        string
		source      string
		output      string
		environment []string
		ldflags     string
	}{
		{
			name:        "nextdeployd (daemon)",
			source:      daemonPath,
			output:      filepath.Join(*outputDir, "nextdeployd"),
			environment: getDaemonEnv(),
			ldflags:     fmt.Sprintf("-s -w -X main.Version=%s -X main.Commit=%s -X main.BuildTime=%s", version, commit, buildTime),
		},
		{
			name:    "nextdeploy (CLI)",
			source:  cliPath,
			output:  filepath.Join(*outputDir, "nextdeploy"),
			ldflags: fmt.Sprintf("-X main.Version=%s -X main.Commit=%s", version, commit),
		},
	}

	// Build each target
	for _, t := range targets {
		if *target != "all" && !strings.Contains(strings.ToLower(t.name), *target) {
			continue
		}

		log.Printf("🔨 Building %s...", t.name)
		cmd := exec.Command("go", "build", "-ldflags", t.ldflags, "-o", t.output, t.source)
		cmd.Env = append(os.Environ(), t.environment...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			log.Fatalf("❌ Build failed for %s: %v", t.name, err)
		}
		log.Printf("✅ Successfully built %s", t.output)
	}
}

// ---------------------- RUN COMMAND ----------------------
func runCmd() {
	log.Println("🚀 Starting NextDeploy daemon...")

	runFlags := flag.NewFlagSet("run", flag.ExitOnError)
	configFile := runFlags.String("config", defaultConfigFile, "Config file path")
	port := runFlags.String("port", defaultPort, "Server port")
	host := runFlags.String("host", defaultHost, "Server host")
	debug := runFlags.Bool("debug", false, "Debug mode")
	runFlags.Parse(os.Args[2:])

	config := Config{
		Port:  *port,
		Host:  *host,
		Debug: *debug,
	}

	// Verify daemon exists
	daemonPath := filepath.Join(defaultOutputDir, "nextdeployd")
	if _, err := os.Stat(daemonPath); os.IsNotExist(err) {
		log.Println("⚠️ Daemon not found, building first...")
		buildCmd()
	}

	log.Printf("⚙️  Configuration:\n- Host: %s\n- Port: %s\n- Debug: %t",
		config.Host, config.Port, config.Debug)

	cmd := exec.Command(daemonPath,
		"--host", config.Host,
		"--port", config.Port,
		fmt.Sprintf("--debug=%t", config.Debug),
		"--config", *configFile,
	)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	log.Println("🌐 Starting daemon process...")
	if err := cmd.Start(); err != nil {
		log.Fatalf("❌ Failed to start daemon: %v", err)
	}

	log.Printf("✅ Daemon running (PID: %d)", cmd.Process.Pid)
	log.Println("🛑 Press CTRL+C to stop")

	if err := cmd.Wait(); err != nil {
		log.Printf("⚠️ Daemon exited: %v", err)
	}
}

// ---------------------- DEV COMMAND ----------------------
func devCmd() {
	log.Println("🚀 Setting up development environment...")

	// 1. Build components
	log.Println("🔨 Building development binaries...")
	buildCmd()

	// 2. Create directory structure
	devDirs := []string{
		filepath.Join(homeDir, ".nextdeploy", "keys"),
		filepath.Join(homeDir, ".nextdeploy", "logs"),
		filepath.Join(homeDir, ".nextdeploy", "cache"),
	}

	for _, dir := range devDirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			log.Fatalf("❌ Failed to create directory %s: %v", dir, err)
		}
	}
	log.Println("📁 Created development directories")

	// 3. Setup binaries (install as service)
	binariesSetup()

	// 4. Check service status instead of starting manually
	log.Println("✅ Development environment ready!")
	log.Println("🌍 Server should be running on http://localhost:8080")
	log.Printf("📝 Logs: %s", filepath.Join(homeDir, ".nextdeploy", "logs", "daemon.log"))
	log.Println("Check status with: sudo systemctl status nextdeploy")
}

// ----------------- Setup the built binarues ------------
func binariesSetup() {
	log.Println("🚀 Setting up binaries for system-wide use...")

	// Paths to built binaries
	cliBinary := filepath.Join(defaultOutputDir, "nextdeploy")
	daemonBinary := filepath.Join(defaultOutputDir, "nextdeployd")

	// Verify binaries exist
	if _, err := os.Stat(cliBinary); os.IsNotExist(err) {
		log.Println("⚠️ CLI binary not found, building first...")
		buildCmd()
	}
	if _, err := os.Stat(daemonBinary); os.IsNotExist(err) {
		log.Println("⚠️ Daemon binary not found, building first...")
		buildCmd()
	}

	// Installation paths
	installDir := "/usr/local/bin"
	systemdDir := "/etc/systemd/system"

	// Check if we have sudo privileges
	sudo := ""
	if os.Geteuid() != 0 {
		sudo = "sudo "
		log.Println("🔒 Requires sudo privileges for system installation")
	}

	// 1. Install binaries to /usr/local/bin
	log.Printf("📦 Installing binaries to %s...", installDir)
	installCmd := fmt.Sprintf("%scp %s %s %s", sudo, cliBinary, daemonBinary, installDir)
	if err := exec.Command("sh", "-c", installCmd).Run(); err != nil {
		log.Fatalf("❌ Failed to install binaries: %v", err)
	}

	// 2. Create systemd service for the daemon
	log.Println("⚙️ Creating systemd service...")
	serviceContent := fmt.Sprintf(`[Unit]
Description=NextDeploy Daemon
After=network.target

[Service]
Type=simple
User=%s
ExecStart=%s/nextdeployd --host=%s --port=%s
Restart=on-failure
RestartSec=5s
Environment=CONFIG_FILE=%s/.nextdeploy/config.yaml
Environment=LOG_FILE=%s/.nextdeploy/logs/daemon.log

[Install]
WantedBy=multi-user.target
`, os.Getenv("USER"), installDir, defaultHost, defaultPort, homeDir, homeDir)
	servicePath := filepath.Join(systemdDir, "nextdeploy.service")
	tmpServicePath := filepath.Join(os.TempDir(), "nextdeploy.service")

	// Write to temp file first
	if err := os.WriteFile(tmpServicePath, []byte(serviceContent), 0644); err != nil {
		log.Fatalf("❌ Failed to create service file: %v", err)
	}

	// Move to systemd directory
	moveCmd := fmt.Sprintf("%smv %s %s", sudo, tmpServicePath, servicePath)
	if err := exec.Command("sh", "-c", moveCmd).Run(); err != nil {
		log.Fatalf("❌ Failed to install service file: %v", err)
	}

	// 3. Reload systemd and enable service
	log.Println("🔄 Reloading systemd...")
	if err := exec.Command("sh", "-c", sudo+"systemctl daemon-reload").Run(); err != nil {
		log.Fatalf("❌ Failed to reload systemd: %v", err)
	}

	log.Println("⚡ Enabling nextdeploy service...")
	if err := exec.Command("sh", "-c", sudo+"systemctl enable nextdeploy.service").Run(); err != nil {
		log.Fatalf("❌ Failed to enable service: %v", err)
	}

	// 4. Add CLI to bashrc/zshrc
	log.Println("📝 Adding CLI to shell configuration...")
	shellConfig := filepath.Join(homeDir, ".bashrc")
	if _, err := os.Stat(filepath.Join(homeDir, ".zshrc")); err == nil {
		shellConfig = filepath.Join(homeDir, ".zshrc")
	}

	// Check if already configured
	configLine := fmt.Sprintf("\nexport PATH=$PATH:%s\n", installDir)
	configData, _ := os.ReadFile(shellConfig)
	if !strings.Contains(string(configData), configLine) {
		f, err := os.OpenFile(shellConfig, os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			log.Printf("⚠️ Failed to open shell config: %v", err)
		} else {
			if _, err := f.WriteString(configLine); err != nil {
				log.Printf("⚠️ Failed to update shell config: %v", err)
			}
			f.Close()
			log.Printf("✅ Updated %s", shellConfig)
		}
	} else {
		log.Printf("ℹ️ %s already configured", shellConfig)
	}

	// 5. Start the service
	log.Println("🚀 Starting nextdeploy daemon...")
	if err := exec.Command("sh", "-c", sudo+"systemctl start nextdeploy.service").Run(); err != nil {
		log.Fatalf("❌ Failed to start service: %v", err)
	}

	log.Println("✅ Successfully installed NextDeploy system-wide!")
	log.Println("   CLI available as 'nextdeploy'")
	log.Println("   Daemon running as system service")
	log.Printf("   Check status with: sudo systemctl status nextdeploy")
} // ---------------------- CLEAN COMMAND ----------------------
func cleanCmd() {
	log.Println("🧹 Cleaning build artifacts...")

	toRemove := []string{
		filepath.Join(defaultOutputDir, "nextdeployd"),
		filepath.Join(defaultOutputDir, "nextdeploy"),
	}

	for _, file := range toRemove {
		if err := os.Remove(file); err != nil && !os.IsNotExist(err) {
			log.Printf("⚠️ Failed to remove %s: %v", file, err)
		} else if os.IsNotExist(err) {
			log.Printf("ℹ️ %s does not exist", file)
		} else {
			log.Printf("✅ Removed %s", file)
		}
	}
}

// ---------------------- PURGE COMMAND ----------------------
func purgeCmd() {
	log.Println("💣 PURGING all NextDeploy artifacts...")

	paths := []string{
		"/usr/local/bin/nextdeploy",
		"/usr/local/bin/nextdeployd",
		"/usr/local/bin/ndctl",
		filepath.Join(homeDir, ".nextdeploy"),
		"/etc/systemd/system/nextdeploy.service",
	}

	sudo := ""
	if os.Geteuid() != 0 {
		sudo = "sudo "
		log.Println("🔒 Using sudo for privileged operations")
	}

	for _, path := range paths {
		cmdStr := fmt.Sprintf("%srm -rf %s", sudo, path)
		if err := exec.Command("sh", "-c", cmdStr).Run(); err != nil {
			log.Printf("⚠️ Failed to remove %s: %v", path, err)
		} else {
			log.Printf("✅ Removed %s", path)
		}
	}

	log.Println("🔄 Reloading systemd daemon...")
	if err := exec.Command("sh", "-c", sudo+"systemctl daemon-reload").Run(); err != nil {
		log.Printf("⚠️ Failed to reload systemd: %v", err)
	}

	log.Println("🛑 Disabling nextdeploy service...")
	exec.Command("sh", "-c", sudo+"systemctl disable nextdeploy.service").Run()
	exec.Command("sh", "-c", sudo+"systemctl stop nextdeploy.service").Run()

	// Clean up shell PATH additions
	shellFiles := []string{".bashrc", ".zshrc"}
	for _, f := range shellFiles {
		shellPath := filepath.Join(homeDir, f)
		if _, err := os.Stat(shellPath); err == nil {
			data, _ := os.ReadFile(shellPath)
			lines := strings.Split(string(data), "\n")
			var cleaned []string
			for _, line := range lines {
				if !strings.Contains(line, "nextdeploy") {
					cleaned = append(cleaned, line)
				}
			}
			_ = os.WriteFile(shellPath, []byte(strings.Join(cleaned, "\n")), 0644)
			log.Printf("✅ Cleaned PATH references in %s", shellPath)
		}
	}

	log.Println("💀 NextDeploy fully purged from system.")
}

// ---------------------- HELPER FUNCTIONS ----------------------
func getHomeDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		log.Fatalf("❌ Failed to get home directory: %v", err)
	}
	return home
}

func getDaemonEnv() []string {
	env := []string{"GOOS=linux", "GOARCH=amd64"}
	if os.Getenv("BUILD_STATIC") != "false" {
		env = append(env, "CGO_ENABLED=0")
	}
	return env
}

func getVersion(override string) string {
	if override != "" {
		return override
	}
	if version := os.Getenv("NEXTDEPLOY_VERSION"); version != "" {
		return version
	}
	if _, err := os.Stat(defaultVersionFile); err == nil {
		if data, err := os.ReadFile(defaultVersionFile); err == nil {
			return strings.TrimSpace(string(data))
		}
	}
	return "dev"
}

func getGitCommit() string {
	cmd := exec.Command("git", "rev-parse", "--short", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(out))
}
