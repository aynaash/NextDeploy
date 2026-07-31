package prepare

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

// Paths for the edge (Caddy) and control-plane (nextdeployd) layers. Same rule
// as the constants in plan.go: the daemon assumes these at runtime, so they are
// named once and shared rather than spelled out per step.
const (
	CaddyfilePath  = "/etc/caddy/Caddyfile"
	CaddyUnitPath  = "/etc/systemd/system/caddy.service"
	DaemonConfDir  = "/etc/nextdeployd"
	DaemonConfPath = DaemonConfDir + "/config.json"
	DaemonUnitPath = "/etc/systemd/system/nextdeployd.service"
	DaemonSockDir  = "/run/nextdeployd"
	DaemonSockPath = DaemonSockDir + "/nextdeployd.sock"

	// corazaPlugin is the module the generated Caddy fragments require. Every
	// fragment carries a coraza_waf block and the main Caddyfile sets
	// `order coraza_waf first`, so a Caddy without it fails `caddy validate`
	// and blocks every deploy — the module is not optional here.
	corazaPlugin = "github.com/corazawaf/coraza-caddy/v2"
)

// socketWaitTimeout bounds the wait for the daemon to bind its socket after
// start. A var so tests don't pay for it.
var socketWaitTimeout = 30 * time.Second

// archScript maps uname -m onto the release-artifact arch names, and is
// prepended to any script that downloads a binary.
const archScript = `ARCH="$(uname -m)"
case "$ARCH" in
  x86_64)        ARCH=amd64 ;;
  aarch64|arm64) ARCH=arm64 ;;
  *) echo "unsupported architecture: $ARCH" >&2; exit 1 ;;
esac
`

// fetchScript picks whichever downloader the host has. Same reasoning as the
// CLI's bootstrap script: requiring a specific one reintroduces the dependency
// this path exists to remove.
const fetchScript = `if command -v curl >/dev/null 2>&1; then
  FETCH_OUT="curl -fsSL --retry 3 --retry-delay 2 -o"
elif command -v wget >/dev/null 2>&1; then
  FETCH_OUT="wget -qO"
else
  echo "neither curl nor wget is available on this host" >&2; exit 1
fi
`

// dopplerStep installs the Doppler CLI.
//
// Optional for the same reason Bun is: the daemon only emits
// `doppler run -- <cmd>` when a deploy actually carries a Doppler token, so a
// host that never uses Doppler runs fine without it. Failing the whole prepare
// because cli.doppler.com was unreachable would punish every non-Doppler user.
func dopplerStep() Step {
	return Step{
		Name: "Install Doppler CLI",
		Done: func(h Host) bool { return h.Exists(BinDir + "/doppler") },
		Apply: func(h Host) error {
			if _, err := h.RunShell(`set -e
if command -v curl >/dev/null 2>&1; then
  curl -fsSL --tlsv1.2 --proto "=https" --retry 3 https://cli.doppler.com/install.sh | sh
elif command -v wget >/dev/null 2>&1; then
  wget -t 3 -qO- https://cli.doppler.com/install.sh | sh
else
  echo "neither curl nor wget is available — cannot install Doppler" >&2; exit 1
fi
`); err != nil {
				return err
			}
			return linkIntoBinDir(h, "doppler")
		},
		Optional: true,
	}
}

