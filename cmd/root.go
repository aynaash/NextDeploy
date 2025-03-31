package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// Root command
var rootCmd = &cobra.Command{
	Use:   "mycli",
	Short: "CLI tool for JSON processing & Docker automation",
	Long:  `A Go-based CLI for processing JSON files and automating Docker image builds & pushes.`,
}

// Execute runs the root command
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}
}
