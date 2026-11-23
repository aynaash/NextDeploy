// Package initialcommand process confiuration variables needed to setup the project
package initialcommand

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/aynaash/nextdeploy/cli/internal/infrasniff"
	"github.com/aynaash/nextdeploy/cli/internal/scaffold"
	"github.com/aynaash/nextdeploy/shared"
	"github.com/aynaash/nextdeploy/shared/config"
	"github.com/aynaash/nextdeploy/shared/nextcore"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
)

// RunInitCommand analyses the current working directory and initializes a
// NextDeploy configuration or scaffolds a new fullstack Next.js project.
func RunInitCommand(cmd *cobra.Command, _ []string) error {
	log := shared.PackageLogger("Initialization", "Initialization")

	log.Info("NextDeploy Initialization")
	log.Info("----------------------------------------")
	log.Info("Analysing project directory...")

	flags, err := parseInitFlags(cmd)
	if err != nil {
		return err
	}

	cwd, _ := os.Getwd()
	appName := detectAppName(cwd)

	nextVersion, _ := nextcore.GetNextJsVersion("package.json")
	if nextVersion == "" {
		for _, sub := range []string{"apps/web", "web", "frontend", "packages/web"} {
			if v, _ := nextcore.GetNextJsVersion(filepath.Join(sub, "package.json")); v != "" {
				nextVersion = v
				log.Info("Next.js %s detected in %s", v, sub)
				break
			}
		}
	}

	hasWrangler := hasWranglerConfig(cwd)
	if hasWrangler && nextVersion == "" {
		log.Info("Cloudflare Wrangler configuration detected.")
		return runSniffExisting(log, appName)
	}

	if nextVersion == "" {
		if flags.mode != "" && strings.ToLower(strings.TrimSpace(flags.mode)) != "scaffold" {
			return fmt.Errorf("--mode=%s requires an existing Next.js project in this directory", flags.mode)
		}
		// No existing Next.js project detected in current directory.
		log.Warn("No Next.js project detected in current directory.")
		log.Info("NextDeploy will generate a complete fullstack Next.js 15 starter for you.\n")
		return runNewProjectInit(log, appName, flags.target, flags.db)
	}

	log.Info("Next.js %s detected", nextVersion)

	pkgManager, _ := nextcore.DetectPackageManager(cwd)
	if pkgManager != nextcore.Unknown {
		log.Info("Package manager: %s\n", pkgManager.String())
	}

	log.Info("\nNextDeploy understands your application.\n")

	targetType, err := selectExistingProjectTarget(log, flags.target)
	if err != nil {
		return err
	}

	// Cloudflare has two paths: scaffold a fresh fullstack app, or sniff the
	// existing app in this directory and prefill its bindings.
	if targetType == "cloudflare" {
		return runCloudflareInit(log, appName, flags.mode, flags.db, nextVersion)
	}

	return writeTemplateConfig(log, targetType, appName, flags.target == "")
}

type initFlags struct {
	target string
	mode   string
	db     string
}

func parseInitFlags(cmd *cobra.Command) (initFlags, error) {
	flagTarget, err := cmd.Flags().GetString("target")
	if err != nil {
		return initFlags{}, fmt.Errorf("read --target: %w", err)
	}
	if flagTarget == "" {
		if p, platformErr := cmd.Flags().GetString("platform"); platformErr == nil && p != "" {
			flagTarget = p
		}
	}
	flagMode, err := cmd.Flags().GetString("mode")
	if err != nil {
		return initFlags{}, fmt.Errorf("read --mode: %w", err)
	}
	flagDB, err := cmd.Flags().GetString("db")
	if err != nil {
		return initFlags{}, fmt.Errorf("read --db: %w", err)
	}

	if flagMode != "" || flagDB != "" {
		if flagTarget == "" {
			return initFlags{}, fmt.Errorf("--mode and --db require --target=cloudflare")
		}
		if normalizeInitTarget(flagTarget) != "cloudflare" {
			return initFlags{}, fmt.Errorf("--mode and --db are only valid with --target=cloudflare")
		}
	}
	if flagMode != "" {
		switch strings.ToLower(strings.TrimSpace(flagMode)) {
		case "scaffold", "sniff", "existing":
		default:
			return initFlags{}, fmt.Errorf("--mode must be one of: scaffold, sniff (got %q)", flagMode)
		}
	}
	if flagDB != "" {
		switch strings.ToLower(strings.TrimSpace(flagDB)) {
		case "d1", "byo", "none":
		default:
			return initFlags{}, fmt.Errorf("--db must be one of: d1, byo, none (got %q)", flagDB)
		}
	}

	return initFlags{target: flagTarget, mode: flagMode, db: flagDB}, nil
}

