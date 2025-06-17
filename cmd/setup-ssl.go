package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"nextdeploy/internal/server"
	"os"
	"strings"
	"time"
)

const (
	sslMetadataDir = "/var/lib/nextdeploy/ssl_metadata"
	caddyConfigDir = "/etc/caddy"
)

var (
	sslDomain      string
	sslEmail       string
	sslStaging     bool
	sslWildcard    bool
	sslDNSProvider string
	sslForce       bool
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

func init() {
	SSLCommand.Flags().StringVarP(&sslDomain, "domain", "d", "", "Domain name for SSL certificate (required)")
	SSLCommand.Flags().StringVarP(&sslEmail, "email", "e", "", "Email for Let's Encrypt (required)")
	SSLCommand.Flags().BoolVar(&sslStaging, "staging", false, "Use Let's Encrypt staging server")
	SSLCommand.Flags().BoolVar(&sslWildcard, "wildcard", false, "Request wildcard certificate (*.domain.com)")
	SSLCommand.Flags().StringVar(&sslDNSProvider, "dns", "", "DNS provider for DNS-01 challenge (required for wildcards)")
	SSLCommand.Flags().BoolVar(&sslForce, "force", false, "Force reconfiguration even if certificate exists")

	SSLCommand.MarkFlagRequired("domain")
	SSLCommand.MarkFlagRequired("email")
}

func runSSLCommand(cmd *cobra.Command, args []string) error {
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
	if !sslForce {
		if exists, err := checkCertificateExists(ctx, serverMgr, serverName); err != nil {
			return fmt.Errorf("certificate check failed: %w", err)
		} else if exists {
			color.Green("✓ SSL certificate already configured for %s", sslDomain)
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

	color.Green("\n✅ SSL setup completed successfully for %s", sslDomain)
	return nil
}

func validateInputs() error {
	if sslWildcard && sslDNSProvider == "" {
		return fmt.Errorf("DNS provider must be specified for wildcard certificates")
	}

	if !strings.Contains(sslDomain, ".") {
		return fmt.Errorf("invalid domain format")
	}

	// Basic email validation
	if !strings.Contains(sslEmail, "@") {
		return fmt.Errorf("invalid email format")
	}

	return nil
}

func checkCertificateExists(ctx context.Context, serverMgr *server.ServerStruct, serverName string) (bool, error) {
	// Check Caddy's certificate storage
	cmd := fmt.Sprintf("sudo test -f /var/lib/caddy/certificates/acme-v02.api.letsencrypt.org-directory/%s/%s.crt && echo exists",
		strings.ReplaceAll(sslDomain, "*", "_wildcard"), sslDomain)

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
		fmt.Sprintf("mkdir -p %s/%s", sslMetadataDir, sslDomain),
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
		fmt.Sprintf("rm -rf %s/%s", sslMetadataDir, sslDomain), nil)
	return err
}

func configureCaddyfile(ctx context.Context, serverMgr *server.ServerStruct, serverName string) error {
	// Generate Caddyfile configuration
	config := generateCaddyConfig()
	tempFile := fmt.Sprintf("%s/%s/Caddyfile", sslMetadataDir, sslDomain)

	// Upload temporary config
	if err := serverMgr.UploadFile(ctx, serverName, tempFile, strings.NewReader(config)); err != nil {
		return fmt.Errorf("failed to upload Caddyfile: %w", err)
	}

	// Move to final location
	cmd := fmt.Sprintf("sudo mv %s %s/Caddyfile.%s && sudo chown caddy:caddy %s/Caddyfile.%s",
		tempFile, caddyConfigDir, sslDomain, caddyConfigDir, sslDomain)

	if _, err := serverMgr.ExecuteCommand(ctx, serverName, cmd, nil); err != nil {
		return fmt.Errorf("failed to move Caddyfile: %w", err)
	}

	// Include in main Caddyfile
	includeCmd := fmt.Sprintf(`echo "import %s/Caddyfile.%s" | sudo tee -a %s/Caddyfile`,
		caddyConfigDir, sslDomain, caddyConfigDir)

	_, err := serverMgr.ExecuteCommand(ctx, serverName, includeCmd, nil)
	return err
}

func generateCaddyConfig() string {
	acmeServer := "https://acme-v02.api.letsencrypt.org/directory"
	if sslStaging {
		acmeServer = "https://acme-staging-v02.api.letsencrypt.org/directory"
	}

	config := fmt.Sprintf("%s {\n", sslDomain)
	config += fmt.Sprintf("  tls %s {\n", sslEmail)
	config += "    protocols tls1.2 tls1.3\n"

	if sslWildcard {
		config += fmt.Sprintf("    dns %s\n", sslDNSProvider)
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
		fmt.Sprintf("sudo rm -f %s/Caddyfile.%s", caddyConfigDir, sslDomain),
		fmt.Sprintf(`sudo sed -i '/import %s\/Caddyfile.%s/d' %s/Caddyfile`,
			caddyConfigDir, sslDomain, caddyConfigDir),
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
	url := fmt.Sprintf("https://%s", strings.TrimPrefix(sslDomain, "*."))
	cmd := fmt.Sprintf(`curl -sSI %s | grep -i "HTTP/.*200"`, url)

	_, err := serverMgr.ExecuteCommand(ctx, serverName, cmd, os.Stdout)
	return err
}

func saveMetadata(ctx context.Context, serverMgr *server.ServerStruct, serverName string) error {
	meta := CertificateMetadata{
		Domain:      sslDomain,
		Email:       sslEmail,
		CreatedAt:   time.Now().Format(time.RFC3339),
		Wildcard:    sslWildcard,
		Staging:     sslStaging,
		DNSProvider: sslDNSProvider,
	}

	jsonData, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	filename := fmt.Sprintf("%s/%s/metadata.json", sslMetadataDir, sslDomain)
	return serverMgr.UploadFile(ctx, serverName, filename, bytes.NewReader(jsonData))
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
