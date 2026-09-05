package dns

import (
	"fmt"
	"os"
	"time"
)

const (
	docsURL = "https://nextdeploy.org/docs"
)

// GenerateVPSGuide creates a dns.md file for VPS deployments.
func GenerateVPSGuide(domain string, serverIP string) error {
	f, err := os.Create("dns.md")
	if err != nil {
		return fmt.Errorf("failed to create DNS guide: %w", err)
	}
	defer f.Close()

	writeHeader(f, domain, "VPS (Direct Server)")

	fmt.Fprintf(f, "Server IP: **%s**\n\n", serverIP)
	writeImportantNotice(f)
	writePropagationInfo(f)

	// Step 1: Root A Record
	fmt.Fprintf(f, "## 📍 Step 1: Root Domain A Record\n\n")
	fmt.Fprintf(f, "Points your main domain directly to your server.\n\n")

	writeARecordInstructions(f, "@", serverIP)
	fmt.Fprintf(f, "\n")

	// Step 2: WWW CNAME Record
	fmt.Fprintf(f, "## 🔄 Step 2: WWW Subdomain\n\n")
	fmt.Fprintf(f, "Ensures `www.%s` works properly.\n\n", domain)

	writeCNAMERecordInstructions(f, "www", domain, "CNAME")
	fmt.Fprintf(f, "\n")

	writeProviderGuidance(f, domain)
	writePitfallsSection(f, domain)
	writeVerificationSection(f)

	// VPS-specific final steps
	fmt.Fprintf(f, "## 🚀 Final Steps\n\n")
	fmt.Fprintf(f, "1. ✅ Save both records in your DNS panel\n")
	fmt.Fprintf(f, "2. ⏱️ Wait 5-10 minutes for propagation\n")
	fmt.Fprintf(f, "3. 🔒 SSL will be automatically provisioned by Caddy on first visit\n")
	fmt.Fprintf(f, "4. 🌐 Visit https://%s to test\n\n", domain)

	return nil
}

func writeHeader(f *os.File, domain string, deploymentType string) {
	fmt.Fprintf(f, "# 🌐 NextDeploy DNS Setup Guide\n\n")
	fmt.Fprintf(f, "Target Domain: **%s**\n", domain)
	fmt.Fprintf(f, "Deployment Type: **%s**\n", deploymentType)
	fmt.Fprintf(f, "Generated: `%s`\n\n", time.Now().Format("2006-01-02 15:04:05 MST"))
}

func writeImportantNotice(f *os.File) {
	fmt.Fprintf(f, "> [!IMPORTANT]\n")
	fmt.Fprintf(f, "> This guide contains **exact** values for your domain. Copy them precisely.\n")
	fmt.Fprintf(f, "> DNS changes can take 5-60 minutes to propagate worldwide.\n")
	fmt.Fprintf(f, "> 📚 [Full Documentation](%s)\n\n", docsURL)
}

func writePropagationInfo(f *os.File) {
	fmt.Fprintf(f, "### ⏱️ DNS Propagation Timeline\n\n")
	fmt.Fprintf(f, "| DNS Server | Typical Time |\n")
	fmt.Fprintf(f, "| :--- | :--- |\n")
	fmt.Fprintf(f, "| Namecheap/Provider | ⚡ Instant (once saved) |\n")
	fmt.Fprintf(f, "| Google (8.8.8.8) | 5-30 minutes |\n")
	fmt.Fprintf(f, "| Cloudflare (1.1.1.1) | 5-30 minutes |\n")
	fmt.Fprintf(f, "| Worldwide | 24-48 hours max |\n\n")
}

func writeCNAMERecordInstructions(f *os.File, host string, value string, recordType string) {
	fmt.Fprintf(f, "| Field | Value |\n")
	fmt.Fprintf(f, "| :--- | :--- |\n")
	fmt.Fprintf(f, "| **Type** | `%s` |\n", recordType)
	fmt.Fprintf(f, "| **Host/Name** | `%s` |\n", host)
	fmt.Fprintf(f, "| **Value/Target** | `%s` |\n", value)
	fmt.Fprintf(f, "| **TTL** | `Automatic` (or 5-30 minutes) |\n\n")
}

func writeARecordInstructions(f *os.File, host string, ip string) {
	fmt.Fprintf(f, "| Field | Value |\n")
	fmt.Fprintf(f, "| :--- | :--- |\n")
	fmt.Fprintf(f, "| **Type** | `A Record` |\n")
	fmt.Fprintf(f, "| **Host/Name** | `%s` |\n", host)
	fmt.Fprintf(f, "| **Value/IP** | `%s` |\n", ip)
	fmt.Fprintf(f, "| **TTL** | `Automatic` |\n\n")
}

