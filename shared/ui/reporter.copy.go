//go:build ignore

// Reference copy of shared/ui/reporter.go — excluded from the build by the tag
// above so it cannot collide with the real file.

package ui

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
)

// plainReporter is the line-at-a-time Reporter shared by the headless and
// basic sessions. It writes complete lines under a mutex, which is what makes
// it safe for the concurrent SSH tails in `nextdeploy logs` to share one
// session.
//
// cli/internal/tui replaces it with a frame-based renderer; the contract is
// identical so call sites never learn which one they hold.
type plainReporter struct {
	mu    sync.Mutex
	out   io.Writer
	theme Theme
}

// newPlainReporter builds a reporter writing to out, defaulting to stdout.
func newPlainReporter(out io.Writer, theme Theme) *plainReporter {
	if out == nil {
		out = os.Stdout
	}
	return &plainReporter{out: out, theme: theme}
}

// Log emits one formatted line at the given level.
func (r *plainReporter) Log(level Level, format string, args ...any) {
	msg := format
	if len(args) > 0 {
		msg = fmt.Sprintf(format, args...)
	}
	r.line(r.theme.Marker(level) + " " + r.theme.Render(r.theme.Ink(level), msg))
}

// Task opens a unit of work. The plain reporter announces it immediately
// rather than animating it, so a CI transcript reads in causal order.
func (r *plainReporter) Task(name string) Task {
	r.line(r.theme.Render(InkAccent, "▸ ") + name)
	return &plainTask{reporter: r, name: name}
}

// Stream returns a writer that tags each line it receives with label.
func (r *plainReporter) Stream(label string) io.Writer {
	return &streamWriter{reporter: r, label: label}
}

// line writes s followed by a newline, serialised against other writers.
func (r *plainReporter) line(s string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	_, _ = fmt.Fprintln(r.out, s)
}

// plainTask narrates one unit of work as discrete lines.
type plainTask struct {
	reporter *plainReporter
	name     string
}

// Update reports a new status for the task.
func (t *plainTask) Update(status string) {
	t.reporter.line("  " + t.reporter.theme.Render(InkMuted, status))
}

// Done marks the task succeeded.
func (t *plainTask) Done() {
	t.reporter.line("  " + t.reporter.theme.Marker(LevelSuccess) + " " + t.name)
}

// Fail marks the task failed and reports why.
func (t *plainTask) Fail(err error) {
	msg := t.name
	if err != nil {
		msg += ": " + err.Error()
	}
	t.reporter.line("  " + t.reporter.theme.Marker(LevelError) + " " +
		t.reporter.theme.Render(InkError, msg))
}

// streamWriter tags foreign byte output with a source label.
//
// It buffers a partial trailing line between writes: an SSH tail delivers
// arbitrary chunks, and splitting a line across two labelled emissions is how
// interleaved sources become unreadable.
type streamWriter struct {
	reporter *plainReporter
	label    string

	mu   sync.Mutex
	tail strings.Builder
}

// Write implements io.Writer, emitting each complete line it accumulates.
func (w *streamWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.tail.Write(p)
	buffered := w.tail.String()
	w.tail.Reset()

	lines := strings.Split(buffered, "\n")
	w.tail.WriteString(lines[len(lines)-1])

	prefix := w.reporter.theme.Render(InkMuted, "["+w.label+"]") + " "
	for _, line := range lines[:len(lines)-1] {
		w.reporter.line(prefix + line)
	}
	return len(p), nil
}
