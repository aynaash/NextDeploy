package cmd

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/aynaash/nextdeploy/cli/internal/server"
	"github.com/aynaash/nextdeploy/cli/internal/serverless"
	"github.com/aynaash/nextdeploy/shared"
	"github.com/aynaash/nextdeploy/shared/config"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show the current status of your deployed application",
	Long:  "Queries the target provider (serverless or VPS daemon) for active PID, uptime, memory, routes, and health status.",
	Run: func(cmd *cobra.Command, args []string) {
		log := shared.PackageLogger("status", "📊 STATUS")
		cfg, err := config.Load()
		if err != nil {
			log.Error("Failed to load config: %v", err)
			os.Exit(1)
		}

		appName := cfg.App.Name
		log.Info("Querying status for %s...", appName)

		ctx := cmd.Context()
		if ctx == nil {
			ctx = context.Background()
		}

		target := strings.ToLower(cfg.ResolveTargetType(""))
		if target == "serverless" {
			statusServerless(ctx, log, cfg)
			return
		}

		statusVPS(ctx, log, cfg)
	},
}

func statusVPS(ctx context.Context, log *shared.Logger, cfg *config.NextDeployConfig) {
	appName := cfg.App.Name

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

	timeoutCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	daemonCmd := fmt.Sprintf("sudo /usr/local/bin/nextdeployd status --appName=%s", shellQuote(appName))
	output, err := srv.ExecuteCommand(timeoutCtx, deploymentServer, daemonCmd, nil)
	if err != nil {
		log.Error("Failed to query daemon: %v\nOutput: %s", err, output)
		os.Exit(1)
	}

	// Strip shell noise — only keep lines starting from "Status:" header
	output = strings.TrimSpace(output)
	if idx := strings.Index(output, "Status:"); idx >= 0 {
		output = output[idx:]
	}

	fmt.Printf("\n🚀 NextDeploy Status: %s\n", appName)
	fmt.Println("──────────────────────────────────────────────────")
	fmt.Println(output)
	fmt.Println("──────────────────────────────────────────────────")
}

