package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"nextdeploy/shared/nextcore"
)

var next = &cobra.Command{
	Use:   "next",
	Short: "command for next parsing",
	Run: func(cmd *cobra.Command, args []string) {
		config, err := nextcore.ParseNextConfig(".")
		if err != nil {
			fmt.Printf("Failed to parse next config: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Next config parsed successfully:")
		fmt.Printf("Project: %s\n", config)
	},
}

func init() {
	rootCmd.AddCommand(next)
}
