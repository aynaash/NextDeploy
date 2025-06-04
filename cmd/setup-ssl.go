package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

/*
Remaining Concerns & TODOs for Production-Readiness:

1. Error Chaining in Rollbacks:
   - Currently, rollback logic logs errors but does not propagate or collect them.
   - This may silently swallow critical cleanup failures.
   - TODO: Collect rollback errors into a slice and report them at the end.

2. DNS Validation Is Weak:
   - Uses basic net.LookupHost with A record assumption.
   - Fails to handle CDNs (e.g., Cloudflare), wildcard delegations, or CNAMEs.
   - TODO: Use a proper DNS library (e.g., miekg/dns) for robust, multi-record validation.

3. Lack of Test Coverage:
   - Business logic lives in `RunE`, tightly coupled to CLI and filesystem.
   - TODO: Extract setup logic into reusable pkg/ssl module.
   - TODO: Add unit tests with mocks and E2E tests using testcontainers or temp fs.

4. Certbot Execution Not Transparent:
   - Certbot likely called via shell (not shown).
   - Side-effects not logged or auditable (stdout/stderr).
   - TODO: Log full commands, redact sensitive info, capture output for audit logs.

5. Hardcoded System Paths:
   - Critical paths (/etc/nginx, /var/www, etc.) are static.
   - Breaks on Alpine, non-standard distros, or containers.
   - TODO: Make paths configurable via flags/env (viper preferred).

6. Missing Telemetry:
   - Trace ID is set but not used meaningfully.
   - TODO: Expose telemetry hook for logging, streaming to file, or emitting metrics.
*/
const (
	nginxAvailableDir   = "/etc/nginx/sites-available"
	nginxEnabledDir     = "/etc/nginx/sites-enabled"
	certbotLogDir       = "/var/log/certbot"
	sslSetupLockFile    = "/tmp/ssl-setup.lock"
	sslMetadataDir      = "/var/lib/ssl-setup/metadata"
	defaultNginxPort    = 8080
	defaultWebRoot      = "/var/www/html"
	defaultCertValidity = 90 * 24 * time.Hour // 90 days
)

var (
	lockFile     *os.File
	lockFileOnce sync.Once
)

type SSLOptions struct {
	Domain           string
	Email           string
	ProxyPort       int
	WebRoot         string
	DryRun          bool
	ForceRenewal    bool
	SkipNginxReload bool
	Staging         bool
	LogLevel        string
	ConfigFile      string
	DNSChallenge    bool
	Wildcard        bool
}

type CertificateMetadata struct {
	Domain        string    `json:"domain"`
	IssuedAt      time.Time `json:"issued_at"`
	ExpiresAt     time.Time `json:"expires_at"`
	Email         string    `json:"email"`
	IsStaging     bool      `json:"is_staging"`
	IsWildcard    bool      `json:"is_wildcard"`
	RenewalConfig string    `json:"renewal_config"`
}

var sslOpts SSLOptions

var sslCmd = &cobra.Command{
	Use:   "setup-ssl",
	Short: "Enterprise SSL certificate and Nginx reverse proxy setup",
	Long: `Automates the complete process of securing a domain with industry best practices:
- Let's Encrypt SSL certificate generation
- Nginx reverse proxy configuration
- DNS validation
- Security headers
- HTTP/2 and modern TLS configuration
- Automated renewal setup`,
	PreRunE: validateSSLFlags,
	RunE:    runSSLSetup,
	PostRun: cleanupSSLSetup,
}

