package shared

import (
	"encoding/json"
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
	LevelTrace LogLevel = iota
	LevelDebug
	LevelInfo
	LevelWarn
	LevelSuccess
	LevelError
	LevelFatal
)

var levelNames = map[LogLevel]string{
	LevelTrace:   "TRACE",
	LevelDebug:   "DEBUG",
	LevelInfo:    "INFO ",
	LevelWarn:    "WARN ",
	LevelError:   "ERROR",
	LevelFatal:   "FATAL",
	LevelSuccess: "GOOD ",
}

var levelColors = map[LogLevel]string{
	LevelTrace:   "\033[38;5;245m", // Gray
	LevelDebug:   "\033[38;5;14m",  // Bright Cyan
	LevelInfo:    "\033[38;5;12m",  // Bright Blue
	LevelWarn:    "\033[38;5;11m",  // Bright Yellow
	LevelError:   "\033[38;5;9m",   // Bright Red
	LevelFatal:   "\033[48;5;9m",   // Red background
	LevelSuccess: "\033[38;5;10m",  // Bright Green
}

var levelEmojis = map[LogLevel]string{
	LevelTrace:   "🔍",
	LevelDebug:   "🐞",
	LevelInfo:    "ℹ️ ",
	LevelWarn:    "⚠️ ",
	LevelError:   "💥",
	LevelFatal:   "☠️ ",
	LevelSuccess: "✨",
}

var levelBanners = map[LogLevel]string{
	LevelTrace:   "▁▁▁▁▁",
	LevelDebug:   "▂▂▂▂▂",
	LevelInfo:    "▃▃▃▃▃",
	LevelWarn:    "▅▅▅▅▅",
	LevelError:   "▆▆▆▆▆",
	LevelFatal:   "▇▇▇▇▇",
	LevelSuccess: "▔▔▔▔▔",
}

// Logger is the main logger struct
type Logger struct {
	mu            sync.Mutex
	minLevel      LogLevel
	logger        *log.Logger
	showCaller    bool
	showTimestamp bool
	showBanner    bool
	packageMap    map[string]string
	colorEnabled  bool
	timeFormat    string
	indentLevel   int
}

// New creates a new Logger instance
func New(out io.Writer, prefix string, flag int, minLevel LogLevel) *Logger {
	return &Logger{
		minLevel:      minLevel,
		logger:        log.New(out, prefix, 0), // We handle flags ourselves
		showCaller:    true,
		showTimestamp: true,
		showBanner:    true,
		packageMap:    make(map[string]string),
		colorEnabled:  true,
		timeFormat:    "2006-01-02 15:04:05.000",
	}
}

