//go:build !windows

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
	"syscall"

	daemoniclient "github.com/aynaash/nextdeploy/daemon/internal/client"
	"github.com/aynaash/nextdeploy/daemon/internal/config"
	"github.com/aynaash/nextdeploy/daemon/internal/daemon"
	daemontypes "github.com/aynaash/nextdeploy/daemon/internal/types"
	"github.com/aynaash/nextdeploy/shared"
	"github.com/aynaash/nextdeploy/shared/updater"
	"github.com/gofrs/flock"
)

// socketPathOverride is set by --socket-path flag at startup and used by
// subcommands (ship, status, etc.) that need to contact the running daemon.
var socketPathOverride string

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "version", "--version", "-v":
			fmt.Printf("nextdeployd %s\n", shared.Version)
			return
		case "update":
			if err := updater.SelfUpdateDaemon(shared.Version); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			return
		case "ship":
			handleShipSubcommand()
			return
		case "secrets":
			handleSecretsSubcommand()
			return
		case "status":
			handleStatusSubcommand()
			return
		case "logs":
			handleLogsSubcommand()
			return
		case "rollback":
			handleRollbackSubcommand()
			return
		case "destroy":
			handleDestroySubcommand()
			return
		case "stop":
			handleStopSubcommand()
			return
		case "remove":
			handleDestroySubcommand() // remove is an alias for destroy
			return
		case "help", "--help", "-h":
			handleHelpSubcommand()
			return
		default:
			if strings.HasPrefix(os.Args[1], "-") {
				// Likely a flag for the daemon itself, fall through
				break
			}
			fmt.Fprintf(os.Stderr, "Unknown command: %s\nRun 'nextdeployd help' for usage.\n", os.Args[1])
			os.Exit(1)
		}
	}

	defaultConfig := "/etc/nextdeployd/config.json"
	if os.Geteuid() != 0 {
		home, err := os.UserHomeDir()
		if err == nil {
			defaultConfig = home + "/.nextdeploy/config.json"
		}
	}

	configPath := flag.String("config", defaultConfig, "Path to config file")
	foreground := flag.Bool("foreground", false, "Run in foreground")
	socketPath := flag.String("socket-path", "", "Override Unix socket path (default: /run/nextdeployd/nextdeployd.sock for root, ~/.nextdeploy/daemon.sock for others)")
	flag.Parse()

	// Store override for subcommand resolution (not used in daemon-start path,
	// but available to getSocketPath if ever called after flag.Parse).
	socketPathOverride = *socketPath

	if !*foreground {
		daemonize()
		return
	}

	runDaemon(*configPath, *socketPath)
}

// getSocketPath returns the Unix socket path the *running* daemon is listening
// on. This is called by CLI subcommands (ship, status, rollback, …) that send
// commands to the already-running daemon process.
func getSocketPath() string {
	// CLI subcommands may themselves receive --socket-path; honour it.
	if socketPathOverride != "" {
		return socketPathOverride
	}
	if os.Geteuid() == 0 {
		// Primary path: inside the RuntimeDirectory that systemd creates.
		return "/run/nextdeployd/nextdeployd.sock"
	}
	home, err := os.UserHomeDir()
	if err == nil {
		return filepath.Join(home, ".nextdeploy", "daemon.sock")
	}
	return "/run/nextdeployd/nextdeployd.sock"
}

func sendDaemonCommand(cmd daemontypes.Command) {
	socketPath := getSocketPath()

	// Load config for security settings
	defaultConfig := "/etc/nextdeployd/config.json"
	if os.Geteuid() != 0 {
		home, _ := os.UserHomeDir()
		defaultConfig = filepath.Join(home, ".nextdeploy", "config.json")
	}
	cfg, _ := config.LoadConfig(defaultConfig)

	clientCfg := daemoniclient.ClientConfig{
		Address:  socketPath,
		Secret:   cfg.SecuritySecret,
		CertFile: cfg.TLSCertFile,
		KeyFile:  cfg.TLSKeyFile,
		CAFile:   cfg.TLSCAFile,
	}

	resp, err := daemoniclient.SendCommand(clientCfg, cmd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error contacting daemon: %v (Is nextdeployd running?)\n", err)
		os.Exit(1)
	}
	if resp != nil {
		if !resp.Success {
			fmt.Fprintf(os.Stderr, "Error: %s\n", resp.Message)
			os.Exit(1)
		}
		fmt.Print(resp.Message)
		if !strings.HasSuffix(resp.Message, "\n") {
			fmt.Println()
		}
	}
}

