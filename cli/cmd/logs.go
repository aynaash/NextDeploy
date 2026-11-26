package cmd

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

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
	Short: "Stream, query, and triage application logs with fault attribution",
	Long: "Streams and analyzes application logs with clear fault attribution ([APP], [NEXTDEPLOY], [PLATFORM]).\n\n" +
		"Supports live Cloudflare Workers tailing, historical Workers Observability queries (--since),\n" +
		"constant-memory triage of exported JSON/NDJSON log files (logs analyze <file> / --file),\n" +
		"CloudWatch on AWS Lambda, and systemd journal on VPS.",
	Run: func(cmd *cobra.Command, args []string) {
		log := shared.PackageLogger("logs", "🚀 LOGS")

		routeFilter, _ := cmd.Flags().GetString("route")
		ownerStr, _ := cmd.Flags().GetString("owner")
		errorsOnly, _ := cmd.Flags().GetBool("errors")
		raw, _ := cmd.Flags().GetBool("raw")
		since, _ := cmd.Flags().GetDuration("since")
		filePath, _ := cmd.Flags().GetString("file")
		outPath, _ := cmd.Flags().GetString("out")
		topN, _ := cmd.Flags().GetInt("top")
		asJSON, _ := cmd.Flags().GetBool("json")
		showAudit, _ := cmd.Flags().GetBool("audit")
		showDaemon, _ := cmd.Flags().GetBool("daemon")
		showAll, _ := cmd.Flags().GetBool("all")

		ownerFilter, err := logs.ParseOwner(ownerStr)
		if err != nil {
			log.Error("%v", err)
			os.Exit(1)
		}

		if filePath != "" {
			if err := runAnalyzeFile(filePath, routeFilter, ownerFilter, outPath, topN, asJSON, raw, false); err != nil {
				log.Error("Log analysis failed: %v", err)
				os.Exit(1)
			}
			return
		}

		cfg, err := config.Load()
		if err != nil {
			log.Error("Failed to load config: %v", err)
			os.Exit(1)
		}

		appName := cfg.App.Name
		ctx := cmd.Context()
		if ctx == nil {
			ctx = context.Background()
		}

		target := strings.ToLower(cfg.ResolveTargetType(""))
		if target == "serverless" {
			if showAudit || showDaemon {
				log.Warn("--audit and --daemon flags apply only to VPS daemon environments. Streaming app logs.")
			}
			logsServerless(ctx, log, cfg, serverlessLogOpts{
				RouteFilter: routeFilter,
				OwnerFilter: ownerFilter,
				ErrorsOnly:  errorsOnly,
				Raw:         raw,
				Since:       since,
				OutPath:     outPath,
				TopN:        topN,
				AsJSON:      asJSON,
				ShowAll:     showAll,
			})
			return
		}

		logsVPS(ctx, log, cfg, appName, routeFilter, showAudit, showDaemon, showAll)
	},
}

var logsAnalyzeCmd = &cobra.Command{
	Use:     "analyze <file>",
	Aliases: []string{"triage"},
	Short:   "Analyze an exported Cloudflare JSON/NDJSON log file in constant memory",
	Long: "Streams a Cloudflare Workers JSON array or NDJSON log export (even 100MB+ or truncated files)\n" +
		"in constant memory, merges 4x duplicate fragments by requestId, and classifies whether each failure\n" +
		"belongs to [APP] (your code), [NEXTDEPLOY] (runtime/adapter), or [PLATFORM] (Cloudflare limits).",
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		log := shared.PackageLogger("logs", "🚀 LOGS")
		routeFilter, _ := cmd.Flags().GetString("route")
		ownerStr, _ := cmd.Flags().GetString("owner")
		outPath, _ := cmd.Flags().GetString("out")
		topN, _ := cmd.Flags().GetInt("top")
		asJSON, _ := cmd.Flags().GetBool("json")
		raw, _ := cmd.Flags().GetBool("raw")
		allLevels, _ := cmd.Flags().GetBool("all-levels")

		ownerFilter, err := logs.ParseOwner(ownerStr)
		if err != nil {
			log.Error("%v", err)
			os.Exit(1)
		}

		if err := runAnalyzeFile(args[0], routeFilter, ownerFilter, outPath, topN, asJSON, raw, allLevels); err != nil {
			log.Error("Log analysis failed: %v", err)
			os.Exit(1)
		}
	},
}

