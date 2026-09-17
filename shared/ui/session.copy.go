//go:build ignore

// Reference copy of shared/ui/session.go — excluded from the build by the tag
// above so it cannot collide with the real file.

package ui

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

// Session owns the terminal for the duration of one command invocation.
//
// Prompting and reporting live on one interface on purpose: they contend for
// the same exclusive resource. Split across two objects, nothing would stop a
// task spinner from rendering underneath an open prompt.
type Session interface {
	Prompter
	Reporter

	// Capability reports what this session's terminal can do.
	Capability() Capability
	// Close releases the terminal and restores any redirected output.
	Close() error
}

// Prompter asks an operator a question. Every method returns ErrNoTTY when
// there is no operator and no pre-seeded answer, and ErrAborted when the
// operator interrupts. A declined confirmation is (false, nil) — a refusal is
// an answer, not a failure.
type Prompter interface {
	Select(ctx context.Context, s *SelectSpec) (string, error)
	MultiSelect(ctx context.Context, s *MultiSelectSpec) ([]string, error)
	Input(ctx context.Context, s *InputSpec) (string, error)
	Secret(ctx context.Context, s *SecretSpec) (string, error)
	Confirm(ctx context.Context, s *ConfirmSpec) (bool, error)
	ConfirmPhrase(ctx context.Context, s *ConfirmPhraseSpec) (bool, error)
}

// Reporter narrates work. While a session is live it is the only thing
// permitted to write to the terminal.
type Reporter interface {
	// Task opens a unit of work that will later succeed or fail.
	Task(name string) Task
	// Log emits one line at the given level.
	Log(level Level, format string, args ...any)
	// Stream returns a writer for byte output produced elsewhere — an SSH
	// session, a subprocess — tagged with label.
	Stream(label string) io.Writer
}

// Task is one unit of narrated work.
type Task interface {
	// Update replaces the task's current status line.
	Update(status string)
	// Done marks the task succeeded.
	Done()
	// Fail marks the task failed and records why.
	Fail(err error)
}

// Level is the severity of a reported line.
//
// It is declared here rather than reused from shared.LogLevel because
// shared.Logger routes its output into a Session: importing shared from this
// package would close an import cycle.
type Level int

// Reporting levels, ordered by increasing severity.
const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelSuccess
	LevelError
)

// Option is one choice in a Select or MultiSelect.
//
// Value and Label are separate so that callers switch on a constant while the
// displayed prose stays free to change. Matching on display text is how a menu
// label becomes load-bearing control flow.
type Option struct {
	// Value is what the caller compares against.
	Value string
	// Label is what the operator reads.
	Label string
	// Hint is dimmed trailing detail, optional.
	Hint string
}

// SelectSpec asks for exactly one of a fixed set of options.
type SelectSpec struct {
	Key     string
	Message string
	Options []Option
	Default string
	Help    string
}

// MultiSelectSpec asks for any subset of a fixed set of options. A headless
// answer is a comma-separated list of Option values.
type MultiSelectSpec struct {
	Key     string
	Message string
	Options []Option
	Default []string
	Help    string
}

// InputSpec asks for a line of free text.
type InputSpec struct {
	Key      string
	Message  string
	Default  string
	Help     string
	Validate func(string) error
}

// SecretSpec asks for a line of text that must not be echoed.
//
// A SecretSpec is never satisfied from Answers: a credential passed as a
// pre-seeded answer would have arrived on a command line and into the shell
// history. Supply secrets through the credential store or the environment.
type SecretSpec struct {
	Key      string
	Message  string
	Help     string
	Validate func(string) error
}

// ConfirmSpec asks a yes/no question.
type ConfirmSpec struct {
	Key     string
	Message string
	Help    string
	Default bool
}

// ConfirmPhraseSpec requires the operator to type Expected back verbatim. It
// guards destructive work, where a reflexive "y" is not consent.
type ConfirmPhraseSpec struct {
	Key      string
	Message  string
	Expected string
	Help     string
}