func handleShipSubcommand() {
	tarball := ""
	dopplerToken := ""
	for _, arg := range os.Args[2:] {
		if after, ok := strings.CutPrefix(arg, "--tarball="); ok {
			tarball = after
			tarball = strings.Trim(tarball, "\"'")
		} else if after, ok := strings.CutPrefix(arg, "--dopplerToken="); ok {
			dopplerToken = after
		} else if after, ok := strings.CutPrefix(arg, "--socket-path="); ok {
			socketPathOverride = after
		}
	}
	if tarball == "" {
		fmt.Fprintln(os.Stderr, "Error: --tarball is required")
		os.Exit(1)
	}
	args := map[string]any{"tarball": tarball}
	if dopplerToken != "" {
		args["dopplerToken"] = dopplerToken
	}
	sendDaemonCommand(daemontypes.Command{Type: "ship", Args: args})
}

func handleSecretsSubcommand() {
	action := ""
	appName := ""
	key := ""
	value := ""
	for _, arg := range os.Args[2:] {
		if after, ok := strings.CutPrefix(arg, "--action="); ok {
			action = after
		} else if after, ok := strings.CutPrefix(arg, "--appName="); ok {
			appName = after
		} else if after, ok := strings.CutPrefix(arg, "--key="); ok {
			key = after
		} else if after, ok := strings.CutPrefix(arg, "--value="); ok {
			value = after
			value = strings.Trim(value, "\"'")
		}
	}
	args := map[string]any{
		"action":  action,
		"appName": appName,
		"key":     key,
		"value":   value,
	}
	sendDaemonCommand(daemontypes.Command{Type: "secrets", Args: args})
}

func handleStatusSubcommand() {
	appName := ""
	for _, arg := range os.Args[2:] {
		if after, ok := strings.CutPrefix(arg, "--appName="); ok {
			appName = after
		}
	}
	sendDaemonCommand(daemontypes.Command{Type: "status", Args: map[string]any{"appName": appName}})
}

func handleLogsSubcommand() {
	appName := ""
	for _, arg := range os.Args[2:] {
		if after, ok := strings.CutPrefix(arg, "--appName="); ok {
			appName = after
		}
	}
	sendDaemonCommand(daemontypes.Command{Type: "logs", Args: map[string]any{"appName": appName}})
}

func handleRollbackSubcommand() {
	appName := ""
	dopplerToken := ""
	toCommit := ""
	steps := 0
	for _, arg := range os.Args[2:] {
		if after, ok := strings.CutPrefix(arg, "--appName="); ok {
			appName = after
		} else if after, ok := strings.CutPrefix(arg, "--dopplerToken="); ok {
			dopplerToken = after
		} else if after, ok := strings.CutPrefix(arg, "--toCommit="); ok {
			toCommit = after
		} else if after, ok := strings.CutPrefix(arg, "--steps="); ok {
			n, err := strconv.Atoi(after)
			if err != nil || n < 0 {
				fmt.Fprintln(os.Stderr, "Error: --steps must be a non-negative integer")
				os.Exit(1)
			}
			steps = n
		}
	}
	if appName == "" {
		fmt.Fprintln(os.Stderr, "Error: --appName is required")
		os.Exit(1)
	}
	args := map[string]any{"appName": appName}
	if dopplerToken != "" {
		args["dopplerToken"] = dopplerToken
	}
	if toCommit != "" {
		args["toCommit"] = toCommit
	}
	if steps > 0 {
		// JSON over the wire decodes numbers as float64; encode as such for symmetry.
		args["steps"] = float64(steps)
	}
	sendDaemonCommand(daemontypes.Command{Type: "rollback", Args: args})
}

func handleStopSubcommand() {
	appName := ""
	for _, arg := range os.Args[2:] {
		if after, ok := strings.CutPrefix(arg, "--appName="); ok {
			appName = after
		}
	}
	if appName == "" {
		fmt.Fprintln(os.Stderr, "Error: --appName is required")
		os.Exit(1)
	}
	sendDaemonCommand(daemontypes.Command{Type: "stop", Args: map[string]any{"appName": appName}})
}