func init() {
	rootCmd.AddCommand(sslCmd)

	// Required flags
	sslCmd.Flags().StringVarP(&sslOpts.Domain, "domain", "d", "", "Primary domain name (required)")
	sslCmd.MarkFlagRequired("domain")

	// Optional flags
	sslCmd.Flags().StringVarP(&sslOpts.Email, "email", "e", "", "Email for Let's Encrypt notifications")
	sslCmd.Flags().IntVarP(&sslOpts.ProxyPort, "port", "p", defaultNginxPort, "Upstream application port")
	sslCmd.Flags().StringVar(&sslOpts.WebRoot, "webroot", defaultWebRoot, "Web root directory for HTTP challenge")
	sslCmd.Flags().BoolVar(&sslOpts.DryRun, "dry-run", false, "Simulate operations without making changes")
	sslCmd.Flags().BoolVar(&sslOpts.ForceRenewal, "force-renewal", false, "Force certificate renewal if exists")
	sslCmd.Flags().BoolVar(&sslOpts.SkipNginxReload, "skip-reload", false, "Skip Nginx reload after configuration")
	sslCmd.Flags().BoolVar(&sslOpts.Staging, "staging", false, "Use Let's Encrypt staging server")
	sslCmd.Flags().StringVar(&sslOpts.LogLevel, "log-level", "info", "Log level (debug, info, warn, error)")
	sslCmd.Flags().StringVar(&sslOpts.ConfigFile, "config", "", "Path to custom configuration file")
	sslCmd.Flags().BoolVar(&sslOpts.DNSChallenge, "dns", false, "Use DNS challenge for wildcard certificates")
	sslCmd.Flags().BoolVar(&sslOpts.Wildcard, "wildcard", false, "Request wildcard certificate")

	// Bind to config file
	viper.BindPFlag("ssl.domain", sslCmd.Flags().Lookup("domain"))
	viper.BindPFlag("ssl.email", sslCmd.Flags().Lookup("email"))
}

// validateSSLFlags performs comprehensive validation of all input flags
func validateSSLFlags(cmd *cobra.Command, args []string) error {
	logger := setupLogger(sslOpts.LogLevel)

	if err := validateDomain(sslOpts.Domain, sslOpts.Wildcard); err != nil {
		return fmt.Errorf("domain validation failed: %w", err)
	}

	if sslOpts.Email != "" && !strings.Contains(sslOpts.Email, "@") {
		return fmt.Errorf("invalid email format: %s", sslOpts.Email)
	}

	if sslOpts.ProxyPort < 1 || sslOpts.ProxyPort > 65535 {
		return fmt.Errorf("invalid port number: %d", sslOpts.ProxyPort)
	}

	if sslOpts.Wildcard && !sslOpts.DNSChallenge {
		logger.Warn("Wildcard certificates require DNS challenge. Enabling DNS challenge automatically.")
		sslOpts.DNSChallenge = true
	}

	if sslOpts.ConfigFile != "" {
		if _, err := os.Stat(sslOpts.ConfigFile); err != nil {
			return fmt.Errorf("config file not found: %w", err)
		}
		viper.SetConfigFile(sslOpts.ConfigFile)
		if err := viper.ReadInConfig(); err != nil {
			return fmt.Errorf("failed to read config file: %w", err)
		}
	}

	return nil
}

// runSSLSetup orchestrates the entire SSL setup process
func runSSLSetup(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	logger := setupLogger(sslOpts.LogLevel)
	traceID := generateTraceID()
	ctx = context.WithValue(ctx, "traceID", traceID)

	logger.Info("Starting enterprise SSL setup", 
		"domain", sslOpts.Domain, 
		"dryRun", sslOpts.DryRun,
		"traceID", traceID)

	// Acquire lock to prevent concurrent execution
	if err := acquireLock(); err != nil {
		return fmt.Errorf("failed to acquire lock: %w", err)
	}

	// Setup rollback function in case of failure
	var success bool
	defer func() {
		if !success {
			logger.Error("SSL setup failed - initiating rollback")
			rollbackSSLSetup(ctx, logger)
		}
	}()

	// Execute setup steps
	steps := []struct {
		name string
		fn   func(context.Context, *slog.Logger) error
	}{
		{"Pre-flight checks", preFlightChecks},
		{"Directory setup", setupDirectories},
		{"Certificate setup", handleCertificateSetup},
		{"Nginx configuration", handleNginxConfig},
		{"Post-install verification", verifySetup},
	}

	for _, step := range steps {
		logger.Info(fmt.Sprintf("Executing step: %s", step.name))
		if err := step.fn(ctx, logger); err != nil {
			return fmt.Errorf("%s failed: %w", step.name, err)
		}
	}

	success = true
	printSuccessMessage(logger)
	return nil
}

// acquireLock prevents concurrent execution of the setup process
func acquireLock() error {
	var err error
	lockFileOnce.Do(func() {
		lockFile, err = os.OpenFile(sslSetupLockFile, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			if os.IsExist(err) {
				err = fmt.Errorf("another SSL setup process is already running")
			}
		} else {
			fmt.Fprintf(lockFile, "pid=%d\n", os.Getpid())
		}
	})
	return err
}