// caddyInstallScript renders the package-manager-specific install.
//
// The rpm path degrades through COPR to a static binary because COPR is not
// available on every RHEL derivative and a hard failure there would strand the
// host with no web server at all.
func caddyInstallScript(pm PackageManager) (string, error) {
	switch pm {
	case PkgApt:
		return `set -e
DEBIAN_FRONTEND=noninteractive apt-get install -y -qq debian-keyring debian-archive-keyring apt-transport-https gnupg
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' \
  | gpg --batch --yes --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' \
  > /etc/apt/sources.list.d/caddy-stable.list
DEBIAN_FRONTEND=noninteractive apt-get update -qq
DEBIAN_FRONTEND=noninteractive apt-get install -y -qq caddy
`, nil

	case PkgDnf, PkgYum:
		mgr := string(pm)
		return `set -e
` + archScript + fetchScript + `
# Try COPR first — it gives us the caddy user and a unit file for free.
if [ "` + mgr + `" = "dnf" ]; then
  dnf install -y -q 'dnf-command(copr)' >/dev/null 2>&1 || true
else
  yum install -y -q yum-plugin-copr >/dev/null 2>&1 || true
fi
` + mgr + ` copr enable -y @caddy/caddy >/dev/null 2>&1 || true
` + mgr + ` install -y -q caddy >/dev/null 2>&1 || true

if command -v caddy >/dev/null 2>&1; then
  exit 0
fi

# Fallback: the official static release tarball.
echo "COPR unavailable — falling back to the static Caddy release"
VER="$(curl -fsSL --retry 3 https://api.github.com/repos/caddyserver/caddy/releases/latest 2>/dev/null \
  | grep '"tag_name"' | head -1 | sed 's/.*"v\([^"]*\)".*/\1/')"
[ -z "$VER" ] && { echo "could not resolve the latest Caddy release" >&2; exit 1; }
$FETCH_OUT /tmp/caddy.tar.gz "https://github.com/caddyserver/caddy/releases/download/v${VER}/caddy_${VER}_linux_${ARCH}.tar.gz"
tar -tzf /tmp/caddy.tar.gz >/dev/null 2>&1 || { echo "downloaded Caddy tarball is corrupt" >&2; rm -f /tmp/caddy.tar.gz; exit 1; }
tar -xzf /tmp/caddy.tar.gz --no-same-owner -C /tmp caddy
install -m 0755 /tmp/caddy /usr/local/bin/caddy
rm -f /tmp/caddy.tar.gz /tmp/caddy
`, nil

	default:
		return "", fmt.Errorf("no supported package manager (apt/dnf/yum) on this host — install Caddy manually")
	}
}

