package core

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"time"
)

func GracefulShutdown(ctx context.Context, mainServer *http.Server, metricsServer *http.Server, logger *slog.Logger, daemonize bool, pidFile string) {
	logger.Info("initiating graceful shutdown")

	shutdownCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	if err := mainServer.Shutdown(shutdownCtx); err != nil {
		logger.Error("main server shutdown error", "error", err)
	}

	if err := metricsServer.Shutdown(shutdownCtx); err != nil {
		logger.Error("metrics server shutdown error", "error", err)
	}

	if daemonize {
		if err := os.Remove(pidFile); err != nil && !os.IsNotExist(err) {
			logger.Error("failed to remove PID file", "error", err)
		}
	}

	logger.Info("shutdown completed")
}