// ErrAborted is returned when the operator interrupts a prompt.
var ErrAborted = errors.New("ui: aborted by operator")

// ErrNoTTY is returned when a question cannot be asked and no answer was
// pre-seeded. Callers test for it with errors.Is; NoTTYError carries the
// detail needed to tell the operator what to pass instead.
var ErrNoTTY = errors.New("ui: no interactive terminal and no pre-seeded answer")

// NoTTYError names the unanswerable prompt.
//
// It exists so that a CI failure is self-explaining. The alternative — folding
// "nobody was there to ask" into an ordinary decline — is what once let
// `nextdeploy apply` skip an entire reconcile and exit 0, shipping a Worker
// whose bindings pointed at resources that were never created.
type NoTTYError struct {
	// Key is the prompt's stable identity, the one to pre-seed.
	Key string
	// Message is the question that went unasked.
	Message string
	// Reason is the Capability reason the session was not interactive.
	Reason string
}

// Error implements error.
func (e *NoTTYError) Error() string {
	return fmt.Sprintf(
		"ui: cannot ask %q (%s) - %s; supply it non-interactively with --answer %s=<value>",
		e.Message, e.Key, e.Reason, e.Key,
	)
}

// Unwrap lets callers match with errors.Is(err, ErrNoTTY).
func (e *NoTTYError) Unwrap() error { return ErrNoTTY }

// Answers pre-seeds prompts by Key, so that a flag-supplied value is a
// supplied answer rather than a skipped question. Every prompt added later
// gains non-interactive support without a new flag branch.
type Answers map[string]string

// Config constructs a Session.
type Config struct {
	// Answers pre-seeds prompts by Key.
	Answers Answers
	// NoTTY and ForceTTY override capability detection.
	NoTTY    bool
	ForceTTY bool
	// In and Out default to os.Stdin and os.Stdout.
	In  *os.File
	Out io.Writer
}

// interactiveFactory is installed by cli/internal/tui. It stays a hook so that
// this package never imports a terminal rendering library, keeping the daemon
// binary free of one.
var interactiveFactory func(Config, Capability) (Session, error)

// RegisterInteractive installs the rich terminal implementation. Call it once,
// from the CLI entry point, before the first New.
func RegisterInteractive(f func(Config, Capability) (Session, error)) {
	interactiveFactory = f
}

// New resolves the terminal capabilities and returns the matching Session.
//
// Non-interactive sessions are a first-class implementation, not a degraded
// one: roughly half of this CLI's invocations come from a CI runner.
func New(cfg Config) (Session, error) {
	c := Detect(DetectOptions{
		NoTTY:    cfg.NoTTY,
		ForceTTY: cfg.ForceTTY,
		In:       cfg.In,
		Out:      cfg.Out,
	})

	if !c.Interactive {
		return newHeadless(cfg, c), nil
	}
	if interactiveFactory != nil {
		return interactiveFactory(cfg, c)
	}
	return newBasic(cfg, c), nil
}

// answer looks up a pre-seeded value for key.
func (a Answers) answer(key string) (string, bool) {
	if a == nil {
		return "", false
	}
	v, ok := a[key]
	return strings.TrimSpace(v), ok
}

// contextKey is the unexported key type for the Session stored in a context.
type contextKey struct{}

// NewContext returns ctx carrying s, so that a cobra command tree can resolve
// one session per invocation without threading it through every signature.
func NewContext(ctx context.Context, s Session) context.Context {
	return context.WithValue(ctx, contextKey{}, s)
}

// FromContext returns the Session carried by ctx.
//
// It falls back to a headless session rather than returning nil, so a caller
// reached from a test or from a path that never built one still fails loudly
// at the prompt with ErrNoTTY instead of panicking.
func FromContext(ctx context.Context) Session {
	if s, ok := ctx.Value(contextKey{}).(Session); ok {
		return s
	}
	return newHeadless(Config{}, Capability{Reason: "no ui.Session in context"})
}
