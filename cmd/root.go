/*
Copyright © 2025 Yussuf
*/
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "nextdeploy",
	Short: "CLI for automating Next.js deployment on any VPS",
	Long: `NextDeploy gives you the freedom to deploy your Next.js app anywhere.

It handles Docker image building and app orchestration with fewer than 5 simple commands.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("🚀 NextDeploy is running... Use --help to see available commands.")
		
	},
}

// Execute runs the root command
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "❌ Error: %v\n", err)
		os.Exit(1)
	}
}

func init() {
	// Global flags
	// rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.nextdeploy.yaml)")

	// Local flags
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
