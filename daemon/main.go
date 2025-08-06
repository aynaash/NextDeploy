package main

// NextDeploy Daemon is a system-level service responsible for managing and
// orchestrating deployed Next.js applications on remote virtual servers (VPS).
//
// It performs container lifecycle management, system metrics collection,
// real-time logging, and failover detection for hosted applications.
//
// DO NOT expose it directly to the public internet without proper authentication.
//
// Author: Yussuf Hersi  caynaashow@gmail.com
// License: MIT
// Source: https://github.com/aynaash/nextdeploy
//
// ─────────────────────────────────────────────────────────────────────────────
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
	version      = "1.0.0"
	buildDate    = ""
	DaemonLogger = shared.PackageLogger("nextdeployd", "NextDeployDaemon")

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

var (
	host        = "0.0.0.0"
	port        = 8080
	keyDir      = "/var/lib/nextdeploy/keys"
	rotateFreq  = 24 * time.Hour
	debug       = true
	logFormat   = "json"
	metricsPort = 9090
	daemonize   = true
	pidfile     = "/var/run/nextdeploy.pid"
	logFile     = "var/log/nextdeployd.log"
)

func startServer(server *http.Server, name string, logger *slog.Logger, errChan chan<- error) {
	DaemonLogger.Info("starting server name:%s address:%s", name, server.Addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		DaemonLogger.Error("server error name:%s error:%s", name, err)
		errChan <- err
	}
}
func init() {
	flag.StringVar(&config.host, "host", "0.0.0.0", "Host to bind to")
	flag.StringVar(&config.port, "port", "8080", "Port to listen on")
	flag.StringVar(&config.keyDir, "key-dir", "/var/lib/nextdeploy/keys", "Directory for key storage")
	flag.DurationVar(&config.rotateFreq, "rotate-freq", 24*time.Hour, "Key rotation frequency")
	flag.BoolVar(&config.debug, "debug", false, "Enable debug mode")
	flag.StringVar(&config.logFormat, "log-format", "json", "Log format (text/json)")
	flag.StringVar(&config.metricsPort, "metrics-port", "9090", "Metrics server port")
	flag.BoolVar(&config.daemonize, "daemonize", false, "Run as daemon")
	flag.StringVar(&config.pidFile, "pidfile", "/var/run/nextdeploy.pid", "PID file location")
	flag.StringVar(&config.logFile, "log-file", "/var/log/nextdeployd.log", "Log file location")
}
func main() {
	flag.Parse()

	logger, logFile := core.SetupLogger(config.daemonize, config.debug, config.logFormat, config.logFile)
	defer logFile.Close()

	if config.daemonize {
		core.Daemonize(logger, config.pidFile)
	}

	DaemonLogger.Info("starting NextDeploy daemon version:%s  -- pid:%s --  config:%s", version, os.Getpid(), config)

	// Initialize components
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup key manager
	keyManager, err := core.SetupKeyManager(logger, config.keyDir, config.rotateFreq)
	if err != nil {
		DaemonLogger.Error("failed to initialize key manager error:%s", err)
		os.Exit(1)
	}
	// Run health checks
	if err := shared.RunCryptoHealthChecks(); err != nil {
		DaemonLogger.Fatal("crypto health checks failed error:%s", err)
	}
	defer keyManager.StopRotation()
	//FIX: fix this logic for bettertrust store management
	auditLog, err := core.NewAuditLog(filepath.Join("audit.log"))
	if err != nil {
		DaemonLogger.Error("failed to initialize audit log error:%s", err)
		os.Exit(1)
	}

	auditLog.AddEntry(shared.AuditLogEntry{
		Timestamp: time.Now(),
		Action:    "daemon_start",
		Message:   "NextDeploy daemon started",
	})

	defer keyManager.StopRotation()

	// Setup HTTP servers with all routes
	mainServer, metricsServer := core.SetupServers(logger, keyManager, config.port, config.host, config.metricsPort)

	// Start servers
	errChan := make(chan error, 2)
	go startServer(mainServer, "main", logger, errChan)
	go startServer(metricsServer, "metrics", logger, errChan)

	// Setup health status
	core.SetGlobalStatus("healthy")
	core.SetComponentStatus("key_manager", "healthy")
	// setup websocket server
	agentID := os.Getenv("NEXTDEPLOY_AGENT_ID")
	wsServer := core.NewWSServer(
		keyManager.GetWSAuthKey(),
		agentID,
	)

	http.HandleFunc("ws", func(w http.ResponseWriter, r *http.Request) {
		auditLog.AddEntry(shared.AuditLogEntry{
			Timestamp: time.Now(),
			Action:    "websocket_connection",
			Message:   "New WebSocket connection established",
			Client:    r.RemoteAddr,
		})
		wsServer.HandleConnection(w, r)
	})

	// Wait for shutdown
	shutdownChan := make(chan os.Signal, 1)
	signal.Notify(shutdownChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	for {
		select {
		case sig := <-shutdownChan:
			logger.Info("received signal", "signal", sig)
			switch sig {
			case syscall.SIGHUP:
				DaemonLogger.Info("received SIGHUP, reloading configuration")
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
