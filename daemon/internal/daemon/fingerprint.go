package daemon

import (
	"crypto/sha256"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

// EnvFingerprint captures the host runtime parameters a locally-compiled
// Next.js artifact is sensitive to. The baseline is recorded on the first
// deploy and re-checked on every subsequent one, so an out-of-band host change
// (an `apt upgrade` that moves glibc, a manual Node bump) surfaces as a clear
// operator warning instead of a silent crash loop on the next restart.
//
// This is the bare-metal equivalent of Docker's image pinning — without a
// container runtime.
type EnvFingerprint struct {
	Node  string `json:"node"`  // e.g. "24.16.0"
	Bun   string `json:"bun"`   // e.g. "1.1.0" ("" if not installed)
	Glibc string `json:"glibc"` // e.g. "2.43"  (version only; distro packaging stripped)
	Arch  string `json:"arch"`  // GOARCH of the daemon == host arch
	OS    string `json:"os"`    // GOOS
}

// CaptureEnvFingerprint probes the host. Every probe is best-effort: a missing
// tool yields an empty field rather than an error, so a partial fingerprint is
// still useful (and Arch/OS are always present from the Go runtime).
func CaptureEnvFingerprint() *EnvFingerprint {
	return &EnvFingerprint{
		Node:  parseVersionOutput(runCmdOutput("node", "-v")),
		Bun:   parseVersionOutput(runCmdOutput("bun", "--version")),
		Glibc: parseGlibcVersion(runCmdOutput("ldd", "--version")),
		Arch:  runtime.GOARCH,
		OS:    runtime.GOOS,
	}
}

// runCmdOutput runs a fixed command and returns stdout, or "" on any error.
func runCmdOutput(name string, args ...string) string {
	out, err := exec.Command(name, args...).Output() // #nosec G204 -- fixed, non-user args
	if err != nil {
		return ""
	}
	return string(out)
}

// parseVersionOutput normalizes `node -v` / `bun --version` output: first line,
// trimmed, with a leading "v" stripped. Empty input → "".
func parseVersionOutput(s string) string {
	line := firstLine(s)
	return strings.TrimPrefix(line, "v")
}

// parseGlibcVersion extracts just the glibc version number from `ldd --version`.
// The first line looks like "ldd (Ubuntu GLIBC 2.43-2ubuntu2) 2.43" — the
// trailing token is the stable version; the parenthetical distro-packaging
// string changes on every patch and would cause false drift, so we take the
// last whitespace-separated token. Empty input → "".
func parseGlibcVersion(s string) string {
	line := firstLine(s)
	if line == "" {
		return ""
	}
	fields := strings.Fields(line)
	return fields[len(fields)-1]
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if before, _, ok := strings.Cut(s, "\n"); ok {
		return strings.TrimSpace(before)
	}
	return s
}

// Hash is a stable token over the whole fingerprint, for quick equality checks.
func (ef *EnvFingerprint) Hash() string {
	data := strings.Join([]string{ef.Node, ef.Bun, ef.Glibc, ef.Arch, ef.OS}, "|")
	h := sha256.Sum256([]byte(data))
	return fmt.Sprintf("%x", h)
}

// DriftWarnings returns a human-readable description of every field that changed
// from baseline to current. An empty slice means the environments match. A
// field that was unknown ("") at baseline is not reported as drift (we only flag
// a value we actually recorded changing out from under the deploy).
func DriftWarnings(baseline, current *EnvFingerprint) []string {
	if baseline == nil || current == nil {
		return nil
	}
	var out []string
	cmp := func(field, was, now string) {
		if was != "" && was != now {
			out = append(out, fmt.Sprintf("%s changed: %q -> %q", field, was, now))
		}
	}
	cmp("node", baseline.Node, current.Node)
	cmp("bun", baseline.Bun, current.Bun)
	cmp("glibc", baseline.Glibc, current.Glibc)
	cmp("arch", baseline.Arch, current.Arch)
	cmp("os", baseline.OS, current.OS)
	return out
}

// CheckRuntimeDrift records the baseline fingerprint on first use and, on
// later deploys, returns warnings describing any host runtime drift. The caller
// surfaces them to the operator (daemon log, which streams to the CLI).
func CheckRuntimeDrift(sm *StateManager) []string {
	current := CaptureEnvFingerprint()
	baseline := sm.GetFingerprint()
	if baseline == nil {
		sm.SetFingerprint(current)
		if err := sm.Save(); err != nil {
			return nil
		}
		return nil
	}
	return DriftWarnings(baseline, current)
}
