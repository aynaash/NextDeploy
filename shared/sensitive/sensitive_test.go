package sensitive

import (
	"bytes"
	"strings"
	"testing"
)

func TestRegisterAndScrub(t *testing.T) {
	t.Cleanup(Clear)
	Clear()

	Register("short", "super-secret-token-value")

	got := Scrub("short and super-secret-token-value here")
	if !strings.Contains(got, "short") {
		t.Fatalf("short value (<%d chars) should not be scrubbed: %q", minRedactLen, got)
	}
	if strings.Contains(got, "super-secret-token-value") {
		t.Fatalf("registered secret should be scrubbed: %q", got)
	}
	if !strings.Contains(got, redaction) {
		t.Fatalf("expected replacement %q in %q", redaction, got)
	}

	Clear()
	if after := Scrub("super-secret-token-value"); after != "super-secret-token-value" {
		t.Fatalf("expected Clear to empty registry, got %q", after)
	}
}

func TestScrub_CommonPatterns(t *testing.T) {
	t.Cleanup(Clear)
	Clear()

	tests := []struct {
		name   string
		in     string
		secret string
		keep   string
	}{
		{
			name:   "bearer token",
			in:     "header Bearer abcdef1234567890",
			secret: "abcdef1234567890",
			keep:   "Bearer ",
		},
		{
			name:   "aws AKIA key",
			in:     "key=" + "AKIA" + "IOSFODNN7EXAMPLE",
			secret: "AKIA" + "IOSFODNN7EXAMPLE",
			keep:   "key=",
		},
		{
			name:   "aws ASIA key",
			in:     "key=" + "ASIA" + "IOSFODNN7EXAMPLE",
			secret: "ASIA" + "IOSFODNN7EXAMPLE",
			keep:   "key=",
		},
		{
			name:   "authorization header",
			in:     "Authorization: Basic_abcdef123456",
			secret: "Basic_abcdef123456",
			keep:   "Authorization: ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := Scrub(tt.in)
			if strings.Contains(out, tt.secret) {
				t.Fatalf("secret %q leaked in %q", tt.secret, out)
			}
			if !strings.Contains(out, tt.keep+redaction) {
				t.Fatalf("got %q, want substring %q", out, tt.keep+redaction)
			}
		})
	}
}

func TestSafeWriters(t *testing.T) {
	t.Cleanup(Clear)
	Clear()
	Register("very-secret-credential")

	var buf bytes.Buffer
	if _, err := Fprint(&buf, "val=", "very-secret-credential"); err != nil {
		t.Fatalf("Fprint: %v", err)
	}
	if _, err := Fprintln(&buf, " line=", "very-secret-credential"); err != nil {
		t.Fatalf("Fprintln: %v", err)
	}
	if _, err := Fprintf(&buf, "fmt=%s", "very-secret-credential"); err != nil {
		t.Fatalf("Fprintf: %v", err)
	}

	out := buf.String()
	if strings.Contains(out, "very-secret-credential") {
		t.Fatalf("secret leaked in writer output: %q", out)
	}
	if strings.Count(out, redaction) != 3 {
		t.Fatalf("expected 3 redactions, got %d in %q", strings.Count(out, redaction), out)
	}
}