func handleHelpSubcommand() {
	fmt.Println("NextDeploy Daemon (nextdeployd)")
	fmt.Println("Usage: nextdeployd <command> [arguments]")
	fmt.Println()
	fmt.Println("Available commands:")
	fmt.Println("  ship --tarball=<path>     Deploy a new release")
	fmt.Println("  status --appName=<name>   Check app status")
	fmt.Println("  stop --appName=<name>     Stop an application")
	fmt.Println("  destroy --appName=<name>  Remove an application")
	fmt.Println("  remove --appName=<name>   Remove an application (alias for destroy)")
	fmt.Println("  logs --appName=<name>     Stream app logs")
	fmt.Println("  rollback --appName=<name> Rollback to previous release")
	fmt.Println("  secrets --action=...      Manage application secrets")
	fmt.Println("  version                   Show version information")
	fmt.Println("  update                    Update nextdeployd to latest version")
	fmt.Println()
	fmt.Println("Run as daemon:")
	fmt.Println("  nextdeployd [--config <path>] [--socket-path <path>] [--foreground]")
}

func handleDestroySubcommand() {
	appName := ""
	for _, arg := range os.Args[2:] {
		if after, ok := strings.CutPrefix(arg, "--appName="); ok {
			appName = after
		}
	}
	if appName == "" {
		fmt.Fprintln(os.Stderr, "Error: --appName is required")
		os.Exit(1)
	}
	sendDaemonCommand(daemontypes.Command{Type: "destroy", Args: map[string]any{"appName": appName}})
}

func daemonize() {
	execPath, err := os.Executable()
	if err != nil {
		log.Fatalf("Error getting executable path: %v", err)
	}
	args := []string{"--foreground=true"}
	if len(os.Args) > 1 {
		for _, arg := range os.Args[1:] {
			if arg != "--foreground" && !strings.HasPrefix(arg, "--foreground=") {
				args = append(args, arg)
			}
		}
	}
	// #nosec G204 G702
	cmd := exec.Command(execPath, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	logDir := "/var/log/nextdeployd"
	if os.Geteuid() != 0 {
		home, err := os.UserHomeDir()
		if err == nil {
			logDir = home + "/.nextdeploy/log"
		}
	}
	if err := os.MkdirAll(logDir, 0750); err != nil {
		log.Fatalf("Error creating log directory: %v", err)
	}
	logFilePath := filepath.Join(logDir, "nextdeployd.log")
	// #nosec G304
	logFile, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		log.Fatalf("Error opening log file: %v", err)
	}
	defer logFile.Close()
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err := cmd.Start(); err != nil {
		log.Fatalf("Error starting daemon: %v", err)
	}
	fmt.Printf("Daemon started with PID %d\n", cmd.Process.Pid)
	fmt.Printf("Logs are being written to %s\n", logFilePath)
	os.Exit(0)
}

func acquireLock() error {
	var lockPath string
	if os.Geteuid() == 0 {
		lockPath = "/run/nextdeployd/nextdeployd.lock"
	} else {
		// 1. Try XDG per-user runtime dir
		if xdg := os.Getenv("XDG_RUNTIME_DIR"); xdg != "" {
			lockPath = filepath.Join(xdg, "nextdeployd.lock")
		} else if home, err := os.UserHomeDir(); err == nil {
			// 2. Try home dir
			lockPath = filepath.Join(home, ".nextdeploy", "nextdeployd.lock")
		} else {
			// 3. Fallback to a UID-protected subdirectory in /tmp
			lockPath = filepath.Join(os.TempDir(), fmt.Sprintf("nextdeployd-%d", os.Getuid()), "nextdeployd.lock")
		}
	}

	// #nosec G301 G703
	if err := os.MkdirAll(filepath.Dir(lockPath), 0750); err != nil {
		return err
	}

	fileLock := flock.New(lockPath)
	locked, err := fileLock.TryLock()
	if err != nil {
		return fmt.Errorf("error acquiring lock: %w", err)
	}
	if !locked {
		return fmt.Errorf("another instance of nextdeployd is already running")
	}

	pidPath := strings.TrimSuffix(lockPath, ".lock") + ".pid"
	// #nosec G306 G703
	if err := os.WriteFile(pidPath, fmt.Appendf(nil, "%d\n", os.Getpid()), 0o600); err != nil {
		_ = fileLock.Unlock() // #nosec G104
		return fmt.Errorf("error writing PID file: %w", err)
	}

	return nil
}

func runDaemon(configPath string, socketPathFlag string) {
	if err := acquireLock(); err != nil {
		log.Fatalf("Lock error: %v", err)
	}
	daemon.StartMetricsServer("127.0.0.1:6060")
	d, err := daemon.NewNextDeployDaemon(configPath, socketPathFlag)
	if err != nil {
		log.Fatalf("Error initializing daemon: %v", err)
	}
	if err := d.Start(); err != nil {
		log.Fatalf("Error starting daemon: %v", err)
	}
}

func isRoot() bool {
	return os.Geteuid() == 0
}

func getSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		Setsid: true,
	}
}
