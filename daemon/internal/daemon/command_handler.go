package daemon

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"time"

	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"io"
	"nextdeploy/daemon/internal/types"
	"nextdeploy/shared/nextcore"
	"path/filepath"
	"strings"
)

type CommandHandler struct {
	config         *types.DaemonConfig
	caddyManager   *CaddyManager
	processManager *ProcessManager
}

func NewCommandHandler(config *types.DaemonConfig) *CommandHandler {
	return &CommandHandler{
		config:         config,
		caddyManager:   NewCaddyManager(),
		processManager: NewProcessManager(),
	}
}

func (ch *CommandHandler) HandleCommand(cmd types.Command) types.Response {
	switch cmd.Type {
	case "setupCaddy":
		return ch.setUpCaddy(cmd.Args)
	case "stopdaemon":
		return ch.stopDaemon(cmd.Args)
	case "restartDaemon":
		return ch.restartDaemon(cmd.Args)
	case "ship":
		return ch.handleShip(cmd.Args)
	default:
		return types.Response{
			Success: false,
			Message: fmt.Sprintf("Unknown command: %s", cmd.Type),
		}
	}
}

func (ch *CommandHandler) stopDaemon(args map[string]interface{}) types.Response {
	fmt.Println("Stopping daemon...")
	ch.Shutdown()
	return types.Response{
		Success: true,
		Message: "Daemon stopped successfully",
	}
}

func (ch *CommandHandler) restartDaemon(args map[string]interface{}) types.Response {
	fmt.Println("Restarting daemon...")
	ch.Shutdown()
	time.Sleep(2 * time.Second) // wait for shutdown
	// restart the process
	execPath, err := os.Executable()
	if err != nil {
		return types.Response{
			Success: false,
			Message: fmt.Sprintf("Failed to get executable path: %v", err),
		}
	}
	cmd := exec.Command(execPath, "--foreground=true", "--config="+ch.config.ConfigPath)
	err = cmd.Start()
	if err != nil {
		return types.Response{
			Success: false,
			Message: fmt.Sprintf("Failed to restart daemon: %v", err),
		}
	}
	return types.Response{
		Success: true,
		Message: "Daemon restarted successfully",
	}
}

func (ch *CommandHandler) Shutdown() {
	// Perform any necessary cleanup here
	log.Println("Shutting down daemon...")
	cmd, err := os.FindProcess(os.Getpid())
	if err != nil {
		log.Printf("Error finding process: %v", err)
		return
	}
	err = cmd.Signal(os.Interrupt)
	if err != nil {
		log.Printf("Error sending interrupt signal: %v", err)
		return
	}
	log.Println("Daemon shutdown complete.")
}

func (ch *CommandHandler) ValidateCommand(cmd types.Command) error {
	allowedCommands := []string{
		"setupCaddy", "stopdaemon", "restartDaemon", "ship",
	}

	for _, allowed := range allowedCommands {
		if cmd.Type == allowed {
			return nil
		}
	}

	return fmt.Errorf("command not allowed: %s", cmd.Type)
}

func (ch *CommandHandler) setUpCaddy(args map[string]interface{}) types.Response {
	setup, ok := args["setup"].(bool)
	if !ok || !setup {
		return types.Response{
			Success: false,
			Message: "setupCaddy requires 'setup' argument set to true",
		}
	}

	log.Println("Setting up Caddy server...")
	// read caddy file from ~/app/.nextdeploy/caddy/Caddyfile
	caddyfilePath := "~/app/.nextdeploy/caddy/Caddyfile"
	caddyfileContent, err := os.ReadFile(caddyfilePath)
	if err != nil {
		return types.Response{
			Success: false,
			Message: fmt.Sprintf("Failed to read Caddyfile: %v", err),
		}
	}
	// write Caddyfile to /etc/caddy/Caddyfile
	err = os.WriteFile("/etc/caddy/Caddyfile", caddyfileContent, 0644)
	if err != nil {
		return types.Response{
			Success: false,
			Message: fmt.Sprintf("Failed to write Caddyfile to /etc/caddy: %v", err),
		}
	}
	// restart caddy service
	var output []byte
	cmd := exec.Command("caddy", "reload", "--config", "/etc/caddy/Caddyfile")
	output, err = cmd.CombinedOutput()
	if err != nil {
		return types.Response{
			Success: false,
			Message: fmt.Sprintf("Failed to reload Caddy: %v - %s", err, string(output)),
		}
	}
	log.Println("Caddy server setup and reloaded successfully.")

	// start the caddy service if not running
	cmd = exec.Command("systemctl", "start", "caddy")
	output, err = cmd.CombinedOutput()
	if err != nil {
		return types.Response{
			Success: false,
			Message: fmt.Sprintf("Failed to start Caddy service: %v - %s", err, string(output)),
		}
	}
	return types.Response{
		Success: true,
		Message: "Caddy server setup and started successfully",
	}
}