// cleanupSSLSetup releases resources after command execution
func cleanupSSLSetup(cmd *cobra.Command, args []string) {
	if lockFile != nil {
		os.Remove(sslSetupLockFile)
		lockFile.Close()
	}
}

// rollbackSSLSetup attempts to revert changes made during a failed setup
func rollbackSSLSetup(ctx context.Context, logger *slog.Logger) {
	// Remove Nginx config if created
	configPath := filepath.Join(nginxAvailableDir, sslOpts.Domain)
	if _, err := os.Stat(configPath); err == nil {
		if err := os.Remove(configPath); err != nil {
			logger.Error("Failed to remove Nginx config during rollback", "error", err)
		}
	}

	// Remove symlink if created
	enabledPath := filepath.Join(nginxEnabledDir, sslOpts.Domain)
	if _, err := os.Lstat(enabledPath); err == nil {
		if err := os.Remove(enabledPath); err != nil {
			logger.Error("Failed to remove Nginx symlink during rollback", "error", err)
		}
	}

	// Try to reload Nginx to clear any partial configuration
	if err := reloadNginx(logger); err != nil {
		logger.Error("Failed to reload Nginx during rollback", "error", err)
	}
}

// validateDomain performs comprehensive domain validation including DNS checks
func validateDomain(domain string, wildcard bool) error {
	if wildcard && !strings.HasPrefix(domain, "*.") {
		return fmt.Errorf("wildcard domain must start with '*.'")
	}

	if !isValidDomain(domain) {
		return fmt.Errorf("invalid domain format: %s", domain)
	}

	// Verify DNS resolution matches server IP
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	addrs, err := net.DefaultResolver.LookupHost(ctx, strings.TrimPrefix(domain, "*."))
	if err != nil {
		return fmt.Errorf("DNS lookup failed: %w", err)
	}

	serverIPs, err := getServerIPs()
	if err != nil {
		return fmt.Errorf("failed to get server IPs: %w", err)
	}

	// Check if any resolved IP matches server IP
	for _, addr := range addrs {
		for _, serverIP := range serverIPs {
			if addr == serverIP {
				return nil
			}
		}
	}

	return fmt.Errorf("domain %s does not resolve to this server's IP (%v)", domain, serverIPs)
}

// getServerIPs returns all non-loopback IP addresses of the server
func getServerIPs() ([]string, error) {
	var ips []string
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	for _, iface := range ifaces {
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			switch v := addr.(type) {
			case *net.IPNet:
				if !v.IP.IsLoopback() {
					ips = append(ips, v.IP.String())
				}
			case *net.IPAddr:
				if !v.IP.IsLoopback() {
					ips = append(ips, v.IP.String())
				}
			}
		}
	}

	if len(ips) == 0 {
		return nil, fmt.Errorf("no non-loopback IP addresses found")
	}

	return ips, nil
}

// handleCertificateSetup manages the certificate lifecycle
func handleCertificateSetup(ctx context.Context, logger *slog.Logger) error {
	certInstalled, err := isCertificateInstalled(sslOpts.Domain)
	if err != nil {
		return fmt.Errorf("certificate check failed: %w", err)
	}

	if !certInstalled || sslOpts.ForceRenewal {
		if err := generateCertificate(ctx, logger); err != nil {
			return err
		}
	} else {
		logger.Info("Valid certificate already exists", "domain", sslOpts.Domain)
	}

	// Save certificate metadata
	if err := saveCertificateMetadata(); err != nil {
		logger.Warn("Failed to save certificate metadata", "error", err)
	}

	return nil
}

