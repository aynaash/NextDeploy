package daemon

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type ProcessManager struct {
	systemdDir string
}

func NewProcessManager() *ProcessManager {
	return &ProcessManager{
		systemdDir: "/etc/systemd/system/",
	}
}

// GenerateServiceFile creates a systemd service file for the Next.js app
func (pm *ProcessManager) GenerateServiceFile(appName, projectDir, outputMode string, dopplerToken string, port int, packageManager string) error {
	serviceName := fmt.Sprintf("nextdeploy-%s.service", appName)
	servicePath := filepath.Join(pm.systemdDir, serviceName)

	var execStart string
	if outputMode == "standalone" {
		cmd := "node server.js"
		if packageManager == "bun" {
			cmd = "bun server.js"
		}
		if dopplerToken != "" {
			execStart = fmt.Sprintf("doppler run --token=%s -- %s", dopplerToken, cmd)
		} else {
			execStart = cmd
		}
	} else if outputMode == "default" {
		cmd := "npm start"
		if packageManager == "bun" {
			cmd = "bun run start"
		} else if packageManager == "yarn" {
			cmd = "yarn start"
		} else if packageManager == "pnpm" {
			cmd = "pnpm start"
		}
		if dopplerToken != "" {
			execStart = fmt.Sprintf("doppler run --token=%s -- %s", dopplerToken, cmd)
		} else {
			execStart = cmd
		}
	} else {
		// export mode doesn't need a process
		log.Printf("Export mode detected for %s; no systemd service needed.", appName)
		return nil
	}

	serviceContent := fmt.Sprintf(`[Unit]
Description=NextDeploy Next.js Application (%s)
After=network.target

[Service]
Type=simple
User=root
WorkingDirectory=%s
ExecStart=%s
Restart=on-failure
Environment=NODE_ENV=production
Environment=PORT=%d

[Install]
WantedBy=multi-user.target
`, appName, projectDir, execStart, port)

	err := os.WriteFile(servicePath, []byte(serviceContent), 0644)
	if err != nil {
		return fmt.Errorf("failed to write systemd service file: %w", err)
	}

	log.Printf("Created systemd service file %s for %s", serviceName, appName)

	// Reload systemd to recognize new service
	return pm.reloadDaemon()
}

func (pm *ProcessManager) reloadDaemon() error {
	cmd := exec.Command("systemctl", "daemon-reload")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to reload systemd daemon: %v - %s", err, out)
	}
	return nil
}

// StartService enables and starts the systemd service
func (pm *ProcessManager) StartService(appName string) error {
	serviceName := fmt.Sprintf("nextdeploy-%s.service", appName)

	// Enable the service to start on boot
	cmd := exec.Command("systemctl", "enable", serviceName)
	if out, err := cmd.CombinedOutput(); err != nil {
		log.Printf("Warning: failed to enable service %s: %s", serviceName, out)
	}

	cmd = exec.Command("systemctl", "start", serviceName)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to start service %s: %v - %s", serviceName, err, out)
	}

	log.Printf("Started systemd service %s", serviceName)
	return nil
}

// StopService stops and disables the systemd service
func (pm *ProcessManager) StopService(appName string) error {
	serviceName := fmt.Sprintf("nextdeploy-%s.service", appName)

	cmd := exec.Command("systemctl", "stop", serviceName)
	if out, err := cmd.CombinedOutput(); err != nil && !strings.Contains(string(out), "not loaded") {
		log.Printf("Warning: failed to stop service %s: %s", serviceName, out)
	}

	cmd = exec.Command("systemctl", "disable", serviceName)
	if out, err := cmd.CombinedOutput(); err != nil && !strings.Contains(string(out), "not loaded") {
		log.Printf("Warning: failed to disable service %s: %s", serviceName, out)
	}

	return nil
}

// RestartService restarts the systemd service
func (pm *ProcessManager) RestartService(appName string) error {
	serviceName := fmt.Sprintf("nextdeploy-%s.service", appName)
	cmd := exec.Command("systemctl", "restart", serviceName)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to restart service %s: %v - %s", serviceName, err, out)
	}
	log.Printf("Restarted systemd service %s", serviceName)
	return nil
}

// RemoveService stops, disables, and deletes the systemd service
func (pm *ProcessManager) RemoveService(appName string) error {
	serviceName := fmt.Sprintf("nextdeploy-%s.service", appName)

	pm.StopService(appName)

	servicePath := filepath.Join(pm.systemdDir, serviceName)
	if err := os.Remove(servicePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove service file %s: %w", servicePath, err)
	}

	return pm.reloadDaemon()
}
