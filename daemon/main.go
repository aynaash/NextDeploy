package main

import (
	"log"

	"github.com/nextdeploy/daemon"
)

func main() {
	log.Println("🚀 Starting NextDeploy Daemon...")

	err := daemon.Start()
	if err != nil {
		log.Fatalf("❌ Failed to start daemon: %v", err)
	}
}
