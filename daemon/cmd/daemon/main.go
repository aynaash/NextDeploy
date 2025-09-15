package main

import (
	"flag"
	"fmt"
	"log"
	"nextdeploy/daemon/internal/daemon"
	"nextdeploy/shared"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

func main() {
	version := flag.Bool("version", false, "Show version info")
	configPath := flag.String("config", "/etc/nextdeployd/config.json", "Path to config file")
	foreground := flag.Bool("foreground", false, "Run in foreground")
	flag.Parse()

	if *version {
		fmt.Printf("nextdeployd version %s\n", shared.Version)
		os.Exit(0)
	}

	if len(os.Args) > 1 && os.Args[1] == "update" {
		fmt.Printf("Checking for updates...\n")
		updateInfo, err := UpdateDaemon()
		if err != nil {
			log.Fatalf("Error checking for updates: %v", err)
			return
		}

		if updateInfo.Updated {
			fmt.Printf("Updated to version %s\n", updateInfo.NewVersion)
		} else {
			fmt.Printf("Already at the latest version (%s)\n", shared.Version)
		}
		return
	}
	if !*foreground {
		daemonize()
		return
	}

	runDaemon(*configPath)
}

func daemonize() {
	// get the executable path
	execPath, err := os.Executable()
	if err != nil {
		log.Fatalf("Error getting executable path: %v", err)
	}
	// return ourselvs in foreground
	args := []string{"--foreground=true"}
	if len(os.Args) > 1 {
		// preserver other args like config path
		for _, arg := range os.Args[2:] {
			if arg != "--foreground" && !strings.HasPrefix(arg, "--foreground=") {
				args = append(args, arg)
			}
		}
	}
	cmd := exec.Command(execPath, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true,
	}
	// redirect output to log file
	logDir := "/var/log/nextdeployd"
	if err := os.MkdirAll(logDir, 0755); err != nil {
		log.Fatalf("Error creating log directory: %v", err)
	}
	logFilePath := filepath.Join(logDir, "nextdeployd.log")
	logFile, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
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

func runDaemon(configPath string) {
	daemon, err := daemon.NewNextDeployDaemon(configPath)
	if err != nil {
		log.Fatalf("Error initializing daemon: %v", err)
	}
	if err := daemon.Start(); err != nil {
		log.Fatalf("Error starting daemon: %v", err)
	}

}
