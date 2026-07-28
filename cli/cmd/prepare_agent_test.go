package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestBootstrapScriptIsPOSIXShellSyntax(t *testing.T) {
	// The script is piped into whatever /bin/sh the target has. A bashism here
	// fails on dash/ash with a syntax error that says nothing useful, so pin
	// that it parses under a POSIX shell.
	sh, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("no sh available")
	}
	p := filepath.Join(t.TempDir(), "bootstrap.sh")
	if err := os.WriteFile(p, []byte(bootstrapScript("--skip-runtimes")), 0o600); err != nil {
		t.Fatal(err)
	}
	// -n parses without executing.
	if out, err := exec.Command(sh, "-n", p).CombinedOutput(); err != nil {
		t.Fatalf("bootstrap script is not valid POSIX sh: %v\n%s", err, out)
	}
}

func TestBootstrapScriptPassesPrepareFlags(t *testing.T) {
	s := bootstrapScript("--skip-runtimes --skip-hardening")
	if !strings.Contains(s, "nextdeployd prepare --skip-runtimes --skip-hardening") {
		t.Errorf("prepare flags not forwarded:\n%s", s)
	}
}

func TestBootstrapScriptHandlesMissingSudo(t *testing.T) {
	// A minimal image running as root frequently has no sudo binary at all.
	s := bootstrapScript("")
	if !strings.Contains(s, `[ "$(id -u)" = "0" ]`) {
		t.Error("script does not check for root before requiring sudo")
	}
	if strings.Contains(s, "\nsudo ") {
		t.Error("script invokes sudo unconditionally instead of via $SUDO")
	}
}

func TestBootstrapScriptAcceptsEitherFetcher(t *testing.T) {
	// Requiring a specific downloader would reintroduce exactly the kind of
	// target-side dependency the agent path exists to remove.
	s := bootstrapScript("")
	for _, want := range []string{"command -v curl", "command -v wget"} {
		if !strings.Contains(s, want) {
			t.Errorf("script missing fetcher probe %q", want)
		}
	}
}

func TestBootstrapScriptMapsArchitectures(t *testing.T) {
	s := bootstrapScript("")
	for _, want := range []string{"x86_64)", "aarch64)", "unsupported architecture"} {
		if !strings.Contains(s, want) {
			t.Errorf("script missing arch handling %q", want)
		}
	}
}

func TestBootstrapScriptVerifiesTheDownloadIsABinary(t *testing.T) {
	// GitHub answers a bad release URL with an HTML 404 page. Without this the
	// "binary" gets installed and fails to exec with a useless message.
	s := bootstrapScript("")
	if !strings.Contains(s, "7f 45 4c 46") {
		t.Error("script does not verify the ELF magic of the download")
	}
}

func TestBootstrapScriptArchMappingMatchesReleaseNames(t *testing.T) {
	// Run the script's own case statement for each uname -m we claim to support
	// and check it yields the arch string used in the release asset name.
	sh, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("no sh available")
	}
	cases := map[string]string{"x86_64": "amd64", "aarch64": "arm64", "arm64": "arm64"}
	for in, want := range cases {
		script := `ARCH="` + in + `"
case "$ARCH" in
  x86_64)  ARCH=amd64 ;;
  aarch64) ARCH=arm64 ;;
  arm64)   ARCH=arm64 ;;
  *) exit 1 ;;
esac
printf '%s' "$ARCH"`
		out, err := exec.Command(sh, "-c", script).Output()
		if err != nil {
			t.Errorf("uname -m %q was rejected: %v", in, err)
			continue
		}
		if string(out) != want {
			t.Errorf("uname -m %q mapped to %q, want %q", in, out, want)
		}
	}
}