// caddySteps provisions the edge: Caddy itself, the Coraza WAF module every
// generated fragment depends on, a valid main Caddyfile, and a running service.
//
// This is the half of the deploy path the runtime steps don't cover. Without
// it, `nextdeploy ship` gets all the way to the Caddy stage of activateRelease
// and fails there — after the release is unpacked and the unit is running,
// which is the most confusing possible place to discover the host was never
// fully provisioned.
func caddySteps(pm PackageManager) []Step {
	return []Step{
		{
			Name: "Install Caddy",
			Done: func(h Host) bool { _, ok := h.LookPath("caddy"); return ok },
			Apply: func(h Host) error {
				script, err := caddyInstallScript(pm)
				if err != nil {
					return err
				}
				if _, err := h.RunShell(script); err != nil {
					return err
				}
				if _, ok := h.LookPath("caddy"); !ok {
					return fmt.Errorf("caddy is still not on PATH after installation")
				}
				return ensureCaddyService(h)
			},
		},
		{
			// Caddy serves /_next/static/* straight from
			// /opt/nextdeploy/apps/<app>/shared_static, but every directory on
			// the way in is 0750 nextdeploy:nextdeploy. Without this membership
			// the caddy user cannot even traverse /opt/nextdeploy, so every
			// hashed asset 403s and the app loads without its JS or CSS while
			// the deploy reports success.
			//
			// The playbook does this in its first phase with ignore_errors,
			// because Caddy might not be installed yet. Running it here instead
			// — right after the install step — means the account exists and a
			// failure is real.
			Name: "Add caddy to the " + ServiceGroup + " group",
			Done: func(h Host) bool { return h.UserInGroup("caddy", ServiceGroup) },
			Apply: func(h Host) error {
				if !h.UserExists("caddy") {
					return fmt.Errorf("the caddy user does not exist — cannot grant it %s group access", ServiceGroup)
				}
				_, err := h.Run("usermod", "-aG", ServiceGroup, "caddy")
				return err
			},
		},
		{
			// The module ID is http.handlers.waf; the directive is coraza_waf.
			// Match either so this doesn't start silently rebuilding Caddy on
			// every run if upstream renames one of them.
			Name: "Ensure Caddy has the Coraza WAF module",
			Done: func(h Host) bool {
				out, err := h.RunShell("caddy list-modules 2>/dev/null")
				if err != nil {
					return false
				}
				lower := strings.ToLower(out)
				return strings.Contains(lower, "coraza") || strings.Contains(lower, "http.handlers.waf")
			},
			Apply: func(h Host) error {
				// Caddy's download API serves a prebuilt binary with the plugin
				// compiled in. The playbook used xcaddy, which drags a full Go
				// toolchain onto the host to compile a binary upstream will
				// hand us ready-made — a build dependency for no benefit.
				if _, err := h.RunShell(`set -e
` + archScript + fetchScript + `
CADDY_BIN="$(command -v caddy)"
[ -n "$CADDY_BIN" ] || { echo "caddy not found on PATH" >&2; exit 1; }
$FETCH_OUT /tmp/caddy.coraza "https://caddyserver.com/api/download?os=linux&arch=${ARCH}&p=` + corazaPlugin + `"
# A failed build request comes back as text, not a binary — catch it here
# rather than letting systemd report a cryptic exec format error.
head -c 4 /tmp/caddy.coraza | od -An -tx1 | grep -q '7f 45 4c 46' || {
  echo "download did not return an ELF binary:" >&2
  head -c 200 /tmp/caddy.coraza >&2
  rm -f /tmp/caddy.coraza
  exit 1
}
install -m 0755 /tmp/caddy.coraza "$CADDY_BIN"
rm -f /tmp/caddy.coraza
`); err != nil {
					return err
				}
				// Only restarts if it is already running; a no-op pre-start.
				_, _ = h.Run("systemctl", "try-restart", "caddy")
				return nil
			},
		},
		{
			Name: "Seed " + CaddyfilePath + " and " + CaddyFragmentDir,
			Done: func(h Host) bool {
				data, err := h.ReadFile(CaddyfilePath)
				if err != nil {
					return false
				}
				body := string(data)
				return strings.Contains(body, "order coraza_waf") &&
					strings.Contains(body, "import "+CaddyFragmentDir)
			},
			Apply: func(h Host) error {
				// 0755 on the fragment dir matches what NewCaddyManager
				// converges to at deploy time; seeding it tighter just means
				// the first deploy widens it again.
				if err := h.MkdirAll(CaddyFragmentDir, 0o755); err != nil {
					return err
				}
				return h.WriteFile(CaddyfilePath, []byte(mainCaddyfile), 0o644)
			},
		},
		{
			Name: "Enable and start Caddy",
			Done: func(h Host) bool {
				_, err := h.Run("systemctl", "is-active", "--quiet", "caddy")
				return err == nil
			},
			Apply: func(h Host) error {
				if _, err := h.Run("systemctl", "daemon-reload"); err != nil {
					return err
				}
				_, err := h.Run("systemctl", "enable", "--now", "caddy")
				return err
			},
		},
	}
}

// ensureCaddyService backfills what a package install would have provided but
// the static-binary fallback does not: the service account Caddy drops to, and
// a unit to run it under. Both are no-ops when the package supplied them.
func ensureCaddyService(h Host) error {
	if !h.GroupExists("caddy") {
		if _, err := h.Run("groupadd", "--system", "caddy"); err != nil {
			return fmt.Errorf("create caddy group: %w", err)
		}
	}
	if !h.UserExists("caddy") {
		if _, err := h.Run("useradd", "--system", "--gid", "caddy",
			"--shell", "/sbin/nologin", "--home-dir", "/var/lib/caddy",
			"--create-home", "caddy"); err != nil {
			return fmt.Errorf("create caddy user: %w", err)
		}
	}
	// Package units live under /lib or /usr/lib; only write ours if neither the
	// package nor a previous run left one.
	for _, p := range []string{
		CaddyUnitPath,
		"/lib/systemd/system/caddy.service",
		"/usr/lib/systemd/system/caddy.service",
	} {
		if h.Exists(p) {
			return nil
		}
	}
	if err := h.WriteFile(CaddyUnitPath, []byte(caddyUnit), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", CaddyUnitPath, err)
	}
	_, err := h.Run("systemctl", "daemon-reload")
	return err
}

