//go:build ignore

// Reference copy of shared/ui/theme.go — excluded from the build by the tag
// above so it cannot collide with the real file.

package ui

import "strings"

// Theme is the single source of truth for how the CLI colours its output.
//
// It emits raw ANSI rather than depending on a styling library, for two
// reasons. This package is linked into the daemon, which has no business
// carrying a terminal renderer; and Phase 2 has yet to settle on lipgloss v1
// or v2, so committing to either here would pin the decision from the wrong
// end. cli/internal/tui builds its lipgloss styles from these tokens.
type Theme struct {
	// color gates every sequence; when false Render returns text unchanged.
	color bool
}

// NewTheme returns a Theme honouring c.Color.
func NewTheme(c Capability) Theme { return Theme{color: c.Color} }

// Ink is a semantic colour role. Call sites name intent, never a colour, so
// the palette can move without a sweep through the command tree.
type Ink int

// The palette roles. The values mirror the levels already used by
// shared.Logger so the two render identically once the sink is wired.
const (
	InkDefault Ink = iota
	InkMuted
	InkAccent
	InkSuccess
	InkWarn
	InkError
	InkBold
)

// ansi maps each role to its escape sequence.
var ansi = map[Ink]string{
	InkDefault: "",
	InkMuted:   "\033[38;5;245m",
	InkAccent:  "\033[38;5;12m",
	InkSuccess: "\033[38;5;10m",
	InkWarn:    "\033[38;5;11m",
	InkError:   "\033[38;5;9m",
	InkBold:    "\033[1m",
}

const ansiReset = "\033[0m"

// Render wraps text in the sequence for ink, or returns it untouched when
// colour is disabled.
func (t Theme) Render(ink Ink, text string) string {
	seq := ansi[ink]
	if !t.color || seq == "" || text == "" {
		return text
	}
	return seq + text + ansiReset
}

// Marker returns the leading glyph for a level: the symbol carries the meaning
// when colour is unavailable, which on a CI runner it always is.
func (t Theme) Marker(level Level) string {
	switch level {
	case LevelDebug:
		return t.Render(InkMuted, "·")
	case LevelInfo:
		return t.Render(InkAccent, "•")
	case LevelWarn:
		return t.Render(InkWarn, "!")
	case LevelSuccess:
		return t.Render(InkSuccess, "✓")
	case LevelError:
		return t.Render(InkError, "✗")
	default:
		return " "
	}
}

// Ink returns the colour role matching a reporting level.
func (t Theme) Ink(level Level) Ink {
	switch level {
	case LevelDebug:
		return InkMuted
	case LevelInfo:
		return InkDefault
	case LevelWarn:
		return InkWarn
	case LevelSuccess:
		return InkSuccess
	case LevelError:
		return InkError
	default:
		return InkDefault
	}
}

// Rule returns a horizontal divider sized to width, clamped to something
// sensible when the terminal dimensions are unknown.
func (t Theme) Rule(width int) string {
	if width <= 0 || width > 100 {
		width = 60
	}
	return t.Render(InkMuted, strings.Repeat("─", width))
}
