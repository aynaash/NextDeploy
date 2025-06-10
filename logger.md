
package main

import (
	"fmt"
	"os"
	"time"
	"github.com/fatih/color"
)

type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
	FATAL
)

type Logger struct {
	minLevel LogLevel
}

func NewLogger(minLevel LogLevel) *Logger {
	return &Logger{minLevel: minLevel}
}

func (l *Logger) log(level LogLevel, label string, c *color.Color, format string, args ...interface{}) {
	if level < l.minLevel {
		return
	}

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	message := fmt.Sprintf(format, args...)
	levelStr := fmt.Sprintf("[%s]", label)
	output := fmt.Sprintf("%s %s %s", timestamp, levelStr, message)

	c.Println(output)

	if level == FATAL {
		os.Exit(1)
	}
}

func (l *Logger) Debug(format string, args ...interface{}) {
	l.log(DEBUG, "DEBUG", color.New(color.FgCyan), format, args...)
}

func (l *Logger) Info(format string, args ...interface{}) {
	l.log(INFO, "INFO", color.New(color.FgGreen), format, args...)
}

func (l *Logger) Warn(format string, args ...interface{}) {
	l.log(WARN, "WARN", color.New(color.FgYellow), format, args...)
}

func (l *Logger) Error(format string, args ...interface{}) {
	l.log(ERROR, "ERROR", color.New(color.FgRed), format, args...)
}

func (l *Logger) Fatal(format string, args ...interface{}) {
	l.log(FATAL, "FATAL", color.New(color.FgHiRed, color.Bold), format, args...)
}

