package server

import (
	"strings"
	"testing"
)

func TestTailBufferKeepsLastBytes(t *testing.T) {
	tb := &tailBuffer{limit: 8}

	if _, err := tb.Write([]byte("abc")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if got := tb.String(); got != "abc" {
		t.Errorf("under limit = %q, want %q", got, "abc")
	}

	// Growing past the limit across several writes keeps the tail.
	if _, err := tb.Write([]byte("defghijk")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if got := tb.String(); got != "defghijk" {
		t.Errorf("across writes = %q, want %q", got, "defghijk")
	}

	// A single write larger than the limit is truncated from the front.
	if _, err := tb.Write([]byte("0123456789")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if got := tb.String(); got != "23456789" {
		t.Errorf("oversized write = %q, want %q", got, "23456789")
	}
}

func TestTailBufferReportsFullWriteLength(t *testing.T) {
	// io.Copy treats a short write as an error, so Write must report len(p)
	// even when it retains less.
	tb := &tailBuffer{limit: 4}
	n, err := tb.Write([]byte("hello world"))
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if n != len("hello world") {
		t.Errorf("n = %d, want %d", n, len("hello world"))
	}
}

func TestTailBufferBoundedUnderStreaming(t *testing.T) {
	tb := &tailBuffer{limit: stderrTailLimit}
	line := strings.Repeat("x", 512) + "\n"
	for i := 0; i < 1000; i++ {
		if _, err := tb.Write([]byte(line)); err != nil {
			t.Fatalf("Write: %v", err)
		}
	}
	if len(tb.buf) > stderrTailLimit {
		t.Errorf("buffer grew to %d bytes, limit %d", len(tb.buf), stderrTailLimit)
	}
}
