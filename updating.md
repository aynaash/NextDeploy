
Based on the release information you provided, I'll create a comprehensive update system that handles both CLI binaries (for all platforms) and daemon binaries (Linux only). Here's the implementation:

```go
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

type UpdateInfo struct {
	Updated    bool
	NewVersion string
	UpdateFrom string
	BinaryType string
}

type ReleaseAsset struct {
	Name        string `json:"name"`
	DownloadURL string `json:"browser_download_url"`
}

type GitHubRelease struct {
	TagName string          `json:"tag_name"`
	Assets  []ReleaseAsset  `json:"assets"`
}

// Constants
const (
	owner = "aynaash"
	repo  = "NextDeploy"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: update [cli|daemon]")
		os.Exit(1)
	}

	binaryType := os.Args[1]
	if binaryType != "cli" && binaryType != "daemon" {
		fmt.Println("Invalid argument. Use 'cli' or 'daemon'")
		os.Exit(1)
	}

	var info UpdateInfo
	var err error

	if binaryType == "cli" {
		info, err = UpdateCLI()
	} else {
		info, err = UpdateDaemon()
	}

	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	if info.Updated {
		fmt.Printf("Successfully updated %s from %s to %s\n", 
			info.BinaryType, info.UpdateFrom, info.NewVersion)
	} else {
		fmt.Printf("%s is already on the latest version: %s\n", 
			info.BinaryType, info.NewVersion)
	}
}

func UpdateCLI() (UpdateInfo, error) {
	currentVersion, err := getCurrentVersion("cli")
	if err != nil {
		return UpdateInfo{}, fmt.Errorf("failed to get current CLI version: %w", err)
	}

	release, err := getLatestRelease()
	if err != nil {
		return UpdateInfo{}, fmt.Errorf("failed to get latest release: %w", err)
	}

	if currentVersion == release.TagName {
		return UpdateInfo{
			Updated:    false,
			NewVersion: release.TagName,
			UpdateFrom: currentVersion,
			BinaryType: "CLI",
		}, nil
	}

	// Determine the appropriate binary for the current platform
	binaryPrefix := "nextdeploy"
	if runtime.GOOS == "windows" {
		binaryPrefix += ".exe"
	}

	arch := runtime.GOARCH
	if arch == "amd64" {
		arch = "amd64"
	} else if arch == "arm64" {
		arch = "arm64"
	} else {
		return UpdateInfo{}, fmt.Errorf("unsupported architecture: %s", arch)
	}

	binaryName := fmt.Sprintf("%s-%s-%s", binaryPrefix, runtime.GOOS, arch)
	if runtime.GOOS == "windows" {
		binaryName = fmt.Sprintf("%s-%s-%s.exe", "nextdeploy", runtime.GOOS, arch)
	}

	checksumName := binaryName + ".sha256"

	// Find the assets
	var binaryAsset, checksumAsset *ReleaseAsset
	for _, asset := range release.Assets {
		if asset.Name == binaryName {
			binaryAsset = &asset
		} else if asset.Name == checksumName {
			checksumAsset = &asset
		}
	}

	if binaryAsset == nil {
		return UpdateInfo{}, fmt.Errorf("binary not found for your platform: %s", binaryName)
	}

	if checksumAsset == nil {
		return UpdateInfo{}, fmt.Errorf("checksum not found for binary: %s", checksumName)
	}

	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "nextdeploy-update")
	if err != nil {
		return UpdateInfo{}, fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	binaryPath := filepath.Join(tmpDir, "nextdeploy")
	if runtime.GOOS == "windows" {
		binaryPath += ".exe"
	}

	// Download and verify binary
	if err := downloadAndVerifyBinary(binaryAsset.DownloadURL, checksumAsset.DownloadURL, binaryPath); err != nil {
		return UpdateInfo{}, fmt.Errorf("failed to download and verify binary: %w", err)
	}

	// Determine installation path
	installPath, err := getCLIInstallPath()
	if err != nil {
		return UpdateInfo{}, fmt.Errorf("failed to determine installation path: %w", err)
	}

	// Replace the binary
	if err := replaceBinary(binaryPath, installPath); err != nil {
		return UpdateInfo{}, fmt.Errorf("failed to replace binary: %w", err)
	}

	return UpdateInfo{
		Updated:    true,
		NewVersion: release.TagName,
		UpdateFrom: currentVersion,
		BinaryType: "CLI",
	}, nil
}

func UpdateDaemon() (UpdateInfo, error) {
	currentVersion, err := getCurrentVersion("daemon")
	if err != nil {
		return UpdateInfo{}, fmt.Errorf("failed to get current daemon version: %w", err)
	}

	release, err := getLatestRelease()
	if err != nil {
		return UpdateInfo{}, fmt.Errorf("failed to get latest release: %w", err)
	}

	if currentVersion == release.TagName {
		return UpdateInfo{
			Updated:    false,
			NewVersion: release.TagName,
			UpdateFrom: currentVersion,
			BinaryType: "Daemon",
		}, nil
	}

	// Determine the appropriate daemon binary for the current platform
	if runtime.GOOS != "linux" {
		return UpdateInfo{}, fmt.Errorf("daemon updates are only supported on Linux")
	}

	arch := runtime.GOARCH
	if arch != "amd64" && arch != "arm64" {
		return UpdateInfo{}, fmt.Errorf("unsupported architecture for daemon: %s", arch)
	}

	binaryName := fmt.Sprintf("nextdeployd-linux-%s", arch)
	checksumName := binaryName + ".sha256"

	// Find the assets
	var binaryAsset, checksumAsset *ReleaseAsset
	for _, asset := range release.Assets {
		if asset.Name == binaryName {
			binaryAsset = &asset
		} else if asset.Name == checksumName {
			checksumAsset = &asset
		}
	}

	if binaryAsset == nil {
		return UpdateInfo{}, fmt.Errorf("daemon binary not found for your architecture: %s", binaryName)
	}

	if checksumAsset == nil {
		return UpdateInfo{}, fmt.Errorf("checksum not found for daemon binary: %s", checksumName)
	}

	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "nextdeployd-update")
	if err != nil {
		return UpdateInfo{}, fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	binaryPath := filepath.Join(tmpDir, "nextdeployd")

	// Download and verify binary
	if err := downloadAndVerifyBinary(binaryAsset.DownloadURL, checksumAsset.DownloadURL, binaryPath); err != nil {
		return UpdateInfo{}, fmt.Errorf("failed to download and verify daemon binary: %w", err)
	}

	// Replace the daemon binary
	if err := replaceBinary(binaryPath, "/usr/local/bin/nextdeployd"); err != nil {
		return UpdateInfo{}, fmt.Errorf("failed to replace daemon binary: %w", err)
	}

	// Restart the daemon service
	if err := restartService("nextdeployd"); err != nil {
		return UpdateInfo{
			Updated:    true,
			NewVersion: release.TagName,
			UpdateFrom: currentVersion,
			BinaryType: "Daemon",
		}, fmt.Errorf("daemon updated but failed to restart service: %w", err)
	}

	return UpdateInfo{
		Updated:    true,
		NewVersion: release.TagName,
		UpdateFrom: currentVersion,
		BinaryType: "Daemon",
	}, nil
}

func getLatestRelease() (*GitHubRelease, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", owner, repo)
	
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	
	// GitHub API requires a User-Agent header
	req.Header.Set("User-Agent", "NextDeploy-Updater")
	
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned status: %s", resp.Status)
	}

	var release GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, err
	}

	return &release, nil
}

func getCurrentVersion(binaryType string) (string, error) {
	var cmd *exec.Cmd
	
	if binaryType == "cli" {
		cmd = exec.Command("nextdeploy", "--version")
	} else {
		cmd = exec.Command("nextdeployd", "--version")
	}
	
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	
	return strings.TrimSpace(string(output)), nil
}

func downloadAndVerifyBinary(binaryURL, checksumURL, savePath string) error {
	// Download the binary
	if err := downloadFile(binaryURL, savePath); err != nil {
		return fmt.Errorf("failed to download binary: %w", err)
	}

	// Download the checksum
	checksumData, err := downloadFileContent(checksumURL)
	if err != nil {
		return fmt.Errorf("failed to download checksum: %w", err)
	}

	// Verify the checksum
	if err := verifyChecksum(savePath, strings.TrimSpace(checksumData)); err != nil {
		return fmt.Errorf("checksum verification failed: %w", err)
	}

	// Set executable permissions
	if err := os.Chmod(savePath, 0755); err != nil {
		return fmt.Errorf("failed to set executable permissions: %w", err)
	}

	return nil
}

func downloadFile(url, path string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status: %s", resp.Status)
	}

	outFile, err := os.Create(path)
	if err != nil {
		return err
	}
	defer outFile.Close()

	_, err = io.Copy(outFile, resp.Body)
	return err
}

func downloadFileContent(url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download failed with status: %s", resp.Status)
	}

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(content), nil
}

func verifyChecksum(filePath, expectedChecksum string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return err
	}

	computedChecksum := hex.EncodeToString(hash.Sum(nil))
	
	if computedChecksum != expectedChecksum {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", 
			expectedChecksum, computedChecksum)
	}
	
	return nil
}

func getCLIInstallPath() (string, error) {
	// Try to find where the current binary is located
	path, err := exec.LookPath("nextdeploy")
	if err != nil {
		// If not found in PATH, use a default location
		if runtime.GOOS == "windows" {
			return filepath.Join(os.Getenv("ProgramFiles"), "NextDeploy", "nextdeploy.exe"), nil
		} else {
			return "/usr/local/bin/nextdeploy", nil
		}
	}
	return path, nil
}

func replaceBinary(source, destination string) error {
	// On Windows, we need to remove the destination first
	if runtime.GOOS == "windows" {
		if err := os.Remove(destination); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	
	return os.Rename(source, destination)
}

func restartService(serviceName string) error {
	// Check if systemctl is available (Linux with systemd)
	if _, err := exec.LookPath("systemctl"); err == nil {
		cmd := exec.Command("sudo", "systemctl", "restart", serviceName)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}
	
	// TODO: Add support for other init systems if needed
	return fmt.Errorf("no supported service manager found")
}
```

## Usage

This update system provides two commands:

1. **Update CLI**: `update cli`
   - Downloads the appropriate CLI binary for your OS/architecture
   - Verifies the checksum
   - Replaces the existing binary

2. **Update Daemon**: `update daemon`
   - Downloads the appropriate daemon binary for your Linux architecture
   - Verifies the checksum
   - Replaces the existing binary
   - Restarts the daemon service

## Key Features

1. **Multi-platform Support**: Handles CLI updates for Windows, macOS, and Linux
2. **Checksum Verification**: Ensures downloaded binaries are authentic and not corrupted
3. **Version Checking**: Only updates if a newer version is available
4. **Proper Error Handling**: Provides clear error messages for troubleshooting
5. **Service Management**: Automatically restarts the daemon after update

## Building and Using

To use this update system, you'll need to:

1. Build it for your platform: `go build -o update`
2. Run it with the appropriate argument:
   - `./update cli` to update the CLI
   - `./update daemon` to update the daemon (Linux only)

This implementation provides a robust, secure update mechanism that handles both CLI and daemon updates with proper checksum verification and error handling.
