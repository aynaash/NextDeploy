package cmd

import (
	"fmt"
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
	secretsCmd.AddCommand(secretsAddCmd)
}

func runSecretsAdd() {
	if secretsProvider != "doppler" {
		utils.Fatal("Only 'doppler' is currently supported")
	}
	if secretsToken == "" || secretsProject == "" {
		utils.Fatal("Both --token and --project are required")
	}

	if _,err := secrets.ValidateDopplerToken(secretsToken, secretsProject, secretsConfig); err != nil {
		utils.Fatal(fmt.Sprintf("Token validation failed: %v", err))
	}

	cfg, err := config.Load()
	if err != nil {
		utils.Fatal(fmt.Sprintf("Failed to load config: %v", err))
	}

	cfg.Secrets = &config.SecretsConfig{
		Provider: secretsProvider,
		Project:  secretsProject,
		Config:   secretsConfig,
	}

	if err := config.Save(cfg); err != nil {
		utils.Fatal(fmt.Sprintf("Failed to save config: %v", err))
	}

	if err := secrets.StoreToken(secretsProvider, secretsToken); err != nil {
		fmt.Printf("⚠️  Warning: Could not store token securely: %v\n", err)
	}

	fmt.Println("✅ Secrets configuration added successfully")
}
