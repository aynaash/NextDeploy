package nextbuild

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

func TestFlagsForTarget(t *testing.T) {
	tests := []struct {
		name string
		in   Target
		want []string
	}{
		{"cloudflare → webpack", TargetCloudflareWorker, []string{"--webpack"}},
		{"aws → none", TargetAWSLambda, nil},
		{"vps → none", TargetVPS, nil},
		{"generic → none", TargetGeneric, nil},
		{"unknown → none", Target("bogus"), nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := flagsForTarget(tt.in); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("flagsForTarget(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestScrubBundlerEnv(t *testing.T) {
	tests := []struct {
		name string
		in   []string
		want []string
	}{
		{"strips turbopack", []string{"TURBOPACK=1", "FOO=bar"}, []string{"FOO=bar"}},
		{"strips disable flag", []string{"NEXT_DISABLE_TURBOPACK=1", "A=b"}, []string{"A=b"}},
		{"strips both", []string{"TURBOPACK=1", "X=1", "NEXT_DISABLE_TURBOPACK=0"}, []string{"X=1"}},
		{"keeps unrelated", []string{"PATH=/bin", "HOME=/h"}, []string{"PATH=/bin", "HOME=/h"}},
		{"key substring kept", []string{"MY_TURBOPACK=1"}, []string{"MY_TURBOPACK=1"}},
		{"malformed no equals", []string{"BROKEN"}, []string{"BROKEN"}},
		{"leading equals kept", []string{"=weird"}, []string{"=weird"}},
		{"empty input", []string{}, []string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := scrubBundlerEnv(tt.in); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("scrubBundlerEnv(%v) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestIndexByte(t *testing.T) {
	tests := []struct {
		name string
		s    string
		b    byte
		want int
	}{
		{"found mid", "a=b", '=', 1},
		{"first char", "=x", '=', 0},
		{"not found", "abc", '=', -1},
		{"empty", "", '=', -1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := indexByte(tt.s, tt.b); got != tt.want {
				t.Fatalf("indexByte(%q, %q) = %d, want %d", tt.s, tt.b, tt.want, got)
			}
		})
	}
}

// fakeNext writes a shell script at <dir>/node_modules/.bin/next that records its
// argv and the TURBOPACK env var to recordPath, then exits with exitCode.
func fakeNext(t *testing.T, dir, recordPath string, exitCode int) {
	t.Helper()
	binDir := filepath.Join(dir, "node_modules", ".bin")
	if err := os.MkdirAll(binDir, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	script := "#!/bin/sh\n" +
		"echo \"argv=$*\" > " + recordPath + "\n" +
		"echo \"TURBOPACK=${TURBOPACK}\" >> " + recordPath + "\n" +
		"exit " + strconv.Itoa(exitCode) + "\n"
	// #nosec G306 -- fake next binary must be executable for the test to run it
	if err := os.WriteFile(filepath.Join(binDir, "next"), []byte(script), 0o755); err != nil {
		t.Fatalf("write fake next: %v", err)
	}
}

//nolint:gocyclo // table-driven build test; branch count reflects the cases covered, not real complexity
func TestRun(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake next binary is a /bin/sh script")
	}

	t.Run("empty ProjectDir", func(t *testing.T) {
		err := Run(context.Background(), Opts{})
		if err == nil || !strings.Contains(err.Error(), "ProjectDir is required") {
			t.Fatalf("want ProjectDir error, got %v", err)
		}
	})

	t.Run("missing next binary", func(t *testing.T) {
		dir := t.TempDir()
		err := Run(context.Background(), Opts{ProjectDir: dir, Target: TargetGeneric})
		if err == nil || !strings.Contains(err.Error(), "node_modules/.bin/next not found") {
			t.Fatalf("want missing-binary error, got %v", err)
		}
	})

	t.Run("cloudflare passes --webpack and scrubs TURBOPACK", func(t *testing.T) {
		dir := t.TempDir()
		rec := filepath.Join(dir, "rec.txt")
		fakeNext(t, dir, rec, 0)
		t.Setenv("TURBOPACK", "1")
		if err := Run(context.Background(), Opts{ProjectDir: dir, Target: TargetCloudflareWorker}); err != nil {
			t.Fatalf("Run: %v", err)
		}
		out := readFile(t, rec)
		if !strings.Contains(out, "argv=build --webpack") {
			t.Fatalf("argv = %q, want build --webpack", out)
		}
		if !strings.Contains(out, "TURBOPACK=\n") && !strings.HasSuffix(strings.TrimRight(out, "\n"), "TURBOPACK=") {
			t.Fatalf("expected TURBOPACK scrubbed (empty), got %q", out)
		}
	})

	t.Run("generic passes plain build", func(t *testing.T) {
		dir := t.TempDir()
		rec := filepath.Join(dir, "rec.txt")
		fakeNext(t, dir, rec, 0)
		if err := Run(context.Background(), Opts{ProjectDir: dir, Target: TargetGeneric}); err != nil {
			t.Fatalf("Run: %v", err)
		}
		if out := readFile(t, rec); !strings.Contains(out, "argv=build\n") {
			t.Fatalf("argv = %q, want plain build", out)
		}
	})

	t.Run("extra args appended", func(t *testing.T) {
		dir := t.TempDir()
		rec := filepath.Join(dir, "rec.txt")
		fakeNext(t, dir, rec, 0)
		if err := Run(context.Background(), Opts{ProjectDir: dir, Target: TargetGeneric, ExtraArgs: []string{"--debug"}}); err != nil {
			t.Fatalf("Run: %v", err)
		}
		if out := readFile(t, rec); !strings.Contains(out, "argv=build --debug") {
			t.Fatalf("argv = %q, want build --debug", out)
		}
	})

	t.Run("non-zero exit surfaces error", func(t *testing.T) {
		dir := t.TempDir()
		rec := filepath.Join(dir, "rec.txt")
		fakeNext(t, dir, rec, 1)
		err := Run(context.Background(), Opts{ProjectDir: dir, Target: TargetGeneric})
		if err == nil || !strings.Contains(err.Error(), "next build failed") {
			t.Fatalf("want next build failed, got %v", err)
		}
	})

	t.Run("cancelled context", func(t *testing.T) {
		dir := t.TempDir()
		rec := filepath.Join(dir, "rec.txt")
		fakeNext(t, dir, rec, 0)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if err := Run(ctx, Opts{ProjectDir: dir, Target: TargetGeneric}); err == nil {
			t.Fatal("want error from cancelled context, got nil")
		}
	})
}

func TestResolveNextBinary(t *testing.T) {
	t.Run("finds local bin", func(t *testing.T) {
		dir := t.TempDir()
		binDir := filepath.Join(dir, "node_modules", ".bin")
		if err := os.MkdirAll(binDir, 0o750); err != nil {
			t.Fatal(err)
		}
		bin := filepath.Join(binDir, "next")
		// #nosec G306 -- fake next binary must be executable for the test to run it
		if err := os.WriteFile(bin, []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatal(err)
		}
		got, err := resolveNextBinary(dir)
		if err != nil || got != bin {
			t.Fatalf("resolveNextBinary = %q, %v; want %q, nil", got, err, bin)
		}
	})

	t.Run("dir-as-binary is rejected", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.MkdirAll(filepath.Join(dir, "node_modules", ".bin", "next"), 0o750); err != nil {
			t.Fatal(err)
		}
		t.Setenv("PATH", t.TempDir()) // no npx
		_, err := resolveNextBinary(dir)
		if err == nil {
			t.Fatal("want error when .bin/next is a directory")
		}
	})

	t.Run("missing with npx present", func(t *testing.T) {
		dir := t.TempDir()
		npxDir := t.TempDir()
		// #nosec G306 -- fake npx must be executable for the test to run it
		if err := os.WriteFile(filepath.Join(npxDir, "npx"), []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatal(err)
		}
		t.Setenv("PATH", npxDir)
		_, err := resolveNextBinary(dir)
		if err == nil || !strings.Contains(err.Error(), "install") {
			t.Fatalf("want install hint, got %v", err)
		}
	})

	t.Run("missing without npx", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("PATH", t.TempDir())
		_, err := resolveNextBinary(dir)
		if err == nil || !strings.Contains(err.Error(), "could not locate") {
			t.Fatalf("want could-not-locate error, got %v", err)
		}
	})
}

func readFile(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read %s: %v", p, err)
	}
	return string(b)
}
