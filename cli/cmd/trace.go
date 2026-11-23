package cmd

import (
	"fmt"
	"os"

	"github.com/aynaash/nextdeploy/shared/agenttrace"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var (
	traceCheckpointObjective  string
	traceCheckpointAgent      string
	traceCheckpointModel      string
	traceCheckpointUnfinished string

	traceTakeoverOpen   bool
	traceTakeoverEditor string

	traceUIPort int

	traceResumeAgent string
)

var traceCmd = &cobra.Command{
	Use:   "trace",
	Short: "Agent Trace: continuity layer for AI coding agents and human takeover",
	Long: `Track the mental model, prompts, modified files, and deployment status 
of AI coding agents (Antigravity IDE, Claude Code, Cursor, Windsurf, Kiro) with NextDeploy.`,
	Run: func(cmd *cobra.Command, args []string) {
		_ = cmd.Help()
	},
}

var traceStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show active agent session, modified files, and deployment state",
	Run: func(cmd *cobra.Command, args []string) {
		brief, err := agenttrace.GenerateTakeoverBrief(".")
		if err != nil {
			fmt.Printf("%s Could not read agent trace: %v\n", errorMsg("✗"), err)
			os.Exit(1)
		}
		fmt.Println(brief)
	},
}

var traceCheckpointCmd = &cobra.Command{
	Use:   "checkpoint",
	Short: "Snapshot agent thought process, intent, and git diffs to .nextdeploy/trace/",
	Run: func(cmd *cobra.Command, args []string) {
		agentName := traceCheckpointAgent
		if agentName == "" {
			agentName = "autonomous-agent"
		}
		modelName := traceCheckpointModel
		if modelName == "" {
			modelName = "auto"
		}
		objective := traceCheckpointObjective
		if objective == "" && len(args) > 0 {
			objective = args[0]
		}
		if objective == "" {
			objective = "Autonomous engineering task"
		}

		cp := agenttrace.HandoffCheckpoint{
			SessionID:      fmt.Sprintf("sess_%d", os.Getpid()),
			Agent:          agentName,
			Model:          modelName,
			Objective:      objective,
			UnfinishedWork: traceCheckpointUnfinished,
		}

		if err := agenttrace.SaveCheckpoint(".", cp); err != nil {
			fmt.Printf("%s Failed to save checkpoint: %v\n", errorMsg("✗"), err)
			os.Exit(1)
		}

		fmt.Printf("%s Checkpoint saved to %s\n", success("✓"), agenttrace.GetTraceDir("."))
		fmt.Printf("  Agent: %s (%s)\n", highlight(agentName), modelName)
		fmt.Printf("  Objective: %s\n", objective)
		fmt.Println("  Run `nextdeploy trace takeover` or `nextdeploy trace ui` to inspect.")
	},
}

var traceTakeoverCmd = &cobra.Command{
	Use:     "takeover",
	Aliases: []string{"handoff"},
	Short:   "Print scannable Developer Takeover Brief and optionally open touched files in $EDITOR",
	Run: func(cmd *cobra.Command, args []string) {
		brief, err := agenttrace.GenerateTakeoverBrief(".")
		if err != nil {
			fmt.Printf("%s Could not read takeover brief: %v\n", errorMsg("✗"), err)
			os.Exit(1)
		}
		fmt.Println(brief)

		if traceTakeoverOpen {
			fmt.Println(command("→ Opening modified files in editor..."))
			if err := agenttrace.OpenTakeoverInEditor(".", traceTakeoverEditor); err != nil {
				fmt.Printf("%s Error opening editor: %v\n", warning("!"), err)
			}
		}
	},
}