func detectAppName(cwd string) string {
	if data, err := os.ReadFile("package.json"); err == nil {
		var pkg struct {
			Name string `json:"name"`
		}
		if json.Unmarshal(data, &pkg) == nil && pkg.Name != "" {
			return config.SlugifyAppName(pkg.Name)
		}
	}
	if cwd != "" {
		base := filepath.Base(cwd)
		if base != "" && base != "." && base != "/" {
			return config.SlugifyAppName(base)
		}
	}
	return "my-app"
}

func hasWranglerConfig(dir string) bool {
	checkDirs := []string{dir, filepath.Join(dir, "apps", "web"), filepath.Join(dir, "web"), filepath.Join(dir, "frontend")}
	for _, d := range checkDirs {
		for _, name := range []string{"wrangler.jsonc", "wrangler.json", "wrangler.toml"} {
			if info, err := os.Stat(filepath.Join(d, name)); err == nil && !info.IsDir() {
				return true
			}
		}
	}
	return false
}

func selectExistingProjectTarget(log *shared.Logger, flagTarget string) (string, error) {
	if flagTarget != "" {
		targetType := normalizeInitTarget(flagTarget)
		if targetType == "" {
			return "", fmt.Errorf("--target must be one of: vps, aws, cloudflare (got %q)", flagTarget)
		}
		log.Info("Target: %s (from --target flag)", targetType)
		return targetType, nil
	}

	if hasWranglerConfig(".") {
		log.Info("Target: cloudflare (automatically detected from wrangler config)")
		return "cloudflare", nil
	}

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
		return "", fmt.Errorf("prompt failed: %w", err)
	}
	switch {
	case strings.Contains(targetChoice, "Cloudflare"):
		return "cloudflare", nil
	case strings.Contains(targetChoice, "Serverless"):
		return "serverless", nil
	default:
		return "vps", nil
	}
}

func writeTemplateConfig(log *shared.Logger, targetType, appName string, interactive bool) error {
	enableGoSidecar := false
	if targetType == "vps" && interactive {
		if err := survey.AskOne(&survey.Confirm{
			Message: "Enable the Go sidecar for eligible prerendered pages and image requests?",
			Default: false,
		}, &enableGoSidecar); err != nil {
			return fmt.Errorf("prompt failed: %w", err)
		}
	}

	// Use the raw template to preserve comments. The domain is configured in the
	// generated nextdeploy.yml (bare hostname or a provider/DNS block), not via a
	// prompt — see the `domain:` field and its comment in the template.
	configContent := config.GetSampleConfigTemplate(targetType)
	configContent = strings.ReplaceAll(configContent, "name: example-app", "name: "+appName)
	if enableGoSidecar {
		var err error
		configContent, err = setGoSidecarEnabled(configContent)
		if err != nil {
			return err
		}
	}

	if err := os.WriteFile("nextdeploy.yml", []byte(configContent), 0o600); err != nil {
		return fmt.Errorf("failed to save configuration: %w", err)
	}

	log.Info("\n🎉 Setup complete! Next steps:")
	log.Info("- Review your nextdeploy.yml configuration")
	log.Info("- Run 'nextdeploy prepare' to prepare a target server")

	return nil
}

func runNewProjectInit(log *shared.Logger, appName, flagTarget, flagDB string) error {
	targetType, err := selectNewProjectTarget(log, flagTarget)
	if err != nil {
		return err
	}

	switch targetType {
	case "cloudflare":
		return runScaffold(log, appName, flagDB, false, "")
	case "vps":
		return scaffoldNewVPS(log, appName, flagTarget == "")
	case "serverless", "aws":
		return scaffoldNewAWS(log, appName)
	default:
		return fmt.Errorf("unsupported platform %q", targetType)
	}
}

