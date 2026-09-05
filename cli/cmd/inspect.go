package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/aynaash/nextdeploy/internal/packaging"
	"github.com/aynaash/nextdeploy/shared/nextcore"
	"github.com/aynaash/nextdeploy/shared/sensitive"
	"github.com/spf13/cobra"
)

var inspectCmd = &cobra.Command{
	Use:   "inspect",
	Short: "Inspect the deployment bundle size and dependencies",
	Long: "Analyzes the .next/standalone directory to identify large dependencies, and\n" +
		"reports the size of the compiled Worker bundle when one has been built.",
	Run: func(cmd *cobra.Command, args []string) {
		projectDir, err := os.Getwd()
		if err != nil {
			sensitive.Printf("✗ Error getting current directory: %v\n", err)
			os.Exit(1)
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

		// The standalone tree is the compiler's INPUT, not the shipped artifact —
		// esbuild bundles and tree-shakes it down. Report it as a diagnostic for
		// finding heavy dependencies, and score the actual Worker bundle instead.
		fmt.Printf("Standalone build (compiler input):\n")
		fmt.Printf("   Total Size:      %.2f MB\n", report.TotalMB)
		fmt.Printf("   node_modules:    %.2f MB\n", report.NodeModulesMB)
		fmt.Printf("   Server Code:     %.2f MB\n", report.ServerCodeMB)
		fmt.Println()

		// workerLimitMiB is Cloudflare's uncompressed Worker size limit, the same
		// on Free and Paid since 2026-09-04 (the older 3 MB / 10 MB *compressed*
		// limits were removed). Only uncompressed size is checked.
		// https://developers.cloudflare.com/workers/platform/limits/#worker-size
		const workerLimitMiB = 64.0

		workerPath := filepath.Join(standaloneDir, ".nextdeploy-cf", "worker.mjs")
		if info, err := os.Stat(workerPath); err == nil {
			workerMiB := float64(info.Size()) / (1024 * 1024)
			fmt.Printf("Compiled Worker (the shipped artifact):\n")
			fmt.Printf("   worker.mjs:      %.2f MiB / %.0f MiB (%.1f%% of limit)\n",
				workerMiB, workerLimitMiB, (workerMiB/workerLimitMiB)*100)
			fmt.Println()
			if workerMiB > workerLimitMiB {
				fmt.Printf("WARNING: the bundle exceeds Cloudflare's %.0f MiB Worker size limit — the deploy will be rejected.\n\n", workerLimitMiB)
			} else if workerMiB > workerLimitMiB*0.8 {
				fmt.Printf("WARNING: the bundle is within 20%% of the %.0f MiB Worker size limit.\n\n", workerLimitMiB)
			}
			// Size is rarely the binding constraint; startup time usually is.
			fmt.Println("Note: a Worker must parse and execute its global scope within 1 second.")
			fmt.Println("      Large bundles and expensive top-level code eat into that budget.")
			fmt.Println()
		} else {
			fmt.Println("No compiled Worker found — run `nextdeploy ship` to produce one,")
			fmt.Println("then re-run inspect to size the artifact that actually ships.")
			fmt.Println()
		}

		fmt.Printf("Top Dependencies by Size:\n")
		for i, offender := range report.TopOffenders {
			fmt.Printf("   %2d. %-30s %.2f MB\n", i+1, offender.Package, offender.SizeMB)
		}

		fmt.Println("\nTips to reduce size:")
		fmt.Println("   - Ensure large binaries (sharp, prisma) are only installed if needed.")
		fmt.Println("   - Move config files, static assets and binary data into R2, KV or D1")
		fmt.Println("     instead of bundling them into the Worker.")
		fmt.Println("   - Use 'next-bundle-analyzer' to find large client-side chunks.")
		fmt.Println("   - Check for large data files mistakenly included in your source tree.")
	},
}

func init() {
	rootCmd.AddCommand(inspectCmd)
}
