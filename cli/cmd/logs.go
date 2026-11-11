package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/aynaash/nextdeploy/cli/internal/logs"
	"github.com/aynaash/nextdeploy/cli/internal/server"
	"github.com/aynaash/nextdeploy/cli/internal/serverless"
	"github.com/aynaash/nextdeploy/shared"
	"github.com/aynaash/nextdeploy/shared/config"
	"github.com/aynaash/nextdeploy/shared/sensitive"
	"github.com/spf13/cobra"
)

var logsCmd = &cobra.Command{
	Use:   "logs",
	Short: "Stream application logs natively from the provider or daemon",
	Long:  "Streams application logs natively: systemd journal on VPS, wrangler tail on Cloudflare Workers, and CloudWatch logs on AWS Lambda.",
	Run: func(cmd *cobra.Command, args []string) {
		log := shared.PackageLogger("logs", "🚀 LOGS")
		cfg, err := config.Load()
		if err != nil {
			log.Error("Failed to load config: %v", err)
			os.Exit(1)
		}

		appName := cfg.App.Name
		routeFilter, _ := cmd.Flags().GetString("route")
		showAudit, _ := cmd.Flags().GetBool("audit")
		showDaemon, _ := cmd.Flags().GetBool("daemon")
		showAll, _ := cmd.Flags().GetBool("all")

		ctx := cmd.Context()
		if ctx == nil {
			ctx = context.Background()
		}

		target := strings.ToLower(cfg.ResolveTargetType(""))
		if target == "serverless" {
			if showAudit || showDaemon {
				log.Warn("--audit and --daemon flags apply only to VPS daemon environments. Streaming app logs.")
			}
			logsServerless(ctx, log, cfg, routeFilter)
			return
		}

		logsVPS(ctx, log, cfg, appName, routeFilter, showAudit, showDaemon, showAll)
	},
}

