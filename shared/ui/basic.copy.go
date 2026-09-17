//go:build ignore

// Reference copy of shared/ui/basic.go — excluded from the build by the tag
// above so it cannot collide with the real file.

package ui

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"golang.org/x/term"
)

// basicSession prompts over plain stdin with no full-screen rendering.
//
// It exists so the package is complete before cli/internal/tui does: every
// call site can migrate onto the Session interface first, and registering the
// rich implementation later upgrades all of them at once without touching a
// single caller. It is also the honest fallback when RegisterInteractive was
// never called, as in the daemon.
type basicSession struct {
	*plainReporter
	answers Answers
	cap     Capability
	in      *os.File
	reader  *bufio.Reader
}

// newBasic builds a plain-stdin interactive session.
func newBasic(cfg Config, c Capability) *basicSession {
	in := cfg.In
	if in == nil {
		in = os.Stdin
	}
	return &basicSession{
		plainReporter: newPlainReporter(cfg.Out, NewTheme(c)),
		answers:       cfg.Answers,
		cap:           c,
		in:            in,
		reader:        bufio.NewReader(in),
	}
}

// Capability reports the detected terminal capabilities.
func (s *basicSession) Capability() Capability { return s.cap }

// Close is a no-op: this session leaves the terminal in cooked mode.
func (s *basicSession) Close() error { return nil }

// ask writes a prompt and reads one line, honouring ctx cancellation.
//
// The read runs on its own goroutine because a blocking stdin read cannot be
// interrupted: on cancellation that goroutine is left parked until the process
// exits, which is the accepted cost of making Ctrl+C responsive.
func (s *basicSession) ask(ctx context.Context, prompt string) (string, error) {
	s.line(prompt)

	type result struct {
		line string
		err  error
	}
	ch := make(chan result, 1)
	go func() {
		l, err := s.reader.ReadString('\n')
		ch <- result{line: l, err: err}
	}()

	select {
	case <-ctx.Done():
		return "", ErrAborted
	case r := <-ch:
		if r.err != nil && !errors.Is(r.err, io.EOF) {
			return "", fmt.Errorf("ui: reading answer: %w", r.err)
		}
		if errors.Is(r.err, io.EOF) && strings.TrimSpace(r.line) == "" {
			return "", ErrAborted
		}
		return strings.TrimSpace(r.line), nil
	}
}

// Select prints a numbered menu and reads a choice, re-asking on bad input.
func (s *basicSession) Select(ctx context.Context, spec *SelectSpec) (string, error) {
	if raw, ok := s.answers.answer(spec.Key); ok {
		if opt, found := matchOption(spec.Options, raw); found {
			return opt.Value, nil
		}
	}
	if len(spec.Options) == 0 {
		return "", fmt.Errorf("ui: %s has no options", spec.Key)
	}

	for {
		s.renderMenu(spec)
		answer, err := s.ask(ctx, "  choice: ")
		if err != nil {
			return "", err
		}
		if answer == "" && spec.Default != "" {
			return spec.Default, nil
		}
		n, convErr := strconv.Atoi(answer)
		if convErr != nil || n < 1 || n > len(spec.Options) {
			s.Log(LevelWarn, "enter a number between 1 and %d", len(spec.Options))
			continue
		}
		return spec.Options[n-1].Value, nil
	}
}

// renderMenu writes the question and its numbered options.
func (s *basicSession) renderMenu(spec *SelectSpec) {
	s.line("")
	s.line(s.theme.Render(InkBold, spec.Message))
	if spec.Help != "" {
		s.line("  " + s.theme.Render(InkMuted, spec.Help))
	}
	for i, o := range spec.Options {
		row := fmt.Sprintf("  %d) %s", i+1, o.Label)
		if o.Hint != "" {
			row += "  " + s.theme.Render(InkMuted, o.Hint)
		}
		s.line(row)
	}
}

