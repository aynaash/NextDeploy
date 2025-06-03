package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

// Color formatters
var (
	headerStyle  = color.New(color.FgHiMagenta, color.Bold)
	commandStyle = color.New(color.FgHiCyan)
	optionStyle  = color.New(color.FgHiYellow)
	successStyle = color.New(color.FgHiGreen)
	errorStyle   = color.New(color.FgHiRed)
	warningStyle = color.New(color.FgHiYellow)
	noteStyle    = color.New(color.FgHiBlack)
)

// secretsCmd represents the secrets command
var secretsCmd = &cobra.Command{
	Use:   "secrets",
	Short: headerStyle.Sprint("🔐 Manage secrets for your NextDeploy projects"),
	Long:  getSecretsDescription(),
	Example: strings.Join([]string{
		"  nextdeploy secrets add API_KEY \"123456\"",
		"  nextdeploy secrets list --show-values",
		"  nextdeploy secrets remove DB_PASSWORD",
	}, "\n"),
}

func init() {
	rootCmd.AddCommand(secretsCmd)
	
	// Add subcommands
	secretsCmd.AddCommand(
		newSecretsAddCmd(),
		newSecretsListCmd(),
		newSecretsRemoveCmd(),
		newSecretsSyncCmd(),
	)
}

func getSecretsDescription() string {
	builder := &strings.Builder{}
	
	headerStyle.Fprintln(builder, "Secrets Management")
	commandStyle.Fprintln(builder, "\nCommand for managing secrets in your NextDeploy projects:")
	
	successStyle.Fprintln(builder, "  • Add secrets from various providers (Doppler, AWS Secrets Manager, etc.)")
	successStyle.Fprintln(builder, "  • List all configured secrets")
	successStyle.Fprintln(builder, "  • Remove sensitive data from your configuration")
	successStyle.Fprintln(builder, "  • Sync secrets with your deployment environment")
	
	noteStyle.Fprintln(builder, "\nNote: All sensitive values are stored securely and never logged.")
	optionStyle.Fprintln(builder, "\nUse 'nextdeploy secrets [command] --help' for more information")
	
	return builder.String()
}

// newSecretsAddCmd creates the 'add' subcommand
func newSecretsAddCmd() *cobra.Command {
	var (
		provider string
		env      string
	)
	
	cmd := &cobra.Command{
		Use:   "add NAME VALUE",
		Short: commandStyle.Sprint("➕ Add a new secret"),
		Long: successStyle.Sprintf(`
Add a new secret to your project configuration.

You can specify secrets for different environments and providers.
Secrets are stored securely and can be synced with your deployment.

%s
`, warningStyle.Sprint("⚠️  Warning: Plaintext values may be visible in your shell history!")),
		Example: strings.Join([]string{
			"  nextdeploy secrets add API_KEY \"123456\"",
			"  nextdeploy secrets add DB_PASSWORD \"securepass\" --provider=doppler",
			"  nextdeploy secrets add JWT_SECRET \"mysecret\" --env=production",
		}, "\n"),
		Args:    cobra.ExactArgs(2),
		PreRun:  validateAddArgs,
		Run:     runSecretsAdd,
	}

	cmd.Flags().StringVarP(&provider, "provider", "p", "doppler", "Secrets provider")
	cmd.Flags().StringVarP(&env, "env", "e", "development", "Environment name")

	return cmd
}

func validateAddArgs(cmd *cobra.Command, args []string) {
	if len(args) < 2 {
		errorStyle.Println("Error: Both name and value are required")
		cmd.Help()
		os.Exit(1)
	}
}


// newSecretsListCmd creates the 'list' subcommand
func newSecretsListCmd() *cobra.Command {
	var (
		showValues bool
		jsonOutput bool
	)
	
	cmd := &cobra.Command{
		Use:   "list",
		Short: commandStyle.Sprint("📋 List all secrets"),
		Long: successStyle.Sprintf(`
List all configured secrets for your project.

By default, only secret names are shown. Use --show-values to display
the actual values (use with caution in shared environments).
`),
		Run: runSecretsList,
	}

	cmd.Flags().BoolVar(&showValues, "show-values", false, "Display secret values")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output in JSON format")

	return cmd
}

func runSecretsList(cmd *cobra.Command, args []string) {
	showValues, _ := cmd.Flags().GetBool("show-values")
	jsonOutput, _ := cmd.Flags().GetBool("json")

	// TODO: Actual implementation would fetch and display secrets
	if jsonOutput {
		fmt.Println(`{"secrets": [{"name": "API_KEY", "provider": "doppler"}]}`)
		return
	}

	headerStyle.Println("\nConfigured Secrets:")
	fmt.Printf("  %-20s %-15s %-10s\n", 
		commandStyle.Sprint("NAME"), 
		commandStyle.Sprint("PROVIDER"), 
		commandStyle.Sprint("ENV"))
	fmt.Printf("  %-20s %-15s %-10s\n", 
		"API_KEY", "doppler", "development")
	
	if showValues {
		warningStyle.Println("\n⚠️  Secret values are shown below (use with caution):")
		fmt.Printf("  %-20s %s\n", "API_KEY", "123456")
	} else {
		noteStyle.Println("\nNote: Use --show-values to display secret values")
	}
}

// newSecretsRemoveCmd creates the 'remove' subcommand
func newSecretsRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "remove NAME",
		Aliases: []string{"rm", "delete"},
		Short:   commandStyle.Sprint("🗑️  Remove a secret"),
		Long: successStyle.Sprintf(`
Remove a secret from your project configuration.

This operation cannot be undone. The secret will be permanently removed
from your configuration and any synced environments.
`),
		Args:    cobra.ExactArgs(1),
		Example: "  nextdeploy secrets remove API_KEY",
		Run:     runSecretsRemove,
	}
}

func runSecretsRemove(cmd *cobra.Command, args []string) {
	name := args[0]
	// TODO: Actual implementation would remove the secret
	successStyle.Printf("✔ Successfully removed secret '%s'\n", commandStyle.Sprint(name))
}

// newSecretsSyncCmd creates the 'sync' subcommand
func newSecretsSyncCmd() *cobra.Command {
	var (
		env      string
		dryRun   bool
	)
	
	cmd := &cobra.Command{
		Use:   "sync",
		Short: commandStyle.Sprint("🔄 Sync secrets with environment"),
		Long: successStyle.Sprintf(`
Synchronize secrets with your deployment environment.

This will update the target environment with all configured secrets
from your selected providers.
`),
		Run: runSecretsSync,
	}

	cmd.Flags().StringVarP(&env, "env", "e", "", "Environment to sync with")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be synced")

	return cmd
}

func runSecretsSync(cmd *cobra.Command, args []string) {
	env, _ := cmd.Flags().GetString("env")
	dryRun, _ := cmd.Flags().GetBool("dry-run")

	if dryRun {
		warningStyle.Println("Dry run mode - no changes will be made")
		// TODO: Show what would be synced
		return
	}

	if env == "" {
		env = "development"
	}

	// TODO: Actual sync implementation
	successStyle.Printf("✔ Successfully synced secrets to %s environment\n", commandStyle.Sprint(env))
}
