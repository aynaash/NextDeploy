package initialcommand

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/aynaash/nextdeploy/cli/internal/infrasniff"
	"github.com/aynaash/nextdeploy/cli/internal/scaffold"
	"github.com/aynaash/nextdeploy/shared"
	"github.com/aynaash/nextdeploy/shared/config"
	"github.com/aynaash/nextdeploy/shared/nextcore"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
)

func RunInitCommand(cmd *cobra.Command, args []string) error {
	log := shared.PackageLogger("Initialization", "Initialization")

	log.Info("NextDeploy Initialization")
	log.Info("----------------------------------------")
	log.Info("Analysing your Next.js project...")

	// Basic analysis without requiring nextdeploy.yml
	appName := "example-app"
	if data, err := os.ReadFile("package.json"); err == nil {
		var pkg struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal(data, &pkg); err == nil && pkg.Name != "" {
			appName = pkg.Name
		}
	}

	nextVersion, _ := nextcore.GetNextJsVersion("package.json")
	if nextVersion != "" {
		log.Info("  ✓ Next.js %s detected", nextVersion)
	}

	cwd, _ := os.Getwd()
	pkgManager, _ := nextcore.DetectPackageManager(cwd)
	if pkgManager != nextcore.Unknown {
		log.Info("  ✓ Package manager: %s\n", pkgManager.String())
	}

	log.Info("\nNextDeploy understands your application.\n")
	prompt := &survey.Select{
		Message: "Where would you like to deploy your Next.js application?",
		Options: []string{
			"VPS (Virtual Private Server - SSH)",
			"Serverless (AWS CloudFront & Lambda)",
			"Serverless (Cloudflare Workers + R2)",
		},
	}
	var targetChoice string
	if err := survey.AskOne(prompt, &targetChoice); err != nil {
		return fmt.Errorf("prompt failed: %w", err)
	}

	targetType := "vps"
	switch {
	case strings.Contains(targetChoice, "Cloudflare"):
		targetType = "cloudflare"
	case strings.Contains(targetChoice, "Serverless"):
		targetType = "serverless"
	}

	// Cloudflare has two paths: scaffold a fresh fullstack app, or sniff the
	// existing app in this directory and prefill its bindings.
	if targetType == "cloudflare" {
		return runCloudflareInit(log, appName)
	}

	// Ask about the custom domain and where its DNS lives, so the generated
	// config carries enough for provider-aware DNS guidance later.
	domainCfg, err := promptDomain()
	if err != nil {
		return fmt.Errorf("prompt failed: %w", err)
	}

	// Use the raw template to preserve comments
	configContent := config.GetSampleConfigTemplate(targetType)
	configContent = strings.ReplaceAll(configContent, "name: example-app", "name: "+appName)
	configContent = strings.Replace(configContent,
		"  domain: app.example.com # Public domain for your app",
		renderDomainYAML(domainCfg), 1)

	if err := os.WriteFile("nextdeploy.yml", []byte(configContent), 0600); err != nil {
		return fmt.Errorf("failed to save configuration: %w", err)
	}

	log.Info("\n🎉 Setup complete! Next steps:")
	log.Info("- Review your nextdeploy.yml configuration")
	log.Info("- Run 'nextdeploy prepare' to prepare a target server")

	return nil
}

// promptDomain asks for the app's custom domain and how its DNS is managed.
// Returns an empty DomainConfig if the user skips (the deploy then uses the
// platform default host). Provider/DNS drive later DNS-setup guidance: a
// Cloudflare-owned zone can be configured via the API, while namecheap/other
// or dns:manual print the records to add at the registrar.
func promptDomain() (config.DomainConfig, error) {
	var name string
	if err := survey.AskOne(&survey.Input{
		Message: "Custom domain for this app (blank = use the platform default):",
		Help:    "e.g. app.example.com — you can add or change this later in nextdeploy.yml.",
	}, &name); err != nil {
		return config.DomainConfig{}, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return config.DomainConfig{}, nil
	}

	var provider string
	if err := survey.AskOne(&survey.Select{
		Message: "Where is this domain registered / managed?",
		Options: []string{"Cloudflare", "Namecheap", "Other registrar"},
		Default: "Other registrar",
	}, &provider); err != nil {
		return config.DomainConfig{}, err
	}

	dc := config.DomainConfig{Name: name, DNS: "manual"}
	switch provider {
	case "Cloudflare":
		dc.Provider = "cloudflare"
		dc.Zone = apexZone(name)
		var auto bool
		if err := survey.AskOne(&survey.Confirm{
			Message: "Configure DNS automatically via the Cloudflare API?",
			Default: true,
		}, &auto); err != nil {
			return config.DomainConfig{}, err
		}
		if auto {
			dc.DNS = "auto"
		}
	case "Namecheap":
		dc.Provider = "namecheap"
	default:
		dc.Provider = "other"
	}
	return dc, nil
}

