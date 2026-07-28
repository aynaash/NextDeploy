package prepare

import (
	"fmt"
	"strings"
)

// Paths and identities the daemon assumes at runtime. Changing one here without
// changing the daemon is how a "successful" prepare produces a server the
// daemon can't use, so they are named once and shared.
const (
	AppRoot    = "/opt/nextdeploy"
	AppsDir    = AppRoot + "/apps"
	UploadsDir = AppRoot + "/uploads"
	TmpDir     = AppRoot + "/tmp"

	ServiceUser  = "nextdeploy"
	ServiceGroup = "nextdeploy"

	CaddyLogDir      = "/var/log/caddy"
	CaddyFragmentDir = "/etc/caddy/nextdeploy.d"

	// BinDir is where systemd ExecStart lines look for runtimes. Units do not
	// inherit a login PATH, so resolveBinary hard-codes this prefix — every
	// runtime we install has to be reachable here.
	BinDir = "/usr/local/bin"

	// NodeMajor is the Node major installed from NodeSource. Pinned so a
	// re-prepare six months from now provisions the same runtime.
	NodeMajor = "22"
)

// Options tunes what a run provisions.
type Options struct {
	// SkipRuntimes leaves Node/Bun/Corepack alone. Useful when the host is
	// managed separately and prepare should only own the NextDeploy layer.
	SkipRuntimes bool
	// SkipHardening leaves fail2ban and logrotate alone.
	SkipHardening bool
}

// PackageManager identifies the host's package manager.
type PackageManager string

const (
	PkgApt     PackageManager = "apt"
	PkgDnf     PackageManager = "dnf"
	PkgYum     PackageManager = "yum"
	PkgUnknown PackageManager = ""
)

// DetectPackageManager picks the host's package manager, preferring dnf over
// yum (dnf is yum's successor and present alongside it on modern RHEL).
func DetectPackageManager(h Host) PackageManager {
	for _, c := range []struct {
		bin string
		pm  PackageManager
	}{
		{"apt-get", PkgApt},
		{"dnf", PkgDnf},
		{"yum", PkgYum},
	} {
		if _, ok := h.LookPath(c.bin); ok {
			return c.pm
		}
	}
	return PkgUnknown
}

// installCmd renders the non-interactive install invocation for pm.
func installCmd(pm PackageManager, pkgs ...string) (string, error) {
	list := strings.Join(pkgs, " ")
	switch pm {
	case PkgApt:
		return "DEBIAN_FRONTEND=noninteractive apt-get install -y -qq " + list, nil
	case PkgDnf:
		return "dnf install -y -q " + list, nil
	case PkgYum:
		return "yum install -y -q " + list, nil
	default:
		return "", fmt.Errorf("no supported package manager (apt/dnf/yum) on this host — install %s manually", list)
	}
}

