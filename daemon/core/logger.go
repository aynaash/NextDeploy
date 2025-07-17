package core

import (
	"fmt"
	"log/slog"
	"os"
)

func SetupLogger() (*slog.Logger, *os.File) {
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
