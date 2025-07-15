package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"time"
)

var (
	host       = flag.String("host", "0.0.0.0", "Host to bind the server to")
	port       = flag.String("port", "8080", "Port to listen on")
	keyDir     = flag.String("key-dir", ".keys", "Directory to store key files")
	rotateFreq = flag.Duration("rotate", 24*time.Hour, "Key rotation frequency")
)

func main() {

	err := os.MkdirAll(*keyDir, 0700)
	if err != nil {
		log.Fatalf("❌ Failed to create key directory: %v", err)
	}
	KeyManager, err := NewKeyManager(*keyDir, *rotateFreq)

	if err != nil {
		log.Fatalf("❌ Failed to initialize key manager: %v", err)
	}
	log.Println("🔑 Key manager initialized successfully.")
	KeyManager.StartRotation()
	mux := http.NewServeMux()
	SetupRoutes(mux, true, KeyManager)

	log.Println("🔧 Setting up routes for deployment, status, and metrics...")

	addr := "127.0.0.1:8371"
	server := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	log.Printf("🌐 Listening on %s for incoming requests...", addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("❌ Failed to start server: %v", err)
	}
}
