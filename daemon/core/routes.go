package core

import (
	"fmt"
	"log/slog"
	"net/http"
	"nextdeploy/daemon/core"
	"time"
)

func SetupServers(logger *slog.Logger, keyManager *core.KeyManager) (*http.Server, *http.Server) {
	// Main server with all routes
	mux := http.NewServeMux()

	// Application management routes
	mux.HandleFunc("/deploy", core.ChainMiddleware(
		core.HandleDeploy,
		core.LoggingMiddleware,
		core.RecoveryMiddleware,
		core.AuthMiddleware(keyManager),
		core.CORSMiddleware(true),
	))

	mux.HandleFunc("/stop", core.ChainMiddleware(
		core.HandleStop,
		core.LoggingMiddleware,
		core.RecoveryMiddleware,
		core.AuthMiddleware(keyManager),
	))

	mux.HandleFunc("/restart", core.ChainMiddleware(
		core.HandleRestart,
		core.LoggingMiddleware,
		core.RecoveryMiddleware,
		core.AuthMiddleware(keyManager),
	))

	mux.HandleFunc("/status", core.ChainMiddleware(
		core.HandleStatus,
		core.LoggingMiddleware,
		core.RecoveryMiddleware,
		core.AuthMiddleware(keyManager),
	))

	// Monitoring routes
	mux.HandleFunc("/metrics", core.ChainMiddleware(
		core.HandleSystemMetrics,
		core.LoggingMiddleware,
		core.RecoveryMiddleware,
	))

	//Identity endpoints
	mux.HandleFunc("/submit-env", core.ChainMiddleware(
		core.HandleSecretsSync,
		core.LoggingMiddleware,
		core.RecoveryMiddleware,
		core.AuthMiddleware(keyManager),
	))
	mux.HandleFunc("/add-identity", core.ChainMiddleware(
		core.HandleAddIdentity,
		core.LoggingMiddleware,
		core.RecoveryMiddleware,
		core.AuthMiddleware(keyManager),
	))
	mux.HandleFunc("/revoke-identity", core.ChainMiddleware(
		core.HandleRevokeIdentity,
		core.LoggingMiddleware,
		core.RecoveryMiddleware,
		core.AuthMiddleware(keyManager),
	))

	mux.HandleFunc("/list-identities", core.ChainMiddleware(
		core.HandleListIdentities,
		core.LoggingMiddleware,
		core.RecoveryMiddleware,
		core.AuthMiddleware(keyManager),
	))
	mux.HandleFunc("/public-key", core.ChainMiddleware(
		core.HandlePublicKey,
		core.LoggingMiddleware,
		core.RecoveryMiddleware,
		core.AuthMiddleware(keyManager),
	))
	// Infrastructure routes
	mux.HandleFunc("/secrets/sync", core.ChainMiddleware(
		core.HandleSecretsSync,
		core.LoggingMiddleware,
		core.RecoveryMiddleware,
		core.AuthMiddleware(keyManager),
	))

	// Health check (no auth)
	mux.HandleFunc("/health", core.ChainMiddleware(
		core.HandleHealthCheck,
		core.LoggingMiddleware,
		core.RecoveryMiddleware,
	))

	mainServer := &http.Server{
		Addr:         fmt.Sprintf("%s:%s", config.host, config.port),
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
		ErrorLog:     slog.NewLogLogger(logger.Handler(), slog.LevelError),
	}

	// Metrics server (simpler, separate port)
	metricsMux := http.NewServeMux()
	metricsMux.HandleFunc("/metrics", core.HandleSystemMetrics)
	metricsMux.HandleFunc("/health", core.HandleHealthCheck)

	metricsServer := &http.Server{
		Addr:         fmt.Sprintf("%s:%s", config.host, config.metricsPort),
		Handler:      metricsMux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  15 * time.Second,
		ErrorLog:     slog.NewLogLogger(logger.Handler(), slog.LevelError),
	}

	return mainServer, metricsServer
}