func selectNewProjectTarget(log *shared.Logger, flagTarget string) (string, error) {
	if flagTarget != "" {
		targetType := normalizeInitTarget(flagTarget)
		if targetType == "" {
			return "", fmt.Errorf("--target/--platform must be one of: vps, aws, cloudflare (got %q)", flagTarget)
		}
		log.Info("Platform: %s (from flag)", targetType)
		return targetType, nil
	}

	prompt := &survey.Select{
		Message: "Which platform would you like to build for?",
		Options: []string{
			"Cloudflare Workers (Edge Serverless + D1 SQLite / Hyperdrive + R2)",
			"VPS / Self-Hosted (Fullstack Node.js + Go Sidecar + Caddy)",
			"AWS Serverless (Lambda + CloudFront + S3)",
		},
	}
	var targetChoice string
	if err := survey.AskOne(prompt, &targetChoice); err != nil {
		return "", fmt.Errorf("prompt failed: %w", err)
	}
	switch {
	case strings.Contains(targetChoice, "Cloudflare"):
		return "cloudflare", nil
	case strings.Contains(targetChoice, "VPS"):
		return "vps", nil
	default:
		return "serverless", nil
	}
}

func scaffoldNewVPS(log *shared.Logger, appName string, interactive bool) error {
	enableGoSidecar := true
	if interactive {
		if err := survey.AskOne(&survey.Confirm{
			Message: "Enable the Go sidecar for sub-millisecond static page and asset caching?",
			Default: true,
		}, &enableGoSidecar); err != nil {
			return fmt.Errorf("prompt failed: %w", err)
		}
	}

	written, skipped, err := scaffold.Scaffold(scaffold.Options{
		AppName:         appName,
		Platform:        scaffold.PlatformVPS,
		EnableGoSidecar: enableGoSidecar,
		Dir:             ".",
	})
	if err != nil {
		return fmt.Errorf("scaffold VPS project: %w", err)
	}
	log.Info("\n🎉 Scaffolded fullstack Next.js project for VPS (conventions by aynaash/Hersi).")
	log.Info("  Wrote %d files.", len(written))
	for _, s := range skipped {
		log.Warn("  Kept existing file (not overwritten): %s", s)
	}
	log.Info("\nNext steps:")
	log.Info("  1. npm install")
	log.Info("  2. npm run dev       (run locally at http://localhost:3000)")
	log.Info("  3. nextdeploy prepare (configure your VPS target server)")
	log.Info("  4. nextdeploy ship   (build and deploy)")
	return nil
}

func scaffoldNewAWS(log *shared.Logger, appName string) error {
	written, skipped, err := scaffold.Scaffold(scaffold.Options{
		AppName:  appName,
		Platform: scaffold.PlatformAWS,
		Dir:      ".",
	})
	if err != nil {
		return fmt.Errorf("scaffold AWS project: %w", err)
	}
	log.Info("\n🎉 Scaffolded fullstack Next.js project for AWS Serverless (conventions by aynaash/Hersi).")
	log.Info("  Wrote %d files.", len(written))
	for _, s := range skipped {
		log.Warn("  Kept existing file (not overwritten): %s", s)
	}
	log.Info("\nNext steps:")
	log.Info("  1. npm install")
	log.Info("  2. npm run dev       (run locally at http://localhost:3000)")
	log.Info("  3. nextdeploy plan   (dry-run preview)")
	log.Info("  4. nextdeploy ship   (deploy to AWS Lambda & CloudFront)")
	return nil
}

func normalizeInitTarget(target string) string {
	switch strings.ToLower(strings.TrimSpace(target)) {
	case "vps":
		return "vps"
	case "aws", "serverless":
		return "serverless"
	case "cloudflare":
		return "cloudflare"
	default:
		return ""
	}
}

func setGoSidecarEnabled(configContent string) (string, error) {
	const disabled = "enabled: false # Opt in after reviewing the sidecar's supported request paths"
	if strings.Count(configContent, disabled) != 1 {
		return "", fmt.Errorf("could not find the Go sidecar setting in the VPS configuration template")
	}
	return strings.Replace(configContent, disabled, strings.Replace(disabled, "false", "true", 1), 1), nil
}

