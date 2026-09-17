package ui

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

// headlessSession answers prompts from pre-seeded Answers and refuses
// everything else.
//
// It is a full implementation, not a fallback. Roughly half this CLI's
// invocations arrive from a CI runner, where the correct behaviour for an
// unanswered question is a loud failure — never a silent default.
type headlessSession struct {
	*plainReporter
	answers Answers
	cap     Capability
}

// newHeadless builds a session for a terminal that cannot be asked anything.
func newHeadless(cfg Config, c Capability) *headlessSession {
	return &headlessSession{
		plainReporter: newPlainReporter(cfg.Out, NewTheme(c)),
		answers:       cfg.Answers,
		cap:           c,
	}
}

// Capability reports the detected terminal capabilities.
func (s *headlessSession) Capability() Capability { return s.cap }

// Close is a no-op: a headless session holds no terminal state.
func (s *headlessSession) Close() error { return nil }

// refuse builds the error for a question that cannot be asked.
func (s *headlessSession) refuse(key, message string) error {
	reason := s.cap.Reason
	if reason == "" {
		reason = "session is not interactive"
	}
	return &NoTTYError{Key: key, Message: message, Reason: reason}
}

// echo records a resolved answer on the transcript, so a CI log shows what was
// decided on the operator's behalf and by which key.
func (s *headlessSession) echo(key, message, value string) {
	s.Log(LevelInfo, "%s %s [%s=%s]",
		s.theme.Render(InkMuted, "?"), message, key, value)
}

// Select resolves a single choice from Answers.
func (s *headlessSession) Select(_ context.Context, spec *SelectSpec) (string, error) {
	raw, ok := s.answers.answer(spec.Key)
	if !ok {
		return "", s.refuse(spec.Key, spec.Message)
	}
	opt, found := matchOption(spec.Options, raw)
	if !found {
		return "", fmt.Errorf("ui: %s=%q is not one of %s",
			spec.Key, raw, optionValues(spec.Options))
	}
	s.echo(spec.Key, spec.Message, opt.Value)
	return opt.Value, nil
}

// MultiSelect resolves a comma-separated subset from Answers.
func (s *headlessSession) MultiSelect(_ context.Context, spec *MultiSelectSpec) ([]string, error) {
	raw, ok := s.answers.answer(spec.Key)
	if !ok {
		return nil, s.refuse(spec.Key, spec.Message)
	}
	if raw == "" {
		s.echo(spec.Key, spec.Message, "(none)")
		return nil, nil
	}

	parts := strings.Split(raw, ",")
	chosen := make([]string, 0, len(parts))
	for _, p := range parts {
		opt, found := matchOption(spec.Options, strings.TrimSpace(p))
		if !found {
			return nil, fmt.Errorf("ui: %s contains %q, not one of %s",
				spec.Key, strings.TrimSpace(p), optionValues(spec.Options))
		}
		chosen = append(chosen, opt.Value)
	}
	s.echo(spec.Key, spec.Message, strings.Join(chosen, ","))
	return chosen, nil
}

// Input resolves free text from Answers.
func (s *headlessSession) Input(_ context.Context, spec *InputSpec) (string, error) {
	raw, ok := s.answers.answer(spec.Key)
	if !ok {
		return "", s.refuse(spec.Key, spec.Message)
	}
	if spec.Validate != nil {
		if err := spec.Validate(raw); err != nil {
			return "", fmt.Errorf("ui: %s is invalid: %w", spec.Key, err)
		}
	}
	s.echo(spec.Key, spec.Message, raw)
	return raw, nil
}

// Secret always refuses.
//
// A credential supplied as a pre-seeded answer would have travelled on a
// command line, into the process table and the shell history. Non-interactive
// runs take secrets from the credential store or the environment instead.
func (s *headlessSession) Secret(_ context.Context, spec *SecretSpec) (string, error) {
	return "", &NoTTYError{
		Key:     spec.Key,
		Message: spec.Message,
		Reason:  s.cap.Reason + "; secrets are never read from --answer",
	}
}

// Confirm resolves a yes/no answer.
func (s *headlessSession) Confirm(_ context.Context, spec *ConfirmSpec) (bool, error) {
	raw, ok := s.answers.answer(spec.Key)
	if !ok {
		return false, s.refuse(spec.Key, spec.Message)
	}
	v, err := strconv.ParseBool(strings.ToLower(raw))
	if err != nil {
		return false, fmt.Errorf("ui: %s=%q is not a boolean", spec.Key, raw)
	}
	s.echo(spec.Key, spec.Message, strconv.FormatBool(v))
	return v, nil
}

// ConfirmPhrase checks a pre-seeded answer against the required phrase.
//
// The mismatch is (false, nil), not an error: the operator supplied an answer
// and it did not match, which is a decline like any other.
func (s *headlessSession) ConfirmPhrase(_ context.Context, spec *ConfirmPhraseSpec) (bool, error) {
	raw, ok := s.answers.answer(spec.Key)
	if !ok {
		return false, s.refuse(spec.Key, spec.Message)
	}
	matched := raw == spec.Expected
	s.echo(spec.Key, spec.Message, strconv.FormatBool(matched))
	return matched, nil
}

// matchOption finds the option whose Value equals raw.
func matchOption(opts []Option, raw string) (Option, bool) {
	for _, o := range opts {
		if o.Value == raw {
			return o, true
		}
	}
	return Option{}, false
}

// optionValues renders the accepted values for an error message.
func optionValues(opts []Option) string {
	vals := make([]string, 0, len(opts))
	for _, o := range opts {
		vals = append(vals, o.Value)
	}
	return strings.Join(vals, ", ")
}
