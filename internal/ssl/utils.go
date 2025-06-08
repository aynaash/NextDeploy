package ssl

import (
	"fmt"
	"os"
	"os/exec"
)

func checkCaddyInstallation() {
	fmt.Println("Checking for Caddy installation...")

	_, err := exec.LookPath("caddy")
	if err != nil {
		fmt.Println("Caddy not found, installing...")

		// Cross-platform installation using Caddy's official install script
		cmd := exec.Command("sh", "-c", "curl https://getcaddy.com | bash -s personal")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			fmt.Printf("Error installing Caddy: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Caddy installed successfully")
	} else {
		fmt.Println("Caddy is already installed")
	}
}

func createCaddyConfig(domain string, backend string) {
	configPath := fmt.Sprintf("/etc/caddy/Caddyfile")

	// Caddy automatically handles:
	// 1. SSL certificate provisioning via Let's Encrypt
	// 2. HTTP to HTTPS redirects
	// 3. SSL certificate renewals
	config := fmt.Sprintf(`%s {
	reverse_proxy %s {
		header_up Host {host}
		header_up X-Real-IP {remote}
		header_up X-Forwarded-For {remote}
		header_up X-Forwarded-Proto {scheme}
	}
}`, domain, backend)

	fmt.Printf("Creating Caddy configuration at %s...\n", configPath)

	err := os.WriteFile(configPath, []byte(config), 0644)
	if err != nil {
		fmt.Printf("Error writing Caddy config: %v\n", err)
		os.Exit(1)
	}

	// Enable and start Caddy service
	enableCmd := exec.Command("systemctl", "enable", "--now", "caddy")
	if err := enableCmd.Run(); err != nil {
		fmt.Printf("Error enabling Caddy service: %v\n", err)
		os.Exit(1)
	}

	// Reload Caddy to apply new configuration
	reloadCmd := exec.Command("systemctl", "reload", "caddy")
	if err := reloadCmd.Run(); err != nil {
		fmt.Printf("Error reloading Caddy: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Caddy configuration created and activated")
}

func printDNSInstructions(domain string) {
	fmt.Println("\n=== DNS Setup Instructions ===")
	fmt.Println("For your domain to work properly, you need to configure DNS records with your domain provider.")

	fmt.Println("\n1. If this server has a static IP address:")
	fmt.Printf("   - Create an A record for %s pointing to your server's IP address\n", domain)
	fmt.Printf("   - Create an A record for www.%s pointing to the same IP (or CNAME to %s)\n", domain, domain)

	fmt.Println("\n2. If you're using a dynamic DNS service:")
	fmt.Printf("   - Configure your dynamic DNS client for %s\n", domain)

	fmt.Println("\nNote: Caddy will automatically handle SSL certificate provisioning once DNS is properly configured.")
	fmt.Println("After DNS changes, it may take up to 48 hours to propagate globally")
	fmt.Println("You can verify DNS resolution with: dig +short " + domain)
}
