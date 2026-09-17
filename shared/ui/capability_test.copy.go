//go:build ignore

// Reference copy of shared/ui/capability_test.go — excluded from the build by the tag
// above so it cannot collide with the real file.

package ui

import (
	"bytes"
	"os"
	"testing"
)

// clearEnv neutralises every variable Detect consults, so a test asserts on
// what it sets rather than on the developer's shell or the CI runner it lands
// on. Without it this file passes locally and inverts under GitHub Actions.
func clearEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{
		envNoTTY, envCI, envActions, envTerm, envNoColor, envForceColr,
	} {
		t.Setenv(k, "")
	}
}

// TestDetectNonInteractiveWithoutTerminal covers the ordinary `go test` and
// piped-stdin case: no terminal, so no questions.
func TestDetectNonInteractiveWithoutTerminal(t *testing.T) {
	clearEnv(t)

	c := Detect(DetectOptions{Out: &bytes.Buffer{}})
	if c.Interactive {
		t.Error("Interactive = true with no terminal attached")
	}
	if c.Reason == "" {
		t.Error("Reason is empty; a refusal must explain itself")
	}
}

func TestDetectPrecedence(t *testing.T) {
	tests := []struct {
		name       string
		env        map[string]string
		opts       DetectOptions
		interactiv bool
		wantColor  bool
	}{
		{
			name: "CI wins over a forced colour",
			env:  map[string]string{envCI: "true", envForceColr: "1"},
			// Colour is still on: a CI log renders ANSI, it just cannot answer.
			wantColor: true,
		},
		{
			name: "NO_COLOR strips colour",
			env:  map[string]string{envNoColor: "1", envForceColr: "1"},
		},
		{
			name: "TERM=dumb strips colour",
			env:  map[string]string{envTerm: "dumb", envForceColr: "1"},
		},
		{
			name:       "ForceTTY overrides detection",
			opts:       DetectOptions{ForceTTY: true},
			interactiv: true,
		},
		{
			name:       "NoTTY forces non-interactive",
			opts:       DetectOptions{NoTTY: true},
			interactiv: false,
		},
		{
			name:       "NoTTY outranks ForceTTY",
			opts:       DetectOptions{NoTTY: true, ForceTTY: true},
			interactiv: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			clearEnv(t)
			for k, v := range tc.env {
				t.Setenv(k, v)
			}

			opts := tc.opts
			opts.Out = &bytes.Buffer{}
			c := Detect(opts)

			if c.Interactive != tc.interactiv {
				t.Errorf("Interactive = %v, want %v (%s)", c.Interactive, tc.interactiv, c.Reason)
			}
			if c.Color != tc.wantColor {
				t.Errorf("Color = %v, want %v", c.Color, tc.wantColor)
			}
		})
	}
}

// TestDetectRequiresBothEnds guards `nextdeploy plan | less`: a terminal on
// stdin and a pipe on stdout must not be treated as promptable, or the menu
// renders into the pager.
func TestDetectRequiresBothEnds(t *testing.T) {
	clearEnv(t)

	c := Detect(DetectOptions{In: os.Stdin, Out: &bytes.Buffer{}})
	if c.Interactive {
		t.Error("Interactive = true with a non-terminal stdout")
	}
}

func TestThemeHonoursColorCapability(t *testing.T) {
	plain := NewTheme(Capability{Color: false})
	if got := plain.Render(InkError, "boom"); got != "boom" {
		t.Errorf("Render() = %q, want unstyled %q", got, "boom")
	}

	colored := NewTheme(Capability{Color: true})
	if got := colored.Render(InkError, "boom"); got == "boom" {
		t.Error("Render() returned unstyled text with colour enabled")
	}
}

// TestFromContextFallsBackHeadless: a caller that never built a session must
// fail at the prompt, not panic on a nil interface.
func TestFromContextFallsBackHeadless(t *testing.T) {
	if s := FromContext(t.Context()); s == nil {
		t.Fatal("FromContext() = nil, want a headless session")
	}
}