func logsServerless(ctx context.Context, log *shared.Logger, cfg *config.NextDeployConfig, routeFilter string) {
	if cfg.Serverless == nil {
		log.Error("Target is 'serverless' but 'serverless' config block is missing.")
		os.Exit(1)
	}

	provider := "aws"
	if cfg.Serverless.Provider != "" {
		provider = strings.ToLower(cfg.Serverless.Provider)
	}

	switch provider {
	case "cloudflare":
		workerName := serverless.CloudflareWorkerName(cfg.App.Name, cfg.App.Environment)
		log.Info("Streaming Cloudflare Workers logs for %s (%s)...", cfg.App.Name, workerName)

		// Try native Go WebSocket tailing first (zero dependencies, native formatting, no Node/wrangler crashes)
		p, err := serverless.New("cloudflare", false)
		if err == nil {
			if initErr := p.Initialize(ctx, cfg); initErr == nil {
				if cfProvider, ok := p.(*serverless.CloudflareProvider); ok {
					tailErr := cfProvider.TailLogs(ctx, cfg, routeFilter, os.Stdout)
					if tailErr == nil || ctx.Err() != nil {
						return
					}
					log.Warn("Native tail stream encountered an issue: %v. Falling back to wrangler...", tailErr)
				}
			}
		}

		execPath, baseArgs, err := findWranglerCommand()
		if err != nil {
			log.Error("%v", err)
			os.Exit(1)
		}

		args := append(baseArgs, "tail", workerName)
		if routeFilter != "" {
			args = append(args, "--search", routeFilter)
		}

		cmd := exec.CommandContext(ctx, execPath, args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		cmd.Env = os.Environ()

		if err := cmd.Run(); err != nil && ctx.Err() == nil {
			log.Error("wrangler tail exited with error: %v", err)
			os.Exit(1)
		}

	case "aws":
		functionName := strings.ToLower(fmt.Sprintf("%s-%s", cfg.App.Name, cfg.App.Environment))
		logGroup := fmt.Sprintf("/aws/lambda/%s", functionName)
		log.Info("Streaming CloudWatch logs for %s (%s)...", cfg.App.Name, logGroup)

		awsPath, err := exec.LookPath("aws")
		if err != nil {
			log.Error("AWS CLI ('aws') not found on PATH. Install AWS CLI to stream CloudWatch logs: https://aws.amazon.com/cli/")
			os.Exit(1)
		}

		args := []string{"logs", "tail", logGroup, "--follow"}
		if routeFilter != "" {
			args = append(args, "--filter-pattern", routeFilter)
		}

		if cfg.Serverless.Region != "" {
			args = append(args, "--region", cfg.Serverless.Region)
		} else if cfg.CloudProvider != nil && cfg.CloudProvider.Region != "" {
			args = append(args, "--region", cfg.CloudProvider.Region)
		}

		cmd := exec.CommandContext(ctx, awsPath, args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		cmd.Env = os.Environ()

		if err := cmd.Run(); err != nil && ctx.Err() == nil {
			log.Error("aws logs tail exited with error: %v", err)
			os.Exit(1)
		}

	default:
		log.Error("Unsupported serverless provider for logs: %s", provider)
		os.Exit(1)
	}
}

func findWranglerCommand() (string, []string, error) {
	if path, err := exec.LookPath("wrangler"); err == nil {
		return path, nil, nil
	}
	if path, err := exec.LookPath("bunx"); err == nil {
		return path, []string{"wrangler"}, nil
	}
	if path, err := exec.LookPath("npx"); err == nil {
		return path, []string{"wrangler"}, nil
	}
	return "", nil, fmt.Errorf("wrangler not found. Install wrangler globally ('npm i -g wrangler' or 'bun add -g wrangler') or install Node/Bun to stream Cloudflare logs")
}

func logsVPS(ctx context.Context, log *shared.Logger, cfg *config.NextDeployConfig, appName, routeFilter string, showAudit, showDaemon, showAll bool) {
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

	agg := logs.NewAggregator(os.Stdout, appName)

	fmt.Printf("\n\033[1;36mNextDeploy Unified Log Stream: %s\033[0m\n", appName)
	fmt.Println("\033[90m──────────────────────────────────────────────────\033[0m")

	if showAll {
		var wg sync.WaitGroup
		wg.Add(3)

		// 1. App Logs
		go func() {
			defer wg.Done()
			streamAppLogs(ctx, srv, deploymentServer, appName, routeFilter, agg.GetWriter(logs.SourceApp))
		}()

		// 2. Audit Logs
		go func() {
			defer wg.Done()
			journalCmd := "tail -f -n 20 /var/log/nextdeployd/audit.log"
			_, _ = srv.ExecuteCommand(ctx, deploymentServer, "sudo "+journalCmd, agg.GetWriter(logs.SourceAudit))
		}()

		// 3. Daemon Logs
		go func() {
			defer wg.Done()
			journalCmd := "tail -f -n 20 /var/log/nextdeployd/nextdeployd.log"
			_, _ = srv.ExecuteCommand(ctx, deploymentServer, "sudo "+journalCmd, agg.GetWriter(logs.SourceDaemon))
		}()

		wg.Wait()
		return
	}

	if showAudit {
		journalCmd := "tail -f -n 50 /var/log/nextdeployd/audit.log"
		// Streaming tail runs until ctx cancel; errors are ignored here, as
		// in the showAll branch above.
		_, _ = srv.ExecuteCommand(ctx, deploymentServer, "sudo "+journalCmd, agg.GetWriter(logs.SourceAudit))
		return
	}

	if showDaemon {
		journalCmd := "tail -f -n 50 /var/log/nextdeployd/nextdeployd.log"
		_, _ = srv.ExecuteCommand(ctx, deploymentServer, "sudo "+journalCmd, agg.GetWriter(logs.SourceDaemon))
		return
	}

	streamAppLogs(ctx, srv, deploymentServer, appName, routeFilter, agg.GetWriter(logs.SourceApp))
}

func streamAppLogs(ctx context.Context, srv *server.ServerStruct, serverName, appName, routeFilter string, out io.Writer) {
	daemonCmd := fmt.Sprintf("sudo /usr/local/bin/nextdeployd logs --appName=%s", shellQuote(appName))
	serviceName, err := srv.ExecuteCommand(ctx, serverName, daemonCmd, nil)
	if err != nil {
		sensitive.Fprintf(os.Stderr, "\033[31mError querying app logs: %v\033[0m\n", err)
		return
	}
	serviceName = strings.TrimSpace(serviceName)

	if serviceName == "APP_NOT_DEPLOYED" {
		fmt.Printf("\033[33mNo logs found.\033[0m The application '%s' is not currently running or has been decommissioned.\n", appName)
		return
	}

	journalCmd := fmt.Sprintf("journalctl -u %s -f -n 50", serviceName)
	if routeFilter != "" {
		journalCmd += fmt.Sprintf(" | grep --line-buffered \"%s\"", routeFilter)
	}

	_, _ = srv.ExecuteCommand(ctx, serverName, "sudo "+journalCmd, out)
}

func init() {
	logsCmd.Flags().String("route", "", "Filter logs by a specific route (e.g. /api/upload)")
	logsCmd.Flags().Bool("audit", false, "Stream the daemon audit log")
	logsCmd.Flags().Bool("daemon", false, "Stream the daemon process log")
	logsCmd.Flags().Bool("all", false, "Stream everything (App + Audit + Daemon)")
	rootCmd.AddCommand(logsCmd)
}