// runCloudflareInit drives the Cloudflare-specific init: scaffold a new
// opinionated fullstack template, or use the current directory's app (sniffing
// its infra to prefill nextdeploy.yml). When flagMode/flagDB are non-empty the
// interactive prompts are bypassed (non-interactive / CI use).
func runCloudflareInit(log *shared.Logger, appName, flagMode, flagDB, nextVersion string) error {
	var choice string
	if flagMode != "" {
		flagMode = strings.ToLower(strings.TrimSpace(flagMode))
		switch flagMode {
		case "scaffold":
			choice = "Scaffold deployment infra (nextdeploy.yml + middleware/proxy + bindings + CI)"
		case "sniff", "existing":
			choice = "Use my existing app in this directory"
		default:
			return fmt.Errorf("--mode must be one of: scaffold, sniff (got %q)", flagMode)
		}
		log.Info("Mode: %s (from --mode flag)", flagMode)
	} else if hasWranglerConfig(".") {
		log.Info("Mode: sniff (automatically adopting declared infrastructure from wrangler config)")
		choice = "Use my existing app in this directory"
	} else {
		if err := survey.AskOne(&survey.Select{
			Message: "Cloudflare deployment — how do you want to start?",
			Options: []string{
				"Scaffold deployment infra (nextdeploy.yml + middleware/proxy + bindings + CI)",
				"Use my existing app in this directory",
			},
		}, &choice); err != nil {
			return fmt.Errorf("prompt failed: %w", err)
		}
	}

	if strings.Contains(choice, "Scaffold") {
		return runScaffold(log, appName, flagDB, true, nextVersion)
	}
	if flagDB != "" {
		return fmt.Errorf("--db is only valid with --mode=scaffold")
	}
	return runSniffExisting(log, appName)
}