func writeProviderGuidance(f *os.File, domain string) {
	fmt.Fprintf(f, "## 📋 Provider-Specific Instructions\n\n")

	// Namecheap
	fmt.Fprintf(f, "### Namecheap\n\n")
	fmt.Fprintf(f, "| Do | Don't |\n")
	fmt.Fprintf(f, "| :--- | :--- |\n")
	fmt.Fprintf(f, "| ✅ Use `@` for root domain | ❌ Never include `.%s` in Host field |\n", domain)
	fmt.Fprintf(f, "| ✅ For www SSL: `_hash.www` in Host | ❌ Don't add trailing dots |\n")
	fmt.Fprintf(f, "| ✅ Copy values exactly as shown | ❌ Don't add extra spaces |\n\n")

	// Cloudflare
	fmt.Fprintf(f, "### Cloudflare\n\n")
	fmt.Fprintf(f, "⚠️ **Critical**: For SSL validation records, ensure the cloud icon is **gray** (DNS only)\n\n")
	fmt.Fprintf(f, "| Record Type | Proxy Status |\n")
	fmt.Fprintf(f, "| :--- | :--- |\n")
	fmt.Fprintf(f, "| SSL Validation Records | ⚪ Gray cloud (DNS only) |\n")
	fmt.Fprintf(f, "| Root/WWW (after SSL) | 🟠 Orange cloud (proxied) optional |\n\n")

	// GoDaddy
	fmt.Fprintf(f, "### GoDaddy\n\n")
	fmt.Fprintf(f, "- Use **@** for root domain\n")
	fmt.Fprintf(f, "- Points to field should NOT have trailing dot\n")
	fmt.Fprintf(f, "- TTL can be left as 1 hour\n\n")
}

func writePitfallsSection(f *os.File, domain string) {
	fmt.Fprintf(f, "## ⚠️ Common Pitfalls to Avoid\n\n")

	pitfalls := []struct {
		Bad  string
		Good string
		Why  string
	}{
		{
			Bad:  "_5f2eb7...nextdeploy.org",
			Good: "_5f2eb7...",
			Why:  "Host field should NOT include your domain name",
		},
		{
			Bad:  "_hash.www.nextdeploy.org",
			Good: "_hash.www",
			Why:  "For www SSL records, stop at '.www'",
		},
		{
			Bad:  "server.example.com.",
			Good: "server.example.com",
			Why:  "Most providers don't want trailing dots in the Value field",
		},
		{
			Bad:  "Waiting 2 minutes and giving up",
			Good: "Waiting 30+ minutes for propagation",
			Why:  "DNS propagation takes time - be patient!",
		},
	}

	fmt.Fprintf(f, "| ❌ Wrong | ✅ Correct | Why |\n")
	fmt.Fprintf(f, "| :--- | :--- | :--- |\n")
	for _, p := range pitfalls {
		fmt.Fprintf(f, "| `%s` | `%s` | %s |\n", p.Bad, p.Good, p.Why)
	}
	fmt.Fprintf(f, "\n")
}

func writeVerificationSection(f *os.File) {
	fmt.Fprintf(f, "## 🔍 How to Verify Records\n\n")

	fmt.Fprintf(f, "After adding records, verify they're working:\n\n")

	fmt.Fprintf(f, "```bash\n")
	fmt.Fprintf(f, "# Check root domain\n")
	fmt.Fprintf(f, "dig nextdeploy.org CNAME +short\n\n")
	fmt.Fprintf(f, "# Check SSL validation records\n")
	fmt.Fprintf(f, "dig _5f2eb7...nextdeploy.org CNAME +short\n")
	fmt.Fprintf(f, "dig @8.8.8.8 _hash.www.nextdeploy.org CNAME +short  # Use Google DNS\n\n")
	fmt.Fprintf(f, "# Watch for propagation\n")
	fmt.Fprintf(f, "watch -n 60 'dig @8.8.8.8 _hash.www.nextdeploy.org CNAME +short'\n")
	fmt.Fprintf(f, "```\n\n")

	fmt.Fprintf(f, "**Expected output**: You should see your server's IP or hostname\n\n")
}

// Helper function to determine record purpose.

// GenerateQuickReference generates a quick reference table for all records.
