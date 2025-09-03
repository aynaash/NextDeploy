package main

import (
	"flag"
	"fmt"
	"log"
	"nextdeploy/daemon/internal/daemon"
	"nextdeploy/shared"
	"os"
)

func main() {
	version := flag.Bool("version", false, "Show version info")
	configPath := flag.String("config", "/etc/nextdeployd/config.json", "Path to config file")
	flag.Parse()

	if *version {
		fmt.Println("NextDeploy Daemon:", shared.Version)
		os.Exit(0)
	}

	if len(os.Args) > 1 && os.Args[1] == "update" {
		fmt.Println("Checking for updates...")
		UpdateDaemon()
		fmt.Println("Update completed successfully.")
		os.Exit(0)
	}

	daemonInstance, err := daemon.NewNextDeployDaemon(*configPath)
	if err != nil {
		log.Fatalf("Failed to create daemon: %v", err)
	}
	if err := daemonInstance.Start(); err != nil {
		log.Fatalf("Daemon failed: %v", err)
	}
}
