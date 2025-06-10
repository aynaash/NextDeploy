package secrets

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"github.com/fatih/color"
	"golang.org/x/term"
	"io"
	"os"
	"path/filepath"
	"syscall"
)

func GetConfigPath(filename string) string {
	// Try to use XDG_CONFIG_HOME if set, otherwise use ~/.config
	configDir := os.Getenv("XDG_CONFIG_HOME")
	if configDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			// Fallback to current directory if we can't get home dir
			return filename
		}
		configDir = filepath.Join(home, ".config")
	}

	// Create nextdeploy subdirectory
	appDir := filepath.Join(configDir, "nextdeploy")
	if err := os.MkdirAll(appDir, 0700); err != nil {
		// Fallback to current directory if we can't create config dir
		return filename
	}

	return filepath.Join(appDir, filename)
}

func GetMasterKey() string {
	// Try to read from environment variable first
	if key := os.Getenv("NEXTDEPLOY_MASTER_KEY"); key != "" {
		return key
	}

	// Try to read from key file
	keyPath := filepath.Join(GetConfigPath(""), ".masterkey")
	if data, err := os.ReadFile(keyPath); err == nil {
		return string(data)
	}

	// Prompt user for master key
	fmt.Print("Enter master key (will be stored securely): ")
	byteKey, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Println() // Add newline after password input
	if err != nil {
		panic("Failed to read master key: " + err.Error())
	}
	key := string(byteKey)

	// Generate a new key if empty
	if key == "" {
		key = generateSecureKey()
		fmt.Println("Generated new secure master key")
	}

	// Store the key securely
	if err := os.WriteFile(keyPath, []byte(key), 0600); err != nil {
		fmt.Printf("Warning: Could not store master key: %v\n", err)
		fmt.Println("You'll need to enter the key again next time.")
		fmt.Printf("Your master key is: %s\n", key)
		fmt.Println("Please store it securely!")
	}

	return key
}

func generateSecureKey() string {
	key := make([]byte, 32) // 256-bit key
	if _, err := rand.Read(key); err != nil {
		panic("Failed to generate secure key: " + err.Error())
	}
	return base64.StdEncoding.EncodeToString(key)
}

type cliLogger struct {
	out    io.Writer
	errOut io.Writer
}

func NewCLILogger() Logger {
	return &cliLogger{
		out:    os.Stdout,
		errOut: os.Stderr,
	}
}

func (l *cliLogger) Debug(msg string, args ...interface{}) {
	if os.Getenv("DEBUG") == "true" {
		message := fmt.Sprintf(msg, args...)
		color.New(color.FgHiBlack).Fprintln(l.out, "DEBUG:", message)
	}
}

func (l *cliLogger) Info(msg string, args ...interface{}) {
	message := fmt.Sprintf(msg, args...)
	color.New(color.FgHiBlue).Fprintln(l.out, message)
}

func (l *cliLogger) Error(msg string, args ...interface{}) {
	message := fmt.Sprintf(msg, args...)
	color.New(color.FgRed).Fprintln(l.errOut, "ERROR:", message)
}

// Optional: Add these methods if your Logger interface requires them
func (l *cliLogger) Success(msg string, args ...interface{}) {
	message := fmt.Sprintf(msg, args...)
	color.New(color.FgGreen).Fprintln(l.out, message)
}

func (l *cliLogger) Warning(msg string, args ...interface{}) {
	message := fmt.Sprintf(msg, args...)
	color.New(color.FgYellow).Fprintln(l.out, "WARNING:", message)
}
