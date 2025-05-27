//go:build ignore
// +build ignore


 package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

type SecretsConfig struct {
	Provider string `yaml:"provider"`
	Token    string `yaml:"token,omitempty"` // Only stored temporarily
	Project  string `yaml:"project"`
	Config   string `yaml:"config"`
}

type AppConfig struct {
	Secrets *SecretsConfig `yaml:"secrets,omitempty"`
	// Add other existing config fields here
}

var (
	secretsProvider string
	secretsToken    string
	secretsProject  string
	secretsConfig   string
)

var secretsAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a secrets provider configuration",
	Long: `Add a secrets provider like Doppler to your project.
Example: nextdeploy secrets add --provider=doppler --token=dp.st.xxx --project=myapp --config=production`,
	Run: func(cmd *cobra.Command, args []string) {
		// Load existing config
		cfg, err := loadConfig()
		if err != nil {
			fmt.Printf("Error loading config: %v\n", err)
			os.Exit(1)
		}

		// Validate inputs
		if secretsProvider != "doppler" {
			fmt.Println("Currently only Doppler is supported as a secrets provider")
			os.Exit(1)
		}

		if secretsToken == "" || secretsProject == "" {
			fmt.Println("Token and project are required")
			os.Exit(1)
		}

		if secretsConfig == "" {
			secretsConfig = "development" // Default value
		}

		// Validate Doppler token
		if err := validateDopplerToken(secretsToken, secretsProject, secretsConfig); err != nil {
			fmt.Printf("Doppler token validation failed: %v\n", err)
			os.Exit(1)
		}

		// Update config
		cfg.Secrets = &SecretsConfig{
			Provider: secretsProvider,
			Project:  secretsProject,
			Config:   secretsConfig,
		}

		// Save config
		if err := saveConfig(cfg); err != nil {
			fmt.Printf("Error saving config: %v\n", err)
			os.Exit(1)
		}

		// Store token securely
		if err := storeTokenSecurely(secretsToken); err != nil {
			fmt.Printf("Warning: could not store token securely: %v\n", err)
		}

		fmt.Println("✅ Secrets configuration added successfully")
	},
}

func init() {
	secretsAddCmd.Flags().StringVarP(&secretsProvider, "provider", "p", "doppler", "Secrets provider (currently only 'doppler')")
	secretsAddCmd.Flags().StringVarP(&secretsToken, "token", "t", "", "Provider access token (required)")
	secretsAddCmd.Flags().StringVarP(&secretsProject, "project", "j", "", "Project name in provider (required)")
	secretsAddCmd.Flags().StringVarP(&secretsConfig, "config", "c", "", "Configuration/environment name (default: development)")
	secretsCmd.AddCommand(secretsAddCmd)
}

func loadConfig() (*AppConfig, error) {
	data, err := os.ReadFile(".nextdeploy.yml")
	if err != nil {
		return nil, fmt.Errorf("could not read config file: %w", err)
	}

	var cfg AppConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("could not parse config file: %w", err)
	}

	return &cfg, nil
}

func saveConfig(cfg *AppConfig) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("could not marshal config: %w", err)
	}

	return os.WriteFile(".nextdeploy.yml", data, 0644)
}

func storeTokenSecurely(token string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	tokenDir := filepath.Join(home, ".nextdeploy", "tokens")
	if err := os.MkdirAll(tokenDir, 0700); err != nil {
		return err
	}

	tokenFile := filepath.Join(tokenDir, "doppler.token")
	return os.WriteFile(tokenFile, []byte(token), 0600)
}

func validateDopplerToken(token, project, config string) error {
	// Implement actual Doppler API validation
	// For now just a placeholder
	return nil
}
