package cmd

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/aynaash/nextdeploy/cli/internal/server"
	"github.com/aynaash/nextdeploy/shared/config"

	"github.com/fatih/color"
)

// The agent path provisions the server with nextdeployd itself instead of
// Ansible.
//
// Ansible needs a Python interpreter on the managed host — not because the
// provisioning needs one, but because Ansible ships Python modules there and
// executes them. On a small server that is real overhead, and a Python version
// mismatch between the control node and the target fails the run with an error
// that reads like a NextDeploy bug. nextdeployd is a static Go binary: the only
// thing this path needs on the target is a shell and curl (or wget).

// releaseAPIURL is where the target resolves the daemon binary from. It is the
// same source the legacy playbook used, so both paths install identical bits.
const releaseAPIURL = "https://api.github.com/repos/aynaash/nextdeploy/releases/latest"

// bootstrapScript installs nextdeployd on the target and runs `prepare`.
//
// Written as POSIX sh (not bash) with no here-doc trickery so it survives being
// passed through an SSH command line on a minimal image. Every step is
// re-runnable: an already-current binary is reused rather than re-downloaded.
func bootstrapScript(prepareArgs string) string {
	return `set -eu

# A minimal image running as root often has no sudo at all, so don't assume it.
if [ "$(id -u)" = "0" ]; then
  SUDO=""
elif command -v sudo >/dev/null 2>&1; then
  SUDO="sudo"
else
  echo "not root and sudo is unavailable — nextdeployd prepare needs to write to /opt, /etc and /usr/local" >&2
  exit 1
fi

ARCH="$(uname -m)"
case "$ARCH" in
  x86_64)  ARCH=amd64 ;;
  aarch64) ARCH=arm64 ;;
  arm64)   ARCH=arm64 ;;
  *) echo "unsupported architecture: $ARCH" >&2; exit 1 ;;
esac

# curl or wget — a minimal image may have only one, and requiring a specific
# fetcher would reintroduce exactly the kind of dependency this path removes.
if command -v curl >/dev/null 2>&1; then
  FETCH="curl -fsSL --retry 3 --retry-delay 2"
  FETCH_OUT="curl -fsSL --retry 3 --retry-delay 2 -o"
elif command -v wget >/dev/null 2>&1; then
  FETCH="wget -qO-"
  FETCH_OUT="wget -qO"
else
  echo "neither curl nor wget is available on this host — install one and re-run" >&2
  exit 1
fi

TAG="$($FETCH ` + releaseAPIURL + ` | grep '"tag_name"' | head -1 | sed 's/.*"tag_name": *"\([^"]*\)".*/\1/')"
if [ -z "$TAG" ]; then
  echo "could not resolve the latest nextdeploy release tag (no network, or GitHub rate limit)" >&2
  exit 1
fi

URL="https://github.com/aynaash/nextdeploy/releases/download/${TAG}/nextdeployd-linux-${ARCH}"
echo "Fetching nextdeployd ${TAG} for linux/${ARCH}..."
$FETCH_OUT /tmp/nextdeployd.new "$URL"

# A 404 from GitHub is an HTML page, not a binary. Catch it here rather than
# letting the "binary" fail to exec with a confusing message.
if ! head -c 4 /tmp/nextdeployd.new | od -An -tx1 | grep -q '7f 45 4c 46'; then
  echo "downloaded file is not an ELF binary (URL: $URL)" >&2
  rm -f /tmp/nextdeployd.new
  exit 1
fi

$SUDO install -m 0755 /tmp/nextdeployd.new /usr/local/bin/nextdeployd
rm -f /tmp/nextdeployd.new
echo "Installed: $(/usr/local/bin/nextdeployd version)"

$SUDO /usr/local/bin/nextdeployd prepare ` + prepareArgs + `
`
}

// runPrepareAgent provisions serverName by installing nextdeployd there and
// running its prepare subcommand. Returns an error rather than exiting so the
// caller owns the exit path.
func runPrepareAgent(ctx context.Context, serverName string, serverCfg config.ServerConfig, out io.Writer, opts agentOptions) error {
	_, _ = color.New(color.FgCyan).Fprintf(out,
		"\n Preparing server: %s (%s) — agent mode, no Python/Ansible required\n\n",
		serverName, serverCfg.Host)

	srv, err := server.New(server.WithConfig(), server.WithSSH())
	if err != nil {
		return fmt.Errorf("connect to %s: %w", serverName, err)
	}
	defer srv.CloseSSHConnection()

	var args []string
	if opts.SkipRuntimes {
		args = append(args, "--skip-runtimes")
	}
	if opts.SkipHardening {
		args = append(args, "--skip-hardening")
	}

	script := bootstrapScript(strings.Join(args, " "))

	// Stream so the operator watches each step land, and so a hang is visible
	// rather than looking like a silent stall.
	if _, err := srv.ExecuteCommand(ctx, serverName, script, out); err != nil {
		return fmt.Errorf("prepare on %s: %w", serverName, err)
	}
	return nil
}

// agentOptions mirrors the daemon's prepare flags so the CLI can pass them on.
type agentOptions struct {
	SkipRuntimes  bool
	SkipHardening bool
}

// legacyAnsibleNotice is printed when the operator opts back into the playbook,
// so the Python requirement is a stated choice rather than a surprise.
func legacyAnsibleNotice(out io.Writer) {
	_, _ = color.New(color.FgYellow).Fprintln(out,
		"Using the legacy Ansible path. It requires ansible-playbook locally AND a Python\n"+
			"interpreter on the target. The default agent path (`nextdeploy prepare`) needs\n"+
			"neither — it runs the same provisioning from the nextdeployd static binary.")
}
