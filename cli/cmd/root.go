/*
NextDeploy - A clean and powerful CLI for Next.js deployments
*/
package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var (
	green   = color.New(color.FgGreen).SprintFunc()
	yellow  = color.New(color.FgYellow).SprintFunc()
	red     = color.New(color.FgRed).SprintFunc()
	bold    = color.New(color.Bold).SprintFunc()
	cyan    = color.New(color.BgCyan).SprintFunc()
	magenta = color.New(color.BgHiMagenta).SprintFunc()
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "nextdeploy",
	Short: "CLI for automating Next.js deployments on any VPS with a custom daemon.",
	Long: fmt.Sprintf(`%s %s

%s
Deploy your Next.js app to *any* VPS — with Docker, SSL, logs, and zero downtime.

%s
%s  Build Docker images with ease
%s  Push and deploy to remote servers in seconds
%s  Configure automatic SSL + monitoring
%s  Ship production-ready builds with full control

%s
Run '%s' to see available commands.
`,
		bold("🚀 NextDeploy"), yellow("v1.0.0"),
		magenta("Simple. Fast. Infrastructure-Agnostic."),
		bold("Features:"),
		green("✓"),
		green("✓"),
		green("✓"),
		green("✓"),
		yellow("👋 Tip:"),
		cyan("nextdeploy --help"),
	),
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("\n%s %s\n\n",
			green("✨ Welcome to"), bold("NextDeploy CLI"),
		)

		if len(args) == 0 {
			fmt.Println(bold("Quick Start:"))
			fmt.Printf("  %s - Initialize a new project\n", cyan("nextdeploy init"))
			fmt.Printf("  %s - Build the image for the app\n", cyan("nextdeploy build"))
			fmt.Printf("  %s - Deploy your app on the vps\n\n", cyan("nextdeploy ship"))

			fmt.Printf("%s %s\n\n",
				yellow("Docs →"), cyan("https://nextdeploy.one/docs"),
			)
		}
	},
}

// Execute runs the root command
func Execute() {
	fmt.Println()

	if err := rootCmd.Execute(); err != nil {
		fmt.Printf("\n%s %s\n\n",
			red("❌ Error:"), err,
		)
		os.Exit(1)
	}

	fmt.Println(strings.Repeat("─", 60))
	fmt.Printf("%s %s\n",
		cyan("Need help?"),
		yellow("Visit https://nextdeploy.one/docs"),
	)
	fmt.Println(strings.Repeat("─", 60))
	fmt.Println()
}

func init() {
	rootCmd.SetHelpTemplate(fmt.Sprintf(`%s
%s
{{if or .Runnable .HasSubCommands}}{{.UsageString}}{{end}}`,
		cyan("✨ NextDeploy CLI Toolkit"),
		yellow("Usage: {{.UseLine}}"),
	))

	rootCmd.SetUsageTemplate(`{{.UseLine}}

  {{.Short}}

{{if .HasAvailableFlags}}Options:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}

{{if .HasAvailableSubCommands}}Commands:
{{range .Commands}}{{if .IsAvailableCommand}}  {{rpad .Name .NamePadding }} {{.Short}}
{{end}}{{end}}{{end}}

Run '{{.CommandPath}} [command] --help' for more information about a command.
`)
}
