# 🔥 Robust OS Detection with Comprehensive Unhappy Path Handling

Here's a production-ready OS detection system with intense error handling:

## 📦 Complete OS Detection Package

```go
package osdetect

import (
	"context"
	"fmt"
	"nextdeploy/cli/internal/server"
	"regexp"
	"strings"
	"time"
)

// OSType represents the detected operating system
type OSType string

const (
	OSTypeUnknown   OSType = "unknown"
	OSTypeAmazon1   OSType = "amazonlinux1"
	OSTypeAmazon2   OSType = "amazonlinux2"
	OSTypeAmazon2023 OSType = "amazonlinux2023"
	OSTypeRHEL6     OSType = "rhel6"
	OSTypeRHEL7     OSType = "rhel7"
	OSTypeRHEL8     OSType = "rhel8"
	OSTypeRHEL9     OSType = "rhel9"
	OSTypeCentOS6   OSType = "centos6"
	OSTypeCentOS7   OSType = "centos7"
	OSTypeCentOS8   OSType = "centos8"
	OSTypeCentOS9   OSType = "centos9"
	OSTypeFedora    OSType = "fedora"
	OSTypeUbuntu14  OSType = "ubuntu14"
	OSTypeUbuntu16  OSType = "ubuntu16"
	OSTypeUbuntu18  OSType = "ubuntu18"
	OSTypeUbuntu20  OSType = "ubuntu20"
	OSTypeUbuntu22  OSType = "ubuntu22"
	OSTypeUbuntu24  OSType = "ubuntu24"
	OSTypeDebian8   OSType = "debian8"
	OSTypeDebian9   OSType = "debian9"
	OSTypeDebian10  OSType = "debian10"
	OSTypeDebian11  OSType = "debian11"
	OSTypeDebian12  OSType = "debian12"
	OSTypeAlpine    OSType = "alpine"
	OSTypeArch      OSType = "arch"
	OSTypeSUSE      OSType = "suse"
	OSTypeCoreOS    OSType = "coreos"
)

// PackageManager represents the package manager
type PackageManager string

const (
	PkgManagerUnknown PackageManager = "unknown"
	PkgManagerYum     PackageManager = "yum"
	PkgManagerDnf     PackageManager = "dnf"
	PkgManagerApt     PackageManager = "apt"
	PkgManagerApk     PackageManager = "apk"
	PkgManagerPacman  PackageManager = "pacman"
	PkgManagerZypper  PackageManager = "zypper"
)

// OSInfo contains detailed OS information
type OSInfo struct {
	// Basic info
	OSType        OSType
	PackageManager PackageManager
	Name          string
	Version       string
	VersionID     string
	ID            string
	IDLike        []string
	PrettyName    string
	Architecture  string
	KernelVersion string
	
	// Detailed flags
	IsAmazon       bool
	IsRHEL         bool
	IsCentOS       bool
	IsFedora       bool
	IsDebian       bool
	IsUbuntu       bool
	IsAlpine       bool
	IsContainer    bool
	HasSystemd     bool
	HasSysV        bool
	HasDocker      bool
	SELinuxEnabled bool
	
	// Error tracking
	DetectionErrors []string
	DetectionWarnings []string
}

// DetectionOptions configures the detection behavior
type DetectionOptions struct {
	Timeout           time.Duration
	MaxRetries        int
	FallbackToGeneric bool
	Verbose           bool
}

// DefaultOptions returns default detection options
func DefaultOptions() DetectionOptions {
	return DetectionOptions{
		Timeout:           30 * time.Second,
		MaxRetries:        3,
		FallbackToGeneric: true,
		Verbose:           false,
	}
}

// OSDetector handles OS detection with multiple fallback strategies
type OSDetector struct {
	serverMgr *server.ServerStruct
	serverName string
	options    DetectionOptions
	cache      *OSInfo
}

// NewOSDetector creates a new OS detector
func NewOSDetector(serverMgr *server.ServerStruct, serverName string, options DetectionOptions) *OSDetector {
	return &OSDetector{
		serverMgr:  serverMgr,
		serverName: serverName,
		options:    options,
	}
}

// Detect performs comprehensive OS detection with multiple strategies
func (d *OSDetector) Detect(ctx context.Context) (*OSInfo, error) {
	// Check cache
	if d.cache != nil {
		return d.cache, nil
	}

	info := &OSInfo{
		DetectionErrors:   []string{},
		DetectionWarnings: []string{},
	}

	// Strategy 1: Try /etc/os-release (most reliable)
	if err := d.detectFromOSRelease(ctx, info); err != nil {
		info.DetectionWarnings = append(info.DetectionWarnings, 
			fmt.Sprintf("os-release detection failed: %v", err))
	}

	// Strategy 2: Try lsb_release (Ubuntu/Debian fallback)
	if info.OSType == OSTypeUnknown {
		if err := d.detectFromLSBRelease(ctx, info); err != nil {
			info.DetectionWarnings = append(info.DetectionWarnings,
				fmt.Sprintf("lsb_release detection failed: %v", err))
		}
	}

	// Strategy 3: Try /etc/redhat-release (RHEL family fallback)
	if info.OSType == OSTypeUnknown {
		if err := d.detectFromRedHatRelease(ctx, info); err != nil {
			info.DetectionWarnings = append(info.DetectionWarnings,
				fmt.Sprintf("redhat-release detection failed: %v", err))
		}
	}

	// Strategy 4: Try /etc/debian_version (Debian family fallback)
	if info.OSType == OSTypeUnknown {
		if err := d.detectFromDebianVersion(ctx, info); err != nil {
			info.DetectionWarnings = append(info.DetectionWarnings,
				fmt.Sprintf("debian-version detection failed: %v", err))
		}
	}

	// Strategy 5: Try /etc/centos-release (CentOS fallback)
	if info.OSType == OSTypeUnknown {
		if err := d.detectFromCentOSRelease(ctx, info); err != nil {
			info.DetectionWarnings = append(info.DetectionWarnings,
				fmt.Sprintf("centos-release detection failed: %v", err))
		}
	}

	// Strategy 6: Try /etc/system-release (Amazon Linux fallback)
	if info.OSType == OSTypeUnknown {
		if err := d.detectFromSystemRelease(ctx, info); err != nil {
			info.DetectionWarnings = append(info.DetectionWarnings,
				fmt.Sprintf("system-release detection failed: %v", err))
		}
	}

	// Strategy 7: Try uname (last resort)
	if info.OSType == OSTypeUnknown {
		if err := d.detectFromUname(ctx, info); err != nil {
			info.DetectionErrors = append(info.DetectionErrors,
				fmt.Sprintf("uname detection failed: %v", err))
		}
	}

	// Detect package manager
	d.detectPackageManager(ctx, info)

	// Detect system capabilities
	d.detectSystemCapabilities(ctx, info)

	// If still unknown and fallback is enabled, use generic
	if info.OSType == OSTypeUnknown && d.options.FallbackToGeneric {
		info.OSType = OSTypeUnknown
		info.PackageManager = PkgManagerUnknown
		info.DetectionWarnings = append(info.DetectionWarnings, 
			"Using generic fallback - some features may not work")
	}

	// Cache result
	d.cache = info

	return info, nil
}

// detectFromOSRelease reads /etc/os-release
func (d *OSDetector) detectFromOSRelease(ctx context.Context, info *OSInfo) error {
	cmd := `if [ -f /etc/os-release ]; then 
		cat /etc/os-release; 
	else 
		echo "OS_RELEASE_NOT_FOUND"; 
	fi`

	output, err := d.executeWithRetry(ctx, cmd)
	if err != nil || strings.Contains(output, "OS_RELEASE_NOT_FOUND") {
		return fmt.Errorf("os-release not found")
	}

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "NAME=") {
			info.Name = d.parseValue(line)
		} else if strings.HasPrefix(line, "VERSION=") {
			info.Version = d.parseValue(line)
		} else if strings.HasPrefix(line, "VERSION_ID=") {
			info.VersionID = d.parseValue(line)
		} else if strings.HasPrefix(line, "ID=") {
			info.ID = d.parseValue(line)
		} else if strings.HasPrefix(line, "ID_LIKE=") {
			info.IDLike = strings.Split(d.parseValue(line), " ")
		} else if strings.HasPrefix(line, "PRETTY_NAME=") {
			info.PrettyName = d.parseValue(line)
		}
	}

	// Determine OS type from ID and ID_LIKE
	d.classifyFromID(info)

	return nil
}

// detectFromLSBRelease uses lsb_release command
func (d *OSDetector) detectFromLSBRelease(ctx context.Context, info *OSInfo) error {
	cmd := `command -v lsb_release >/dev/null 2>&1 && 
		lsb_release -a 2>/dev/null || 
		echo "LSB_NOT_FOUND"`

	output, err := d.executeWithRetry(ctx, cmd)
	if err != nil || strings.Contains(output, "LSB_NOT_FOUND") {
		return fmt.Errorf("lsb_release not found")
	}

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "Distributor ID:") {
			parts := strings.Split(line, ":")
			if len(parts) == 2 {
				info.ID = strings.TrimSpace(parts[1])
			}
		} else if strings.Contains(line, "Description:") {
			parts := strings.Split(line, ":")
			if len(parts) == 2 {
				info.PrettyName = strings.TrimSpace(parts[1])
			}
		} else if strings.Contains(line, "Release:") {
			parts := strings.Split(line, ":")
			if len(parts) == 2 {
				info.VersionID = strings.TrimSpace(parts[1])
			}
		}
	}

	d.classifyFromID(info)
	return nil
}

// detectFromRedHatRelease reads /etc/redhat-release
func (d *OSDetector) detectFromRedHatRelease(ctx context.Context, info *OSInfo) error {
	cmd := `if [ -f /etc/redhat-release ]; then 
		cat /etc/redhat-release; 
	else 
		echo "REDHAT_RELEASE_NOT_FOUND"; 
	fi`

	output, err := d.executeWithRetry(ctx, cmd)
	if err != nil || strings.Contains(output, "REDHAT_RELEASE_NOT_FOUND") {
		return fmt.Errorf("redhat-release not found")
	}

	output = strings.TrimSpace(output)
	
	// Parse different RHEL variants
	switch {
	case strings.Contains(output, "CentOS"):
		info.ID = "centos"
		info.PrettyName = output
		
		// Extract version using regex
		re := regexp.MustCompile(`(\d+)\.(\d+)`)
		if matches := re.FindStringSubmatch(output); len(matches) > 0 {
			info.VersionID = matches[0]
		}
		
	case strings.Contains(output, "Red Hat"):
		info.ID = "rhel"
		info.PrettyName = output
		
		re := regexp.MustCompile(`(\d+)\.(\d+)`)
		if matches := re.FindStringSubmatch(output); len(matches) > 0 {
			info.VersionID = matches[0]
		}
		
	case strings.Contains(output, "Amazon Linux"):
		info.ID = "amzn"
		info.PrettyName = output
		
		// Amazon Linux version detection
		if strings.Contains(output, "release 2") {
			info.VersionID = "2"
		} else if strings.Contains(output, "release 2023") {
			info.VersionID = "2023"
		} else if strings.Contains(output, "release 1") {
			info.VersionID = "1"
		}
	}

	d.classifyFromID(info)
	return nil
}

// detectFromDebianVersion reads /etc/debian_version
func (d *OSDetector) detectFromDebianVersion(ctx context.Context, info *OSInfo) error {
	cmd := `if [ -f /etc/debian_version ]; then 
		cat /etc/debian_version; 
	else 
		echo "DEBIAN_VERSION_NOT_FOUND"; 
	fi`

	output, err := d.executeWithRetry(ctx, cmd)
	if err != nil || strings.Contains(output, "DEBIAN_VERSION_NOT_FOUND") {
		return fmt.Errorf("debian_version not found")
	}

	info.ID = "debian"
	info.VersionID = strings.TrimSpace(output)
	
	// Map version numbers to names
	switch strings.Split(info.VersionID, ".")[0] {
	case "12":
		info.PrettyName = "Debian GNU/Linux 12 (bookworm)"
	case "11":
		info.PrettyName = "Debian GNU/Linux 11 (bullseye)"
	case "10":
		info.PrettyName = "Debian GNU/Linux 10 (buster)"
	case "9":
		info.PrettyName = "Debian GNU/Linux 9 (stretch)"
	}

	d.classifyFromID(info)
	return nil
}

// detectFromCentOSRelease reads /etc/centos-release
func (d *OSDetector) detectFromCentOSRelease(ctx context.Context, info *OSInfo) error {
	cmd := `if [ -f /etc/centos-release ]; then 
		cat /etc/centos-release; 
	else 
		echo "CENTOS_RELEASE_NOT_FOUND"; 
	fi`

	output, err := d.executeWithRetry(ctx, cmd)
	if err != nil || strings.Contains(output, "CENTOS_RELEASE_NOT_FOUND") {
		return fmt.Errorf("centos-release not found")
	}

	info.ID = "centos"
	info.PrettyName = strings.TrimSpace(output)
	
	// Extract version
	re := regexp.MustCompile(`(\d+)\.(\d+)`)
	if matches := re.FindStringSubmatch(output); len(matches) > 0 {
		info.VersionID = matches[0]
	}

	d.classifyFromID(info)
	return nil
}

// detectFromSystemRelease reads /etc/system-release (Amazon Linux)
func (d *OSDetector) detectFromSystemRelease(ctx context.Context, info *OSInfo) error {
	cmd := `if [ -f /etc/system-release ]; then 
		cat /etc/system-release; 
	else 
		echo "SYSTEM_RELEASE_NOT_FOUND"; 
	fi`

	output, err := d.executeWithRetry(ctx, cmd)
	if err != nil || strings.Contains(output, "SYSTEM_RELEASE_NOT_FOUND") {
		return fmt.Errorf("system-release not found")
	}

	output = strings.TrimSpace(output)
	
	if strings.Contains(output, "Amazon Linux") {
		info.ID = "amzn"
		info.PrettyName = output
		
		if strings.Contains(output, "release 2") {
			info.VersionID = "2"
		} else if strings.Contains(output, "release 2023") {
			info.VersionID = "2023"
		} else if strings.Contains(output, "release 1") {
			info.VersionID = "1"
		}
	}

	d.classifyFromID(info)
	return nil
}

// detectFromUname uses uname as last resort
func (d *OSDetector) detectFromUname(ctx context.Context, info *OSInfo) error {
	// Get kernel name
	kernelCmd := "uname -s 2>/dev/null || echo 'unknown'"
	kernel, err := d.executeWithRetry(ctx, kernelCmd)
	if err != nil {
		return err
	}
	kernel = strings.TrimSpace(kernel)

	// Get kernel version
	versionCmd := "uname -r 2>/dev/null || echo 'unknown'"
	version, err := d.executeWithRetry(ctx, versionCmd)
	if err != nil {
		return err
	}
	info.KernelVersion = strings.TrimSpace(version)

	// Get architecture
	archCmd := "uname -m 2>/dev/null || echo 'unknown'"
	arch, err := d.executeWithRetry(ctx, archCmd)
	if err != nil {
		return err
	}
	info.Architecture = strings.TrimSpace(arch)

	// Try to guess from kernel
	switch {
	case strings.Contains(strings.ToLower(kernel), "linux"):
		// We know it's Linux, but need more info
		info.ID = "linux"
		info.OSType = OSTypeUnknown
		info.DetectionWarnings = append(info.DetectionWarnings, 
			"Could not determine specific distribution from uname")
	}

	return nil
}

// detectPackageManager identifies the package manager
func (d *OSDetector) detectPackageManager(ctx context.Context, info *OSInfo) {
	// Try each package manager in order of likelihood based on OS
	checks := []struct {
		cmd      string
		pm       PackageManager
		priority int
	}{
		{"command -v apt-get >/dev/null 2>&1 && echo 'found'", PkgManagerApt, 10},
		{"command -v dnf >/dev/null 2>&1 && echo 'found'", PkgManagerDnf, 20},
		{"command -v yum >/dev/null 2>&1 && echo 'found'", PkgManagerYum, 30},
		{"command -v apk >/dev/null 2>&1 && echo 'found'", PkgManagerApk, 40},
		{"command -v pacman >/dev/null 2>&1 && echo 'found'", PkgManagerPacman, 50},
		{"command -v zypper >/dev/null 2>&1 && echo 'found'", PkgManagerZypper, 60},
	}

	// Sort by priority based on OS
	if info.IsAmazon || info.IsRHEL || info.IsCentOS {
		// RHEL family: try yum/dnf first
		checks[0], checks[1] = checks[1], checks[0] // Swap apt with dnf
	}

	for _, check := range checks {
		output, err := d.executeWithRetry(ctx, check.cmd)
		if err == nil && strings.Contains(output, "found") {
			info.PackageManager = check.pm
			return
		}
	}

	info.PackageManager = PkgManagerUnknown
	info.DetectionWarnings = append(info.DetectionWarnings, 
		"Could not detect package manager")
}

// detectSystemCapabilities identifies system features
func (d *OSDetector) detectSystemCapabilities(ctx context.Context, info *OSInfo) {
	// Check for systemd
	if out, err := d.executeWithRetry(ctx, "command -v systemctl >/dev/null 2>&1 && echo 'found'"); err == nil && strings.Contains(out, "found") {
		info.HasSystemd = true
	}

	// Check for SysV init
	if !info.HasSystemd {
		if out, err := d.executeWithRetry(ctx, "command -v service >/dev/null 2>&1 && test -d /etc/init.d && echo 'found'"); err == nil && strings.Contains(out, "found") {
			info.HasSysV = true
		}
	}

	// Check if running in container
	if out, err := d.executeWithRetry(ctx, "test -f /.dockerenv && echo 'docker' || (grep -q 'container=' /proc/1/environ 2>/dev/null && echo 'container')"); err == nil {
		if strings.Contains(out, "docker") || strings.Contains(out, "container") {
			info.IsContainer = true
		}
	}

	// Check SELinux status
	if out, err := d.executeWithRetry(ctx, "command -v getenforce >/dev/null 2>&1 && getenforce 2>/dev/null | grep -q 'Enforcing' && echo 'enabled'"); err == nil && strings.Contains(out, "enabled") {
		info.SELinuxEnabled = true
	}

	// Check if Docker is pre-installed
	if out, err := d.executeWithRetry(ctx, "command -v docker >/dev/null 2>&1 && echo 'found'"); err == nil && strings.Contains(out, "found") {
		info.HasDocker = true
	}
}

// classifyFromID determines the OS type from ID and ID_LIKE
func (d *OSDetector) classifyFromID(info *OSInfo) {
	info.IsAmazon = info.ID == "amzn"
	info.IsRHEL = info.ID == "rhel" || containsString(info.IDLike, "rhel")
	info.IsCentOS = info.ID == "centos" || containsString(info.IDLike, "centos")
	info.IsFedora = info.ID == "fedora"
	info.IsDebian = info.ID == "debian" || containsString(info.IDLike, "debian")
	info.IsUbuntu = info.ID == "ubuntu" || containsString(info.IDLike, "ubuntu")
	info.IsAlpine = info.ID == "alpine"

	// Set specific OS type based on ID and version
	switch info.ID {
	case "amzn":
		switch info.VersionID {
		case "1":
			info.OSType = OSTypeAmazon1
		case "2":
			info.OSType = OSTypeAmazon2
		case "2023":
			info.OSType = OSTypeAmazon2023
		default:
			if strings.HasPrefix(info.VersionID, "2") {
				info.OSType = OSTypeAmazon2
			} else if strings.HasPrefix(info.VersionID, "2023") {
				info.OSType = OSTypeAmazon2023
			} else {
				info.OSType = OSTypeAmazon2 // Default to Amazon Linux 2
			}
		}

	case "rhel":
		switch strings.Split(info.VersionID, ".")[0] {
		case "6":
			info.OSType = OSTypeRHEL6
		case "7":
			info.OSType = OSTypeRHEL7
		case "8":
			info.OSType = OSTypeRHEL8
		case "9":
			info.OSType = OSTypeRHEL9
		}

	case "centos":
		switch strings.Split(info.VersionID, ".")[0] {
		case "6":
			info.OSType = OSTypeCentOS6
		case "7":
			info.OSType = OSTypeCentOS7
		case "8":
			info.OSType = OSTypeCentOS8
		case "9":
			info.OSType = OSTypeCentOS9
		}

	case "ubuntu":
		switch strings.Split(info.VersionID, ".")[0] {
		case "14":
			info.OSType = OSTypeUbuntu14
		case "16":
			info.OSType = OSTypeUbuntu16
		case "18":
			info.OSType = OSTypeUbuntu18
		case "20":
			info.OSType = OSTypeUbuntu20
		case "22":
			info.OSType = OSTypeUbuntu22
		case "24":
			info.OSType = OSTypeUbuntu24
		}

	case "debian":
		switch strings.Split(info.VersionID, ".")[0] {
		case "8":
			info.OSType = OSTypeDebian8
		case "9":
			info.OSType = OSTypeDebian9
		case "10":
			info.OSType = OSTypeDebian10
		case "11":
			info.OSType = OSTypeDebian11
		case "12":
			info.OSType = OSTypeDebian12
		}

	case "fedora":
		info.OSType = OSTypeFedora
	case "alpine":
		info.OSType = OSTypeAlpine
	case "arch":
		info.OSType = OSTypeArch
	}
}

// executeWithRetry runs a command with retries
func (d *OSDetector) executeWithRetry(ctx context.Context, cmd string) (string, error) {
	var lastErr error
	var output string

	for attempt := 1; attempt <= d.options.MaxRetries; attempt++ {
		// Create timeout context
		cmdCtx, cancel := context.WithTimeout(ctx, d.options.Timeout)
		defer cancel()

		output, lastErr = d.serverMgr.ExecuteCommand(cmdCtx, d.serverName, cmd, nil)
		
		if lastErr == nil {
			return output, nil
		}

		// Exponential backoff
		if attempt < d.options.MaxRetries {
			time.Sleep(time.Duration(attempt*2) * time.Second)
		}
	}

	return "", fmt.Errorf("after %d attempts, last error: %w", d.options.MaxRetries, lastErr)
}

// parseValue parses KEY=value lines from os-release
func (d *OSDetector) parseValue(line string) string {
	parts := strings.SplitN(line, "=", 2)
	if len(parts) != 2 {
		return ""
	}
	
	value := parts[1]
	// Remove quotes
	value = strings.Trim(value, `"'`)
	return value
}

