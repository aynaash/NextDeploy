package core

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
)

var (
	appStatusStore = make(map[string]AppStatus)
	statusMutex    = &sync.RWMutex{} // Protects appStatusStore
)

type RouteConfig struct {
	CORSEnabled bool
	KeyManager  *KeyManager
	DebugMode   bool
}

func SetupRoutes(mux *http.ServeMux, config RouteConfig) {
	// Middleware chain
	chain := func(h http.HandlerFunc) http.HandlerFunc {
		return ChainMiddleware(
			h,
			LoggingMiddleware,
			RecoveryMiddleware,
			AuthMiddleware(config.KeyManager),
			CORSMiddleware(config.CORSEnabled),
		)
	}
	mux.HandleFunc("/health", ChainMiddleware(
		HandleHealthCheck,
		LoggingMiddleware,
		RecoveryMiddleware,
	))

	// Readiness probe
	mux.HandleFunc("/health/ready", ChainMiddleware(
		HandleReadinessCheck,
		LoggingMiddleware,
		RecoveryMiddleware,
	))

	// Liveness probe
	mux.HandleFunc("/health/live", ChainMiddleware(
		HandleLivenessCheck,
		LoggingMiddleware,
		RecoveryMiddleware,
	))
	// Application management routes
	mux.HandleFunc("/deploy", chain(HandleDeploy))
	mux.HandleFunc("/stop", chain(HandleStop))
	mux.HandleFunc("/restart", chain(HandleRestart))
	mux.HandleFunc("/status", chain(HandleStatus))

	// Monitoring routes
	mux.HandleFunc("/metrics", chain(HandleSystemMetrics))

	// Infrastructure routes
	mux.HandleFunc("/secrets/sync", chain(HandleSecretsSync))
	mux.HandleFunc("/proxy/configure", chain(HandleProxyConfig))
	mux.HandleFunc("/certs/rotate", chain(HandleCertRotate))
	mux.HandleFunc("/swap", chain(HandleBlueGreenSwap))

	// NextCore integration
	mux.HandleFunc("/nextcore/intake", chain(HandleNextCoreIntake))

	// WebSocket routes (with different middleware)
	wsChain := func(h http.HandlerFunc) http.HandlerFunc {
		return ChainMiddleware(
			h,
			LoggingMiddleware,
			RecoveryMiddleware,
			AuthMiddleware(config.KeyManager),
		)
	}
	mux.HandleFunc("/ws/logs", wsChain(HandleLogStream))
	mux.HandleFunc("/ws/metrics", wsChain(HandleMetricsStream))

	// Key management routes
	mux.HandleFunc("/public-key", chain(HandlePublicKey))
	mux.HandleFunc("/submit-env/", chain(HandleSubmitEnv))

	// Health check route (no auth required)
	mux.HandleFunc("/health", ChainMiddleware(
		HandleHealthCheck,
		LoggingMiddleware,
		RecoveryMiddleware,
	))
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

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, DaemonResponse{
		Success: false,
		Message: message,
	})
}
