package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
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

	"github.com/spf13/cobra"
)

var shipVerbose bool

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

		cfg, err := config.Load()
		if err != nil {
			log.Error("Failed to load config: %v", err)
			os.Exit(1)
		}

		if git.IsDirty() {
			log.Warn(" Git directory is dirty (uncommitted changes).")
			log.Warn("   Commit before shipping for cleaner deployment provenance.")
		}

		result, err := buildflow.Run(context.Background(), buildflow.Opts{
			ProjectDir: ".",
			Cfg:        cfg,
			Force:      false,
			Log:        log,
		})
		if err != nil {
			log.Error("Build flow failed: %v", err)
			os.Exit(1)
		}

		if result.EffectiveTarget == "serverless" {
			shipServerless(log, cfg, &result.Payload)
			return
		}
		shipVPS(log, cfg, result)
	},
}

func shipServerless(log *shared.Logger, cfg *config.NextDeployConfig, meta *nextcore.NextCorePayload) {
	log.Info("Deployment Target: SERVERLESS (provider=%s)", cfg.Serverless.Provider)
	if cfg.Serverless == nil {
		log.Error("Inferred 'serverless' target but 'serverless' config block is missing.")
		os.Exit(1)
	}
	if err := serverless.Deploy(context.Background(), cfg, meta, shipVerbose); err != nil {
		log.Error("Serverless deployment failed: %v", err)
		os.Exit(1)
	}
}

func shipVPS(log *shared.Logger, cfg *config.NextDeployConfig, result *buildflow.Result) {
	log.Info("Deployment Target: VPS (Traditional Server)")
	meta := &result.Payload

	if meta.AppName != "" {
		domain := meta.Domain
		if domain == "" {
			domain = cfg.App.Domain
		}
		if domain != "" {
			caddyPlan := caddy.GenerateCaddyfile(meta.AppName, domain, string(meta.OutputMode), meta.Config.Port, "/opt/nextdeploy/apps/"+meta.AppName+"/current", meta.DetectedFeatures, meta.DistDir, meta.ExportDir)
			log.Info("  Caddy Configuration Plan Preview:")
			for _, line := range strings.Split(caddyPlan, "\n") {
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

	if cfg.App.Domain != "" {
		if err := dns.GenerateVPSGuide(cfg.App.Domain, deploymentServer); err != nil {
			log.Warn("Failed to generate DNS guide: %v", err)
		} else {
			log.Info("   DNS Guide Generated: dns.md (Point %s to %s)", cfg.App.Domain, deploymentServer)
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

	remotePath := fmt.Sprintf("/opt/nextdeploy/uploads/nextdeploy_%s_%d.tar.gz", cfg.App.Name, time.Now().Unix())
	log.Info("Uploading %s to %s on %s...", tarballName, remotePath, deploymentServer)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	if err := srv.UploadFile(ctx, deploymentServer, tarballName, remotePath); err != nil {
		log.Error("Failed to upload tarball: %v", err)
		os.Exit(1)
	}

	log.Info("Upload complete. Triggering daemon to process deployment...")

	daemonCmd := fmt.Sprintf("sudo /usr/local/bin/nextdeployd ship --tarball=\"%s\" --socket-path=/run/nextdeployd/nextdeployd.sock", remotePath)
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
		ServerIP:       deploymentServer,
		CustomDomain:   cfg.App.Domain,
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
		log.Info("│  🚨  DNS ACTION REQUIRED: Point your domain to %s     ", deploymentServer)
		log.Info("│     Open this report for the full DNS setup strategy.      │")
		log.Info("└────────────────────────────────────────────────────────────┘")
	} else {
		log.Warn("Failed to generate visual report: %v", err)
	}
}

func init() {
	shipCmd.Flags().BoolVarP(&shipVerbose, "verbose", "v", false, "Print detailed deployment logs (S3 uploads, Lambda steps, CloudFront status)")
	rootCmd.AddCommand(shipCmd)
}
