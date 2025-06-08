package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"log"
	"os"
	"path/filepath"
)

// TODO: Make this configurable via a flag or env variable
const sslMetadataDir = "/var/lib/nextdeploy/ssl_metadata"

// SSLCommand represents the 'ssl' command in the CLI.
var SSLCommand = &cobra.Command{
	Use:   "ssl",
	Short: "Automate SSL setup and Caddy configuration for a domain.",
	RunE:  runSSLCommand,
}

var (
	domain      string
	email       string
	staging     bool
	wildcard    bool
	dnsProvider string
)

func init() {
	SSLCommand.Flags().StringVar(&domain, "domain", "", "Domain name for the SSL certificate (e.g. example.com)")
	SSLCommand.Flags().StringVar(&email, "email", "", "Email address for Let's Encrypt registration")
	SSLCommand.Flags().BoolVar(&staging, "staging", false, "Use Let's Encrypt staging server for testing")
	SSLCommand.Flags().BoolVar(&wildcard, "wildcard", false, "Request a wildcard certificate")
	SSLCommand.Flags().StringVar(&dnsProvider, "dns", "", "DNS provider for DNS-01 challenge (required for wildcard)")
	SSLCommand.MarkFlagRequired("domain")
	SSLCommand.MarkFlagRequired("email")
}

func runSSLCommand(cmd *cobra.Command, args []string) error {
	// TODO: Validate domain name using stricter checks (FQDN regex, IP rejection unless explicitly supported)
	// TODO: Validate email format (currently no format checking)

	lockFilePath := filepath.Join(os.TempDir(), fmt.Sprintf("nextdeploy-ssl-%s.lock", domain))
	lockAcquired, err := acquireLock(lockFilePath)
	if err != nil {
		return err
	}
	if !lockAcquired {
		log.Printf("Another SSL setup process is already running for domain: %s", domain)
		return nil
	}
	defer releaseLock(lockFilePath)

	// TODO: Respect --staging flag by modifying ACME config in Caddyfile
	// TODO: Implement DNS-01 support using --dns and --wildcard
	// NOTE: These flags are currently parsed but not used. This is misleading behavior.

	steps := []step{
		{"Configuring Caddyfile", configureCaddyfile}, // TODO: Implement full Caddyfile logic here
		{"Starting Caddy", startCaddy},                // TODO: Actually run or reload Caddy and handle errors
		{"Verifying HTTPS", verifyHTTPS},              // TODO: Perform TLS check and domain validation here
		{"Saving Certificate Metadata", saveMetadata}, // TODO: Actually persist metadata (currently stubbed)
	}

	for _, s := range steps {
		log.Println(s.description)
		if err := s.fn(); err != nil {
			log.Printf("Error: %v. Rolling back changes...", err)
			rollbackChanges()
			return err
		}
	}

	log.Printf("SSL setup and Caddy configuration completed successfully for domain: %s", domain)
	return nil
}

func acquireLock(lockFilePath string) (bool, error) {
	// NOTE: Simple file lock, doesn't prevent cross-user race conditions (use flock or system-wide lock if needed)
	if _, err := os.Stat(lockFilePath); err == nil {
		return false, nil
	}

	lockFile, err := os.Create(lockFilePath)
	if err != nil {
		return false, fmt.Errorf("failed to create lock file: %w", err)
	}
	lockFile.Close()
	return true, nil
}

func releaseLock(lockFilePath string) {
	_ = os.Remove(lockFilePath)
}

// step represents a setup step with a description and function.
type step struct {
	description string
	fn          func() error
}

func configureCaddyfile() error {
	// TODO: Generate or modify the Caddyfile to include the SSL domain block
	// TODO: Handle wildcard domains and ACME challenge customization
	log.Println("Stub: configureCaddyfile not implemented")
	return nil
}

func startCaddy() error {
	// TODO: Actually reload/start Caddy server (e.g., systemctl restart caddy or caddy reload)
	// TODO: Capture and log stderr/stdout from the process
	log.Println("Stub: startCaddy not implemented")
	return nil
}

func verifyHTTPS() error {
	// TODO: Use Go's net/http and crypto/tls to validate HTTPS and cert expiry
	// TODO: Retry if domain takes time to propagate DNS changes
	log.Println("Stub: verifyHTTPS not implemented")
	return nil
}

func saveMetadata() error {
	// TODO: Write metadata to disk in JSON format under sslMetadataDir/domain.json
	log.Println("Stub: saveMetadata not implemented")
	return nil
}

func rollbackChanges() {
	// TODO: Undo partial changes — remove partial Caddyfile blocks, clean metadata files, etc.
	log.Println("Stub: rollbackChanges not implemented")
}

type CertificateMetadata struct {
	Domain      string `json:"domain"`
	Email       string `json:"email"`
	CreatedAt   string `json:"created_at"`
	ExpiryDate  string `json:"expiry_date"`
	Wildcard    bool   `json:"wildcard"`
	Staging     bool   `json:"staging"`
	DNSProvider string `json:"dns_provider"`
}

// NOTE: This metadata struct is unused. Hook it into saveMetadata() to provide cert auditing/logging.