// MultiSelect reads a comma-separated list of menu numbers.
func (s *basicSession) MultiSelect(ctx context.Context, spec *MultiSelectSpec) ([]string, error) {
	if raw, ok := s.answers.answer(spec.Key); ok {
		return (&headlessSession{
			plainReporter: s.plainReporter,
			answers:       Answers{spec.Key: raw},
			cap:           s.cap,
		}).MultiSelect(ctx, spec)
	}

	s.renderMenu(&SelectSpec{Message: spec.Message, Options: spec.Options, Help: spec.Help})
	answer, err := s.ask(ctx, "  choices (comma-separated, blank for none): ")
	if err != nil {
		return nil, err
	}
	if answer == "" {
		return spec.Default, nil
	}

	parts := strings.Split(answer, ",")
	chosen := make([]string, 0, len(parts))
	for _, p := range parts {
		n, convErr := strconv.Atoi(strings.TrimSpace(p))
		if convErr != nil || n < 1 || n > len(spec.Options) {
			return nil, fmt.Errorf("ui: %q is not a listed choice", strings.TrimSpace(p))
		}
		chosen = append(chosen, spec.Options[n-1].Value)
	}
	return chosen, nil
}

// Input reads a line of free text, re-asking while validation fails.
func (s *basicSession) Input(ctx context.Context, spec *InputSpec) (string, error) {
	if raw, ok := s.answers.answer(spec.Key); ok {
		return raw, nil
	}

	label := spec.Message
	if spec.Default != "" {
		label += " (" + spec.Default + ")"
	}
	for {
		answer, err := s.ask(ctx, s.theme.Render(InkBold, label)+" ")
		if err != nil {
			return "", err
		}
		if answer == "" {
			answer = spec.Default
		}
		if spec.Validate != nil {
			if vErr := spec.Validate(answer); vErr != nil {
				s.Log(LevelWarn, "%v", vErr)
				continue
			}
		}
		return answer, nil
	}
}

// Secret reads a line without echoing it.
//
// Answers is never consulted: a credential on a command line reaches the
// process table and the shell history.
func (s *basicSession) Secret(ctx context.Context, spec *SecretSpec) (string, error) {
	if !term.IsTerminal(int(s.in.Fd())) {
		return "", &NoTTYError{
			Key:     spec.Key,
			Message: spec.Message,
			Reason:  "stdin cannot disable echo, so a secret would be typed in the clear",
		}
	}

	for {
		s.line(s.theme.Render(InkBold, spec.Message) + " ")
		raw, err := readPassword(ctx, int(s.in.Fd()))
		if err != nil {
			return "", err
		}
		answer := strings.TrimSpace(raw)
		if spec.Validate != nil {
			if vErr := spec.Validate(answer); vErr != nil {
				s.Log(LevelWarn, "%v", vErr)
				continue
			}
		}
		return answer, nil
	}
}

// readPassword reads from fd with echo disabled, honouring ctx cancellation.
func readPassword(ctx context.Context, fd int) (string, error) {
	type result struct {
		b   []byte
		err error
	}
	ch := make(chan result, 1)
	go func() {
		b, err := term.ReadPassword(fd)
		ch <- result{b: b, err: err}
	}()

	select {
	case <-ctx.Done():
		return "", ErrAborted
	case r := <-ch:
		if r.err != nil {
			return "", fmt.Errorf("ui: reading secret: %w", r.err)
		}
		return string(r.b), nil
	}
}

// Confirm reads a yes/no answer. An empty line takes the default: pressing
// Enter is a real answer, not a failed prompt.
func (s *basicSession) Confirm(ctx context.Context, spec *ConfirmSpec) (bool, error) {
	if raw, ok := s.answers.answer(spec.Key); ok {
		v, err := strconv.ParseBool(strings.ToLower(raw))
		if err != nil {
			return false, fmt.Errorf("ui: %s=%q is not a boolean", spec.Key, raw)
		}
		return v, nil
	}

	hint := "[y/N]"
	if spec.Default {
		hint = "[Y/n]"
	}
	answer, err := s.ask(ctx, s.theme.Render(InkBold, spec.Message)+" "+hint+": ")
	if err != nil {
		return false, err
	}
	switch strings.ToLower(answer) {
	case "y", "yes":
		return true, nil
	case "":
		return spec.Default, nil
	default:
		return false, nil
	}
}

// ConfirmPhrase requires Expected to be typed back verbatim.
func (s *basicSession) ConfirmPhrase(ctx context.Context, spec *ConfirmPhraseSpec) (bool, error) {
	if raw, ok := s.answers.answer(spec.Key); ok {
		return raw == spec.Expected, nil
	}

	s.Log(LevelWarn, "%s", spec.Message)
	answer, err := s.ask(ctx,
		fmt.Sprintf("  type %s to confirm: ", s.theme.Render(InkBold, spec.Expected)))
	if err != nil {
		return false, err
	}
	return answer == spec.Expected, nil
}