// Plan builds the ordered provisioning steps for this host.
//
// Order matters and encodes real dependencies: the group must exist before the
// user that joins it, the user before the directories it owns, Node before the
// Corepack shims that Node provides.
func Plan(h Host, opts Options) []Step {
	pm := DetectPackageManager(h)
	var steps []Step

	steps = append(steps,
		Step{
			Name: "Refresh package index",
			Apply: func(h Host) error {
				switch pm {
				case PkgApt:
					_, err := h.RunShell("DEBIAN_FRONTEND=noninteractive apt-get update -qq")
					return err
				case PkgDnf, PkgYum:
					// dnf/yum refresh on demand; an explicit clean is enough.
					_, err := h.RunShell(string(pm) + " clean all -q")
					return err
				}
				return fmt.Errorf("no supported package manager (apt/dnf/yum) on this host")
			},
		},
		Step{
			Name: "Install base packages (curl, tar, ca-certificates)",
			Done: func(h Host) bool {
				for _, b := range []string{"curl", "tar"} {
					if _, ok := h.LookPath(b); !ok {
						return false
					}
				}
				return true
			},
			Apply: func(h Host) error {
				cmd, err := installCmd(pm, "curl", "tar", "ca-certificates", "unzip")
				if err != nil {
					return err
				}
				_, err = h.RunShell(cmd)
				return err
			},
		},
		Step{
			Name: "Create " + ServiceGroup + " group",
			Done: func(h Host) bool { return h.GroupExists(ServiceGroup) },
			Apply: func(h Host) error {
				_, err := h.Run("groupadd", "--system", ServiceGroup)
				return err
			},
		},
		Step{
			Name: "Create " + ServiceUser + " system user",
			Done: func(h Host) bool { return h.UserExists(ServiceUser) },
			Apply: func(h Host) error {
				_, err := h.Run("useradd", "--system", "--gid", ServiceGroup,
					"--shell", "/sbin/nologin", "--no-create-home", ServiceUser)
				return err
			},
		},
		Step{
			Name: "Create " + AppRoot + " directory tree",
			Done: func(h Host) bool {
				for _, d := range []string{AppRoot, AppsDir, UploadsDir, TmpDir} {
					if !h.Exists(d) {
						return false
					}
				}
				return true
			},
			Apply: func(h Host) error {
				for _, d := range []string{AppRoot, AppsDir, UploadsDir, TmpDir} {
					if err := h.MkdirAll(d, 0o750); err != nil {
						return fmt.Errorf("mkdir %s: %w", d, err)
					}
					if err := h.Chown(d, ServiceUser, ServiceGroup); err != nil {
						return fmt.Errorf("chown %s: %w", d, err)
					}
				}
				return nil
			},
		},
	)

	if !opts.SkipRuntimes {
		steps = append(steps, runtimeSteps(pm)...)
	}
	if !opts.SkipHardening {
		steps = append(steps, hardeningSteps()...)
	}
	return steps
}

// runtimeSteps installs the JS runtimes the daemon can emit an ExecStart for.
//
// The daemon supports npm/yarn/pnpm/bun and will happily write
// ExecStart=/usr/local/bin/pnpm for a pnpm app. If pnpm isn't here, that app
// ships "successfully" and then fails at unit start. A support matrix in one
// component and a provisioning list in another that disagree is a latent
// outage — this function IS the provisioning half of that matrix.
func runtimeSteps(pm PackageManager) []Step {
	return []Step{
		{
			Name: "Install Node.js " + NodeMajor + " (NodeSource, signed apt repo)",
			Done: func(h Host) bool { return h.Exists(BinDir+"/node") || hasWorkingNode(h) },
			Apply: func(h Host) error {
				if pm == PkgApt {
					// A signed apt repo with the major pinned is reproducible
					// and auditable in a way `curl https://webi.sh/node | sh`
					// never is: apt verifies the GPG signature of what it
					// installs, and the version is not "whatever is latest".
					script := fmt.Sprintf(
						"set -e\n"+
							"curl -fsSL --retry 3 --retry-delay 2 https://deb.nodesource.com/setup_%s.x | bash -\n"+
							"DEBIAN_FRONTEND=noninteractive apt-get install -y -qq nodejs\n", NodeMajor)
					if _, err := h.RunShell(script); err != nil {
						return err
					}
				} else {
					if _, err := h.RunShell(fmt.Sprintf(
						"set -e\ncurl -fsSL --retry 3 --retry-delay 2 https://rpm.nodesource.com/setup_%s.x | bash -\n%s install -y -q nodejs\n",
						NodeMajor, string(pm))); err != nil {
						return err
					}
				}
				return linkIntoBinDir(h, "node", "npm", "npx")
			},
		},
		{
			Name: "Enable Corepack shims (pnpm, yarn)",
			Done: func(h Host) bool {
				return h.Exists(BinDir+"/pnpm") && h.Exists(BinDir+"/yarn")
			},
			Apply: func(h Host) error {
				// Corepack is Node's built-in package-manager manager. The shim
				// reads the app's own "packageManager" field and fetches that
				// exact version on first run, so the version follows the app
				// rather than being pinned to whatever the host installed.
				if _, err := h.RunShell(
					"set -e\n" +
						"corepack enable\n" +
						"corepack prepare pnpm@latest --activate\n" +
						"corepack prepare yarn@stable --activate\n"); err != nil {
					return err
				}
				return linkIntoBinDir(h, "pnpm", "yarn")
			},
		},
		{
			Name: "Install Bun",
			Done: func(h Host) bool { return h.Exists(BinDir + "/bun") },
			Apply: func(h Host) error {
				if _, err := h.RunShell(
					"set -e\n" +
						"export BUN_INSTALL=/usr/local/lib/bun\n" +
						"curl -fsSL https://bun.sh/install | bash\n" +
						"install -m0755 \"$BUN_INSTALL/bin/bun\" " + BinDir + "/bun\n"); err != nil {
					return err
				}
				return nil
			},
			// Bun is the one runtime an app opts into rather than defaults to;
			// a failure here shouldn't block a Node/pnpm deployment.
			Optional: true,
		},
	}
}