// daemonSteps brings up the control plane: the HMAC secret the command
// signature is verified against, the unit that runs nextdeployd, and a socket
// with the ownership the CLI expects.
//
// Everything else prepare does is inert without this. `nextdeployd ship` on the
// host is only a socket client — if no daemon is listening, a fully provisioned
// server still rejects every deploy with "is nextdeployd running?".
func daemonSteps() []Step {
	return []Step{
		{
			Name: "Write " + DaemonConfPath + " with an HMAC secret",
			Done: func(h Host) bool { return h.Exists(DaemonConfPath) },
			Apply: func(h Host) error {
				// The daemon self-generates a secret on first start if this is
				// absent, but writing it here closes that window and pins the
				// file at 0600: the socket is group-readable so the nextdeploy
				// group can connect, and only root can read the secret needed
				// to sign a command.
				secret, err := randomHex(32)
				if err != nil {
					return err
				}
				if err := h.MkdirAll(DaemonConfDir, 0o750); err != nil {
					return err
				}
				body := fmt.Sprintf("{\n  \"security_secret\": %q\n}\n", secret)
				return h.WriteFile(DaemonConfPath, []byte(body), 0o600)
			},
		},
		{
			Name: "Install the nextdeployd systemd unit",
			Done: func(h Host) bool {
				data, err := h.ReadFile(DaemonUnitPath)
				return err == nil && string(data) == daemonUnit
			},
			Apply: func(h Host) error {
				if err := h.WriteFile(DaemonUnitPath, []byte(daemonUnit), 0o644); err != nil {
					return err
				}
				if _, err := h.Run("systemctl", "daemon-reload"); err != nil {
					return err
				}
				// Pick up a changed unit if the daemon is already running;
				// no-op otherwise, so this is safe before the first start.
				_, _ = h.Run("systemctl", "try-restart", "nextdeployd")
				return nil
			},
		},
		{
			Name: "Start nextdeployd and secure its control socket",
			Done: func(h Host) bool {
				if !h.Exists(DaemonSockPath) {
					return false
				}
				_, err := h.Run("systemctl", "is-active", "--quiet", "nextdeployd")
				return err == nil
			},
			Apply: func(h Host) error {
				if _, err := h.Run("systemctl", "enable", "--now", "nextdeployd"); err != nil {
					return err
				}
				if err := waitForSocket(h, DaemonSockPath, socketWaitTimeout); err != nil {
					return err
				}
				// 0660 root:nextdeploy is the whole local authorization model —
				// HandleCommand skips the IP allow-list for unix peers precisely
				// because these permissions already gate them.
				if err := h.Chown(DaemonSockPath, "root", ServiceGroup); err != nil {
					return fmt.Errorf("chown %s: %w", DaemonSockPath, err)
				}
				if _, err := h.Run("chmod", "0660", DaemonSockPath); err != nil {
					return fmt.Errorf("chmod %s: %w", DaemonSockPath, err)
				}
				return nil
			},
		},
	}
}

// waitForSocket polls until the daemon has bound its socket. systemd returns
// from `enable --now` once the process is forked, not once it is listening, so
// without this the chown below races the bind and fails on a fast return.
func waitForSocket(h Host, path string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		if h.Exists(path) {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf(
				"nextdeployd did not create %s within %s — check `journalctl -u nextdeployd`",
				path, timeout)
		}
		time.Sleep(250 * time.Millisecond)
	}
}

func randomHex(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate random secret: %w", err)
	}
	return hex.EncodeToString(buf), nil
}