// runScaffold writes the Cloudflare fullstack starter (infraOnly=false) or
// deployment infra only (infraOnly=true, for an existing Next.js app) into cwd.
// When flagDB is non-empty the database prompt is bypassed (non-interactive / CI).
func runScaffold(log *shared.Logger, appName, flagDB string, infraOnly bool, nextVersion string) error {
	var dbChoice string
	if flagDB != "" {
		flagDB = strings.ToLower(strings.TrimSpace(flagDB))
		switch flagDB {
		case "d1":
			dbChoice = "Cloudflare D1 (native SQLite)"
		case "byo":
			dbChoice = "Bring my own Postgres/MySQL (via Hyperdrive)"
		case "none":
			dbChoice = "None (no database)"
		default:
			return fmt.Errorf("--db must be one of: d1, byo, none (got %q)", flagDB)
		}
		log.Info("Database: %s (from --db flag)", flagDB)
	} else {
		if err := survey.AskOne(&survey.Select{
			Message: "Which database?",
			Options: []string{
				"None (no database)",
				"Cloudflare D1 (native SQLite)",
				"Bring my own Postgres/MySQL (via Hyperdrive)",
			},
		}, &dbChoice); err != nil {
			return fmt.Errorf("prompt failed: %w", err)
		}
	}
	variant := scaffold.DBD1
	switch {
	case strings.Contains(dbChoice, "None"):
		variant = scaffold.DBNone
	case strings.Contains(dbChoice, "Bring my own"):
		variant = scaffold.DBBYO
	}

	written, skipped, err := scaffold.Scaffold(scaffold.Options{
		AppName:     appName,
		Platform:    scaffold.PlatformCloudflare,
		DBVariant:   variant,
		Dir:         ".",
		InfraOnly:   infraOnly,
		NextVersion: nextVersion,
	})
	if err != nil {
		return fmt.Errorf("scaffold: %w", err)
	}

	if infraOnly {
		log.Info("\n🎉 Scaffolded Cloudflare deployment infra (nextdeploy.yml + middleware/proxy + lib/env.ts + CI).")
	} else {
		log.Info("\n🎉 Scaffolded Cloudflare fullstack Next.js project (conventions by aynaash/Hersi).")
	}
	log.Info("  Wrote %d files (db=%s).", len(written), variant)
	for _, s := range skipped {
		log.Warn("  Kept existing file (not overwritten): %s", s)
	}
	log.Info("\nNext steps:")
	step := 1
	if !infraOnly {
		log.Info("  %d. npm install", step)
		step++
		log.Info("  %d. npm run dev       (run locally at http://localhost:3000)", step)
		step++
	}
	log.Info("  %d. nextdeploy secrets set AUTH_SECRET \"$(openssl rand -hex 32)\"", step)
	step++
	if variant == scaffold.DBBYO {
		log.Info("  %d. nextdeploy secrets set DATABASE_URL \"postgres://user:pass@host:5432/db\"  (or export DATABASE_URL)", step)
		step++
	}
	log.Info("  %d. nextdeploy plan   (dry-run preview of Cloudflare resources)", step)
	step++
	switch variant {
	case scaffold.DBNone:
		log.Info("  %d. nextdeploy apply  (provision KV from nextdeploy.yml)", step)
	case scaffold.DBBYO:
		log.Info("  %d. nextdeploy apply  (provision Hyperdrive/KV from nextdeploy.yml)", step)
	default:
		log.Info("  %d. nextdeploy apply  (provision D1/KV from nextdeploy.yml)", step)
	}
	step++
	log.Info("  %d. nextdeploy ship", step)
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
		// Don't clobber an existing config (it may carry hand-tuned bindings),
		// but DO add the domain block to it if it has none — that's the field
		// the CF deploy reads to attach a custom domain. The full sniff result
		// still goes to nextdeploy.suggested.yml for the rest.
		if added, err := addDomainStubToFile("nextdeploy.yml"); err != nil {
			log.Warn("Could not add a domain block to nextdeploy.yml: %v", err)
		} else if added {
			log.Info("Added app.domain to nextdeploy.yml — set a value to attach a custom domain on ship.")
		}
		log.Warn("nextdeploy.yml already exists — writing the full suggestion to nextdeploy.suggested.yml.")
		if err := os.WriteFile("nextdeploy.suggested.yml", []byte(configContent), 0o600); err != nil {
			return fmt.Errorf("failed to save suggestion: %w", err)
		}
	} else if err := os.WriteFile("nextdeploy.yml", []byte(configContent), 0o600); err != nil {
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

// domainStub is the single active `domain:` line inserted under `app:`. Active
// (not commented) and valid as-is (empty domain), so the user just fills in the
// value — no uncommenting, no indentation or duplicate-key mistakes. For a
// custom registrar/DNS mode they can expand it into a name/provider/dns/zone block.
const domainStub = "  domain: \"\" # optional custom domain, e.g. example.com — attached automatically on ship\n"

// addDomainStubToFile inserts domainStub into an existing config file when its
// app block has no domain. Returns whether it modified the file.
func addDomainStubToFile(path string) (bool, error) {
	cleanPath := filepath.Clean(path)
	data, err := os.ReadFile(cleanPath) // #nosec G304 -- path is the project's own nextdeploy.yml
	if err != nil {
		return false, err
	}
	out, added := insertDomainStub(string(data))
	if !added {
		return false, nil
	}
	if err := os.WriteFile(cleanPath, []byte(out), 0o600); err != nil { // #nosec G304,G703 -- path is the project's own nextdeploy.yml
		return false, err
	}
	return true, nil
}

// insertDomainStub adds the commented domain block to a nextdeploy.yml's app
// block, right after the `name:` line, when no domain is set. It only edits text
// (no YAML round-trip) so existing comments and formatting are preserved. It is
// a no-op (added=false) if the app block already declares a domain or has no
// recognizable `app:` / `name:` anchor.
func insertDomainStub(content string) (out string, added bool) {
	lines := strings.Split(content, "\n")
	inApp := false
	nameIdx := -1
	for i, ln := range lines {
		switch {
		case strings.HasPrefix(ln, "app:"):
			inApp = true
		case inApp && isTopLevelYAMLKey(ln):
			inApp = false
		}
		if !inApp {
			continue
		}
		if isIndentedDomainKey(ln) {
			return content, false
		}
		if nameIdx == -1 && strings.HasPrefix(ln, "  name:") {
			nameIdx = i
		}
	}
	if nameIdx == -1 {
		return content, false
	}
	stub := strings.TrimRight(domainStub, "\n")
	newLines := make([]string, 0, len(lines)+6)
	newLines = append(newLines, lines[:nameIdx+1]...)
	newLines = append(newLines, strings.Split(stub, "\n")...)
	newLines = append(newLines, lines[nameIdx+1:]...)
	return strings.Join(newLines, "\n"), true
}

func isTopLevelYAMLKey(ln string) bool {
	return ln != "" && ln[0] != ' ' && ln[0] != '\t' && ln[0] != '#'
}

func isIndentedDomainKey(ln string) bool {
	return strings.HasPrefix(ln, "  ") && strings.HasPrefix(strings.TrimSpace(ln), "domain:")
}
