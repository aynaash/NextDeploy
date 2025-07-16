package main

// TODO(nextdeploy): Critical daemon improvements for production readiness
//
// ─────────────────────────────────────────────────────────────────────────────
// 🔒 SECURITY / PORTABILITY
// ─────────────────────────────────────────────────────────────────────────────
// - [ ] Add TLS support for both main and metrics servers (self-signed or config-driven).
// - [ ] Replace hardcoded paths (/var/lib, /var/run, /var/log) with configurable or XDG-based paths.
// - [ ] Add ENV var + config file (YAML/TOML) support using a config loader (e.g., Viper or custom).
//
// ─────────────────────────────────────────────────────────────────────────────
// ⚙️  SYSTEM RESILIENCE / ABSTRACTION
// ─────────────────────────────────────────────────────────────────────────────
// - [ ] Wrap all goroutines (e.g., startServer) with panic recovery + stack trace logging.
// - [ ] Create a `DaemonApp` struct to encapsulate and manage lifecycle of components (keyManager, servers, etc).
// - [ ] Decouple core component initialization out of main(); use start()/stop() methods per subsystem.
// - [ ] Add startup readiness vs liveness health endpoints (/health/startup, /health/live).
//
// ─────────────────────────────────────────────────────────────────────────────
// 📈 OBSERVABILITY
// ─────────────────────────────────────────────────────────────────────────────
// - [ ] Add request correlation ID middleware to all HTTP handlers (traceability).
// - [ ] Integrate OpenTelemetry support for tracing + metrics.
// - [ ] Expand /metrics to include more detailed runtime stats (GC, memory, goroutines, etc).
//
// ─────────────────────────────────────────────────────────────────────────────
// 🤖 BACKGROUND DAEMONS + SUPERVISION
// ─────────────────────────────────────────────────────────────────────────────
// - [ ] Build a Supervisor pattern to manage background daemons (deployment monitor, log stream, container checker).
// - [ ] Add daemon health probes + restart policies for background workers.
//
// ─────────────────────────────────────────────────────────────────────────────
// 🧠 FUTURE-SCALING ARCHITECTURE
// ─────────────────────────────────────────────────────────────────────────────
// - [ ] Prepare coordination/locking layer (Redis, etcd, or file-based) for future HA/multi-node deployments.
// - [ ] Allow dynamic reloading of config (SIGHUP logic needs to reload flags/config/env).
import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"nextdeploy/daemon/core"
	"os"
	"os/exec"
	"os/signal"
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

func daemonize(logger *slog.Logger) {
	if os.Getppid() != 1 {
		args := []string{}
		for _, arg := range os.Args[1:] {
			if arg != "--daemon" {
				args = append(args, arg)
			}
		}

		cmd := exec.Command(os.Args[0], args...)
		cmd.Stdout = nil
		cmd.Stderr = nil
		cmd.Stdin = nil
		cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

		if err := cmd.Start(); err != nil {
			logger.Error("failed to daemonize", "error", err)
			os.Exit(1)
		}

		if err := os.WriteFile(config.pidFile, []byte(fmt.Sprintf("%d", cmd.Process.Pid)), 0644); err != nil {
			logger.Error("failed to write PID file", "error", err)
			os.Exit(1)
		}

		os.Exit(0)
	}
}

func setupLogger() (*slog.Logger, *os.File) {
	var logOutput *os.File
	var err error

	if config.daemonize {
		logOutput, err = os.OpenFile(config.logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			fmt.Printf("failed to open log file: %v\n", err)
			os.Exit(1)
		}
	} else {
		logOutput = os.Stdout
	}

	opts := &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}
	if config.debug {
		opts.Level = slog.LevelDebug
	}

	var handler slog.Handler
	if config.logFormat == "json" {
		handler = slog.NewJSONHandler(logOutput, opts)
	} else {
		handler = slog.NewTextHandler(logOutput, opts)
	}

	return slog.New(handler), logOutput
}

func setupKeyManager(logger *slog.Logger) (*core.KeyManager, error) {
	logger.Info("initializing key manager", "key_dir", config.keyDir, "rotation_interval", config.rotateFreq)

	if err := os.MkdirAll(config.keyDir, 0700); err != nil {
		logger.Error("failed to create key directory", "error", err)
		return nil, err
	}

	keyManager, err := core.NewKeyManager(config.keyDir, config.rotateFreq)
	if err != nil {
		logger.Error("failed to initialize key manager", "error", err)
		return nil, err
	}

	keyManager.StartRotation()
	logger.Info("key manager initialized successfully")
	return keyManager, nil
}

func setupServers(logger *slog.Logger, keyManager *core.KeyManager) (*http.Server, *http.Server) {
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

func startServer(server *http.Server, name string, logger *slog.Logger, errChan chan<- error) {
	logger.Info("starting server", "name", name, "address", server.Addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error("server error", "name", name, "error", err)
		errChan <- err
	}
}

func gracefulShutdown(ctx context.Context, mainServer *http.Server, metricsServer *http.Server, logger *slog.Logger) {
	logger.Info("initiating graceful shutdown")

	shutdownCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	if err := mainServer.Shutdown(shutdownCtx); err != nil {
		logger.Error("main server shutdown error", "error", err)
	}

	if err := metricsServer.Shutdown(shutdownCtx); err != nil {
		logger.Error("metrics server shutdown error", "error", err)
	}

	if config.daemonize {
		if err := os.Remove(config.pidFile); err != nil && !os.IsNotExist(err) {
			logger.Error("failed to remove PID file", "error", err)
		}
	}

	logger.Info("shutdown completed")
}

func main() {
	flag.Parse()

	logger, logFile := setupLogger()
	defer logFile.Close()

	if config.daemonize {
		daemonize(logger)
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
	defer keyManager.StopRotation()

	// Setup HTTP servers with all routes
	mainServer, metricsServer := setupServers(logger, keyManager)

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
				gracefulShutdown(ctx, mainServer, metricsServer, logger)
				return
			}
		case err := <-errChan:
			logger.Error("server error", "error", err)
			core.SetGlobalStatus("unhealthy")
			gracefulShutdown(ctx, mainServer, metricsServer, logger)
			return
		}
	}
}
