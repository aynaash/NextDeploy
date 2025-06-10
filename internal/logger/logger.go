package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"
)

// LogLevel represents different log levels
type LogLevel int

const (
	LevelDebug LogLevel = iota
	LevelInfo
	LevelWarn
	LevelSuccess
	LevelError
	LevelFatal
)

var levelNames = map[LogLevel]string{
	LevelDebug:   "DEBUG",
	LevelInfo:    "INFO",
	LevelWarn:    "WARN",
	LevelError:   "ERROR",
	LevelFatal:   "FATAL",
	LevelSuccess: "SUCCESS",
}

var levelColors = map[LogLevel]string{
	LevelDebug:   "\033[36m",   // Cyan
	LevelInfo:    "\033[32m",   // Green
	LevelWarn:    "\033[33m",   // Yellow
	LevelError:   "\033[31m",   // Red
	LevelFatal:   "\033[31;1m", // Bright Red
	LevelSuccess: "\033[32;1m", // Bright Green
}

var levelEmojis = map[LogLevel]string{
	LevelDebug:   "🐛",
	LevelInfo:    "ℹ️",
	LevelWarn:    "⚠️",
	LevelError:   "❌",
	LevelFatal:   "💀",
	LevelSuccess: "✅",
}

// Logger is the main logger struct
type Logger struct {
	mu         sync.Mutex
	minLevel   LogLevel
	logger     *log.Logger
	showCaller bool
	packageMap map[string]string
}

// New creates a new Logger instance
func New(out io.Writer, prefix string, flag int, minLevel LogLevel) *Logger {
	return &Logger{
		minLevel:   minLevel,
		logger:     log.New(out, prefix, flag),
		showCaller: true,
		packageMap: make(map[string]string),
	}
}

// DefaultLogger creates a logger with default settings
func DefaultLogger() *Logger {
	return New(os.Stdout, "", log.Ldate|log.Ltime, LevelDebug)
}

// SetLevel sets the minimum log level
func (l *Logger) SetLevel(level LogLevel) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.minLevel = level
}

// SetOutput sets the output destination
func (l *Logger) SetOutput(w io.Writer) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.logger.SetOutput(w)
}

// EnableCallerInfo enables/disables caller information
func (l *Logger) EnableCallerInfo(enable bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.showCaller = enable
}

// RegisterPackage registers a package with a custom emoji/name
func (l *Logger) RegisterPackage(pkg string, displayName string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.packageMap[pkg] = displayName
}

// Log logs a message at a specific level
func (l *Logger) Log(level LogLevel, msg string, args ...interface{}) {
	if level < l.minLevel {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	var callerInfo string
	if l.showCaller {
		// Get caller info (file and line number)
		_, file, line, ok := runtime.Caller(2) // 2 levels up the stack
		if ok {
			// Shorten file path
			parts := strings.Split(file, "/")
			if len(parts) > 3 {
				file = strings.Join(parts[len(parts)-3:], "/")
			}
			callerInfo = fmt.Sprintf("%s:%d", file, line)
		}
	}

	// Get package display name if registered
	var pkgDisplay string
	if l.logger.Prefix() != "" {
		if display, exists := l.packageMap[l.logger.Prefix()]; exists {
			pkgDisplay = display + " "
		}
	}

	// Format the log message
	levelName := levelNames[level]
	levelColor := levelColors[level]
	levelEmoji := levelEmojis[level]
	resetColor := "\033[0m"

	formattedMsg := fmt.Sprintf(msg, args...)
	logLine := fmt.Sprintf("%s%s%s %s %s%s",
		levelColor, levelName, resetColor,
		levelEmoji,
		pkgDisplay,
		formattedMsg)

	if callerInfo != "" {
		logLine += fmt.Sprintf(" \033[90m(%s)\033[0m", callerInfo)
	}

	l.logger.Println(logLine)
}

// Debug logs a debug message
func (l *Logger) Debug(msg string, args ...interface{}) {
	l.Log(LevelDebug, msg, args...)
}

// Info logs an info message
func (l *Logger) Info(msg string, args ...interface{}) {
	l.Log(LevelInfo, msg, args...)
}

// Warn logs a warning message
func (l *Logger) Warn(msg string, args ...interface{}) {
	l.Log(LevelWarn, msg, args...)
}

// Error logs an error message
func (l *Logger) Error(msg string, args ...interface{}) {
	l.Log(LevelError, msg, args...)
}

// Fatal logs a fatal message and exits
func (l *Logger) Fatal(msg string, args ...interface{}) {
	l.Log(LevelFatal, msg, args...)
	os.Exit(1)
}

// Success logs a success message
func (l *Logger) Success(msg string, args ...interface{}) {
	l.Log(LevelSuccess, msg, args...)
}

// WithPrefix returns a new Logger with the specified prefix
func (l *Logger) WithPrefix(prefix string) *Logger {
	l.mu.Lock()
	defer l.mu.Unlock()

	newLogger := &Logger{
		minLevel:   l.minLevel,
		logger:     log.New(l.logger.Writer(), prefix, l.logger.Flags()),
		showCaller: l.showCaller,
		packageMap: l.packageMap,
	}

	// Copy the package map
	for k, v := range l.packageMap {
		newLogger.packageMap[k] = v
	}

	return newLogger
}

// PackageLogger creates a logger with package-specific settings
func PackageLogger(pkgName string, displayName string) *Logger {
	logger := DefaultLogger()
	logger.RegisterPackage(pkgName, displayName)
	return logger.WithPrefix(pkgName)
}

// Timed logs the duration of a function execution
func (l *Logger) Timed(label string, fn func()) {
	start := time.Now()
	l.Info("⏳ Starting %s...", label)
	fn()
	l.Info("✅ Completed %s in %v", label, time.Since(start))
}

// JSON logs data in JSON format (simplified version)
func (l *Logger) JSON(level LogLevel, data map[string]interface{}) {
	if level < l.minLevel {
		return
	}

	var sb strings.Builder
	sb.WriteString("{")
	first := true
	for k, v := range data {
		if !first {
			sb.WriteString(", ")
		}
		sb.WriteString(fmt.Sprintf("\"%s\": \"%v\"", k, v))
		first = false
	}
	sb.WriteString("}")

	l.Log(level, sb.String())
}

// func main() {
// 	// Create a default logger
// 	log := logger.DefaultLogger()
//
// 	// Create a package-specific logger
// 	pkgLog := logger.PackageLogger("network", "🌐 NETWORK")
//
// 	log.Info("Starting application...")
// 	log.Debug("Debug information: %s", "some debug data")
// 	log.Warn("This is a warning")
//
// 	pkgLog.Info("Connected to server")
// 	pkgLog.Error("Connection failed: %s", "timeout")
//
// 	// Measure execution time
// 	log.Timed("expensive operation", func() {
// 		// Simulate work
// 		time.Sleep(500 * time.Millisecond)
// 	})
//
// 	// Log JSON data
// 	log.JSON(logger.LevelInfo, map[string]interface{}{
// 		"user":    "john_doe",
// 		"id":      12345,
// 		"status":  "active",
// 	})
//
// 	// Fatal example (commented out to prevent exit)
// 	// log.Fatal("Critical error, exiting")
// }