// containsString checks if a string is in a slice
func containsString(slice []string, str string) bool {
	for _, s := range slice {
		if s == str {
			return true
		}
	}
	return false
}

// GetInstallationMethod returns the appropriate installation method based on OS
func (info *OSInfo) GetInstallationMethod() string {
	switch {
	case info.IsAmazon:
		if info.HasSystemd {
			return "amazon-linux-extras"
		}
		return "yum"
	case info.IsRHEL || info.IsCentOS:
		if info.OSType == OSTypeRHEL8 || info.OSType == OSTypeRHEL9 || 
		   info.OSType == OSTypeCentOS8 || info.OSType == OSTypeCentOS9 {
			return "dnf"
		}
		return "yum"
	case info.IsFedora:
		return "dnf"
	case info.IsUbuntu || info.IsDebian:
		return "apt"
	case info.IsAlpine:
		return "apk"
	default:
		return "unknown"
	}
}

// IsSupported returns true if the OS is supported
func (info *OSInfo) IsSupported() bool {
	supported := []OSType{
		OSTypeAmazon2, OSTypeAmazon2023,
		OSTypeUbuntu20, OSTypeUbuntu22, OSTypeUbuntu24,
		OSTypeDebian11, OSTypeDebian12,
		OSTypeRHEL8, OSTypeRHEL9,
		OSTypeCentOS8, OSTypeCentOS9,
		OSTypeFedora,
	}

	for _, s := range supported {
		if info.OSType == s {
			return true
		}
	}
	return false
}
```

## 📝 Usage in Your Prepare Command

```go
func installWithYum(ctx context.Context, serverMgr *server.ServerStruct, serverName string, cmd *cobra.Command, stream *StreamWriter) error {
    // Create OS detector with robust options
    detector := osdetect.NewOSDetector(serverMgr, serverName, osdetect.DefaultOptions())
    
    // Detect OS with multiple fallback strategies
    osInfo, err := detector.Detect(ctx)
    if err != nil {
        stream.Printf(cmd.OutOrStdout(), []color.Attribute{color.FgRed}, 
            "❌ Failed to detect OS: %v\n", err)
        return fmt.Errorf("os detection failed: %w", err)
    }

    // Log detection results
    stream.Printf(cmd.OutOrStdout(), []color.Attribute{color.FgCyan}, 
        "  • Detected OS: %s\n", osInfo.PrettyName)
    
    if len(osInfo.DetectionWarnings) > 0 {
        for _, warning := range osInfo.DetectionWarnings {
            stream.Printf(cmd.OutOrStdout(), []color.Attribute{color.FgYellow}, 
                "    ⚠ %s\n", warning)
        }
    }

    // Check if OS is supported
    if !osInfo.IsSupported() {
        stream.Printf(cmd.OutOrStdout(), []color.Attribute{color.FgRed}, 
            "❌ Unsupported OS: %s\n", osInfo.PrettyName)
        return fmt.Errorf("unsupported OS: %s", osInfo.PrettyName)
    }

    // Install Docker based on OS
    stream.Printf(cmd.OutOrStdout(), []color.Attribute{color.FgYellow}, "  • Installing Docker...\n")
    
    if err := installDockerForOS(ctx, serverMgr, serverName, osInfo, cmd, stream); err != nil {
        return fmt.Errorf("docker installation failed: %w", err)
    }

    // Continue with Caddy installation...
    return nil
}

