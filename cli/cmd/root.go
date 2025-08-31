/*
NextDeploy - A clean and powerful CLI for Next.js deployments
*/
package cmd

import (
	"fmt"
	"nextdeploy/shared"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

// Semantic color functions
var (
	title       = color.New(color.FgHiBlue, color.Bold).SprintFunc()
	success     = color.New(color.FgHiGreen).SprintFunc()
	warning     = color.New(color.FgHiYellow, color.Bold).SprintFunc()
	errorMsg    = color.New(color.FgHiRed, color.Bold).SprintFunc()
	command     = color.New(color.FgCyan).SprintFunc()
	highlight   = color.New(color.Bold).SprintFunc()
	versionFlag = false
)

// rootCmd is the main command
var rootCmd = &cobra.Command{
	Use:   "nextdeploy",
	Short: "CLI for automating Next.js deployments on any VPS with a custom daemon.",
	Long: fmt.Sprintf(`%s %s

%s
Deploy your Next.js app to *any* VPS — with Docker, SSL, logs, and zero downtime.

%s
%s Build Docker images with ease
%s Push and deploy to remote servers in seconds
%s Configure automatic SSL + monitoring
%s Ship production-ready builds with full control

%s %s
`,
		title("🚀 NextDeploy"), warning("v1.0.0"),
		highlight("Simple. Fast. Infrastructure-Agnostic."),
		highlight("Features:"),
		success("✓"),
		success("✓"),
		success("✓"),
		success("✓"),
		warning("Tip:"), command("nextdeploy --help"),
	),
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("\n%s %s\n\n",
			success("✨ Welcome to"), highlight("NextDeploy CLI"),
		)

		if len(args) == 0 {
			fmt.Println(highlight("Quick Start:"))
			fmt.Printf("  %s - Initialize a new project\n", command("nextdeploy init"))
			fmt.Printf("  %s - Build the image for the app\n", command("nextdeploy build"))
			fmt.Printf("  %s - Deploy your app on the VPS\n\n", command("nextdeploy ship"))

			fmt.Printf("%s %s\n\n",
				warning("Docs →"), command("https://nextdeploy.one/docs"),
			)
		}
	},
}

func Execute() {
	fmt.Println()

	if versionFlag {
		fmt.Println("NextDeploy version:", shared.Version)
		os.Exit(0)
	}

	if err := rootCmd.Execute(); err != nil {
		fmt.Printf("\n%s %s\n\n",
			errorMsg("❌ Error:"), err,
		)
		os.Exit(1)
	}

	fmt.Println(strings.Repeat("─", 60))
	fmt.Printf("%s %s\n",
		command("Need help?"),
		warning("Visit https://nextdeploy.one/docs"),
	)
	fmt.Println(strings.Repeat("─", 60))
	fmt.Println()
}

func init() {
	rootCmd.PersistentFlags().BoolVarP(&versionFlag, "version", "v", false, "Show version information")
	rootCmd.SetHelpTemplate(fmt.Sprintf(`%s
%s

{{if or .Runnable .HasSubCommands}}{{.UsageString}}{{end}}`,
		title("✨ NextDeploy CLI Toolkit"),
		warning("Usage: {{.UseLine}}"),
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