var traceUICmd = &cobra.Command{
	Use:   "ui",
	Short: "Launch interactive local web dashboard at http://localhost:4044",
	Run: func(cmd *cobra.Command, args []string) {
		port := traceUIPort
		if port == 0 {
			port = 4044
		}
		fmt.Printf("%s Starting NextDeploy Agent Trace Dashboard at %s...\n",
			title("🚀 NextDeploy"),
			color.CyanString("http://localhost:%d", port),
		)
		fmt.Println(color.HiBlackString("Press Ctrl+C to stop."))

		if err := agenttrace.StartTraceUI(".", port); err != nil {
			fmt.Printf("%s Server error: %v\n", errorMsg("✗"), err)
			os.Exit(1)
		}
	},
}

var traceResumeCmd = &cobra.Command{
	Use:   "resume",
	Short: "Resume the session in another AI agent without losing context",
	Run: func(cmd *cobra.Command, args []string) {
		targetAgent := traceResumeAgent
		if targetAgent == "" {
			targetAgent = "claude"
		}

		cp, err := agenttrace.ReadLatestCheckpoint(".")
		if err != nil || cp == nil {
			fmt.Printf("%s No checkpoint found in %s. Run `nextdeploy trace checkpoint` first.\n",
				warning("!"), agenttrace.GetTraceDir("."))
			os.Exit(1)
		}

		modFiles, _ := agenttrace.GetGitModifiedFiles(".")

		fmt.Printf("\n%s Ready to transfer to %s:\n\n", success("✓"), highlight(targetAgent))
		fmt.Printf("--- AGENT HANDOFF PROMPT ---\n")
		fmt.Printf("Resume session %s (%s via %s)\n", cp.SessionID, cp.Model, cp.Agent)
		fmt.Printf("Objective: %s\n", cp.Objective)
		if cp.UnfinishedWork != "" {
			fmt.Printf("Unfinished work: %s\n", cp.UnfinishedWork)
		}
		fmt.Printf("Modified files (%d):\n", len(modFiles))
		for _, f := range modFiles {
			fmt.Printf("  - %s\n", f)
		}
		fmt.Printf("----------------------------\n\n")
	},
}

func init() {
	// Checkpoint flags
	traceCheckpointCmd.Flags().StringVarP(&traceCheckpointObjective, "objective", "m", "", "Intent or goal of the agent session")
	traceCheckpointCmd.Flags().StringVar(&traceCheckpointAgent, "agent", "", "Agent name (e.g. antigravity, claude, cursor)")
	traceCheckpointCmd.Flags().StringVar(&traceCheckpointModel, "model", "", "Model identifier (e.g. gemini-3.8-flash, claude-3-7-sonnet)")
	traceCheckpointCmd.Flags().StringVar(&traceCheckpointUnfinished, "unfinished", "", "Summary of remaining work")

	// Takeover flags
	traceTakeoverCmd.Flags().BoolVarP(&traceTakeoverOpen, "open", "o", false, "Open all modified files directly in $EDITOR")
	traceTakeoverCmd.Flags().StringVar(&traceTakeoverEditor, "editor", "", "Editor binary to use (default: $EDITOR, nvim, code, vim, nano)")

	// UI flags
	traceUICmd.Flags().IntVarP(&traceUIPort, "port", "p", 4044, "Local port for the web dashboard")

	// Resume flags
	traceResumeCmd.Flags().StringVarP(&traceResumeAgent, "agent", "a", "claude", "Target AI agent (claude, cursor, antigravity)")

	// Attach subcommands
	traceCmd.AddCommand(traceStatusCmd)
	traceCmd.AddCommand(traceCheckpointCmd)
	traceCmd.AddCommand(traceTakeoverCmd)
	traceCmd.AddCommand(traceUICmd)
	traceCmd.AddCommand(traceResumeCmd)

	rootCmd.AddCommand(traceCmd)

	// Direct root alias: nextdeploy takeover [-o]
	rootCmd.AddCommand(&cobra.Command{
		Use:   "takeover",
		Short: "Print Developer Takeover Brief and optionally open touched files in $EDITOR",
		Run:   traceTakeoverCmd.Run,
	})
}
