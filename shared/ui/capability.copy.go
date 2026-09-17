//go:build ignore

// Reference copy of shared/ui/capability.go — excluded from the build by the tag
// above so it cannot collide with the real file.

// Package ui owns every interactive surface of the CLI: the prompts that ask
// an operator a question and the reporting that narrates long-running work.
//
// It deliberately depends on nothing heavier than golang.org/x/term. The
// daemon binary links this package through shared.Logger, so the terminal
// rendering stack lives in cli/internal/tui and is registered into this
// package at process start — see RegisterInteractive.
package ui

import (
	"io"
	"os"
	"strings"

	"golang.org/x/term"
)

// Capability describes what the attached terminal can do. It is resolved once
// per process by Detect and does not change afterwards.
type Capability struct {
	// Interactive reports whether a question can be put to a human.
	Interactive bool
	// Color reports whether ANSI sequences are safe to emit.
	Color bool
	// Width and Height are the terminal dimensions, zero when unknown.
	Width  int
	Height int
	// Reason names why the session is not Interactive. It is surfaced in
	// NoTTYError so an operator reading a CI log learns why they were never
	// asked, rather than only that they weren't.
	Reason string
}

// DetectOptions carries the explicit overrides that outrank the environment.
type DetectOptions struct {
	// NoTTY forces a non-interactive session even on a real terminal.
	NoTTY bool
	// ForceTTY asserts interactivity the detection would otherwise deny. It
	// does not affect Color.
	ForceTTY bool
	// In and Out default to os.Stdin and os.Stdout when nil.
	In  *os.File
	Out io.Writer
}

// Environment variables consulted by Detect, in the precedence order applied
// by the function body.
const (
	envNoTTY     = "NEXTDEPLOY_NO_TTY"
	envCI        = "CI"
	envActions   = "GITHUB_ACTIONS"
	envTerm      = "TERM"
	envNoColor   = "NO_COLOR"
	envForceColr = "FORCE_COLOR"
)

// Detect resolves the terminal capabilities.
//
// Precedence, highest first: explicit options, NEXTDEPLOY_NO_TTY, the CI
// markers, TERM=dumb, then a real terminal check. Interactivity requires a
// terminal on *both* ends: `nextdeploy plan | less` has a terminal on stdin
// and a pipe on stdout, and rendering a prompt into that pipe is how a menu
// ends up in a pager instead of in front of the operator.
func Detect(opts DetectOptions) Capability {
	in, out := opts.In, opts.Out
	if in == nil {
		in = os.Stdin
	}
	if out == nil {
		out = os.Stdout
	}

	c := Capability{Color: colorAllowed(out)}
	c.Width, c.Height = sizeOf(out)

	switch {
	case opts.NoTTY:
		c.Reason = "--no-tty was passed"
	case truthy(os.Getenv(envNoTTY)):
		c.Reason = envNoTTY + " is set"
	case os.Getenv(envCI) != "" || os.Getenv(envActions) != "":
		c.Reason = "running on a CI runner (" + envCI + " is set)"
	case os.Getenv(envTerm) == "dumb":
		c.Reason = "TERM=dumb"
	case !isTerminal(in):
		c.Reason = "stdin is not a terminal (piped, redirected, or closed)"
	case !isTerminal(out):
		c.Reason = "stdout is not a terminal (piped or redirected)"
	default:
		c.Interactive = true
	}

	// ForceTTY is applied last so it can override the environment, but NoTTY
	// still outranks it: when an operator has asked for both, the safe reading
	// is the one that declines to prompt.
	if opts.ForceTTY && !opts.NoTTY {
		c.Interactive = true
		c.Reason = ""
	}
	return c
}

// colorAllowed applies the NO_COLOR convention. FORCE_COLOR overrides a
// non-terminal stdout so piping into a file that is later `cat`-ed keeps its
// styling, which is how the deploy reports are usually read back.
func colorAllowed(out io.Writer) bool {
	if os.Getenv(envNoColor) != "" || os.Getenv(envTerm) == "dumb" {
		return false
	}
	if truthy(os.Getenv(envForceColr)) {
		return true
	}
	return isTerminal(out)
}

// isTerminal reports whether w is backed by a character device.
func isTerminal(w any) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	return term.IsTerminal(int(f.Fd()))
}

// sizeOf returns the terminal dimensions, or zeroes when out is not a
// terminal or the ioctl fails.
func sizeOf(out io.Writer) (width, height int) {
	f, ok := out.(*os.File)
	if !ok {
		return 0, 0
	}
	w, h, err := term.GetSize(int(f.Fd()))
	if err != nil {
		return 0, 0
	}
	return w, h
}

// truthy reads the loose boolean spelling used by shell environments.
func truthy(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}