type serverlessLogOpts struct {
	RouteFilter string
	OwnerFilter logs.Owner
	ErrorsOnly  bool
	Raw         bool
	Since       time.Duration
	OutPath     string
	TopN        int
	AsJSON      bool
	ShowAll     bool
}

func runAnalyzeFile(filePath, routeFilter string, ownerFilter logs.Owner, outPath string, topN int, asJSON, raw, allLevels bool) error {
	var mergedOut io.Writer
	if outPath != "" {
		f, err := os.Create(outPath) // #nosec G304 — user-requested output path
		if err != nil {
			return fmt.Errorf("create output file: %w", err)
		}
		bw := bufio.NewWriterSize(f, 1<<16)
		defer func() {
			_ = bw.Flush()
			_ = f.Close()
		}()
		mergedOut = bw
	}

	rep, err := logs.AnalyzeFile(filePath, logs.AnalyzeOptions{
		RouteFilter:     routeFilter,
		OwnerFilter:     ownerFilter,
		AllLevels:       allLevels,
		MergedNDJSONOut: mergedOut,
	})
	if err != nil {
		return err
	}

	return printAnalysisOutput(os.Stdout, rep, topN, asJSON, raw, outPath)
}

func printAnalysisOutput(w io.Writer, rep *logs.AnalysisReport, topN int, asJSON, raw bool, outPath string) error {
	if asJSON {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(rep)
	}

	logs.FormatReport(w, rep, topN)

	if raw && len(rep.Records) > 0 {
		fmt.Fprintln(w, "\033[1mMERGED REQUEST RECORDS (--raw)\033[0m")
		for _, rec := range rep.Records {
			fmt.Fprintf(w, "  %s %s %s %s %d (%s) %s\n",
				rec.TS, logs.OwnerBadge(rec.Owner), rec.Method, rec.Path, rec.Status, rec.RequestID, rec.Message)
		}
		fmt.Fprintln(w)
	}

	if outPath != "" {
		fmt.Fprintf(w, "Wrote %d classified records to %s\n", len(rep.Records), outPath)
	}
	return nil
}

