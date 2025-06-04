package ssl

import (
	"fmt"
	"os"
	"os/exec"
)

func installCertbot() {
	fmt.Println("Checking for certbot installation...")

	_, err := exec.LookPath("certbot")
	if err != nil {
		fmt.Println("Certbot not found, installing...")
		cmd := exec.Command("apt-get", "install", "-y", "certbot", "python3-certbot-nginx")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			fmt.Printf("Error installing certbot: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Certbot installed successfully")
	} else {
		fmt.Println("Certbot is already installed")
	}
}

func generateCertificate(domain string) {
	fmt.Printf("Generating SSL certificate for %s...\n", domain)

	cmd := exec.Command("certbot", "certonly", "--nginx", "-d", domain, "--non-interactive", "--agree-tos", "--email", "admin@"+domain)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		fmt.Printf("Error generating certificate: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("SSL certificate generated successfully")
}

func createNginxConfig(domain string) {
	configPath := fmt.Sprintf("/etc/nginx/sites-available/%s", domain)
	config := fmt.Sprintf(`server {
    listen 80;
    server_name %s;
    return 301 https://$host$request_uri;
}

server {
    listen 443 ssl;
    server_name %s;

    ssl_certificate /etc/letsencrypt/live/%s/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/%s/privkey.pem;

    location / {
        proxy_pass http://localhost:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}`, domain, domain, domain, domain)

	fmt.Printf("Creating Nginx configuration at %s...\n", configPath)

	// Write config file
	err := os.WriteFile(configPath, []byte(config), 0644)
	if err != nil {
		fmt.Printf("Error writing Nginx config: %v\n", err)
		os.Exit(1)
	}

	// Enable site
	linkCmd := exec.Command("ln", "-s", configPath, fmt.Sprintf("/etc/nginx/sites-enabled/%s", domain))
	if err := linkCmd.Run(); err != nil {
		fmt.Printf("Error enabling Nginx site: %v\n", err)
		os.Exit(1)
	}

	// Test and reload Nginx
	testCmd := exec.Command("nginx", "-t")
	if err := testCmd.Run(); err != nil {
		fmt.Printf("Nginx configuration test failed: %v\n", err)
		os.Exit(1)
	}

	reloadCmd := exec.Command("systemctl", "reload", "nginx")
	if err := reloadCmd.Run(); err != nil {
		fmt.Printf("Error reloading Nginx: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Nginx configuration created and activated")
}

func printDNSInstructions(domain string) {
	fmt.Println("\n=== DNS Setup Instructions ===")
	fmt.Println("For your domain to work properly, you need to configure DNS records with your domain provider.")
	fmt.Println("\n1. If this server has a static IP address:")
	fmt.Printf("   - Create an A record for %s pointing to your server's IP address\n", domain)
	fmt.Printf("   - Create an A record for www.%s pointing to the same IP (or CNAME to %s)\n", domain, domain)

	fmt.Println("\n2. If you're using a dynamic DNS service:")
	fmt.Printf("   - Configure your dynamic DNS client for %s\n", domain)

	fmt.Println("\n3. For email and other services:")
	fmt.Println("   - You may need additional MX or TXT records")

	fmt.Println("\nAfter DNS changes, it may take up to 48 hours to propagate globally")
	fmt.Println("You can verify DNS resolution with: dig +short " + domain)
}
