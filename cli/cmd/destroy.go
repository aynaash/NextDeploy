package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/aynaash/nextdeploy/cli/internal/server"
	"github.com/aynaash/nextdeploy/cli/internal/serverless"
	"github.com/aynaash/nextdeploy/shared"
	"github.com/aynaash/nextdeploy/shared/config"
	"github.com/aynaash/nextdeploy/shared/nextcore"
	"github.com/spf13/cobra"
)

var (
	destroyForce bool
	destroyYes   bool
)

func destroyBlocked(protected, force bool) (bool, string) {
	if protected && !force {
		return true, "deletion_protection is enabled for this app — refusing to destroy. " +
			"Re-run with --force to override (this can delete the R2 bucket and its data)."
	}
	return false, ""
}

var destroyCmd = &cobra.Command{
	Use:   "destroy",
	Short: "Remove the application and its provisioned resources",
	Long: "Decommissions the deployment. For VPS this removes the app files/services; " +
		"for serverless it deletes the Worker and its R2 bucket. This is destructive and " +
		"can erase data — guarded by deletion_protection and an interactive confirmation.",
	Run: func(cmd *cobra.Command, args []string) {
		log := shared.PackageLogger("destroy", " DESTROY")
		log.Info("Starting NextDeploy destruction process...")

		cfg, err := config.Load()
		if err != nil {
			log.Error("Failed to load config: %v", err)
			os.Exit(1)
		}

		if blocked, reason := destroyBlocked(cfg.App.DeletionProtection, destroyForce); blocked {
			log.Error("%s", reason)
			os.Exit(1)
		}
		if !destroyYes {
			log.Warn("This will permanently destroy app %q and its resources (services, Caddy config, or the Worker + R2 bucket).", cfg.App.Name)
			fmt.Printf("Type the app name %q to confirm destruction: ", cfg.App.Name)
			if !confirmExact(cfg.App.Name) {
				log.Info("Aborted — name did not match; nothing was destroyed.")
				return
			}
		}

		var meta nextcore.NextCorePayload
		metadataBytes, err := os.ReadFile(".nextdeploy/metadata.json")
		if err != nil {
			log.Warn("No deployment metadata found (did you deploy yet?). destruction may be incomplete.")
		} else {
			_ = json.Unmarshal(metadataBytes, &meta)
		}

		appName := cfg.App.Name
		if meta.AppName != "" {
			appName = meta.AppName
		}

		switch cfg.TargetType {
		case "serverless":
			log.Info("Targeting SERVERLESS for destruction...")

			if cfg.Serverless == nil {
				log.Error("TargetType is 'serverless' but 'serverless' config block is missing.")
				os.Exit(1)
			}

			var p serverless.Provider
			switch cfg.Serverless.Provider {
			case "aws":
				p = serverless.NewAWSProvider(false)
			case "cloudflare":
				p = serverless.NewCloudflareProvider()
			default:
				log.Error("Unsupported serverless provider: %s", cfg.Serverless.Provider)
				os.Exit(1)
			}

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
			defer cancel()

			if err := p.Initialize(ctx, cfg); err != nil {
				log.Error("Failed to initialize serverless provider: %v", err)
				os.Exit(1)
			}

			if err := p.Destroy(ctx, cfg); err != nil {
				log.Error("Serverless destruction failed: %v", err)
				os.Exit(1)
			}

			log.Info(" Serverless resources successfully destroyed.")

		case "vps":
			if len(cfg.Servers) == 0 {
				log.Error("TargetType is 'vps' but no servers are defined in config.")
				os.Exit(1)
			}

			log.Info("Targeting VPS for destruction of app: %s", appName)

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

			log.Info("Connecting to %s...", deploymentServer)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()

			// 4. Trigger daemon to destroy app
			log.Info("Triggering daemon to destroy app: %s...", appName)
			destroyCmd := fmt.Sprintf("sudo /usr/local/bin/nextdeployd destroy --appName=%s --socket-path=/run/nextdeployd/nextdeployd.sock", shellQuote(appName))
			output, err := srv.ExecuteCommand(ctx, deploymentServer, destroyCmd, os.Stdout)
			if err != nil {
				log.Error("Failed to destroy app via daemon: %v\nOutput: %s", err, output)
				os.Exit(1)
			}

			log.Info("App resources successfully destroyed by daemon.")

		default:
			log.Error("Unknown or unsupported target_type: %s", cfg.TargetType)
			log.Info("Supported types are: vps, serverless")
			os.Exit(1)
		}

		log.Info("\n App and all associated resources (services, Caddy configs, and files) have been decommissioned.")
		log.Info("Note: Manual DNS records or project-specific external integrations may still require manual cleanup.")
	},
}

func confirmExact(expected string) bool {
	var answer string
	if _, err := fmt.Scanln(&answer); err != nil {
		return false
	}
	return answer == expected
}

func init() {
	destroyCmd.Flags().BoolVar(&destroyForce, "force", false, "Override deletion_protection")
	destroyCmd.Flags().BoolVar(&destroyYes, "yes", false, "Skip the interactive confirmation (non-interactive)")
	rootCmd.AddCommand(destroyCmd)
}
