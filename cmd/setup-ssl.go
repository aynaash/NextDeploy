package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
	"nextdeploy/internal/config"
	"nextdeploy/internal/logger"
	"nextdeploy/internal/server"
	"os"
	"strings"
	"time"
)

const (
	sslMetadataDir = "/var/lib/nextdeploy/ssl_metadata"
	caddyConfigDir = "/etc/caddy"
)

type SSLConfig struct {
	Domain      string `yaml:"domain"`
	Email       string `yaml:"email"`
	Staging     bool   `yaml:"staging"`
	Wildcard    bool   `yaml:"wildcard"`
	DNSProvider string `yaml:"dns_provider"`
	Force       bool   `yaml:"force"`
}

var (
	sslConfigFile string
	sslConfig     SSLConfig
)

var SSLCommand = &cobra.Command{
	Use:   "ssl",
	Short: "Automate SSL setup and Caddy configuration",
	Long: `Configure SSL certificates and Caddy for domains with:
- Automatic Let's Encrypt certificates
- DNS-01 challenge support for wildcards
- Staging environment for testing
- Configuration rollback on failure`,
	RunE: runSSLCommand,
}
var (
	sslogs = logger.PackageLogger("ssl", "SSL")
)

func init() {
	rootCmd.AddCommand(SSLCommand)
}

func runSSLCommand(cmd *cobra.Command, args []string) error {
	// Load configuration from YAML file

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}
	sslogs.Debug("Loaded configuration", "config", cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	// Validate inputs
	if err := validateInputs(); err != nil {
		return err
	}

	// Initialize server manager
	serverMgr, err := server.New(
		server.WithConfig(),
		server.WithSSH(),
	)
	if err != nil {
		return fmt.Errorf("failed to initialize server manager: %w", err)
	}
	defer serverMgr.CloseSSHConnections()

	// Get deployment server
	serverName, err := serverMgr.GetDeploymentServer()
	if err != nil {
		return fmt.Errorf("failed to get deployment server: %w", err)
	}

	// Check if certificate already exists
	if !sslConfig.Force {
		if exists, err := checkCertificateExists(ctx, serverMgr, serverName); err != nil {
			return fmt.Errorf("certificate check failed: %w", err)
		} else if exists {
			color.Green("✓ SSL certificate already configured for %s", sslConfig.Domain)
			return nil
		}
	}

	// Setup steps with rollback capability
	steps := []struct {
		name string
		fn   func(context.Context, *server.ServerStruct, string) error
		roll func(context.Context, *server.ServerStruct, string) error
	}{
		{
			"Create working directory",
			createWorkingDir,
			removeWorkingDir,
		},
		{
			"Configure Caddyfile",
			configureCaddyfile,
			rollbackCaddyfile,
		},
		{
			"Reload Caddy",
			reloadCaddy,
			nil, // No rollback needed - next step will verify
		},
		{
			"Verify HTTPS",
			verifyHTTPS,
			nil,
		},
		{
			"Save metadata",
			saveMetadata,
			nil,
		},
	}

	// Execute steps
	for _, step := range steps {
		color.Cyan("\n▶ %s", step.name)
		if err := step.fn(ctx, serverMgr, serverName); err != nil {
			color.Red("✖ %s failed: %v", step.name, err)

			// Execute rollback if defined
			if step.roll != nil {
				color.Yellow("↩ Rolling back %s", step.name)
				if rollbackErr := step.roll(ctx, serverMgr, serverName); rollbackErr != nil {
					color.Red("✖ Rollback failed: %v", rollbackErr)
				}
			}

			return fmt.Errorf("%s failed: %w", step.name, err)
		}
		color.Green("✓ %s completed", step.name)
	}

	color.Green("\n✅ SSL setup completed successfully for %s", sslConfig.Domain)
	return nil
}

func loadSSLConfig() error {
	data, err := os.ReadFile(sslConfigFile)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	if err := yaml.Unmarshal(data, &sslConfig); err != nil {
		return fmt.Errorf("failed to parse YAML config: %w", err)
	}

	return nil
}

func validateInputs() error {
	if sslConfig.Wildcard && sslConfig.DNSProvider == "" {
		return fmt.Errorf("DNS provider must be specified for wildcard certificates")
	}

	if !strings.Contains(sslConfig.Domain, ".") {
		return fmt.Errorf("invalid domain format")
	}

	// Basic email validation
	if !strings.Contains(sslConfig.Email, "@") {
		return fmt.Errorf("invalid email format")
	}

	return nil
}

func checkCertificateExists(ctx context.Context, serverMgr *server.ServerStruct, serverName string) (bool, error) {
	// Check Caddy's certificate storage
	cmd := fmt.Sprintf("sudo test -f /var/lib/caddy/certificates/acme-v02.api.letsencrypt.org-directory/%s/%s.crt && echo exists",
		strings.ReplaceAll(sslConfig.Domain, "*", "_wildcard"), sslConfig.Domain)

	output, err := serverMgr.ExecuteCommand(ctx, serverName, cmd, nil)
	if err != nil {
		return false, fmt.Errorf("certificate check command failed: %w", err)
	}

	return strings.Contains(output, "exists"), nil
}

