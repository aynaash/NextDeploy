//go:build ignore

// Reference copy of shared/ui/headless_test.go — excluded from the build by the tag
// above so it cannot collide with the real file.

package ui

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
)

// newTestHeadless builds a headless session writing into buf.
func newTestHeadless(buf *bytes.Buffer, answers Answers) *headlessSession {
	return newHeadless(
		Config{Answers: answers, Out: buf},
		Capability{Reason: "test"},
	)
}

var targetSpec = SelectSpec{
	Key:     "init.target",
	Message: "Where would you like to deploy?",
	Default: "vps",
	Options: []Option{
		{Value: "vps", Label: "VPS (SSH)"},
		{Value: "cloudflare", Label: "Serverless (Cloudflare Workers + R2)"},
	},
}

// TestHeadlessRefusesUnansweredPrompt is the regression guard for the incident
// recorded at cli/cmd/apply.go:132 — an unanswerable question must fail, never
// resolve to a default.
func TestHeadlessRefusesUnansweredPrompt(t *testing.T) {
	var buf bytes.Buffer
	s := newTestHeadless(&buf, nil)

	got, err := s.Select(context.Background(), &targetSpec)
	if !errors.Is(err, ErrNoTTY) {
		t.Fatalf("Select() error = %v, want ErrNoTTY", err)
	}
	if got != "" {
		t.Errorf("Select() = %q, want empty; a default must never apply headlessly", got)
	}

	var noTTY *NoTTYError
	if !errors.As(err, &noTTY) {
		t.Fatalf("error %v is not a *NoTTYError", err)
	}
	if noTTY.Key != targetSpec.Key {
		t.Errorf("NoTTYError.Key = %q, want %q", noTTY.Key, targetSpec.Key)
	}
	if !strings.Contains(err.Error(), targetSpec.Key) {
		t.Errorf("error message %q does not name the key to pre-seed", err.Error())
	}
}

func TestHeadlessResolvesSeededAnswer(t *testing.T) {
	var buf bytes.Buffer
	s := newTestHeadless(&buf, Answers{"init.target": "cloudflare"})

	got, err := s.Select(context.Background(), &targetSpec)
	if err != nil {
		t.Fatalf("Select() error = %v", err)
	}
	if got != "cloudflare" {
		t.Errorf("Select() = %q, want %q", got, "cloudflare")
	}
	if !strings.Contains(buf.String(), "init.target=cloudflare") {
		t.Errorf("transcript %q does not record the decision", buf.String())
	}
}

func TestHeadlessRejectsUnlistedAnswer(t *testing.T) {
	var buf bytes.Buffer
	s := newTestHeadless(&buf, Answers{"init.target": "aws"})

	if _, err := s.Select(context.Background(), &targetSpec); err == nil {
		t.Fatal("Select() error = nil, want a rejection of the unlisted value")
	} else if errors.Is(err, ErrNoTTY) {
		t.Errorf("a supplied-but-invalid answer must not read as ErrNoTTY: %v", err)
	}
}

// TestHeadlessSecretAlwaysRefuses documents that a credential is never taken
// from Answers, since that path runs through the shell history.
func TestHeadlessSecretAlwaysRefuses(t *testing.T) {
	var buf bytes.Buffer
	s := newTestHeadless(&buf, Answers{"creds.api-token": "hunter2"})

	got, err := s.Secret(context.Background(), &SecretSpec{
		Key: "creds.api-token", Message: "API token",
	})
	if !errors.Is(err, ErrNoTTY) {
		t.Fatalf("Secret() error = %v, want ErrNoTTY", err)
	}
	if got != "" {
		t.Errorf("Secret() = %q, want empty", got)
	}
}

// TestHeadlessConfirmDeclineIsNotAnError pins the distinction the whole
// package exists to preserve: a refusal is an answer.
func TestHeadlessConfirmDeclineIsNotAnError(t *testing.T) {
	var buf bytes.Buffer
	s := newTestHeadless(&buf, Answers{"apply.confirm": "false"})

	ok, err := s.Confirm(context.Background(), &ConfirmSpec{
		Key: "apply.confirm", Message: "Apply these changes?",
	})
	if err != nil {
		t.Fatalf("Confirm() error = %v, want nil", err)
	}
	if ok {
		t.Error("Confirm() = true, want false")
	}
}

func TestHeadlessConfirmPhrase(t *testing.T) {
	spec := ConfirmPhraseSpec{
		Key: "destroy.confirm-name", Message: "This destroys the app", Expected: "my-app",
	}

	tests := []struct {
		name    string
		answers Answers
		want    bool
		wantErr error
	}{
		{"match", Answers{spec.Key: "my-app"}, true, nil},
		{"mismatch is a decline", Answers{spec.Key: "other"}, false, nil},
		{"absent is a refusal", nil, false, ErrNoTTY},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			s := newTestHeadless(&buf, tc.answers)

			got, err := s.ConfirmPhrase(context.Background(), &spec)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("ConfirmPhrase() error = %v, want %v", err, tc.wantErr)
			}
			if got != tc.want {
				t.Errorf("ConfirmPhrase() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestHeadlessMultiSelect(t *testing.T) {
	spec := MultiSelectSpec{
		Key:     "init.bindings",
		Message: "Which bindings?",
		Options: []Option{{Value: "kv"}, {Value: "d1"}, {Value: "r2"}},
	}
	var buf bytes.Buffer
	s := newTestHeadless(&buf, Answers{spec.Key: "kv, r2"})

	got, err := s.MultiSelect(context.Background(), &spec)
	if err != nil {
		t.Fatalf("MultiSelect() error = %v", err)
	}
	if strings.Join(got, ",") != "kv,r2" {
		t.Errorf("MultiSelect() = %v, want [kv r2]", got)
	}
}

// TestStreamWriterBuffersPartialLines covers the concurrent SSH tails in
// cli/cmd/logs.go:56-80: a chunk that splits mid-line must not emit twice.
func TestStreamWriterBuffersPartialLines(t *testing.T) {
	var buf bytes.Buffer
	s := newTestHeadless(&buf, nil)
	w := s.Stream("APP")

	if _, err := w.Write([]byte("first line\nsecond ")); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if got := strings.Count(buf.String(), "\n"); got != 1 {
		t.Fatalf("emitted %d lines after a partial write, want 1: %q", got, buf.String())
	}
	if _, err := w.Write([]byte("half\n")); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if !strings.Contains(buf.String(), "[APP] second half") {
		t.Errorf("rejoined line missing from %q", buf.String())
	}
}
