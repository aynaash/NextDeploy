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

	// Use the raw template to preserve comments. The domain is configured in the
	// generated nextdeploy.yml (bare hostname or a provider/DNS block), not via a
	// prompt — see the `domain:` field and its comment in the template.
	configContent := config.GetSampleConfigTemplate(targetType)
	configContent = strings.ReplaceAll(configContent, "name: example-app", "name: "+appName)

	if err := os.WriteFile("nextdeploy.yml", []byte(configContent), 0600); err != nil {
		return fmt.Errorf("failed to save configuration: %w", err)
	}

	log.Info("\n🎉 Setup complete! Next steps:")
	log.Info("- Review your nextdeploy.yml configuration")
	log.Info("- Run 'nextdeploy prepare' to prepare a target server")

	return nil
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

	// Domain is configured in the generated nextdeploy.yml (see the commented
	// `domain:` block under `app:`), not prompted on the CLI.
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
