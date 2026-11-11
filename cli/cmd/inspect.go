package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/aynaash/nextdeploy/internal/packaging"
	"github.com/aynaash/nextdeploy/shared/config"
	"github.com/aynaash/nextdeploy/shared/nextcore"
	"github.com/aynaash/nextdeploy/shared/sensitive"
	"github.com/spf13/cobra"
)

var inspectCmd = &cobra.Command{
	Use:   "inspect",
	Short: "Inspect the deployment bundle size and dependencies",
	Long:  "Analyzes the .next/standalone directory to identify large dependencies and estimate deployment size.",
	Run: func(cmd *cobra.Command, args []string) {
		projectDir, err := os.Getwd()
		if err != nil {
			sensitive.Printf("✗ Error getting current directory: %v\n", err)
			os.Exit(1)
		}

		cfg, _ := config.Load()
		target := "vps"
		provider := ""
		if cfg != nil {
			target = strings.ToLower(cfg.ResolveTargetType(""))
			if cfg.Serverless != nil {
				provider = strings.ToLower(cfg.Serverless.Provider)
			}
		}

		payload, err := nextcore.LoadMetadata()
		distDir := ".next"
		if err == nil {
			distDir = payload.DistDir
		}

		standaloneDir := filepath.Join(projectDir, distDir, "standalone")
		if _, err := os.Stat(standaloneDir); os.IsNotExist(err) {
			fmt.Printf("✗ standalone directory not found at %s\n", standaloneDir)
			fmt.Println("  Ensure 'output: \"standalone\"' is set in your next.config.mjs and run 'nextdeploy build' first.")
			os.Exit(1)
		}

		fmt.Printf("🔍 Inspecting bundle in %s...\n\n", standaloneDir)

		report, err := packaging.AuditStandaloneSize(standaloneDir)
		if err != nil {
			sensitive.Printf("✗ Error auditing bundle: %v\n", err)
			os.Exit(1)
		}

		totalMB := report.TotalMB

		if target == "serverless" && provider == "aws" {
			lambdaLimit := 250.0
			usagePercent := (totalMB / lambdaLimit) * 100

			fmt.Printf("AWS Lambda Package Analysis:\n")
			fmt.Printf("   Total Size:      %.2f MB / %.0f MB (%.1f%% usage)\n", totalMB, lambdaLimit, usagePercent)
			fmt.Printf("   node_modules:    %.2f MB\n", report.NodeModulesMB)
			fmt.Printf("   Server Code:     %.2f MB\n", report.ServerCodeMB)
			fmt.Println()

			if totalMB > lambdaLimit {
				fmt.Printf("⚠️  WARNING: Your bundle exceeds the AWS Lambda 250MB unzipped limit!\n")
			} else if totalMB > 200 {
				fmt.Printf("⚠️  WARNING: Your bundle is approaching the 250MB limit.\n")
			}
		} else if target == "serverless" && provider == "cloudflare" {
			fmt.Printf("Cloudflare Workers Bundle Analysis:\n")
			fmt.Printf("   Standalone Size: %.2f MB\n", totalMB)
			fmt.Printf("   node_modules:    %.2f MB\n", report.NodeModulesMB)
			fmt.Printf("   Server Code:     %.2f MB\n", report.ServerCodeMB)
			fmt.Println()
		} else {
			fmt.Printf("VPS Standalone Bundle Analysis:\n")
			fmt.Printf("   Total Size:      %.2f MB\n", totalMB)
			fmt.Printf("   node_modules:    %.2f MB\n", report.NodeModulesMB)
			fmt.Printf("   Server Code:     %.2f MB\n", report.ServerCodeMB)
			fmt.Println()
		}

		fmt.Printf("Top Dependencies by Size:\n")
		for i, offender := range report.TopOffenders {
			fmt.Printf("   %2d. %-30s %.2f MB\n", i+1, offender.Package, offender.SizeMB)
		}

		fmt.Println("\nTips to reduce size:")
		fmt.Println("   - Ensure specific large binaries (sharp, prisma) are only installed if needed.")
		fmt.Println("   - Use 'next-bundle-analyzer' to find large client-side chunks.")
		fmt.Println("   - Check for large data files mistakenly included in your source tree.")
	},
}

func init() {
	rootCmd.AddCommand(inspectCmd)
}
