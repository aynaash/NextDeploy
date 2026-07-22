package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

type phase struct {
	Num       int
	Title     string
	Narrative string
	Ref       string
	Function  string
	Input     string
	Output    string
	Notes     []string
}

type explanation struct {
	Name        string
	Synopsis    string
	Summary     string
	Phases      []phase
	SubPipeline *subPipeline
	DataFlow    string
}

type subPipeline struct {
	Title  string
	Entry  string 
	Phases []phase
}

func registerExplain(parent *cobra.Command, e *explanation) {
	var codeFlag bool
	explainCmd := &cobra.Command{
		Use:   "explain",
		Short: "Explain what `" + e.Name + "` does end-to-end",
		Long: "Prints a step-by-step explanation of `" + e.Name + "` so you\n" +
			"understand what will happen before you run it.\n\n" +
			"Default output is a plain-English narrative. With --code, each step is\n" +
			"annotated with the file:line of the Go function that runs it and the\n" +
			"input/output types — useful for tracing a real run through the codebase.",
		Run: func(cmd *cobra.Command, args []string) {
			var b strings.Builder
			if codeFlag {
				renderExplainCodeMode(&b, e)
			} else {
				renderExplainNarrativeMode(&b, e)
			}
			fmt.Fprint(os.Stdout, b.String())
		},
	}
	explainCmd.Flags().BoolVar(&codeFlag, "code", false,
		"Show file:line references + data flow + inner sub-pipeline")
	parent.AddCommand(explainCmd)
}


func renderExplainNarrativeMode(b *strings.Builder, e *explanation) {
	fmt.Fprintf(b, "nextdeploy %s — end-to-end pipeline\n", e.Name)
	b.WriteString(strings.Repeat("═", 60) + "\n\n")
	if e.Summary != "" {
		b.WriteString(wrap(e.Summary, 72, "") + "\n\n")
	}

	if len(e.Phases) == 0 {
		b.WriteString("  (no steps documented — this command is atomic)\n\n")
	}
	for _, p := range e.Phases {
		fmt.Fprintf(b, "%2d. %s\n", p.Num, p.Title)
		fmt.Fprintf(b, "    %s\n", wrap(p.Narrative, 72, "    "))
		for _, note := range p.Notes {
			fmt.Fprintf(b, "    — %s\n", wrap(note, 70, "      "))
		}
		b.WriteString("\n")
	}

	b.WriteString(strings.Repeat("─", 60) + "\n")
	b.WriteString("Tip: run with --code for file:line refs and data flow.\n")
}

func renderExplainCodeMode(b *strings.Builder, e *explanation) {
	fmt.Fprintf(b, "nextdeploy %s — code-level trace\n", e.Name)
	b.WriteString(strings.Repeat("═", 60) + "\n\n")
	b.WriteString("Each phase annotated with the Go function that runs it and its\n")
	b.WriteString("primary input/output types. Line numbers reflect the current\n")
	b.WriteString("main branch and may drift — use the function names as the\n")
	b.WriteString("stable anchor.\n\n")

	for _, p := range e.Phases {
		writeCodePhase(b, p)
	}

	if e.SubPipeline != nil {
		b.WriteString(strings.Repeat("─", 60) + "\n")
		b.WriteString(e.SubPipeline.Title + "\n")
		b.WriteString(strings.Repeat("─", 60) + "\n\n")
		if e.SubPipeline.Entry != "" {
			fmt.Fprintf(b, "Entry: %s\n\n", e.SubPipeline.Entry)
		}
		for _, p := range e.SubPipeline.Phases {
			writeCodePhase(b, p)
		}
	}

	if e.DataFlow != "" {
		b.WriteString(strings.Repeat("─", 60) + "\n")
		b.WriteString("Data flow\n\n")
		b.WriteString(e.DataFlow)
		if !strings.HasSuffix(e.DataFlow, "\n") {
			b.WriteString("\n")
		}
	}
}

func writeCodePhase(b *strings.Builder, p phase) {
	fmt.Fprintf(b, "  [%2d] %s\n", p.Num, p.Title)
	if p.Ref != "" {
		fmt.Fprintf(b, "       ref:      %s\n", p.Ref)
	}
	if p.Function != "" {
		fmt.Fprintf(b, "       func:     %s\n", p.Function)
	}
	if p.Input != "" {
		fmt.Fprintf(b, "       input:    %s\n", p.Input)
	}
	if p.Output != "" {
		fmt.Fprintf(b, "       output:   %s\n", p.Output)
	}
	if p.Narrative != "" {
		fmt.Fprintf(b, "       why:      %s\n", wrap(p.Narrative, 60, "                 "))
	}
	for _, note := range p.Notes {
		fmt.Fprintf(b, "       note:     %s\n", wrap(note, 60, "                 "))
	}
	b.WriteString("\n")
}

func wrap(s string, width int, indent string) string {
	words := strings.Fields(s)
	if len(words) == 0 {
		return ""
	}
	var out strings.Builder
	lineLen := 0
	for i, w := range words {
		if i > 0 && lineLen+1+len(w) > width {
			out.WriteString("\n")
			out.WriteString(indent)
			lineLen = 0
		} else if i > 0 {
			out.WriteString(" ")
			lineLen++
		}
		out.WriteString(w)
		lineLen += len(w)
	}
	return out.String()
}
