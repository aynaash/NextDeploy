package main

import (
	"encoding/json"
	"log"
	"net/http"
)

var appStatusStore = make(map[string]AppStatus)

func SetupRoutes(mux *http.ServeMux, corsEnabled bool, KeyManager *KeyManager) {
	// Application management routes
	mux.HandleFunc("/deploy", HandleDeploy)
	mux.HandleFunc("/stop", HandleStop)
	mux.HandleFunc("/restart", HandleRestart)
	mux.HandleFunc("/status", HandleStatus)

	// Monitoring routes
	mux.HandleFunc("/metrics", HandleSystemMetrics)

	// Infrastructure routes
	mux.HandleFunc("/secrets/sync", HandleSecretsSync)
	mux.HandleFunc("/proxy/configure", HandleProxyConfig)
	mux.HandleFunc("/certs/rotate", HandleCertRotate)
	mux.HandleFunc("/swap", HandleBlueGreenSwap)

	// NextCore integration
	mux.HandleFunc("/nextcore/intake", CorsMiddleware(HandleNextCoreIntake))

	// WebSocket routes
	mux.HandleFunc("/ws/logs", HandleLogStream)
	mux.HandleFunc("/ws/metrics", HandleMetricsStream)
	// Key management routes
	mux.HandleFunc("/public-key", HandlePublicKey)
	mux.HandleFunc("submit-env/", HandleSubmitEnv)
}

// Helper functions
func writeJSON(w http.ResponseWriter, status int, response DaemonResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Error writing response: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}
