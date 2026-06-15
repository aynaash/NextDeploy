package daemon

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/aynaash/nextdeploy/shared/config"
)

// processAlive reports whether a PID still refers to a live process. Signal 0
// performs existence/permission checks without delivering a signal: nil means
// alive, EPERM means alive-but-not-ours, ESRCH means gone.
func processAlive(pidStr string) bool {
	pid, err := strconv.Atoi(pidStr)
	if err != nil || pid <= 0 {
		return false
	}
	err = syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}

type ProcessManager struct {
	systemdDir  string
	serviceName string
}

func NewProcessManager() *ProcessManager {
	return &ProcessManager{
		systemdDir:  "/etc/systemd/system/",
		serviceName: "nextdeploy.service",
	}
}

func (pm *ProcessManager) GenerateServiceFile(appName, projectDir, outputMode string, dopplerToken string, port int, packageManager string, releaseID string, limits *config.ResourceLimits) (string, bool, error) {
	serviceName := fmt.Sprintf("nextdeploy-%s-%s.service", appName, releaseID)
	servicePath := filepath.Join(pm.systemdDir, serviceName)

	log.Printf("[process] Generating service file: %s (mode=%s, dir=%s, port=%d, pkg=%s)",
		servicePath, outputMode, projectDir, port, packageManager)

	execStart, err := pm.resolveExecStart(outputMode, packageManager, dopplerToken)
	if err != nil {
		return "", false, err
	}
	if execStart == "" { // Export mode
		log.Printf("[process] Export mode detected for %s; no systemd service needed.", appName)
		return "", false, nil
	}

	// Validate before any value reaches the unit file — a crafted resource
	// string must never be able to inject extra directives.
	if err := limits.Validate(); err != nil {
		return "", false, err
	}
	resourceBlock := renderResourceLimits(limits)

	serviceContent := fmt.Sprintf(`[Unit]
Description=NextDeploy Next.js Application (%s)
After=network.target

[Service]
Type=simple
User=nextdeploy
Group=nextdeploy
WorkingDirectory=%s
ExecStart=%s
Restart=on-failure
RestartSec=5s

# Lifecycle: guarantee fast, complete port release on rollout. A hung Node
# process (unclosed pool, stuck async) is SIGKILLed after the timeout, and
# KillMode=control-group reaps the whole cgroup so no child lingers on the port.
TimeoutStopSec=10s
KillMode=control-group
KillSignal=SIGTERM
FinalKillSignal=SIGKILL
OOMPolicy=stop
Environment=NODE_ENV=production
Environment=PORT=%d
EnvironmentFile=-%s/.env.nextdeploy
%s
# Security Sandboxing
ProtectSystem=strict
ProtectHome=yes
PrivateTmp=yes
NoNewPrivileges=yes
ProtectControlGroups=yes
ProtectKernelModules=yes
ProtectKernelTunables=yes
RestrictAddressFamilies=AF_UNIX AF_INET AF_INET6
RestrictNamespaces=yes
RestrictRealtime=yes
LockPersonality=yes
ReadWritePaths=%s

[Install]
WantedBy=multi-user.target
`, appName, projectDir, execStart, port, projectDir, resourceBlock, projectDir)

	if dopplerToken != "" {
		envFilePath := filepath.Join(projectDir, ".env.nextdeploy")
		envContent := fmt.Sprintf("DOPPLER_TOKEN=%s\n", dopplerToken)
		if err := os.WriteFile(envFilePath, []byte(envContent), 0600); err != nil {
			log.Printf("[process] Warning: failed to write environment file: %v", err)
		}
		_ = os.Chmod(envFilePath, 0600)
	}

	log.Printf("[process] Writing service file to %s", servicePath)
	// #nosec G301
	// #nosec G301
	if err := os.MkdirAll(filepath.Dir(servicePath), 0750); err != nil {
		return "", false, fmt.Errorf("failed to create systemd dir: %w", err)
	}

	// #nosec G306
	err = os.WriteFile(servicePath, []byte(serviceContent), 0644)
	if err != nil {
		return "", false, fmt.Errorf("failed to write systemd service file %s: %w", servicePath, err)
	}

	if _, statErr := os.Stat(servicePath); statErr != nil {
		return "", false, fmt.Errorf("service file written but not found on disk: %w", statErr)
	}

	log.Printf("[process] Created systemd service file %s for %s", serviceName, appName)

	if err := pm.reloadDaemon(); err != nil {
		return "", false, fmt.Errorf("daemon-reload after writing %s: %w", serviceName, err)
	}

	time.Sleep(500 * time.Millisecond)

	return serviceName, true, nil
}

