package main

import (
	"log"
	"net/http"
	"nextdeploy/daemon"
)

func main() {

	mux := http.NewServeMux()
	daemon.SetupRoutes(mux)

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
