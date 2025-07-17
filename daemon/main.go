package main

// Command nextdeploy-daemon
//
// NextDeploy Daemon is a system-level service responsible for managing and
// orchestrating deployed Next.js applications on remote virtual servers (VPS).
//
// It performs container lifecycle management, system metrics collection,
// real-time logging, and failover detection for hosted applications.
//
// This daemon is designed to be used **insecurely behind trusted infrastructure** (e.g., SSH, VPN).
// DO NOT expose it directly to the public internet without proper authentication.
//
// Author: Yussuf Hersi <dev@hersi.dev> || yussufhersi219@gmail.com
// License: MIT
// Source: https://github.com/aynaash/nextdeploy
//
// ─────────────────────────────────────────────────────────────────────────────
import (
	"context"
	"flag"
	"log"
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

func init() {
	flag.StringVar(&config.host, "host", "0.0.0.0", "Host to bind to")
	flag.StringVar(&config.port, "port", "8080", "Main service port")
	flag.StringVar(&config.keyDir, "key-dir", "/var/lib/nextdeploy/keys", "Key directory")
	flag.DurationVar(&config.rotateFreq, "rotate", 24*time.Hour, "Key rotation frequency")
	flag.BoolVar(&config.debug, "debug", false, "Enable debug logging")
	flag.StringVar(&config.logFormat, "log-format", "json", "Log format (text/json)")
	flag.StringVar(&config.metricsPort, "metrics-port", "9090", "Metrics port")
	flag.BoolVar(&config.daemonize, "daemon", false, "Run as daemon")
	flag.StringVar(&config.pidFile, "pid-file", "/var/run/nextdeploy.pid", "PID file path")
	flag.StringVar(&config.logFile, "log-file", "/var/log/nextdeploy.log", "Log file path")
}

func startServer(server *http.Server, name string, logger *slog.Logger, errChan chan<- error) {
	logger.Info("starting server", "name", name, "address", server.Addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error("server error", "name", name, "error", err)
		errChan <- err
	}
}

func main() {
	flag.Parse()

	logger, logFile := SetupLogger()
	defer logFile.Close()

	if config.daemonize {
		Daemonize(logger)
	}

	logger.Info("starting NextDeploy daemon",
		"version", version,
		"pid", os.Getpid(),
		"config", config)

	// Initialize components
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup key manager
	keyManager, err := setupKeyManager(logger)
	if err != nil {
		os.Exit(1)
	}
	// Run health checks
	if err := shared.RunCryptoHealthChecks(); err != nil {
		log.Fatalf("Crypto health checks failed: %v", err)
	}

	defer keyManager.StopRotation()

	//FIX: fix this logic for bettertrust store management
	auditLog, err := core.NewAuditLog(filepath.Join("audit.log"))
	if err != nil {
		logger.Error("failed to initialize audit log", "error", err)
		os.Exit(1)
	}

	auditLog.AddEntry(shared.AuditLogEntry{
		Timestamp: time.Now(),
		Action:    "start",
	})

	defer keyManager.StopRotation()

	// Setup HTTP servers with all routes
	mainServer, metricsServer := SetupServers(logger, keyManager)

	// Start servers
	errChan := make(chan error, 2)
	go startServer(mainServer, "main", logger, errChan)
	go startServer(metricsServer, "metrics", logger, errChan)

	// Setup health status
	core.SetGlobalStatus("healthy")
	core.SetComponentStatus("key_manager", "healthy")

	// Wait for shutdown
	shutdownChan := make(chan os.Signal, 1)
	signal.Notify(shutdownChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	for {
		select {
		case sig := <-shutdownChan:
			logger.Info("received signal", "signal", sig)
			switch sig {
			case syscall.SIGHUP:
				logger.Info("reloading configuration")
				// Implement config reload here
			default:
				core.SetGlobalStatus("shutting_down")
				GracefulShutdown(ctx, mainServer, metricsServer, logger)
				return
			}
		case err := <-errChan:
			logger.Error("server error", "error", err)
			core.SetGlobalStatus("unhealthy")
			GracefulShutdown(ctx, mainServer, metricsServer, logger)
			return
		}
	}
}