// DefaultLogger creates a logger with default settings
func DefaultLogger() *Logger {
	return New(os.Stdout, "", 0, LevelDebug)
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

// EnableTimestamp enables/disables timestamp
func (l *Logger) EnableTimestamp(enable bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.showTimestamp = enable
}

// EnableBanner enables/disables the level banner
func (l *Logger) EnableBanner(enable bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.showBanner = enable
}

// EnableColor enables/disables color output
func (l *Logger) EnableColor(enable bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.colorEnabled = enable
}

// SetTimeFormat sets the timestamp format (default: "2006-01-02 15:04:05.000")
func (l *Logger) SetTimeFormat(format string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.timeFormat = format
}

// RegisterPackage registers a package with a custom emoji/name
func (l *Logger) RegisterPackage(pkg string, displayName string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.packageMap[pkg] = displayName
}

// Indent increases the indentation level
func (l *Logger) Indent() *Logger {
	l.mu.Lock()
	defer l.mu.Unlock()
	return &Logger{
		minLevel:      l.minLevel,
		logger:        l.logger,
		showCaller:    l.showCaller,
		showTimestamp: l.showTimestamp,
		showBanner:    l.showBanner,
		packageMap:    l.packageMap,
		colorEnabled:  l.colorEnabled,
		timeFormat:    l.timeFormat,
		indentLevel:   l.indentLevel + 1,
	}
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
	levelBanner := levelBanners[level]
	resetColor := "\033[0m"

	if !l.colorEnabled {
		levelColor = ""
		resetColor = ""
	}

	formattedMsg := fmt.Sprintf(msg, args...)

	// Add indentation
	indent := strings.Repeat("  ", l.indentLevel)
	formattedMsg = indent + strings.Replace(formattedMsg, "\n", "\n"+indent, -1)

	var logLine strings.Builder

	// Timestamp
	if l.showTimestamp {
		logLine.WriteString(fmt.Sprintf("\033[90m%s\033[0m ", time.Now().Format(l.timeFormat)))
	}

	// Level banner
	if l.showBanner {
		logLine.WriteString(fmt.Sprintf("%s%s%s ", levelColor, levelBanner, resetColor))
	}

	// Level info
	logLine.WriteString(fmt.Sprintf("%s%s%s %s ", levelColor, levelName, resetColor, levelEmoji))

	// Package display
	if pkgDisplay != "" {
		logLine.WriteString(fmt.Sprintf("%s", pkgDisplay))
	}

	// Message
	logLine.WriteString(formattedMsg)

	// Caller info
	if callerInfo != "" {
		logLine.WriteString(fmt.Sprintf(" \033[90m(%s)\033[0m", callerInfo))
	}

	l.logger.Println(logLine.String())
}

// Trace logs a trace message (most verbose)
func (l *Logger) Trace(msg string, args ...interface{}) {
	l.Log(LevelTrace, msg, args...)
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
		minLevel:      l.minLevel,
		logger:        log.New(l.logger.Writer(), prefix, 0),
		showCaller:    l.showCaller,
		showTimestamp: l.showTimestamp,
		showBanner:    l.showBanner,
		packageMap:    make(map[string]string),
		colorEnabled:  l.colorEnabled,
		timeFormat:    l.timeFormat,
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

// Timed logs the duration of a function execution with a spinner animation
func (l *Logger) Timed(label string, fn func()) {
	start := time.Now()
	done := make(chan bool)

	go func() {
		spinner := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
		i := 0
		for {
			select {
			case <-done:
				return
			default:
				l.mu.Lock()
				levelColor := levelColors[LevelInfo]
				resetColor := "\033[0m"
				if !l.colorEnabled {
					levelColor = ""
					resetColor = ""
				}
				msg := fmt.Sprintf("%s%s%s %s %s...", levelColor, levelNames[LevelInfo], resetColor, spinner[i], label)
				if l.showTimestamp {
					msg = fmt.Sprintf("\033[90m%s\033[0m %s", time.Now().Format(l.timeFormat), msg)
				}
				l.logger.Print("\r" + msg)
				l.mu.Unlock()
				i = (i + 1) % len(spinner)
				time.Sleep(100 * time.Millisecond)
			}
		}
	}()

	fn()
	close(done)

	// Clear the spinner line
	l.logger.Print("\r\033[K")

	duration := time.Since(start)
	l.Info("%s completed in %s", label, duration)
}

// JSON logs data in pretty-printed JSON format
func (l *Logger) JSON(level LogLevel, data interface{}) {
	if level < l.minLevel {
		return
	}

	jsonData, err := json.MarshalIndent(data, strings.Repeat("  ", l.indentLevel), "  ")
	if err != nil {
		l.Error("Failed to marshal JSON: %v", err)
		return
	}

	fmtattedJSON := string(jsonData)
	fmt.Println("" + fmtattedJSON)

}

// Table logs tabular data
func (l *Logger) Table(level LogLevel, headers []string, rows [][]string) {
	if level < l.minLevel {
		return
	}

	// Calculate column widths
	colWidths := make([]int, len(headers))
	for i, h := range headers {
		colWidths[i] = len(h)
	}

	for _, row := range rows {
		for i, cell := range row {
			if len(cell) > colWidths[i] {
				colWidths[i] = len(cell)
			}
		}
	}

	// Build the table
	var table strings.Builder

	// Header
	table.WriteString("\n")
	for i, h := range headers {
		table.WriteString(fmt.Sprintf(" %-*s ", colWidths[i], h))
		if i < len(headers)-1 {
			table.WriteString("│")
		}
	}

	// Separator
	table.WriteString("\n")
	for i, w := range colWidths {
		table.WriteString(strings.Repeat("─", w+2))
		if i < len(colWidths)-1 {
			table.WriteString("┼")
		}
	}
	table.WriteString("\n")

	// Rows
	for _, row := range rows {
		for i, cell := range row {
			table.WriteString(fmt.Sprintf(" %-*s ", colWidths[i], cell))
			if i < len(row)-1 {
				table.WriteString("│")
			}
		}
		table.WriteString("\n")
	}

	l.Info("%s", table.String())

}

// Progress creates a progress bar
func (l *Logger) Progress(level LogLevel, current, total int, label string) {
	if level < l.minLevel {
		return
	}

	const barWidth = 30
	progress := float64(current) / float64(total)
	filled := int(barWidth * progress)

	bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)

	levelColor := levelColors[level]
	resetColor := "\033[0m"
	if !l.colorEnabled {
		levelColor = ""
		resetColor = ""
	}

	msg := fmt.Sprintf("%s [%s] %3.0f%% %s", label, bar, progress*100, levelEmojis[level])

	l.mu.Lock()
	defer l.mu.Unlock()

	var logLine strings.Builder

	if l.showTimestamp {
		logLine.WriteString(fmt.Sprintf("\033[90m%s\033[0m ", time.Now().Format(l.timeFormat)))
	}

	logLine.WriteString(fmt.Sprintf("%s%s%s %s %s",
		levelColor, levelNames[level], resetColor,
		levelEmojis[level], msg))

	l.logger.Print("\r" + logLine.String())
	if current >= total {
		l.logger.Println()
	}
}

// package main
//
// import (
// 	"os"
// 	"time"
// 	"yourmodulepath/shared"
// )
//
// func main() {
// 	// Create and configure logger
// 	logger := shared.DefaultLogger()
// 	logger.SetLevel(shared.LevelDebug)
// 	logger.RegisterPackage("main", "🏁 MAIN")
//
// 	// Basic logging
// 	logger.Info("Application starting")
//
// 	// Package-specific logging
// 	dbLogger := shared.PackageLogger("database", "📦 DB")
// 	apiLogger := shared.PackageLogger("api", "🌐 API")
//
// 	dbLogger.Info("Connecting to database...")
// 	apiLogger.Info("Starting HTTP server...")
//
// 	// Timed operation
// 	logger.Timed("Data processing", func() {
// 		time.Sleep(1 * time.Second)
// 		indented := logger.Indent()
// 		indented.Info("Processing chunk 1")
// 		time.Sleep(500 * time.Millisecond)
// 		indented.Info("Processing chunk 2")
// 	})
//
// 	// JSON logging
// 	config := map[string]interface{}{
// 		"env":     "production",
// 		"version": "1.2.3",
// 		"ports":   []int{8080, 8081},
// 	}
// 	logger.JSON(shared.LevelDebug, config)
//
// 	// Table logging
// 	headers := []string{"ID", "Name", "Status"}
// 	rows := [][]string{
// 		{"1", "Service A", "OK"},
// 		{"2", "Service B", "WARNING"},
// 		{"3", "Service C", "ERROR"},
// 	}
// 	logger.Table(shared.LevelInfo, headers, rows)
//
// 	// Progress bar
// 	logger.Info("Processing items:")
// 	total := 25
// 	for i := 0; i <= total; i++ {
// 		logger.Progress(shared.LevelInfo, i, total, "Items")
// 		time.Sleep(100 * time.Millisecond)
// 	}
//
// 	logger.Success("All operations completed successfully!")
// }