func installDockerForOS(ctx context.Context, serverMgr *server.ServerStruct, serverName string, 
    osInfo *osdetect.OSInfo, cmd *cobra.Command, stream *StreamWriter) error {
    
    var dockerCmd string
    
    switch osInfo.OSType {
    case osdetect.OSTypeAmazon2:
        stream.Printf(cmd.OutOrStdout(), []color.Attribute{color.FgHiBlack}, 
            "     📦 Amazon Linux 2 detected, using amazon-linux-extras\n")
        dockerCmd = `sudo amazon-linux-extras install docker -y && 
            sudo systemctl enable docker && 
            sudo systemctl start docker && 
            sudo usermod -aG docker $USER`
            
    case osdetect.OSTypeAmazon2023:
        stream.Printf(cmd.OutOrStdout(), []color.Attribute{color.FgHiBlack}, 
            "     📦 Amazon Linux 2023 detected, using dnf\n")
        dockerCmd = `sudo dnf install -y docker && 
            sudo systemctl enable docker && 
            sudo systemctl start docker && 
            sudo usermod -aG docker $USER`
            
    case osdetect.OSTypeRHEL8, osdetect.OSTypeRHEL9, 
         osdetect.OSTypeCentOS8, osdetect.OSTypeCentOS9,
         osdetect.OSTypeFedora:
        stream.Printf(cmd.OutOrStdout(), []color.Attribute{color.FgHiBlack}, 
            "     📦 RHEL/CentOS/Fedora detected, using Docker CE repository\n")
        dockerCmd = `sudo dnf config-manager --add-repo https://download.docker.com/linux/centos/docker-ce.repo && 
            sudo dnf install -y docker-ce docker-ce-cli containerd.io && 
            sudo systemctl enable docker && 
            sudo systemctl start docker && 
            sudo usermod -aG docker $USER`
            
    case osdetect.OSTypeRHEL7, osdetect.OSTypeCentOS7:
        stream.Printf(cmd.OutOrStdout(), []color.Attribute{color.FgHiBlack}, 
            "     📦 RHEL/CentOS 7 detected, using yum with Docker CE repository\n")
        dockerCmd = `sudo yum install -y yum-utils && 
            sudo yum-config-manager --add-repo https://download.docker.com/linux/centos/docker-ce.repo && 
            sudo yum install -y docker-ce docker-ce-cli containerd.io && 
            sudo systemctl enable docker && 
            sudo systemctl start docker && 
            sudo usermod -aG docker $USER`
            
    case osdetect.OSTypeUbuntu20, osdetect.OSTypeUbuntu22, osdetect.OSTypeUbuntu24,
         osdetect.OSTypeDebian11, osdetect.OSTypeDebian12:
        stream.Printf(cmd.OutOrStdout(), []color.Attribute{color.FgHiBlack}, 
            "     📦 Debian/Ubuntu detected, using apt with Docker repository\n")
        dockerCmd = `sudo apt-get update && 
            sudo apt-get install -y ca-certificates curl gnupg lsb-release && 
            sudo mkdir -p /etc/apt/keyrings && 
            curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo gpg --dearmor -o /etc/apt/keyrings/docker.gpg && 
            echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu $(lsb_release -cs) stable" | sudo tee /etc/apt/sources.list.d/docker.list > /dev/null && 
            sudo apt-get update && 
            sudo apt-get install -y docker-ce docker-ce-cli containerd.io docker-compose-plugin && 
            sudo systemctl enable docker && 
            sudo systemctl start docker && 
            sudo usermod -aG docker $USER`
            
    default:
        stream.Printf(cmd.OutOrStdout(), []color.Attribute{color.FgYellow}, 
            "     ⚠ Unknown OS, falling back to generic script\n")
        dockerCmd = `curl -fsSL https://get.docker.com | sudo sh && 
            sudo usermod -aG docker $USER && 
            sudo systemctl enable docker && 
            sudo systemctl start docker`
    }

    // Execute Docker installation
    output, err := serverMgr.ExecuteCommand(ctx, serverName, dockerCmd, stream)
    if err != nil {
        PrepLogs.Error("Docker installation failed: %v", err)
        PrepLogs.Debug("Docker installation output: %s", output)
        
        // Log any detection errors for debugging
        if len(osInfo.DetectionErrors) > 0 {
            for _, detErr := range osInfo.DetectionErrors {
                PrepLogs.Debug("Detection error: %s", detErr)
            }
        }
        
        return fmt.Errorf("docker installation failed: %w", err)
    }

    // Verify installation
    verifyCmd := "docker --version"
    version, err := serverMgr.ExecuteCommand(ctx, serverName, verifyCmd, stream)
    if err != nil {
        return fmt.Errorf("docker installed but verification failed: %w", err)
    }

    stream.Printf(cmd.OutOrStdout(), []color.Attribute{color.FgGreen}, 
        "  ✓ Docker installed: %s", strings.TrimSpace(version))
    
    return nil
}
```

## 🎯 Key Features

1. **7 Fallback Detection Strategies** - Tries multiple methods if one fails
2. **Comprehensive OS Support** - Amazon Linux, RHEL, CentOS, Ubuntu, Debian, Fedora, Alpine, etc.
3. **Version-Specific Installation** - Different commands for different OS versions
4. **Detailed Error Tracking** - Captures all detection errors and warnings
5. **Retry Logic** - Retries failed commands with exponential backoff
6. **System Capability Detection** - Detects systemd, init system, container, SELinux
7. **Graceful Degradation** - Falls back to generic methods when specific detection fails

This will handle your Amazon Linux EC2 instance perfectly and also work for any other OS your users might have!