func logsServerless(ctx context.Context, log *shared.Logger, cfg *config.NextDeployConfig, opts serverlessLogOpts) {
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

		var ndjsonWriter io.Writer
		if opts.OutPath != "" {
			f, err := os.Create(opts.OutPath) // #nosec G304 — user-requested output path
			if err != nil {
				log.Error("Failed to create output file %s: %v", opts.OutPath, err)
				os.Exit(1)
			}
			bw := bufio.NewWriterSize(f, 1<<16)
			defer func() {
				_ = bw.Flush()
				_ = f.Close()
			}()
			ndjsonWriter = bw
		}

		if opts.Since > 0 {
			log.Info("Querying Cloudflare Workers telemetry for %s (%s) over last %s...", cfg.App.Name, workerName, opts.Since)
			p, err := serverless.New("cloudflare", false)
			if err != nil {
				log.Error("Failed to create Cloudflare provider: %v", err)
				os.Exit(1)
			}
			if err := p.Initialize(ctx, cfg); err != nil {
				log.Error("Failed to initialize Cloudflare provider: %v", err)
				os.Exit(1)
			}
			cfProvider, ok := p.(*serverless.CloudflareProvider)
			if !ok {
				log.Error("Unexpected Cloudflare provider type")
				os.Exit(1)
			}

			rep, err := cfProvider.QueryLogs(ctx, cfg, serverless.QueryOptions{
				Since:       opts.Since,
				RouteFilter: opts.RouteFilter,
				OwnerFilter: opts.OwnerFilter,
				ErrorsOnly:  opts.ErrorsOnly || !opts.ShowAll,
				MaxEvents:   2000,
				NDJSONOut:   ndjsonWriter,
			})
			if err != nil {
				log.Error("Cloudflare telemetry query failed: %v", err)
				os.Exit(1)
			}
			if err := printAnalysisOutput(os.Stdout, rep, opts.TopN, opts.AsJSON, opts.Raw, opts.OutPath); err != nil {
				log.Error("Failed to write output: %v", err)
				os.Exit(1)
			}
			return
		}

		log.Info("Streaming Cloudflare Workers logs for %s (%s)...", cfg.App.Name, workerName)

		// Try native Go WebSocket tailing first (zero dependencies, native formatting, fault ownership badges)
		p, err := serverless.New("cloudflare", false)
		if err == nil {
			if initErr := p.Initialize(ctx, cfg); initErr == nil {
				if cfProvider, ok := p.(*serverless.CloudflareProvider); ok {
					tailErr := cfProvider.TailLogsWithOptions(ctx, cfg, serverless.TailOptions{
						RouteFilter: opts.RouteFilter,
						ErrorsOnly:  opts.ErrorsOnly,
						OwnerFilter: opts.OwnerFilter,
						Raw:         opts.Raw,
						QuietAssets: !opts.Raw,
						NDJSONOut:   ndjsonWriter,
					}, os.Stdout)
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
		if opts.RouteFilter != "" {
			args = append(args, "--search", opts.RouteFilter)
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
		if opts.RouteFilter != "" {
			args = append(args, "--filter-pattern", opts.RouteFilter)
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
	logsCmd.Flags().BoolP("errors", "e", false, "Show only errors (4xx/5xx, exceptions, error logs)")
	logsCmd.Flags().String("owner", "", "Filter errors by owner: app, nextdeploy, platform, unknown")
	logsCmd.Flags().Bool("raw", false, "Show raw uncollapsed logs and full stack traces")
	logsCmd.Flags().Duration("since", 0, "Query recent Cloudflare Workers logs (e.g. 15m, 1h, 24h) and print grouped triage summary")
	logsCmd.Flags().StringP("file", "f", "", "Analyze an exported Cloudflare JSON/NDJSON log file offline (constant-memory streaming)")
	logsCmd.Flags().StringP("out", "o", "", "Write deduplicated, classified logs to an NDJSON file")
	logsCmd.Flags().Int("top", 15, "Maximum rows per section in triage summary")
	logsCmd.Flags().Bool("json", false, "Output triage summary as JSON")
	logsCmd.Flags().Bool("audit", false, "Stream the daemon audit log")
	logsCmd.Flags().Bool("daemon", false, "Stream the daemon process log")
	logsCmd.Flags().Bool("all", false, "Stream everything (App + Audit + Daemon, or include non-errors with --since)")

	logsAnalyzeCmd.Flags().String("route", "", "Filter logs by a specific route (e.g. /docs)")
	logsAnalyzeCmd.Flags().String("owner", "", "Filter errors by owner: app, nextdeploy, platform, unknown")
	logsAnalyzeCmd.Flags().StringP("out", "o", "", "Write deduplicated, classified logs to an NDJSON file")
	logsAnalyzeCmd.Flags().Int("top", 15, "Maximum rows per section in triage summary")
	logsAnalyzeCmd.Flags().Bool("json", false, "Output triage summary as JSON")
	logsAnalyzeCmd.Flags().Bool("raw", false, "Also print individual merged request records")
	logsAnalyzeCmd.Flags().Bool("all-levels", false, "Include non-error log events in analysis")

	logsCmd.AddCommand(logsAnalyzeCmd)
	rootCmd.AddCommand(logsCmd)
}
