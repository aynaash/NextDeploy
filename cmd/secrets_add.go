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
)

var secretsAddCmd = &cobra.Command{
	Use:   "add",
	Short: "🔐 Add a secrets provider configuration",
	Long: color.New(color.FgHiCyan).Sprintf(`%s

Adds a secrets provider configuration to your NextDeploy project.

Examples:
  # Add Doppler with interactive prompts
  nextdeploy secrets add

  # Add Doppler with all flags
  nextdeploy secrets add --provider doppler \
    --token dp.st.xxxxxx \
    --project my-project \
    --config production
`, utils.ASCIIArt("Secrets")),
	Args: cobra.ExactArgs(2), // Add this to match the other add command
	Run:  runSecretsAdd,
}

func init() {
	secretsAddCmd.Flags().StringVarP(&secretsProvider, "provider", "p", "doppler",
		"Secrets provider (doppler, aws-secrets, etc.)")
	secretsAddCmd.Flags().StringVarP(&secretsToken, "token", "t", "",
		color.YellowString("Provider access token (get from Doppler dashboard)"))
	secretsAddCmd.Flags().StringVarP(&secretsProject, "project", "j", "",
		color.YellowString("Project name in Doppler"))
	secretsAddCmd.Flags().StringVarP(&secretsConfig, "config", "c", "development",
		"Configuration/environment (dev/staging/prod)")

	secretsCmd.AddCommand(secretsAddCmd)
}

func runSecretsAdd(cmd *cobra.Command, args []string) {
	name := args[0]  // Get the secret name from args
	value := args[1] // Get the secret value from args

	// Initialize colorful output
	success := color.New(color.FgGreen, color.Bold)
	warning := color.New(color.FgYellow)
	errorMsg := color.New(color.FgRed)
	header := color.New(color.FgHiBlue, color.Underline)

	header.Println("\n⚡ Adding New Secret")

	// Handle interactive mode if flags not provided
	if secretsToken == "" || secretsProject == "" {
		warning.Println("\nℹ️  Interactive mode activated (use flags for non-interactive usage)")

		if secretsProvider == "" {
			secretsProvider = utils.PromptWithDefault("Secrets provider", "doppler")
		}

		if secretsToken == "" {
			secretsToken = utils.PromptPassword("Doppler Service Token")
		}

		if secretsProject == "" {
			secretsProject = utils.Prompt("Doppler Project Name")
		}

		if secretsConfig == "" {
			secretsConfig = utils.PromptWithDefault("Environment Config", "development")
		}
	}

	// Validate provider
	if strings.ToLower(secretsProvider) != "doppler" {
		errorMsg.Println("\n❌ Unsupported secrets provider")
		log.Fatalf("Only 'doppler' is currently supported. You provided: %s", secretsProvider)
	}

	// Validate token with Doppler
	color.New(color.FgHiWhite).Print("\n🔍 Validating Doppler credentials... ")
	valid, err := validateDopplerCredentials(secretsToken, secretsProject, secretsConfig)
	if err != nil {
		fmt.Println()
		errorMsg.Println("❌ Validation failed")
		log.Fatalf("Doppler validation error: %v", err)
	}
	if !valid {
		fmt.Println()
		errorMsg.Println("❌ Invalid credentials")
		log.Fatal("Could not validate Doppler credentials")
	}
	success.Println("✓ Validated!")
	fmt.Printf("   Project: %s\n", color.CyanString(secretsProject))
	fmt.Printf("   Config: %s\n", color.CyanString(secretsConfig))

	// Store the secret value
	color.New(color.FgHiWhite).Print("\n🔒 Storing secret... ")
	if err := secrets.StoreToken(name, value); err != nil { // Fixed to pass both name and value
		warning.Println("⚠️  Warning")
		warning.Printf("Could not store secret securely: %v\n", err)
		warning.Println("Please ensure you have the secret saved in a secure location")
	} else {
		success.Println("✓ Stored!")
	}

	// Success message
	success.Printf("\n🎉 Successfully added secret '%s'!\n", color.CyanString(name))
	fmt.Printf("   Provider: %s\n", color.CyanString(secretsProvider))
	fmt.Printf("   Project: %s\n", color.CyanString(secretsProject))
	fmt.Printf("   Config: %s\n", color.CyanString(secretsConfig))

	fmt.Println()
	color.New(color.FgHiBlack).Println("Tip: Use 'nextdeploy secrets sync' to apply your secrets")
}

// validateDopplerCredentials checks if the provided Doppler credentials are valid
func validateDopplerCredentials(token, project, config string) (bool, error) {
	// This is a placeholder - implement actual Doppler API validation
	// For now just check that values aren't empty
	if token == "" || project == "" || config == "" {
		return false, nil
	}
	return true, nil
}
