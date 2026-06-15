package cmd

import (
	"fmt"
	"os"

	"github.com/aynaash/nextdeploy/shared/telemetry"
	"github.com/spf13/cobra"
)

var telemetryCmd = &cobra.Command{
	Use:   "telemetry [status|on|off]",
	Short: "View or change anonymous deploy telemetry",
	Long: "NextDeploy sends a single anonymous event on a successful deploy to power the\n" +
		"public \"apps shipped\" counter. It records only a random install ID, the deploy\n" +
		"target, and version — never your code, project name, domain, or secrets.\n\n" +
		"Telemetry is opt-out. You can also disable it with DO_NOT_TRACK=1 or\n" +
		"NEXTDEPLOY_TELEMETRY=0.",
	ValidArgs: []string{"status", "on", "off"},
	Args:      cobra.MatchAll(cobra.MaximumNArgs(1), cobra.OnlyValidArgs),
	Run: func(_ *cobra.Command, args []string) {
		action := "status"
		if len(args) == 1 {
			action = args[0]
		}
		switch action {
		case "on":
			if err := telemetry.Enable(); err != nil {
				fmt.Fprintf(os.Stderr, "failed to enable telemetry: %v\n", err)
				os.Exit(1)
			}
			fmt.Println("✅ telemetry enabled — thanks for helping show NextDeploy's reach.")
		case "off":
			if err := telemetry.Disable(); err != nil {
				fmt.Fprintf(os.Stderr, "failed to disable telemetry: %v\n", err)
				os.Exit(1)
			}
			fmt.Println("✅ telemetry disabled — no events will be sent.")
		default:
			fmt.Println(telemetry.StatusLine())
		}
	},
}

func init() {
	rootCmd.AddCommand(telemetryCmd)
}
