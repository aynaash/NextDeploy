package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/aynaash/nextdeploy/cli/internal/buildflow"
	"github.com/aynaash/nextdeploy/cli/internal/dns"
	"github.com/aynaash/nextdeploy/cli/internal/server"
	"github.com/aynaash/nextdeploy/cli/internal/serverless"
	"github.com/aynaash/nextdeploy/shared"
	"github.com/aynaash/nextdeploy/shared/caddy"
	"github.com/aynaash/nextdeploy/shared/config"
	"github.com/aynaash/nextdeploy/shared/git"
	"github.com/aynaash/nextdeploy/shared/nextcore"
	"github.com/aynaash/nextdeploy/shared/telemetry"

	"github.com/spf13/cobra"
)

var (
	shipVerbose     bool
	shipNoProvision bool
	shipVerify      bool
	shipForce       bool
	shipEnvironment string
)

var shipCmd = &cobra.Command{
	Use:     "ship",
	Aliases: []string{"deploy"},
	Short:   "Build and deploy: validates, runs `next build`, and ships to the configured target",
	Long: "Ships the deployment artifact to the target configured in nextdeploy.yml. " +
		"Replaces the prior `nextdeploy build && nextdeploy ship` two-step — ship now " +
		"runs the build flow itself (target-aware: Cloudflare forces --webpack, AWS / VPS " +
		"use vanilla `next build`).",
	Run: func(cmd *cobra.Command, args []string) {
		log := shared.PackageLogger("ship", "🚀 SHIP")
		log.Info("Starting NextDeploy ship process...")

		// Signal-aware context so Ctrl+C / SIGTERM cancels the build + deploy
		// cleanly (the ctx is threaded through every provider call) rather than
		// killing the process mid-upload and leaving half-applied state.
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()

		cfg, err := config.Load()
		if err != nil {
			log.Error("Failed to load config: %v", err)
			os.Exit(1)
		}

		// Environment override (preview / staging stacks). Applied before any
		// provider is initialized so every derived resource name is namespaced.
		if err := applyEnvironmentOverride(cfg, shipEnvironment); err != nil {
			log.Error("%v", err)
			os.Exit(1)
		}
		if shipEnvironment != "" {
			log.Info("Deploying to environment: %s", cfg.App.Environment)
		}

		if git.IsDirty() {
			log.Warn(" Git directory is dirty (uncommitted changes).")
			log.Warn("   Commit before shipping for cleaner deployment provenance.")
		}

		result, err := buildflow.Run(ctx, buildflow.Opts{
			ProjectDir: ".",
			Cfg:        cfg,
			Force:      shipForce,
			Log:        log,
		})
		if err != nil {
			log.Error("Build flow failed: %v", err)
			os.Exit(1)
		}

		if result.EffectiveTarget == "serverless" {
			shipServerless(ctx, log, cfg, &result.Payload)
			// Reached only on success — shipServerless exits the process on failure.
			telemetry.RecordShipSuccess(cfg.Serverless.Provider, shared.Version)
			return
		}
		shipVPS(log, cfg, result)
		telemetry.RecordShipSuccess("vps", shared.Version)
	},
}

func shipServerless(ctx context.Context, log *shared.Logger, cfg *config.NextDeployConfig, meta *nextcore.NextCorePayload) {
	log.Info("Deployment Target: SERVERLESS (provider=%s)", cfg.Serverless.Provider)
	if cfg.Serverless == nil {
		log.Error("Inferred 'serverless' target but 'serverless' config block is missing.")
		os.Exit(1)
	}
	if err := serverless.Deploy(ctx, cfg, meta, shipVerbose, !shipNoProvision, shipVerify); err != nil {
		if ctx.Err() != nil {
			log.Warn("Deploy interrupted — state may be partial. Re-run `nextdeploy ship` " +
				"to converge (steps are idempotent).")
		}
		log.Error("Serverless deployment failed: %v", err)
		os.Exit(1)
	}

	// Emit the Worker script name on stdout in a stable, greppable form so CI
	// can compose the deployment URL (https://<worker>.<subdomain>.workers.dev)
	// without the CLI having to call the Workers Subdomains API — that would
	// add SDK surface and another token scope to print something the account
	// already knows. Cloudflare-only; other providers name resources
	// differently.
	if strings.EqualFold(cfg.Serverless.Provider, "cloudflare") {
		fmt.Printf("NEXTDEPLOY_WORKER_NAME=%s\n",
			serverless.CloudflareWorkerName(cfg.App.Name, cfg.App.Environment))
	}
}

