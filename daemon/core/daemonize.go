package core

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"syscall"
)

func Daemonize(logger *slog.Logger) {
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

		if err := os.WriteFile(Config.pidFile, []byte(fmt.Sprintf("%d", cmd.Process.Pid)), 0644); err != nil {
			logger.Error("failed to write PID file", "error", err)
			os.Exit(1)
		}

		os.Exit(0)
	}
}