// renderResourceLimits emits the systemd cgroup directives for the opt-in
// resource block. Returns "" when nothing is configured so the unit file is
// byte-for-byte identical to the pre-feature output (limits off by default).
// Values are assumed already validated by config.ResourceLimits.Validate.
func renderResourceLimits(limits *config.ResourceLimits) string {
	if limits == nil {
		return ""
	}
	var b strings.Builder
	if limits.CPUQuota != "" {
		// CPUAccounting is implicit on cgroup v2 but harmless and explicit.
		fmt.Fprintf(&b, "CPUAccounting=true\nCPUQuota=%s\n", limits.CPUQuota)
	}
	if limits.MemoryMax != "" {
		fmt.Fprintf(&b, "MemoryAccounting=true\nMemoryMax=%s\n", limits.MemoryMax)
	}
	if limits.MemoryHigh != "" {
		fmt.Fprintf(&b, "MemoryHigh=%s\n", limits.MemoryHigh)
	}
	if b.Len() == 0 {
		return ""
	}
	return "\n# --- Resource limits (cgroup, opt-in via nextdeploy.yml) ---\n" + b.String()
}

func (pm *ProcessManager) resolveExecStart(outputMode, packageManager, dopplerToken string) (string, error) {
	var cmd string
	switch outputMode {
	case "standalone":
		bin := resolveBinary("node")
		if packageManager == "bun" {
			bin = resolveBinary("bun")
		}
		cmd = fmt.Sprintf("%s server.js", bin)
	case "default":
		bin := resolveBinary("npm")
		args := "start"
		switch packageManager {
		case "bun":
			bin = resolveBinary("bun")
			args = "run start"
		case "yarn":
			bin = resolveBinary("yarn")
		case "pnpm":
			bin = resolveBinary("pnpm")
		}
		cmd = fmt.Sprintf("%s %s", bin, args)
	case "export":
		return "", nil
	default:
		return "", fmt.Errorf("unknown output mode: %q", outputMode)
	}

	if dopplerToken != "" {
		return fmt.Sprintf("%s run -- %s", resolveBinary("doppler"), cmd), nil
	}
	return cmd, nil
}

func resolveBinary(name string) string {
	candidates := map[string]string{
		"node":    "/usr/local/bin/node",
		"bun":     "/usr/local/bin/bun",
		"npm":     "/usr/local/bin/npm",
		"yarn":    "/usr/local/bin/yarn",
		"pnpm":    "/usr/local/bin/pnpm",
		"doppler": "/usr/local/bin/doppler",
	}

	if path, ok := candidates[name]; ok {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	if path, err := exec.LookPath(name); err == nil {
		log.Printf("[process] Resolved %s via PATH: %s", name, path)
		return path
	}

	// Fallback to resolveTool if candidates and PATH fail
	return resolveTool(name)
}

func (pm *ProcessManager) reloadDaemon() error {
	log.Printf("[process] Running systemctl daemon-reload")
	systemctl := resolveTool("systemctl")
	// #nosec G204
	cmd := exec.Command(systemctl, "daemon-reload")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to reload systemd daemon: %w - %s", err, out)
	}
	log.Printf("[process] systemctl daemon-reload succeeded")
	return nil
}

