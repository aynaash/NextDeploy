package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
)

// Logger configuration
type LoggerConfig struct {
	LogDir      string
	LogFileName string
	MaxFileSize int64 // in bytes
	MaxBackups  int
}

// DefaultLogger creates a logger with default configuration
func DefaultLogger() *log.Logger {
	config := LoggerConfig{
		LogDir:      "/var/log/my-daemon",
		LogFileName: "daemon.log",
		MaxFileSize: 10 * 1024 * 1024, // 10MB
		MaxBackups:  5,
	}

	return SetupLogger(config)
}

// SetupLogger creates and configures a logger
func SetupLogger(config LoggerConfig) *log.Logger {
	// Create log directory if it doesn't exist
	if err := os.MkdirAll(config.LogDir, 0755); err != nil {
		log.Fatalf("Failed to create log directory: %v", err)
	}

	logPath := filepath.Join(config.LogDir, config.LogFileName)

	// Open log file (append mode, create if not exists)
	file, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalf("Failed to open log file: %v", err)
	}

	// Create logger with timestamp and file/line information
	logger := log.New(file, "", log.LstdFlags|log.Lshortfile)

	return logger
}

// RotateLog checks if log rotation is needed and performs it
func RotateLog(config LoggerConfig) error {
	logPath := filepath.Join(config.LogDir, config.LogFileName)

	// Check if file exists and its size
	info, err := os.Stat(logPath)
	if os.IsNotExist(err) {
		return nil // No file to rotate
	}
	if err != nil {
		return fmt.Errorf("failed to stat log file: %v", err)
	}

	// Check if rotation is needed
	if info.Size() < config.MaxFileSize {
		return nil
	}

	// Rotate logs
	for i := config.MaxBackups - 1; i >= 0; i-- {
		oldLog := fmt.Sprintf("%s.%d", logPath, i)
		newLog := fmt.Sprintf("%s.%d", logPath, i+1)

		if _, err := os.Stat(oldLog); err == nil {
			if i == config.MaxBackups-1 {
				// Remove the oldest backup
				os.Remove(oldLog)
			} else {
				os.Rename(oldLog, newLog)
			}
		}
	}

	// Rename current log to .0
	if err := os.Rename(logPath, logPath+".0"); err != nil {
		return fmt.Errorf("failed to rotate log: %v", err)
	}

	return nil
}

// LogMessage logs a message with rotation check
func LogMessage(logger *log.Logger, config LoggerConfig, level, message string) {
	// Check and perform log rotation if needed
	if err := RotateLog(config); err != nil {
		logger.Printf("ERROR: Log rotation failed: %v", err)
	}

	// Log the message with timestamp and level
	logger.Printf("[%s] %s", level, message)
}

// Simple logging functions
func LogInfo(logger *log.Logger, config LoggerConfig, message string) {
	LogMessage(logger, config, "INFO", message)
}

func LogError(logger *log.Logger, config LoggerConfig, message string) {
	LogMessage(logger, config, "ERROR", message)
}

func LogWarning(logger *log.Logger, config LoggerConfig, message string) {
	LogMessage(logger, config, "WARNING", message)
}
