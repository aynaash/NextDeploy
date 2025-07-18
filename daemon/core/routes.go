package core

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

func SetupServers(logger *slog.Logger, keyManager *KeyManager, port string, host string, metricsPort string) (*http.Server, *http.Server) {
	// Main server with all routes
	mux := http.NewServeMux()

	// Application management routes
	mux.HandleFunc("/deploy", ChainMiddleware(
		HandleDeploy,
		LoggingMiddleware,
		RecoveryMiddleware,
		AuthMiddleware(keyManager),
		CORSMiddleware(true),
	))

	mux.HandleFunc("/stop", ChainMiddleware(
		HandleStop,
		LoggingMiddleware,
		RecoveryMiddleware,
		AuthMiddleware(keyManager),
	))

	mux.HandleFunc("/restart", ChainMiddleware(
		HandleRestart,
		LoggingMiddleware,
		RecoveryMiddleware,
		AuthMiddleware(keyManager),
	))

	mux.HandleFunc("/status", ChainMiddleware(
		HandleStatus,
		LoggingMiddleware,
		RecoveryMiddleware,
		AuthMiddleware(keyManager),
	))

	// Monitoring routes
	mux.HandleFunc("/metrics", ChainMiddleware(
		HandleSystemMetrics,
		LoggingMiddleware,
		RecoveryMiddleware,
	))

	//Identity endpoints
	mux.HandleFunc("/submit-env", ChainMiddleware(
		HandleSecretsSync,
		LoggingMiddleware,
		RecoveryMiddleware,
		AuthMiddleware(keyManager),
	))
	mux.HandleFunc("/add-identity", ChainMiddleware(
		HandleAddIdentity,
		LoggingMiddleware,
		RecoveryMiddleware,
		AuthMiddleware(keyManager),
	))
	mux.HandleFunc("/revoke-identity", ChainMiddleware(
		HandleRevokeIdentity,
		LoggingMiddleware,
		RecoveryMiddleware,
		AuthMiddleware(keyManager),
	))

	mux.HandleFunc("/list-identities", ChainMiddleware(
		HandleListIdentities,
		LoggingMiddleware,
		RecoveryMiddleware,
		AuthMiddleware(keyManager),
	))
	mux.HandleFunc("/public-key", ChainMiddleware(
		HandlePublicKey,
		LoggingMiddleware,
		RecoveryMiddleware,
		AuthMiddleware(keyManager),
	))
	// Infrastructure routes
	mux.HandleFunc("/secrets/sync", ChainMiddleware(
		HandleSecretsSync,
		LoggingMiddleware,
		RecoveryMiddleware,
		AuthMiddleware(keyManager),
	))

	// Health check (no auth)
	mux.HandleFunc("/health", ChainMiddleware(
		HandleHealthCheck,
		LoggingMiddleware,
		RecoveryMiddleware,
	))

	mainServer := &http.Server{
		Addr:         fmt.Sprintf("%s:%s", host, port),
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
		ErrorLog:     slog.NewLogLogger(logger.Handler(), slog.LevelError),
	}

	// Metrics server (simpler, separate port)
	metricsMux := http.NewServeMux()
	metricsMux.HandleFunc("/metrics", HandleSystemMetrics)
	metricsMux.HandleFunc("/health", HandleHealthCheck)

	metricsServer := &http.Server{
		Addr:         fmt.Sprintf("%s:%s", host, metricsPort),
		Handler:      metricsMux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  15 * time.Second,
		ErrorLog:     slog.NewLogLogger(logger.Handler(), slog.LevelError),
	}

	return mainServer, metricsServer
}
