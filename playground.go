//go:build ignore
// +build ignore

// internal/server/preparation/manager.go
package main

import (
	"context"
	"flag"
	"log/slog"
	"net/http"
	"nextdeploy/daemon/core"
	"nextdeploy/shared"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

var (
	version   = "1.0.0"
	buildDate = ""

	config = struct {
		host        string
		port        string
		keyDir      string
		rotateFreq  time.Duration
		debug       bool
		logFormat   string
		metricsPort string
		daemonize   bool
		pidFile     string
		logFile     string
	}{}
)

func startServer(server *http.Server, name string, logger *slog.Logger, errChan chan<- error) {
	logger.Info("starting server", "name", name, "address", server.Addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error("server error", "name", name, "error", err)
		errChan <- err
	}
}

func main() {
	flag.Parse()

	// Initialize logging
	logger, logFile := core.SetupLogger(config.daemonize, config.debug, config.logFormat, config.logFile)
	defer logFile.Close()

	if config.daemonize {
		core.Daemonize(logger, config.pidFile)
	}

	logger.Info("starting NextDeploy daemon",
		"version", version,
		"pid", os.Getpid(),
		"config", config)

	// Initialize components
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup secure key manager with automatic rotation
	keyManager, err := core.NewSecureKeyManager(config.keyDir, config.rotateFreq)
	if err != nil {
		logger.Error("failed to initialize key manager", "error", err)
		os.Exit(1)
	}
	defer func() {
		keyManager.StopRotation()
		if currentKey := keyManager.GetCurrentKey(); currentKey != nil {
			shared.ZeroBytes(currentKey.ECDHPrivate.Bytes())
			shared.ZeroBytes(currentKey.SignPrivate)
		}
	}()

	// Run cryptographic health checks
	if err := shared.RunCryptoHealthChecks(); err != nil {
		logger.Error("crypto health checks failed", "error", err)
		os.Exit(1)
	}

	// Start key rotation
	go keyManager.StartRotation()

	// Setup audit logging
	auditLog, err := core.NewAuditLog(filepath.Join(config.keyDir, "audit.log"))
	if err != nil {
		logger.Error("failed to initialize audit log", "error", err)
		os.Exit(1)
	}
	auditLog.AddEntry(shared.AuditLogEntry{
		Timestamp: time.Now(),
		Action:    "daemon_start",
		Message:   "NextDeploy daemon initialized",
	})

	// Setup HTTP servers
	mainServer, metricsServer := core.SetupServers(logger, keyManager, config.port, config.host, config.metricsPort)

	// Initialize WebSocket server with dedicated auth key
	agentID := os.Getenv("NEXTDEPLOY_AGENT_ID")
	wsServer := core.NewWSServer(
		keyManager.GetWSAuthKey(),
		agentID,
		keyManager, // Pass key manager for key exchange
		logger,
		auditLog,
	)

	// Register WebSocket endpoint
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		auditLog.AddEntry(shared.AuditLogEntry{
			Timestamp: time.Now(),
			Action:    "ws_connection_attempt",
			Client:    r.RemoteAddr,
		})
		wsServer.HandleConnection(w, r)
	})

	// Start servers
	errChan := make(chan error, 2)
	go startServer(mainServer, "main", logger, errChan)
	go startServer(metricsServer, "metrics", logger, errChan)

	// Set health status
	core.SetGlobalStatus("healthy")
	core.SetComponentStatus("key_manager", "healthy")
	core.SetComponentStatus("websocket", "healthy")

	// Setup graceful shutdown
	shutdownChan := make(chan os.Signal, 1)
	signal.Notify(shutdownChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	for {
		select {
		case sig := <-shutdownChan:
			logger.Info("received signal", "signal", sig)
			switch sig {
			case syscall.SIGHUP:
				logger.Info("reloading configuration")
				// TODO: Implement config reload
				auditLog.AddEntry(shared.AuditLogEntry{
					Timestamp: time.Now(),
					Action:    "config_reload",
					Message:   "Received SIGHUP",
				})
			default:
				auditLog.AddEntry(shared.AuditLogEntry{
					Timestamp: time.Now(),
					Action:    "shutdown",
					Message:   "Received termination signal",
				})
				core.SetGlobalStatus("shutting_down")
				core.GracefulShutdown(ctx, mainServer, metricsServer, logger, config.daemonize, config.pidFile)
				return
			}

		case err := <-errChan:
			logger.Error("server error", "error", err)
			auditLog.AddEntry(shared.AuditLogEntry{
				Timestamp: time.Now(),
				Action:    "server_error",
				Message:   err.Error(),
			})
			core.SetGlobalStatus("unhealthy")
			core.GracefulShutdown(ctx, mainServer, metricsServer, logger, config.daemonize, config.pidFile)
			return
		}
	}
}