func statusServerless(ctx context.Context, log *shared.Logger, cfg *config.NextDeployConfig) {
	if cfg.Serverless == nil {
		log.Error("Target is 'serverless' but 'serverless' config block is missing.")
		os.Exit(1)
	}

	appName := cfg.App.Name
	providerName := "aws"
	if cfg.Serverless.Provider != "" {
		providerName = strings.ToLower(cfg.Serverless.Provider)
	}

	p, err := serverless.New(providerName, false)
	if err != nil {
		log.Error("Failed to initialize serverless provider: %v", err)
		os.Exit(1)
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if err := p.Initialize(timeoutCtx, cfg); err != nil {
		log.Error("Failed to initialize provider: %v", err)
		os.Exit(1)
	}

	resMap, err := p.GetResourceMap(timeoutCtx, cfg)
	if err != nil {
		log.Error("Failed to query serverless resources: %v", err)
		os.Exit(1)
	}

	probeURL := ""
	if resMap.CustomDomain != "" {
		probeURL = resMap.CustomDomain
	} else if resMap.CloudFrontDomain != "" {
		probeURL = resMap.CloudFrontDomain
	} else if resMap.FunctionURL != "" {
		probeURL = resMap.FunctionURL
	}

	statusDesc := resMap.Status
	if statusDesc == "" {
		statusDesc = "Online"
	}

	probeFailed := false
	if probeURL != "" && resMap.Status != "Not Deployed" {
		probe := probeEndpoint(timeoutCtx, probeURL)
		if probe.err != nil {
			statusDesc = fmt.Sprintf("Unreachable (%v)", probe.err)
			probeFailed = true
		} else if probe.statusCode >= 200 && probe.statusCode < 400 {
			statusDesc = fmt.Sprintf("Online (HTTP %d - %v)", probe.statusCode, probe.duration.Round(time.Millisecond))
		} else {
			statusDesc = fmt.Sprintf("Degraded (HTTP %d - %v)", probe.statusCode, probe.duration.Round(time.Millisecond))
			probeFailed = true
		}
	}

	fmt.Printf("\n🚀 NextDeploy Status: %s\n", appName)
	fmt.Println("──────────────────────────────────────────────────")
	fmt.Printf("Target:        Serverless (%s)\n", providerName)
	if cfg.App.Environment != "" {
		fmt.Printf("Environment:   %s\n", cfg.App.Environment)
	}
	fmt.Printf("Status:        %s\n", statusDesc)

	if resMap.WorkerName != "" {
		if providerName == "aws" {
			fmt.Printf("Function:      %s\n", resMap.WorkerName)
		} else {
			fmt.Printf("Worker:        %s\n", resMap.WorkerName)
		}
	} else if resMap.LambdaARN != "" {
		fmt.Printf("Function ARN:  %s\n", resMap.LambdaARN)
	}

	if resMap.ActiveVersion != "" {
		fmt.Printf("Active Ver:    %s\n", resMap.ActiveVersion)
	}

	if resMap.CustomDomain != "" {
		fmt.Printf("Domain:        https://%s\n", strings.TrimPrefix(strings.TrimPrefix(resMap.CustomDomain, "https://"), "http://"))
	}
	if resMap.CloudFrontDomain != "" {
		fmt.Printf("CloudFront:    %s\n", resMap.CloudFrontDomain)
	}
	if resMap.FunctionURL != "" {
		fmt.Printf("Function URL:  %s\n", resMap.FunctionURL)
	}
	if resMap.S3BucketName != "" {
		if providerName == "cloudflare" {
			fmt.Printf("Storage (R2):  %s\n", resMap.S3BucketName)
		} else {
			fmt.Printf("Storage (S3):  %s\n", resMap.S3BucketName)
		}
	}
	if resMap.CertificateStatus != "" {
		fmt.Printf("SSL Status:    %s\n", resMap.CertificateStatus)
	}

	if providerName == "cloudflare" && cfg.Serverless.Cloudflare != nil && cfg.Serverless.Cloudflare.Bindings != nil {
		b := cfg.Serverless.Cloudflare.Bindings
		var bindings []string
		for _, d1 := range b.D1 {
			bindings = append(bindings, fmt.Sprintf("%s (d1)", d1.Name))
		}
		for _, r2 := range b.R2 {
			bindings = append(bindings, fmt.Sprintf("%s (r2)", r2.Name))
		}
		for _, kv := range b.KV {
			bindings = append(bindings, fmt.Sprintf("%s (kv)", kv.Name))
		}
		for _, h := range b.Hyperdrive {
			bindings = append(bindings, fmt.Sprintf("%s (hyperdrive)", h.Name))
		}
		if b.Queues != nil {
			for _, q := range b.Queues.Producers {
				bindings = append(bindings, fmt.Sprintf("%s (queue)", q.Name))
			}
		}
		if len(bindings) > 0 {
			fmt.Printf("Bindings:      %s\n", strings.Join(bindings, ", "))
		}
	}

	if !resMap.DeploymentTime.IsZero() {
		fmt.Printf("Deployed At:   %s\n", resMap.DeploymentTime.Format(time.RFC1123))
	}
	fmt.Println("──────────────────────────────────────────────────")

	if probeFailed || resMap.Status == "Not Deployed" {
		os.Exit(1)
	}
}

type probeResult struct {
	statusCode int
	duration   time.Duration
	err        error
}

func probeEndpoint(ctx context.Context, rawURL string) probeResult {
	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		rawURL = "https://" + rawURL
	}
	client := &http.Client{
		Timeout: 10 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("stopped after 10 redirects")
			}
			return nil
		},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return probeResult{err: err}
	}
	req.Header.Set("User-Agent", "NextDeploy-Status-Probe/"+shared.Version)

	start := time.Now()
	resp, err := client.Do(req)
	duration := time.Since(start)
	if err != nil {
		return probeResult{duration: duration, err: err}
	}
	defer resp.Body.Close()
	return probeResult{statusCode: resp.StatusCode, duration: duration}
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
