package core

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

// Daemonize turns the current process into a daemon.
// pidFile specifies where to write the PID file, logger is used for logging.

func Daemonize(logger *slog.Logger, pidFile string) error {
	if os.Getpid() == 1 {
		// Already a daemon
		return nil
	}
	args := make([]string, 0, len(os.Args[1:]))
	for _, arg := range os.Args[1:] {
		if arg != "--daemon" {
			args = append(args, arg)
		}
	}
	if err := os.MkdirAll(filepath.Dir(pidFile), 0755); err != nil {
		return fmt.Errorf("failed to create PID file directory: %w", err)
	}
	cmd := exec.Command(os.Args[0], args...)
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true, // Create new session
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start daemon process: %w", err)
	}
	tempFile := pidFile + ".tmp"
	if err := os.WriteFile(tempFile, []byte(fmt.Sprintf("%d", cmd.Process.Pid)), 0644); err != nil {
		// Try to kill the started process if we can't write PID file
		_ = cmd.Process.Kill()
		return fmt.Errorf("failed to write PID file: %w", err)
	}
	if err := os.Rename(tempFile, pidFile); err != nil {
		_ = cmd.Process.Kill()
		return fmt.Errorf("failed to rename PID file: %w", err)
	}
	logger.Info("daemonized process started", "pid", cmd.Process.Pid, "pid_file", pidFile)
	os.Exit(0) // Exit the parent process
	return nil
}