func shipVPS(log *shared.Logger, cfg *config.NextDeployConfig, result *buildflow.Result) {
	log.Info("Deployment Target: VPS (Traditional Server)")
	meta := &result.Payload

	if meta.AppName != "" {
		domain := meta.Domain
		if domain == "" {
			domain = cfg.App.Domain.Name
		}
		if domain != "" {
			caddyPlan := caddy.GenerateCaddyfile(meta.AppName, domain, string(meta.OutputMode), meta.Config.Port, "/opt/nextdeploy/apps/"+meta.AppName+"/current", meta.DetectedFeatures, meta.DistDir, meta.ExportDir)
			log.Info("  Caddy Configuration Plan Preview:")
			for line := range strings.SplitSeq(caddyPlan, "\n") {
				if strings.TrimSpace(line) != "" {
					log.Info("  %s", line)
				}
			}
		}
	}

	srv, err := server.New(server.WithConfig(), server.WithSSH())
	if err != nil {
		log.Error("Failed to initialize server connection: %v", err)
		os.Exit(1)
	}
	defer srv.CloseSSHConnection()

	deploymentServer, err := srv.GetDeploymentServer()
	if err != nil {
		log.Error("Failed to get deployment server: %v", err)
		os.Exit(1)
	}
	log.Info("Deployment server: %s", deploymentServer)

	// deploymentServer is the config NAME (the SSH client map key). The DNS guide
	// and the report tell the operator where to point an A record, so they need
	// the actual address — a record pointing at "production" is not a record.
	serverHost, err := srv.ServerHost(deploymentServer)
	if err != nil {
		log.Error("Could not resolve the address of server %q: %v", deploymentServer, err)
		os.Exit(1)
	}

	if cfg.App.Domain.Name != "" {
		if err := dns.GenerateVPSGuide(cfg.App.Domain.Name, serverHost); err != nil {
			log.Warn("Failed to generate DNS guide: %v", err)
		} else {
			log.Info("   DNS Guide Generated: dns.md (Point %s to %s)", cfg.App.Domain.Name, serverHost)
		}
	}

	tarballName := result.TarballPath
	if tarballName == "" {
		tarballName = "app.tar.gz"
	}
	if _, err := os.Stat(tarballName); os.IsNotExist(err) {
		log.Error("Deployment artifact %s not found. Run `nextdeploy build` to produce it (or remove --skip-build flags upstream).", tarballName)
		os.Exit(1)
	}

	// Echo the effective inputs before acting on them. Half of "silent" bugs
	// are the tool never saying what it was about to do — a reused cached build
	// on a dirty tree is exactly the state where config and artifact diverge.
	log.Info("┌─ SHIP ────────────────────────────────────────────────")
	log.Info("│  app.name : %s", cfg.App.Name)
	log.Info("│  domain   : %s", cfg.App.Domain.Name)
	log.Info("│  target   : %s", result.EffectiveTarget)
	log.Info("│  server   : %s (%s)", deploymentServer, serverHost)
	log.Info("│  artifact : %s", tarballName)
	if result.Skipped {
		log.Warn("│  build    : REUSED CACHED BUILD (no `next build` this run)")
		if git.IsDirty() {
			log.Warn("│             working tree is DIRTY — pass --force to rebuild if")
			log.Warn("│             the cached compile predates your latest source edit.")
		}
	} else {
		log.Info("│  build    : fresh")
	}
	log.Info("└───────────────────────────────────────────────────────")

	// Cheap invariant, checked before a multi-MB upload: the artifact's own
	// metadata.json is what the daemon validates, so if it disagrees with the
	// config we are shipping something stale. With the config fingerprint in
	// the build lock this should never trip — which is exactly what makes it a
	// good assertion: it catches a future regression of that mechanism.
	if artifactName, err := tarballAppName(tarballName); err == nil {
		if artifactName != cfg.App.Name {
			log.Error("Artifact app name %q does not match config app.name %q — the build is stale.\n"+
				"  Run `nextdeploy ship --force` to rebuild and repackage.", artifactName, cfg.App.Name)
			os.Exit(1)
		}
	} else if !errors.Is(err, errNoMetadata) {
		log.Warn("Could not read app name from %s (%v) — skipping the pre-upload staleness check.", tarballName, err)
	}

	remotePath := fmt.Sprintf("/opt/nextdeploy/uploads/nextdeploy_%s_%d.tar.gz", cfg.App.Name, time.Now().Unix())
	log.Info("Uploading %s to %s on %s...", tarballName, remotePath, deploymentServer)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	if err := srv.UploadFile(ctx, deploymentServer, tarballName, remotePath); err != nil {
		log.Error("Failed to upload tarball: %v", err)
		os.Exit(1)
	}

	log.Info("Upload complete. Triggering daemon to process deployment...")

	daemonCmd := fmt.Sprintf("sudo /usr/local/bin/nextdeployd ship --tarball=%s --socket-path=/run/nextdeployd/nextdeployd.sock", shellQuote(remotePath))
	output, err := srv.ExecuteCommand(ctx, deploymentServer, daemonCmd, os.Stdout)
	if err != nil {
		log.Error("Failed to trigger daemon (ensure nextdeployd is in PATH): %v\nOutput: %s", err, output)
		os.Exit(1)
	}

	log.Info("Ship successful! Deployment instructions relayed to the daemon.")

	port := meta.Config.Port
	if port == 0 {
		port = cfg.App.Port
	}
	if port == 0 {
		port = 3000
	}

	dnsProvider := "other"
	if cfg.SSLConfig != nil {
		dnsProvider = cfg.SSLConfig.DNSProvider
	} else if cfg.SSL != nil {
		dnsProvider = cfg.SSL.DNSProvider
	}

	resMap := server.VPSResourceMap{
		AppName:        cfg.App.Name,
		Environment:    "production",
		ServerIP:       serverHost,
		CustomDomain:   cfg.App.Domain.Name,
		Port:           port,
		DeploymentTime: time.Now(),
		DNSProvider:    dnsProvider,
	}

	reportPath, err := server.GenerateVPSResourceView(&cfg.App, resMap)
	if err == nil {
		log.Info("┌────────────────────────────────────────────────────────────┐")
		log.Success("│  ✨  VPS DEPLOYMENT REPORT READY                           │")
		log.Info("├────────────────────────────────────────────────────────────┤")
		log.Info("│  Location: %s", reportPath)
		log.Info("│                                                            │")
		log.Info("│  🚨  DNS ACTION REQUIRED: Point your domain to %s     ", serverHost)
		log.Info("│     Open this report for the full DNS setup strategy.      │")
		log.Info("└────────────────────────────────────────────────────────────┘")
	} else {
		log.Warn("Failed to generate visual report: %v", err)
	}
}

func init() {
	shipCmd.Flags().BoolVarP(&shipVerbose, "verbose", "v", false, "Print detailed deployment logs (S3 uploads, Lambda steps, CloudFront status)")
	shipCmd.Flags().BoolVar(&shipNoProvision, "no-provision", false, "Skip reconciling declared Cloudflare resources (KV/Hyperdrive/D1) before deploying")
	shipCmd.Flags().BoolVar(&shipVerify, "verify", false, "Fail the deploy if the post-deploy smoke check does not pass (for CI)")
	// Every cache needs a manual invalidation escape hatch for the moment the
	// automatic one is wrong. `build` had --force; `ship` — the command people
	// actually run — did not, so working around a stale artifact meant dropping
	// to a sibling command or deleting lock files by hand.
	shipCmd.Flags().BoolVarP(&shipForce, "force", "f", false,
		"Force a full rebuild even if the incremental state matches")
	shipCmd.Flags().StringVarP(&shipEnvironment, "environment", "e", "",
		"Deploy to a named environment (e.g. pr-42, staging). Overrides app.environment and "+
			"isolates the Worker, R2 bucket and resources under <app>-<environment>")
	rootCmd.AddCommand(shipCmd)
}