// generateCertificate executes certbot with appropriate parameters
func generateCertificate(ctx context.Context, logger *slog.Logger) error {
	if err := ensureCertbotInstalled(logger); err != nil {
		return err
	}

	certbotArgs := []string{
		"certonly",
		"--non-interactive",
		"--agree-tos",
		"-d", sslOpts.Domain,
		"--logs-dir", certbotLogDir,
	}

	if sslOpts.DNSChallenge {
		certbotArgs = append(certbotArgs, "--dns-route53") // Adjust for other DNS providers as needed
	} else {
		certbotArgs = append(certbotArgs, "--nginx")
	}

	if sslOpts.Email != "" {
		certbotArgs = append(certbotArgs, "--email", sslOpts.Email)
	} else {
		certbotArgs = append(certbotArgs, "--register-unsafely-without-email")
	}

	if sslOpts.Staging {
		certbotArgs = append(certbotArgs, "--test-cert")
		logger.Warn("Using Let's Encrypt staging server - certificates will not be valid")
	}

	if sslOpts.DryRun {
		logger.Info("Dry run: would execute certbot with args", "args", certbotArgs)
		return nil
	}

	cmd := exec.CommandContext(ctx, "certbot", certbotArgs...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = io.MultiWriter(&stdout, os.Stdout)
	cmd.Stderr = io.MultiWriter(&stderr, os.Stderr)

	traceID := ctx.Value("traceID").(string)
	logger.Info("Executing certbot", "command", cmd.String(), "traceID", traceID)

	if err := cmd.Run(); err != nil {
		logger.Error("Certbot execution failed",
			"stdout", stdout.String(),
			"stderr", stderr.String(),
			"traceID", traceID)
		return fmt.Errorf("certbot failed: %w", err)
	}

	logger.Info("SSL certificate successfully obtained")
	return nil
}

// saveCertificateMetadata stores information about the issued certificate
func saveCertificateMetadata() error {
	if sslOpts.DryRun {
		return nil
	}

	if err := os.MkdirAll(sslMetadataDir, 0755); err != nil {
		return fmt.Errorf("failed to create metadata directory: %w", err)
	}

	certPath := fmt.Sprintf("/etc/letsencrypt/live/%s/cert.pem", sslOpts.Domain)
	output, err := exec.Command("openssl", "x509", "-in", certPath, "-noout", "-dates").CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to read certificate dates: %w", err)
	}

	dates := parseOpenSSLDates(string(output))
	if dates.notBefore.IsZero() || dates.notAfter.IsZero() {
		return fmt.Errorf("failed to parse certificate dates")
	}

	metadata := CertificateMetadata{
		Domain:        sslOpts.Domain,
		IssuedAt:      dates.notBefore,
		ExpiresAt:     dates.notAfter,
		Email:         sslOpts.Email,
		IsStaging:     sslOpts.Staging,
		IsWildcard:    sslOpts.Wildcard,
		RenewalConfig: fmt.Sprintf("/etc/letsencrypt/renewal/%s.conf", sslOpts.Domain),
	}

	metadataPath := filepath.Join(sslMetadataDir, fmt.Sprintf("%s.json", sslOpts.Domain))
	file, err := os.Create(metadataPath)
	if err != nil {
		return fmt.Errorf("failed to create metadata file: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(metadata); err != nil {
		return fmt.Errorf("failed to encode metadata: %w", err)
	}

	return nil
}

type certDates struct {
	notBefore time.Time
	notAfter  time.Time
}

func parseOpenSSLDates(output string) certDates {
	var dates certDates
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "notBefore=") {
			dates.notBefore, _ = time.Parse("Jan 2 15:04:05 2006 MST", strings.TrimPrefix(line, "notBefore="))
		}
		if strings.HasPrefix(line, "notAfter=") {
			dates.notAfter, _ = time.Parse("Jan 2 15:04:05 2006 MST", strings.TrimPrefix(line, "notAfter="))
		}
	}
	return dates
}

// generateTraceID creates a unique identifier for the current operation
func generateTraceID() string {
	return fmt.Sprintf("%d-%s", os.Getpid(), time.Now().Format("20060102-150405"))
}

// ... (other existing functions remain mostly the same, but should be updated to use the traceID from context)

// Example of updated function using traceID:
func verifyHTTPAccess(ctx context.Context, logger *slog.Logger) error {
	traceID := ctx.Value("traceID").(string)
	logger.Debug("Verifying HTTP access", "domain", sslOpts.Domain, "traceID", traceID)

	client := &http.Client{
		Timeout: 15 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	// Verify HTTP -> HTTPS redirect
	httpURL := fmt.Sprintf("http://%s", sslOpts.Domain)
	resp, err := client.Get(httpURL)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusMovedPermanently {
		return fmt.Errorf("expected HTTP 301 redirect, got %d", resp.StatusCode)
	}

	// Verify HTTPS access
	httpsURL := fmt.Sprintf("https://%s", sslOpts.Domain)
	resp, err = client.Get(httpsURL)
	if err != nil {
		return fmt.Errorf("HTTPS request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("expected HTTP 200, got %d", resp.StatusCode)
	}

	logger.Info("HTTP/HTTPS access verified successfully", "traceID", traceID)
	return nil
}