// renderDomainYAML renders a DomainConfig as the `domain:` value for the
// generated nextdeploy.yml, matching the template's 2-space `app:` indentation.
// A name-only domain stays a compact scalar; provider/dns/zone expand to a block.
func renderDomainYAML(dc config.DomainConfig) string {
	if dc.Name == "" {
		return `  domain: "" # Public domain for your app (blank uses the platform default)`
	}
	if dc.Provider == "" && dc.DNS == "" && dc.Zone == "" {
		return fmt.Sprintf("  domain: %s # Public domain for your app", dc.Name)
	}
	var b strings.Builder
	b.WriteString("  domain:\n")
	b.WriteString(fmt.Sprintf("    name: %s\n", dc.Name))
	if dc.Provider != "" {
		b.WriteString(fmt.Sprintf("    provider: %s # namecheap | cloudflare | other\n", dc.Provider))
	}
	if dc.DNS != "" {
		b.WriteString(fmt.Sprintf("    dns: %s # auto (provider API) | manual (print records)\n", dc.DNS))
	}
	if dc.Zone != "" {
		b.WriteString(fmt.Sprintf("    zone: %s", dc.Zone))
	}
	return strings.TrimRight(b.String(), "\n")
}

// apexZone reduces a hostname to its registrable apex using a simple
// last-two-labels heuristic (app.example.com -> example.com). Multi-part TLDs
// (example.co.uk) won't be perfect; the user can adjust `zone` in nextdeploy.yml.
func apexZone(host string) string {
	host = strings.TrimSuffix(host, ".")
	labels := strings.Split(host, ".")
	if len(labels) <= 2 {
		return host
	}
	return strings.Join(labels[len(labels)-2:], ".")
}

// runCloudflareInit drives the Cloudflare-specific init: scaffold a new
// opinionated fullstack template, or use the current directory's app (sniffing
// its infra to prefill nextdeploy.yml).
func runCloudflareInit(log *shared.Logger, appName string) error {
	var choice string
	if err := survey.AskOne(&survey.Select{
		Message: "Cloudflare deployment — how do you want to start?",
		Options: []string{
			"Scaffold deployment infra (nextdeploy.yml + proxy.ts + bindings + CI)",
			"Use my existing app in this directory",
		},
	}, &choice); err != nil {
		return fmt.Errorf("prompt failed: %w", err)
	}

	if strings.Contains(choice, "Scaffold") {
		return runScaffold(log, appName)
	}
	return runSniffExisting(log, appName)
}

// runScaffold writes the opinionated fullstack starter into the cwd.
func runScaffold(log *shared.Logger, appName string) error {
	var dbChoice string
	if err := survey.AskOne(&survey.Select{
		Message: "Which database?",
		Options: []string{
			"Cloudflare D1 (native SQLite)",
			"Bring my own Postgres/MySQL (via Hyperdrive)",
		},
	}, &dbChoice); err != nil {
		return fmt.Errorf("prompt failed: %w", err)
	}
	variant := scaffold.DBD1
	if strings.Contains(dbChoice, "Bring my own") {
		variant = scaffold.DBBYO
	}

	written, skipped, err := scaffold.Scaffold(scaffold.Options{
		AppName: appName, DBVariant: variant, Dir: ".",
	})
	if err != nil {
		return fmt.Errorf("scaffold: %w", err)
	}

	log.Info("\n🎉 Scaffolded Cloudflare deployment infra (conventions by aynaash/Hersi).")
	log.Info("  Wrote %d files (db=%s). This is a deploy pipeline, not your app — build that yourself.", len(written), variant)
	for _, s := range skipped {
		log.Warn("  Kept existing file (not overwritten): %s", s)
	}
	log.Info("\nNext steps:")
	log.Info("  1. npm install  &  build your app in app/")
	log.Info("  2. nextdeploy secrets set AUTH_SECRET \"$(openssl rand -hex 32)\"")
	log.Info("  3. nextdeploy apply  (provision D1/KV/R2/Hyperdrive from nextdeploy.yml)")
	log.Info("  4. nextdeploy ship")
	return nil
}

// runSniffExisting scans the cwd app and prefills nextdeploy.yml.
func runSniffExisting(log *shared.Logger, appName string) error {
	res, err := infrasniff.Sniff(".")
	if err != nil {
		return fmt.Errorf("sniff infra: %w", err)
	}

	log.Info("\n🔎 %s", res.Summary())

	configContent := res.RenderNextDeployYAML(appName)
	if _, statErr := os.Stat("nextdeploy.yml"); statErr == nil {
		log.Warn("nextdeploy.yml already exists — writing suggestion to nextdeploy.suggested.yml instead.")
		if err := os.WriteFile("nextdeploy.suggested.yml", []byte(configContent), 0600); err != nil {
			return fmt.Errorf("failed to save suggestion: %w", err)
		}
	} else if err := os.WriteFile("nextdeploy.yml", []byte(configContent), 0600); err != nil {
		return fmt.Errorf("failed to save configuration: %w", err)
	}

	if secrets := res.SecretsChecklist(); len(secrets) > 0 {
		log.Info("\nSet these server secrets before shipping:")
		for _, s := range secrets {
			log.Info("  nextdeploy secrets set %s <value>", s)
		}
	}
	log.Info("\n🎉 Setup complete! Review nextdeploy.yml, then run 'nextdeploy ship'.")
	return nil
}
