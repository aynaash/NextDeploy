package main

import (
	"flag"
	"log"
	"nextdeploy/shared"
	"os"
)

var (
	daemonAddr = flag.String("daemon", "http://localhost:8080", "Daemon address")
	envFile    = flag.String("env", ".env", "Path to .env file")
	trustStore = flag.String("trust-store", ".trusted_keys.json", "Path to trust store")
)

func daemonConnection() {
	flag.Parse()

	// Load or initialize trust store
	store, err := LoadTrustStore(*trustStore)
	if err != nil {
		log.Fatalf("Failed to load trust store: %v", err)
	}

	// Fetch daemon's public key
	daemonKey, err := FetchDaemonPublicKey(*daemonAddr, store)
	if err != nil {
		log.Fatalf("Failed to get daemon public key: %v", err)
	}

	// Read and parse the .env file
	envContent, err := os.ReadFile(*envFile)
	if err != nil {
		log.Fatalf("Failed to read .env file: %v", err)
	}

	env, err := shared.ParseEnvFile(envContent)
	if err != nil {
		log.Fatalf("Failed to parse .env file: %v", err)
	}

	// Encrypt and send the environment
	if err := EncryptAndSendEnv(env, daemonKey, *daemonAddr); err != nil {
		log.Fatalf("Failed to send environment: %v", err)
	}

	log.Println("Environment successfully sent to daemon")
}