func (ch *CommandHandler) handleShip(args map[string]interface{}) types.Response {
	tarballRaw, ok := args["tarball"]
	if !ok {
		return types.Response{Success: false, Message: "Missing 'tarball' argument"}
	}
	tarballPath := tarballRaw.(string)

	log.Printf("Starting shipment from tarball: %s", tarballPath)

	// 1. Unpack to temporary directory to read metadata early
	tmpUnpackDir := filepath.Join(os.TempDir(), fmt.Sprintf("nextdeploy-unpack-%d", time.Now().Unix()))
	os.MkdirAll(tmpUnpackDir, 0755)
	defer os.RemoveAll(tmpUnpackDir)

	if err := extractTarGz(tarballPath, tmpUnpackDir); err != nil {
		return types.Response{Success: false, Message: fmt.Sprintf("Failed to extract tarball: %v", err)}
	}

	metadataPath := filepath.Join(tmpUnpackDir, ".nextdeploy", "metadata.json")
	metadataBytes, err := os.ReadFile(metadataPath)
	if err != nil {
		return types.Response{Success: false, Message: fmt.Sprintf("Failed to read metadata.json: %v", err)}
	}

	var meta nextcore.NextCorePayload
	if err := json.Unmarshal(metadataBytes, &meta); err != nil {
		return types.Response{Success: false, Message: fmt.Sprintf("Failed to parse metadata.json: %v", err)}
	}

	appName := meta.AppName
	if appName == "" {
		appName = "default-app"
	}
	domain := meta.Domain
	if domain == "" {
		domain = "localhost" // fallback
	}
	// Fallback port since NextCorePayload does not expose port directly yet
	port := 3000

	// 2. Prepare Release Directory Strategy
	timestamp := time.Now().Unix()
	releaseDir := filepath.Join("/opt/nextdeploy/apps", appName, "releases", fmt.Sprintf("%d", timestamp))
	currentSymlink := filepath.Join("/opt/nextdeploy/apps", appName, "current")

	os.MkdirAll(filepath.Dir(releaseDir), 0755)

	// Move unpacked files to the new release directory
	cmd := exec.Command("mv", tmpUnpackDir, releaseDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		return types.Response{Success: false, Message: fmt.Sprintf("Failed to move app bundle to %s: %v %s", releaseDir, err, string(out))}
	}

	// 3. TODO: Yusuf - Dynamic Port Allocation
	// Instead of hardcoding 3000, dynamically find an open port here (e.g., 3001, 3002).
	// var port int = FindOpenPort()

	// 4. Update the Current Symlink
	os.Remove(currentSymlink)
	if err := os.Symlink(releaseDir, currentSymlink); err != nil {
		return types.Response{Success: false, Message: fmt.Sprintf("Failed to create current symlink: %v", err)}
	}

	// 5. Configure systemd process for the NEW release
	// Attempt to resolve doppler token from args or env
	dopplerTokenRaw, _ := args["dopplerToken"]
	dopplerToken, _ := dopplerTokenRaw.(string)

	if err := ch.processManager.GenerateServiceFile(appName, currentSymlink, string(meta.OutputMode), dopplerToken, port, meta.PackageManager); err != nil {
		return types.Response{Success: false, Message: fmt.Sprintf("Failed to generate service file: %v", err)}
	}

	// 6. TODO: Yusuf - Zero-Downtime Orchestration
	// Currently, this restarts the service forcefully, bringing down the old one.
	// To achieve true zero-downtime:
	// a) Start the NEW systemd service (e.g., nextdeploy-{appName}-{timestamp}.service) on the new dynamic port.
	// b) Run a Health Check (HTTP GET) against the new port to ensure it's alive.
	// c) If healthy, re-configure Caddy (below) to point to the new port and reload.
	// d) Finally, gracefully stop the OLD systemd service.

	if err := ch.processManager.StartService(appName); err != nil {
		return types.Response{Success: false, Message: fmt.Sprintf("Failed to start service: %v", err)}
	}

	// 7. Configure Caddy to point to the new symlink and release port
	if err := ch.caddyManager.GenerateConfig(appName, domain, string(meta.OutputMode), port, currentSymlink); err != nil {
		return types.Response{Success: false, Message: fmt.Sprintf("Failed to configure Caddy: %v", err)}
	}
	if err := ch.caddyManager.EnsureMainCaddyfile(); err != nil {
		return types.Response{Success: false, Message: fmt.Sprintf("Failed to setup main Caddyfile: %v", err)}
	}
	ch.caddyManager.Reload()

	// Clean up tarball
	os.Remove(tarballPath)

	return types.Response{
		Success: true,
		Message: fmt.Sprintf("App %s deployed successfully to %s", appName, domain),
	}
}

func extractTarGz(gzipStream, dest string) error {
	f, err := os.Open(gzipStream)
	if err != nil {
		return err
	}
	defer f.Close()

	uncompressedStream, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer uncompressedStream.Close()

	tarReader := tar.NewReader(uncompressedStream)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		target := filepath.Join(dest, header.Name)

		// prevent zipslip
		if !strings.HasPrefix(target, filepath.Clean(dest)+string(os.PathSeparator)) {
			continue
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			os.MkdirAll(filepath.Dir(target), 0755)
			outFile, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode))
			if err != nil {
				return err
			}
			if _, err := io.Copy(outFile, tarReader); err != nil {
				outFile.Close()
				return err
			}
			outFile.Close()
		}
	}
	return nil
}
