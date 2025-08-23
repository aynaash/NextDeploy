package cmd

import (
	"context"
	"fmt"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"net/url"
	"nextdeploy/shared/config"
	"nextdeploy/shared"
	"nextdeploy/cli/internal/server"
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
	Enabled     bool   `yaml:"enabled"`
	Provider    string `yaml:"provider"`
	AutoRenew   bool   `yaml:"auto_renew"`
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
	sslogs = shared.PackageLogger("ssl", "SSL")
)

func init() {
	rootCmd.AddCommand(SSLCommand)
}

func runSSLCommand(cmd *cobra.Command, args []string) error {
	// Load configuration from YAML file

	// Load main configuration
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Extract domain from URL if necessary
	domain := cfg.App.Domain
	if strings.HasPrefix(domain, "http") {
		if u, err := url.Parse(domain); err == nil {
			domain = u.Hostname()
		}
	}

	// Initialize sslConfig from the loaded configuration
	sslConfig := SSLConfig{
		Domain:      domain, // Fallback to app domain if SSL domain not specified
		Email:       cfg.SSL.Email,
		Staging:     cfg.SSL.Staging,
		Wildcard:    cfg.SSL.Wildcard,
		DNSProvider: cfg.SSL.DNSProvider,
		Force:       cfg.SSL.Force,
		Enabled:     cfg.SSL.Enabled,
		Provider:    cfg.SSL.Provider,
		AutoRenew:   cfg.SSL.AutoRenew,
	}

	// If SSL-specific domain is set, use that instead
	if cfg.SSL.Domain != "" {
		sslConfig.Domain = cfg.SSL.Domain
	}

	sslogs.Debug("SSL Configuration", "config", sslConfig)

	// Validate inputs
	if err := validateInputs(&sslConfig); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	// Initialize server manager
	serverMgr, err := server.New(
		server.WithConfig(),
		server.WithSSH(),
	)
	if err != nil {
		return fmt.Errorf("failed to initialize server manager: %w", err)
	}
	sslogs.Debug("Server manager initialized")
	defer serverMgr.CloseSSHConnection()

	// Get deployment server

	serverName, err := TargetServer(ctx, serverMgr)

	if err != nil {
		return fmt.Errorf("failed to get deployment server: %w", err)
	}
	sslogs.Debug("Deployment server successfully ", "server", serverName)

	// Check if certificate already exists
	if !sslConfig.Force {
		if exists, err := checkCertificateExists(ctx, serverMgr, serverName, domain); err != nil {
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
func TargetServer(ctx context.Context, serverMgr *server.ServerStruct) (string, error) {
	if depServer, err := serverMgr.GetDeploymentServer(); err == nil {
		sslogs.Debug("Using deployment server:%s", depServer)
		return depServer, nil
	}

	// fallback to first available server
	servers := serverMgr.ListServers()
	if len(servers) == 0 {
		return "", fmt.Errorf("no server configured")
	}

	sslogs.Warn("No deployment server configured, using first server:%s", servers[0])
	return servers[0], nil
}
func validateInputs(sslConfig *SSLConfig) error {
	sslogs.Debug("Validating SSL configuration", "config", sslConfig)
	// Validate domain

	// Domain format validation
	if !strings.Contains(sslConfig.Domain, ".") || strings.HasPrefix(sslConfig.Domain, ".") ||
		strings.HasSuffix(sslConfig.Domain, ".") {
		return fmt.Errorf("invalid domain format: must be a fully qualified domain name (e.g., 'example.com')")
	}

	// Wildcard validation
	if sslConfig.Wildcard {
		if sslConfig.DNSProvider == "" {
			return fmt.Errorf("DNS provider is required for wildcard certificates (e.g., 'cloudflare', 'route53')")
		}
		if !strings.HasPrefix(sslConfig.Domain, "*.") {
			sslogs.Warn("wildcard domains must start with '*.' (e.g., '*.example.com')")
		}
	}

	// Email validation
	if sslConfig.Email == "" {
		return fmt.Errorf("email is required for SSL certificate registration")
	}
	if !isValidEmail(sslConfig.Email) {
		return fmt.Errorf("invalid email format: must contain '@' and valid domain (e.g., 'admin@example.com')")
	}

	// Staging environment warning
	if sslConfig.Staging {
		color.Yellow("⚠️  Using Let's Encrypt staging environment - certificates will not be trusted")
	}

	return nil
}

func isValidEmail(email string) bool {
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return false
	}
	if parts[0] == "" || parts[1] == "" {
		return false
	}
	return strings.Contains(parts[1], ".")
}
func checkCertificateExists(ctx context.Context, serverMgr *server.ServerStruct, serverName string, domain string) (bool, error) {
	// TODO: Implement proper certificate existence checking logic
	// Current implementation is mocked for development purposes

	// Mock scenarios:
	// - Return true for staging environment to test renewal flow
	// - Return false for production to test new certificate flow
	// - Simulate error cases for testing error handling

	// Mock 1: Always return false in production to test new cert issuance
	if !sslConfig.Staging {
		sslogs.Debug("Mock: No existing certificate found (production mode)")
		return false, nil
	}

	// Mock 2: Return true in staging environment
	if sslConfig.Staging {
		sslogs.Debug("Mock: Existing certificate found (staging mode)")
		return true, nil
	}

	// Mock 3: Simulate error case (uncomment to test error handling)
	// return false, fmt.Errorf("mock certificate check error")

	// TODO: Actual implementation should:
	// 1. Check multiple certificate storage locations:
	//    - Caddy's default location (/var/lib/caddy/certificates)
	//    - System certificate stores
	//    - Custom locations from config

	// 2. Verify certificate is valid and not expired
	//    - Check expiration date
	//    - Verify domain matches
	//    - Check certificate chain

	// 3. Handle different certificate types:
	//    - Regular certificates
	//    - Wildcard certificates
	//    - SAN certificates

	// 4. Support multiple ACME providers:
	//    - Let's Encrypt
	//    - Other ACME-compatible CAs

	// Example of what real implementation might look like:
	/*
	   certPath := fmt.Sprintf(
	       "/var/lib/caddy/certificates/acme-v02.api.letsencrypt.org-directory/%s/%s.crt",
	       strings.ReplaceAll(domain, "*", "_wildcard"),
	       domain,
	   )

	   cmd := fmt.Sprintf(
	       "sudo test -f %s && sudo openssl x509 -in %s -noout -checkend 86400 && echo valid",
	       certPath,
	       certPath,
	   )

	   output, err := serverMgr.ExecuteCommand(ctx, serverName, cmd, nil)
	   if err != nil {
	       return false, fmt.Errorf("certificate check failed: %w", err)
	   }

	   return strings.Contains(output, "valid"), nil
	*/

	// Fallback return (should never reach here in mock mode)
	return false, nil
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

func saveMetadata(ctx context.Context, serverMgr *server.ServerStruct, serverName string) error {
	// TODO: Implement complete metadata saving logic
	// Current implementation is mocked for development

	color.Yellow("⚠️  Mock: Simulating metadata save for domain %s", sslConfig.Domain)

	// Create mock metadata structure
	meta := CertificateMetadata{
		Domain:      sslConfig.Domain,
		Email:       sslConfig.Email,
		CreatedAt:   time.Now().Format(time.RFC3339),
		Wildcard:    sslConfig.Wildcard,
		Staging:     sslConfig.Staging,
		DNSProvider: sslConfig.DNSProvider,
	}

	// Log what would be saved
	sslogs.Debug("Would save certificate metadata",
		"domain", meta.Domain,
		"email", meta.Email,
		"wildcard", meta.Wildcard,
		"staging", meta.Staging)

	/*
	   REAL IMPLEMENTATION SHOULD:
	   1. Create JSON metadata with all certificate details
	   2. Handle proper file paths and permissions
	   3. Upload to server in the correct location
	   4. Include additional validation and error handling

	   Example implementation:
	   jsonData, err := json.MarshalIndent(meta, "", "  ")
	   if err != nil {
	       return fmt.Errorf("failed to marshal metadata: %w", err)
	   }

	   // Create local temporary file first
	   localFile := "/tmp/ssl_metadata.json"
	   if err := os.WriteFile(localFile, jsonData, 0644); err != nil {
	       return fmt.Errorf("failed to create local metadata file: %w", err)
	   }

	   // Upload to server
	   remotePath := fmt.Sprintf("%s/%s/metadata.json", sslMetadataDir, sslConfig.Domain)
	   if err := serverMgr.UploadFile(ctx, serverName, remotePath, localFile); err != nil {
	       return fmt.Errorf("failed to upload metadata: %w", err)
	   }

	   // Clean up local file
	   defer os.Remove(localFile)

	   // Set proper permissions on server
	   permCmd := fmt.Sprintf("sudo chown caddy:caddy %s", remotePath)
	   if _, err := serverMgr.ExecuteCommand(ctx, serverName, permCmd, nil); err != nil {
	       return fmt.Errorf("failed to set metadata file permissions: %w", err)
	   }
	*/

	return nil
}
func configureCaddyfile(ctx context.Context, serverMgr *server.ServerStruct, serverName string) error {
	// TODO: Implement complete Caddyfile configuration logic
	// Current implementation is mocked for development

	color.Yellow("⚠️  Mock: Skipping actual Caddyfile configuration")
	sslogs.Debug("Would generate and configure Caddyfile for domain", "domain", sslConfig.Domain)

	/*
	   REAL IMPLEMENTATION SHOULD:
	   1. Generate proper Caddyfile configuration based on:
	      - Domain (including wildcard handling)
	      - SSL provider (Let's Encrypt, etc.)
	      - DNS challenge configuration
	      - TLS settings

	   2. Handle file operations:
	      - Create temporary local file
	      - Upload to server
	      - Move to final location
	      - Set proper permissions

	   3. Example implementation:
	   config := generateCaddyConfig()
	   tempFile := fmt.Sprintf("%s/%s/Caddyfile", sslMetadataDir, sslConfig.Domain)

	   if err := serverMgr.UploadFile(ctx, serverName, tempFile, config); err != nil {
	       return fmt.Errorf("failed to upload Caddyfile: %w", err)
	   }

	   finalPath := fmt.Sprintf("%s/Caddyfile.%s", caddyConfigDir, sslConfig.Domain)
	   cmd := fmt.Sprintf("sudo mv %s %s && sudo chown caddy:caddy %s",
	       tempFile, finalPath, finalPath)

	   if _, err := serverMgr.ExecuteCommand(ctx, serverName, cmd, nil); err != nil {
	       return fmt.Errorf("failed to move Caddyfile: %w", err)
	   }

	   includeCmd := fmt.Sprintf(`echo "import %s" | sudo tee -a %s/Caddyfile`,
	       finalPath, caddyConfigDir)

	   _, err := serverMgr.ExecuteCommand(ctx, serverName, includeCmd, nil)
	   return err
	*/

	return nil
}

func generateCaddyConfig() string {
	// TODO: Implement complete Caddyfile generation
	// Current implementation returns mock config

	/*
	   REAL IMPLEMENTATION SHOULD:
	   1. Handle different ACME providers
	   2. Support both HTTP and DNS challenges
	   3. Configure proper TLS settings
	   4. Handle wildcard certificates
	   5. Include all necessary directives

	   Example:
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
	*/

	return "# Mock Caddyfile configuration\n" +
		"# Real implementation will generate proper config\n"
}

func rollbackCaddyfile(ctx context.Context, serverMgr *server.ServerStruct, serverName string) error {
	// TODO: Implement complete rollback logic
	// Current implementation is mocked

	color.Yellow("⚠️  Mock: Skipping actual Caddyfile rollback")
	sslogs.Debug("Would rollback Caddyfile configuration for domain", "domain", sslConfig.Domain)

	/*
	   REAL IMPLEMENTATION SHOULD:
	   1. Remove the generated Caddyfile
	   2. Clean up any temporary files
	   3. Remove the import line from main Caddyfile
	   4. Handle errors gracefully

	   Example:
	   cmds := []string{
	       fmt.Sprintf("sudo rm -f %s/Caddyfile.%s", caddyConfigDir, sslConfig.Domain),
	       fmt.Sprintf(`sudo sed -i '/import %s\/Caddyfile.%s/d' %s/Caddyfile`,
	           caddyConfigDir, sslConfig.Domain, caddyConfigDir),
	   }

	   for _, cmd := range cmds {
	       if _, err := serverMgr.ExecuteCommand(ctx, serverName, cmd, nil); err != nil {
	           sslogs.Warn("Failed during rollback", "cmd", cmd, "error", err)
	       }
	   }
	*/

	return nil
}

func reloadCaddy(ctx context.Context, serverMgr *server.ServerStruct, serverName string) error {
	// TODO: Implement proper Caddy reload logic
	// Current implementation is mocked

	color.Yellow("⚠️  Mock: Skipping actual Caddy reload")
	sslogs.Debug("Would reload Caddy service")

	/*
	   REAL IMPLEMENTATION SHOULD:
	   1. Handle different reload methods
	   2. Fallback to restart if reload fails
	   3. Verify service status

	   Example:
	   _, err := serverMgr.ExecuteCommand(ctx, serverName,
	       "sudo systemctl reload caddy || sudo systemctl restart caddy", os.Stdout)
	   return err
	*/

	return nil
}

func verifyHTTPS(ctx context.Context, serverMgr *server.ServerStruct, serverName string) error {
	// TODO: Implement proper HTTPS verification
	// Current implementation is mocked

	color.Yellow("⚠️  Mock: Skipping actual HTTPS verification")
	sslogs.Debug("Would verify HTTPS access for domain", "domain", sslConfig.Domain)

	/*
	   REAL IMPLEMENTATION SHOULD:
	   1. Perform proper TLS verification
	   2. Check certificate validity
	   3. Verify domain matches
	   4. Handle different verification methods

	   Example:
	   url := fmt.Sprintf("https://%s", strings.TrimPrefix(sslConfig.Domain, "*."))
	   cmd := fmt.Sprintf(`curl -sSI %s | grep -i "HTTP/.*200"`, url)

	   _, err := serverMgr.ExecuteCommand(ctx, serverName, cmd, os.Stdout)
	   return err
	*/

	return nil
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
