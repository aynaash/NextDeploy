package cmd

import (
	"fmt"
	"log"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"nextdeploy/internal/secrets"
	"nextdeploy/internal/utils"
)

var (
	secretsProvider string
	secretsToken    string
	secretsProject  string
	secretsConfig   string
	encryptSecret   bool
)

var secretsAddCmd = &cobra.Command{
	Use:   "add [name] [value]",
	Short: "🔐 Add a new secret",
	Long: color.New(color.FgHiCyan).Sprintf(`%s
Adds a new secret to your project's secret management system.

Examples:
  # Add secret with interactive prompts
  nextdeploy secrets add DB_PASSWORD "mysecurepassword"

  # Add encrypted secret with all flags
  nextdeploy secrets add API_KEY "12345" \
    --provider doppler \
    --token dp.st.xxxxxx \
    --project my-project \
    --config production \
    --encrypt
`, utils.ASCIIArt("Secrets")),
	Args: cobra.ExactArgs(2),
	Run:  runSecretsAdd,
}

func init() {
	secretsAddCmd.Flags().StringVarP(&secretsProvider, "provider", "p", "doppler",
		"Secrets provider (doppler, aws-secrets, etc.)")
	secretsAddCmd.Flags().StringVarP(&secretsToken, "token", "t", "",
		"Provider access token")
	secretsAddCmd.Flags().StringVarP(&secretsProject, "project", "j", "",
		"Project name in secrets provider")
	secretsAddCmd.Flags().StringVarP(&secretsConfig, "config", "c", "development",
		"Configuration/environment (dev/staging/prod)")
	secretsAddCmd.Flags().BoolVarP(&encryptSecret, "encrypt", "e", false,
		"Encrypt the secret value before storing")

	secretsCmd.AddCommand(secretsAddCmd)
}

func runSecretsAdd(cmd *cobra.Command, args []string) {
	name := args[0]
	value := args[1]

	// Initialize output styling
	success := color.New(color.FgGreen, color.Bold)
	warning := color.New(color.FgYellow)
	errorMsg := color.New(color.FgRed)
	header := color.New(color.FgHiBlue, color.Underline)

	header.Println("\n⚡ Adding New Secret")

	// Initialize secret manager
	sm, err := initSecretManager()
	if err != nil {
		errorMsg.Println("❌ Failed to initialize secret manager")
		log.Fatalf("Error: %v", err)
	}

	// Store the secret
	color.New(color.FgHiWhite).Print("\n🔒 Storing secret... ")
	if err := sm.UpdateSecret(name, value, encryptSecret); err != nil {
		errorMsg.Println("❌ Failed to store secret")
		log.Fatalf("Error: %v", err)
	}
	success.Println("✓ Stored!")

	// Verify secret was stored
	color.New(color.FgHiWhite).Print("\n🔍 Verifying secret... ")
	storedValue, err := sm.GetSecret(name)
	if err != nil {
		warning.Println("⚠️  Warning: Could not verify secret")
		warning.Printf("Error: %v\n", err)
	} else if storedValue != value {
		warning.Println("⚠️  Warning: Retrieved value doesn't match")
	} else {
		success.Println("✓ Verified!")
	}

	// Success message
	success.Printf("\n🎉 Successfully added secret '%s'!\n", color.CyanString(name))
	if encryptSecret {
		fmt.Printf("   Encryption: %s\n", color.CyanString("enabled"))
	}
	fmt.Printf("   Provider: %s\n", color.CyanString(secretsProvider))
	if secretsProject != "" {
		fmt.Printf("   Project: %s\n", color.CyanString(secretsProject))
	}
	if secretsConfig != "" {
		fmt.Printf("   Config: %s\n", color.CyanString(secretsConfig))
	}

	fmt.Println()
	color.New(color.FgHiBlack).Println("Tip: Use 'nextdeploy secrets sync' to apply your secrets")
}

func initSecretManager() (secrets.SecretManager, error) {
	// Handle interactive mode if flags not provided
	if secretsToken == "" || secretsProject == "" {
		color.New(color.FgYellow).Println("\nℹ️  Interactive mode activated (use flags for non-interactive usage)")

		if secretsProvider == "" {
			secretsProvider = utils.PromptWithDefault("Secrets provider", "doppler")
		}

		if secretsToken == "" {
			secretsToken = utils.PromptPassword("Provider Service Token")
		}

		if secretsProject == "" && secretsProvider == "doppler" {
			secretsProject = utils.Prompt("Project Name")
		}

		if secretsConfig == "" {
			secretsConfig = utils.PromptWithDefault("Environment Config", "development")
		}
	}

	// Validate provider
	if strings.ToLower(secretsProvider) != "doppler" {
		return nil, fmt.Errorf("only 'doppler' provider is currently supported")
	}

	// Initialize secret manager
	configPath := secrets.GetConfigPath("nextdeploy.yaml")
	masterKey := secrets.GetMasterKey()

	logger := secrets.NewCLILogger()

	return secrets.NewSecretManager(
		configPath,
		masterKey,
		logger,
		secretsProvider == "doppler",
	)
}
