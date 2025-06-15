
Here’s your **Cobra CLI boilerplate** for the `ssl-setup` command, designed to:

> ✅ Read `nextdeploy.yml`
> ✅ Extract domain, app name, email
> ✅ Use **Kadi.zone** to generate an SSL cert
> ✅ SSH into server from config
> ✅ Install the SSL cert, configure reverse proxy (e.g. nginx/Caddy)
> ✅ Notify user to update DNS A record manually

---

## 🔥 Command: `nextdeploy ssl-setup`

---

### 📁 Project Structure Addition

```
copra-cli/
├── cmd/
│   └── ssl_setup.go     <-- new command here
├── internal/
│   ├── ssl/             <-- handles cert issuance via kadi.zone
│   ├── ssh/             <-- shared SSH logic
│   └── config/          <-- shared config loader
```

---

### 📦 `cmd/ssl_setup.go`

```go
package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"copra-cli/internal/ssl"
)

var sslSetupCmd = &cobra.Command{
	Use:   "ssl-setup",
	Short: "Sets up SSL certificate for your app using kadi.zone",
	Long: `Reads nextdeploy.yml, extracts domain + app + email, 
requests SSL cert from kadi.zone, SSHs into the server and installs it.`,
	Run: func(cmd *cobra.Command, args []string) {
		err := ssl.Setup()
		if err != nil {
			fmt.Printf("❌ SSL setup failed: %v\n", err)
		} else {
			fmt.Println("✅ SSL setup completed successfully!")
		}
	},
}

func init() {
	rootCmd.AddCommand(sslSetupCmd)
}
```

---

### 🧠 `internal/ssl/setup.go`

```go
package ssl

import (
	"context"
	"fmt"

	"copra-cli/internal/config"
	"copra-cli/internal/ssh"
)

func Setup() error {
	ctx := context.Background()

	// 1. Load Config
	cfg, err := config.Load("./nextdeploy.yml")
	if err != nil {
		return fmt.Errorf("could not load config: %w", err)
	}
	domain := cfg.App.Domain
	email := cfg.App.Email
	appName := cfg.App.Name

	fmt.Println("🌐 Domain:", domain)
	fmt.Println("📧 Email:", email)
	fmt.Println("📦 App:", appName)

	// 2. Generate SSL Cert via Kadi.zone
	fmt.Println("🔒 Requesting SSL cert from kadi.zone...")
	certData, err := RequestCertificateFromKadi(domain, email)
	if err != nil {
		return fmt.Errorf("failed to request SSL cert: %w", err)
	}
	fmt.Println("✅ Certificate issued successfully!")

	// 3. Connect to server via SSH
	fmt.Println("🔌 Connecting to server for installation...")
	client, err := ssh.NewClient(cfg.Server.IP, cfg.Server.SSHKey)
	if err != nil {
		return fmt.Errorf("SSH connection failed: %w", err)
	}
	defer client.Close()

	// 4. Upload cert files
	err = client.Upload(certData.CertPath, "/etc/ssl/"+domain+".crt")
	if err != nil {
		return fmt.Errorf("failed to upload cert: %w", err)
	}
	err = client.Upload(certData.KeyPath, "/etc/ssl/"+domain+".key")
	if err != nil {
		return fmt.Errorf("failed to upload key: %w", err)
	}

	// 5. Configure reverse proxy
	fmt.Println("⚙️ Setting up reverse proxy config...")
	err = client.Exec(GenerateNginxConfig(domain, appName))
	if err != nil {
		return fmt.Errorf("reverse proxy setup failed: %w", err)
	}

	// 6. Reload nginx
	client.Exec("sudo systemctl reload nginx")

	// 7. Notify user to update DNS
	fmt.Printf("\n🚨 FINAL STEP:\nPlease update your domain DNS A record to point to: %s\n", cfg.Server.IP)

	return nil
}
```

---

### 🔩 Mock: `RequestCertificateFromKadi`

```go
type CertData struct {
	CertPath string
	KeyPath  string
}

func RequestCertificateFromKadi(domain, email string) (*CertData, error) {
	// Simulate request to Kadi.zone (replace with real API logic)
	// You should save these locally or in /tmp
	return &CertData{
		CertPath: "/tmp/" + domain + ".crt",
		KeyPath:  "/tmp/" + domain + ".key",
	}, nil
}
```

---

### 🔧 Example Nginx Config Generator (Inline Command)

```go
func GenerateNginxConfig(domain, app string) string {
	return fmt.Sprintf(`
sudo bash -c 'cat > /etc/nginx/sites-available/%s <<EOF
server {
    listen 443 ssl;
    server_name %s;

    ssl_certificate /etc/ssl/%s.crt;
    ssl_certificate_key /etc/ssl/%s.key;

    location / {
        proxy_pass http://localhost:3000;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }
}
EOF

ln -s /etc/nginx/sites-available/%s /etc/nginx/sites-enabled/
'`, app, domain, domain, domain, app)
}
```

---

### 🧪 Run It

```bash
go run main.go ssl-setup
```

---

### 📢 Brutal Truth

* You’re still assuming a 1-server, 1-domain setup. Think ahead: wildcard certs? subdomains?
* This needs **idempotency**. If rerun, it should not break existing nginx config.
* You’ll need **real error catching for kadi.zone API**, not this mock.
* You also need **rate-limiting logic and retries** around cert issuance.
* Consider integrating **Caddy** as a smarter auto-renewing alternative to nginx + Certbot.

---

### 🧱 Want Next?

I can:

* Implement the real HTTP request to Kadi.zone (if there's a public API)
* Scaffold a working Caddy + systemd config
* Generate certs locally using acme.sh or Certbot if Kadi.zone fails

Let me know how deep you want to take this.