// hasWorkingNode reports whether node resolves and runs, wherever it lives.
func hasWorkingNode(h Host) bool {
	if _, ok := h.LookPath("node"); !ok {
		return false
	}
	_, err := h.Run("node", "--version")
	return err == nil
}

// linkIntoBinDir symlinks each named binary into BinDir. systemd ExecStart
// needs an absolute path (units don't inherit a login PATH), which is why the
// daemon's resolveBinary hard-codes /usr/local/bin — so whatever a package
// manager or installer put elsewhere has to be reachable there.
func linkIntoBinDir(h Host, names ...string) error {
	for _, n := range names {
		target := BinDir + "/" + n
		if h.Exists(target) {
			continue
		}
		src, ok := h.LookPath(n)
		if !ok {
			return fmt.Errorf("%s installed but not on PATH — cannot link into %s", n, BinDir)
		}
		if src == target {
			continue
		}
		if _, err := h.Run("ln", "-sf", src, target); err != nil {
			return fmt.Errorf("link %s -> %s: %w", src, target, err)
		}
	}
	return nil
}

// hardeningSteps provisions the ops safety net: log rotation and the fail2ban
// jails, including one that actually reads the WAF's audit log.
func hardeningSteps() []Step {
	return []Step{
		{
			Name: "Create " + CaddyLogDir + " and Caddy fragment dir",
			Done: func(h Host) bool { return h.Exists(CaddyLogDir) && h.Exists(CaddyFragmentDir) },
			Apply: func(h Host) error {
				for _, d := range []string{CaddyLogDir, CaddyFragmentDir} {
					if err := h.MkdirAll(d, 0o755); err != nil {
						return err
					}
				}
				// Best-effort: caddy may not have a user yet on a bare host.
				_ = h.Chown(CaddyLogDir, "caddy", "caddy")
				return nil
			},
		},
		{
			Name: "Install logrotate policy for " + CaddyLogDir,
			Done: func(h Host) bool { return h.Exists("/etc/logrotate.d/caddy") },
			Apply: func(h Host) error {
				return h.WriteFile("/etc/logrotate.d/caddy", []byte(logrotateCaddy), 0o644)
			},
		},
		{
			Name: "Install fail2ban filters and jails (access log + Coraza WAF)",
			Done: func(h Host) bool {
				return h.Exists("/etc/fail2ban/jail.d/caddy.conf") &&
					h.Exists("/etc/fail2ban/jail.d/caddy-waf.conf")
			},
			Apply: func(h Host) error {
				if _, ok := h.LookPath("fail2ban-client"); !ok {
					return fmt.Errorf("fail2ban is not installed — ban-on-abuse is off")
				}
				files := map[string]string{
					"/etc/fail2ban/filter.d/caddy-auth.conf": filterCaddyAuth,
					"/etc/fail2ban/filter.d/caddy-waf.conf":  filterCaddyWAF,
					"/etc/fail2ban/jail.d/caddy.conf":        jailCaddy,
					"/etc/fail2ban/jail.d/caddy-waf.conf":    jailCaddyWAF,
				}
				for path, content := range files {
					if err := h.MkdirAll(dirOf(path), 0o755); err != nil {
						return err
					}
					if err := h.WriteFile(path, []byte(content), 0o644); err != nil {
						return fmt.Errorf("write %s: %w", path, err)
					}
				}
				_, err := h.Run("systemctl", "restart", "fail2ban")
				return err
			},
			// A host without fail2ban still runs apps; warn, don't abort.
			Optional: true,
		},
	}
}

func dirOf(path string) string {
	if i := strings.LastIndexByte(path, '/'); i > 0 {
		return path[:i]
	}
	return "/"
}
