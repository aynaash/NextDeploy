package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"
	"nextdeploy/internal/config"
	"nextdeploy/internal/secrets"
	"nextdeploy/internal/utils"
)

var (
	secretsProvider string
	secretsToken    string
	secretsProject  string
	secretsConfig   string
)

var secretsAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a secrets provider configuration",
	Long:  `Adds a secrets provider like Doppler to your NextDeploy project.`,
	Run: func(cmd *cobra.Command, args []string) {
		runSecretsAdd()
	},
}

func init() {
	secretsAddCmd.Flags().StringVarP(&secretsProvider, "provider", "p", "doppler", "Secrets provider (currently only 'doppler')")
	secretsAddCmd.Flags().StringVarP(&secretsToken, "token", "t", "", "Provider access token (required)")
	secretsAddCmd.Flags().StringVarP(&secretsProject, "project", "j", "", "Project name in provider (required)")
	secretsAddCmd.Flags().StringVarP(&secretsConfig, "config", "c", "development", "Configuration/environment name")
//	secretsAddCmd.MarkFlagRequired(&token, "token")
//	secretsAddCmd.MarkFlagRequired(&project, "project")
	secretsCmd.AddCommand(secretsAddCmd)
}

func runSecretsAdd() {
	// Validate provider
	if secretsProvider != "doppler" {
		utils.Fatal("Only 'doppler' is currently supported as secrets provider")
	}

	// Validate token with Doppler
	_, err := secrets.ValidateDopplerToken(secretsToken, secretsProject, secretsConfig)
	if err != nil {
		utils.Fatal(fmt.Sprintf("Token validation failed: %v", err))
	}

	// Load existing config or create new
	cfg, err := config.Load()
	if err != nil {
		utils.Fatal(fmt.Sprintf("Failed to load config: %v", err))
	}

	// Set secrets configuration
	cfg.Secrets = &config.SecretsConfig{
		Provider: secretsProvider,
		Project:  secretsProject,
		Config:   secretsConfig,
	}

	// Save configuration
	configPath := filepath.Join(".", "nextdeploy.yml")
	if err := config.Save(cfg, configPath); err != nil {
		utils.Fatal(fmt.Sprintf("Failed to save config: %v", err))
	}

	// Store token securely
	if err := secrets.StoreToken(secretsProvider, secretsToken); err != nil {
		fmt.Printf("⚠️  Warning: Could not store token securely: %v\n", err)
		fmt.Println("Please ensure you have the token saved in a secure location")
	}

	fmt.Printf("✅ Successfully added %s secrets configuration\n", secretsProvider)
	fmt.Printf("  Project: %s\n", secretsProject)
	fmt.Printf("  Config: %s\n", secretsConfig)
}