func createWorkingDir(ctx context.Context, serverMgr *server.ServerStruct, serverName string) error {
	cmds := []string{
		fmt.Sprintf("sudo mkdir -p %s", sslMetadataDir),
		fmt.Sprintf("sudo chown $(whoami): %s", sslMetadataDir),
		fmt.Sprintf("mkdir -p %s/%s", sslMetadataDir, sslConfig.Domain),
	}

	for _, cmd := range cmds {
		if _, err := serverMgr.ExecuteCommand(ctx, serverName, cmd, nil); err != nil {
			return fmt.Errorf("failed to create working directory: %w", err)
		}
	}
	return nil
}

func removeWorkingDir(ctx context.Context, serverMgr *server.ServerStruct, serverName string) error {
	_, err := serverMgr.ExecuteCommand(ctx, serverName,
		fmt.Sprintf("rm -rf %s/%s", sslMetadataDir, sslConfig.Domain), nil)
	return err
}

func configureCaddyfile(ctx context.Context, serverMgr *server.ServerStruct, serverName string) error {
	// Generate Caddyfile configuration
	config := generateCaddyConfig()
	tempFile := fmt.Sprintf("%s/%s/Caddyfile", sslMetadataDir, sslConfig.Domain)

	// Upload temporary config
	if err := serverMgr.UploadFile(ctx, serverName, tempFile, config); err != nil {
		return fmt.Errorf("failed to upload Caddyfile: %w", err)
	}

	// Move to final location
	cmd := fmt.Sprintf("sudo mv %s %s/Caddyfile.%s && sudo chown caddy:caddy %s/Caddyfile.%s",
		tempFile, caddyConfigDir, sslConfig.Domain, caddyConfigDir, sslConfig.Domain)

	if _, err := serverMgr.ExecuteCommand(ctx, serverName, cmd, nil); err != nil {
		return fmt.Errorf("failed to move Caddyfile: %w", err)
	}

	// Include in main Caddyfile
	includeCmd := fmt.Sprintf(`echo "import %s/Caddyfile.%s" | sudo tee -a %s/Caddyfile`,
		caddyConfigDir, sslConfig.Domain, caddyConfigDir)

	_, err := serverMgr.ExecuteCommand(ctx, serverName, includeCmd, nil)
	return err
}

func generateCaddyConfig() string {
	acmeServer := "https://acme-v02.api.letsencrypt.org/directory"
	if sslConfig.Staging {
		acmeServer = "https://acme-staging-v02.api.letsencrypt.org/directory"
	}

	config := fmt.Sprintf("%s {\n", sslConfig.Domain)
	config += fmt.Sprintf("  tls %s {\n", sslConfig.Email)
	config += "    protocols tls1.2 tls1.3\n"

	if sslConfig.Wildcard {
		config += fmt.Sprintf("    dns %s\n", sslConfig.DNSProvider)
	} else {
		config += "    http\n"
	}

	config += fmt.Sprintf("    acme_ca %s\n", acmeServer)
	config += "  }\n"
	config += "}\n"

	return config
}

func rollbackCaddyfile(ctx context.Context, serverMgr *server.ServerStruct, serverName string) error {
	cmds := []string{
		fmt.Sprintf("sudo rm -f %s/Caddyfile.%s", caddyConfigDir, sslConfig.Domain),
		fmt.Sprintf(`sudo sed -i '/import %s\/Caddyfile.%s/d' %s/Caddyfile`,
			caddyConfigDir, sslConfig.Domain, caddyConfigDir),
	}

	for _, cmd := range cmds {
		if _, err := serverMgr.ExecuteCommand(ctx, serverName, cmd, nil); err != nil {
			return err
		}
	}
	return nil
}

func reloadCaddy(ctx context.Context, serverMgr *server.ServerStruct, serverName string) error {
	_, err := serverMgr.ExecuteCommand(ctx, serverName,
		"sudo systemctl reload caddy || sudo systemctl restart caddy", os.Stdout)
	return err
}

func verifyHTTPS(ctx context.Context, serverMgr *server.ServerStruct, serverName string) error {
	// Simple curl check - could be enhanced with proper TLS verification
	url := fmt.Sprintf("https://%s", strings.TrimPrefix(sslConfig.Domain, "*."))
	cmd := fmt.Sprintf(`curl -sSI %s | grep -i "HTTP/.*200"`, url)

	_, err := serverMgr.ExecuteCommand(ctx, serverName, cmd, os.Stdout)
	return err
}

func saveMetadata(ctx context.Context, serverMgr *server.ServerStruct, serverName string) error {
	meta := CertificateMetadata{
		Domain:      sslConfig.Domain,
		Email:       sslConfig.Email,
		CreatedAt:   time.Now().Format(time.RFC3339),
		Wildcard:    sslConfig.Wildcard,
		Staging:     sslConfig.Staging,
		DNSProvider: sslConfig.DNSProvider,
	}

	jsonData, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	filename := fmt.Sprintf("%s/%s/metadata.json", sslMetadataDir, sslConfig.Domain)
	return serverMgr.UploadFile(ctx, serverName, filename, string(jsonData))
}

type CertificateMetadata struct {
	Domain      string `json:"domain"`
	Email       string `json:"email"`
	CreatedAt   string `json:"created_at"`
	ExpiryDate  string `json:"expiry_date,omitempty"`
	Wildcard    bool   `json:"wildcard"`
	Staging     bool   `json:"staging"`
	DNSProvider string `json:"dns_provider,omitempty"`
}