func (pm *ProcessManager) StartService(serviceName string) error {
	systemctl := resolveTool("systemctl")
	log.Printf("[process] Enabling service %s", serviceName)
	// #nosec G204
	cmd := exec.Command(systemctl, "enable", serviceName)
	if out, err := cmd.CombinedOutput(); err != nil {
		log.Printf("[process] Warning: failed to enable service %s: %v - %s", serviceName, err, string(out))
	}

	log.Printf("[process] Starting service %s", serviceName)
	// #nosec G204
	cmd = exec.Command(systemctl, "start", serviceName)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to start service %s: %w - %s", serviceName, err, string(out))
	}

	log.Printf("[process] Started systemd service %s", serviceName)
	return nil
}

func (pm *ProcessManager) StopService(serviceName string) error {
	systemctl := resolveTool("systemctl")

	// Get MainPID before stopping
	// #nosec G204
	pidCmd := exec.Command(systemctl, "show", "-p", "MainPID", "--value", serviceName)
	pidOut, _ := pidCmd.CombinedOutput()
	pidStr := strings.TrimSpace(string(pidOut))

	// systemctl stop blocks until the unit is fully stopped. With the unit's
	// TimeoutStopSec/KillSignal=SIGTERM, systemd sends SIGTERM first and gives
	// Node a grace window to drain in-flight requests (streaming, SSE, slow
	// uploads), only escalating to SIGKILL itself after the timeout. So after a
	// successful stop the process is already gone — no manual kill needed.
	// #nosec G204
	cmd := exec.Command(systemctl, "stop", serviceName)
	stopFailed := false
	if out, err := cmd.CombinedOutput(); err != nil && !strings.Contains(string(out), "not loaded") {
		stopFailed = true
		log.Printf("[process] Warning: systemctl stop %s failed: %v - %s", serviceName, err, string(out))
	}

	// Last resort: only force-kill if the graceful stop failed AND the process
	// is somehow still alive. Under normal operation this never fires; it guards
	// against a wedged systemctl that returned an error without reaping the unit.
	if stopFailed && pidStr != "" && pidStr != "0" && processAlive(pidStr) {
		log.Printf("[process] Graceful stop failed; force-killing process %s for service %s", pidStr, serviceName)
		// #nosec G204
		_ = exec.Command("sudo", "kill", "-9", pidStr).Run()
		// Also kill the process group to be sure
		// #nosec G204
		_ = exec.Command("sudo", "kill", "-9", "-"+pidStr).Run()
	}

	// #nosec G204
	cmd = exec.Command(systemctl, "disable", serviceName)
	if out, err := cmd.CombinedOutput(); err != nil && !strings.Contains(string(out), "not loaded") {
		return fmt.Errorf("failed to disable service %s: %w - %s", serviceName, err, string(out))
	}

	return nil
}

func (pm *ProcessManager) CurrentServiceName() string {
	return pm.serviceName
}

func (pm *ProcessManager) RestartService(serviceName string) error {
	systemctl := resolveTool("systemctl")
	// #nosec G204
	cmd := exec.Command(systemctl, "restart", serviceName)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to restart service %s: %w - %s", serviceName, err, out)
	}
	log.Printf("Restarted systemd service %s", serviceName)
	return nil
}

func (pm *ProcessManager) RemoveService(serviceName string) error {
	_ = pm.StopService(serviceName)
	servicePath := filepath.Join(pm.systemdDir, serviceName)
	if err := os.Remove(servicePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove service file %s: %w", servicePath, err)
	}

	return pm.reloadDaemon()
}

func (pm *ProcessManager) FindAppServices(appName string) ([]string, error) {
	files, err := os.ReadDir(pm.systemdDir)
	if err != nil {
		return nil, err
	}

	var services []string
	prefix := fmt.Sprintf("nextdeploy-%s-", appName)
	legacyName := fmt.Sprintf("nextdeploy-%s.service", appName)

	for _, f := range files {
		name := f.Name()
		if strings.HasPrefix(name, prefix) && strings.HasSuffix(name, ".service") {
			services = append(services, name)
		} else if name == legacyName {
			services = append(services, name)
		}
	}
	return services, nil
}
