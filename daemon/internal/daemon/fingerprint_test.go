package daemon

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestParseVersionOutput(t *testing.T) {
	cases := map[string]string{
		"v24.16.0\n":          "24.16.0", // node
		"1.1.0\n":             "1.1.0",   // bun
		"  v20.0.0 ":          "20.0.0",
		"":                    "",
		"v18.1.0\nextra line": "18.1.0", // only first line
	}
	for in, want := range cases {
		if got := parseVersionOutput(in); got != want {
			t.Errorf("parseVersionOutput(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParseGlibcVersion_StripsDistroPackaging(t *testing.T) {
	// The packaging string (2.43-2ubuntu2) changes on every patch; only the
	// trailing version is stable. Both distros below must yield the bare version.
	cases := map[string]string{
		"ldd (Ubuntu GLIBC 2.43-2ubuntu2) 2.43\nmore\n": "2.43",
		"ldd (GNU libc) 2.35\n":                         "2.35",
		"ldd (Debian GLIBC 2.31-13+deb11u7) 2.31":       "2.31",
		"": "",
	}
	for in, want := range cases {
		if got := parseGlibcVersion(in); got != want {
			t.Errorf("parseGlibcVersion(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestHash_StableAndSensitive(t *testing.T) {
	a := &EnvFingerprint{Node: "24.16.0", Bun: "1.1.0", Glibc: "2.43", Arch: "amd64", OS: "linux"}
	b := &EnvFingerprint{Node: "24.16.0", Bun: "1.1.0", Glibc: "2.43", Arch: "amd64", OS: "linux"}
	c := &EnvFingerprint{Node: "22.0.0", Bun: "1.1.0", Glibc: "2.43", Arch: "amd64", OS: "linux"}
	if a.Hash() != b.Hash() {
		t.Error("identical fingerprints must hash equal")
	}
	if a.Hash() == c.Hash() {
		t.Error("differing Node version must change the hash")
	}
}

func TestDriftWarnings(t *testing.T) {
	base := &EnvFingerprint{Node: "24.16.0", Glibc: "2.43", Arch: "amd64", OS: "linux"}

	// No change → no warnings.
	if w := DriftWarnings(base, base); len(w) != 0 {
		t.Errorf("expected no drift, got %v", w)
	}

	// Node + glibc changed → two warnings naming the fields.
	moved := &EnvFingerprint{Node: "22.0.0", Glibc: "2.31", Arch: "amd64", OS: "linux"}
	w := DriftWarnings(base, moved)
	joined := strings.Join(w, "\n")
	if !strings.Contains(joined, "node changed") || !strings.Contains(joined, "glibc changed") {
		t.Errorf("expected node+glibc drift, got %v", w)
	}
	if strings.Contains(joined, "arch changed") {
		t.Errorf("arch did not change; should not be reported: %v", w)
	}
}

func TestDriftWarnings_UnknownBaselineNotReported(t *testing.T) {
	// A field that was empty (unknown) at baseline must not be flagged as drift.
	base := &EnvFingerprint{Node: "24.16.0", Bun: "", Glibc: "2.43"}
	now := &EnvFingerprint{Node: "24.16.0", Bun: "1.1.0", Glibc: "2.43"}
	if w := DriftWarnings(base, now); len(w) != 0 {
		t.Errorf("empty baseline field should not be drift, got %v", w)
	}
}

func TestDriftWarnings_NilSafe(t *testing.T) {
	if w := DriftWarnings(nil, &EnvFingerprint{}); w != nil {
		t.Errorf("nil baseline should yield nil, got %v", w)
	}
}

func TestCaptureEnvFingerprint_AlwaysHasArchOS(t *testing.T) {
	fp := CaptureEnvFingerprint()
	if fp.Arch != runtime.GOARCH || fp.OS != runtime.GOOS {
		t.Errorf("arch/os should come from the Go runtime: %+v", fp)
	}
}

func TestCheckRuntimeDrift_RecordsBaselineThenDetects(t *testing.T) {
	sm := NewStateManager(filepath.Join(t.TempDir(), "state.json"))

	// First call: no baseline → records, no warnings.
	if w := CheckRuntimeDrift(sm); len(w) != 0 {
		t.Errorf("first call should record baseline with no warnings, got %v", w)
	}
	if sm.GetFingerprint() == nil {
		t.Fatal("baseline fingerprint should be recorded")
	}

	// Simulate an out-of-band host change by mutating the stored baseline.
	base := sm.GetFingerprint()
	sm.SetFingerprint(&EnvFingerprint{
		Node: "0.0.0-old", Glibc: base.Glibc, Arch: base.Arch, OS: base.OS,
	})

	// Next call: baseline differs from the live capture → drift warning.
	w := CheckRuntimeDrift(sm)
	if len(w) == 0 || !strings.Contains(strings.Join(w, "\n"), "node changed") {
		t.Errorf("expected node drift warning, got %v", w)
	}
}

func TestStateManager_FingerprintPersists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	sm := NewStateManager(path)
	sm.SetFingerprint(&EnvFingerprint{Node: "24.16.0", Glibc: "2.43", Arch: "amd64", OS: "linux"})
	if err := sm.Save(); err != nil {
		t.Fatal(err)
	}
	// Reload from disk.
	sm2 := NewStateManager(path)
	fp := sm2.GetFingerprint()
	if fp == nil || fp.Node != "24.16.0" || fp.Glibc != "2.43" {
		t.Errorf("fingerprint did not persist/reload: %+v", fp)
	}
}
