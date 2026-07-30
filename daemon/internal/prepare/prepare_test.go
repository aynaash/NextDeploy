package prepare

import (
	"errors"
	"io"
	"strings"
	"testing"
)

// installEverything models a host where each installer actually works: the
// command lands the binaries it claims to. Used to reach a "fully provisioned"
// state so the idempotency tests have something to converge on.
func installEverything(h *fakeHost, cmd string) {
	switch {
	case strings.Contains(cmd, "nodesource"):
		h.bins["node"] = "/usr/bin/node"
		h.bins["npm"] = "/usr/bin/npm"
		h.bins["npx"] = "/usr/bin/npx"
		h.bins["corepack"] = "/usr/bin/corepack"
	case strings.Contains(cmd, "corepack enable"):
		h.bins["pnpm"] = "/usr/bin/pnpm"
		h.bins["yarn"] = "/usr/bin/yarn"
	case strings.Contains(cmd, "bun.sh/install"):
		h.files[BinDir+"/bun"] = []byte("elf")
	case strings.Contains(cmd, "cli.doppler.com"):
		h.bins["doppler"] = "/usr/bin/doppler"
	case strings.Contains(cmd, "dl.cloudsmith.io"), strings.Contains(cmd, "copr enable"):
		h.bins["caddy"] = "/usr/bin/caddy"
		h.files["/lib/systemd/system/caddy.service"] = []byte("packaged unit")
	case strings.Contains(cmd, "caddyserver.com/api/download"):
		// The replacement binary is the one that reports the module.
		h.outputs["caddy list-modules"] = "http.handlers.waf\n"
	case strings.Contains(cmd, "apt-get install"):
		for _, b := range []string{"curl", "tar", "unzip"} {
			h.bins[b] = "/usr/bin/" + b
		}
		if strings.Contains(cmd, "fail2ban") {
			h.bins["fail2ban-client"] = "/usr/bin/fail2ban-client"
		}
	}
	// systemd starting the daemon is what materializes the control socket.
	if strings.Contains(cmd, "enable --now nextdeployd") {
		h.files[DaemonSockPath] = []byte("socket")
	}
	// `ln -sf <src> /usr/local/bin/<name>` materializes the link.
	if strings.HasPrefix(cmd, "ln -sf ") {
		parts := strings.Fields(cmd)
		if len(parts) == 4 {
			h.files[parts[3]] = []byte("symlink")
		}
	}
	// Both invocations put the account name last. Read it rather than assuming
	// the service account — prepare also creates the `caddy` user on the
	// static-fallback path.
	if strings.HasPrefix(cmd, "groupadd ") {
		f := strings.Fields(cmd)
		h.groups[f[len(f)-1]] = true
	}
	if strings.HasPrefix(cmd, "useradd ") {
		f := strings.Fields(cmd)
		h.users[f[len(f)-1]] = true
	}
}

func TestDetectPackageManager(t *testing.T) {
	cases := []struct {
		name string
		bins []string
		want PackageManager
	}{
		{"debian", []string{"apt-get"}, PkgApt},
		{"fedora", []string{"dnf"}, PkgDnf},
		{"centos7", []string{"yum"}, PkgYum},
		// dnf is yum's successor and ships alongside it; prefer dnf.
		{"rhel8 has both", []string{"yum", "dnf"}, PkgDnf},
		{"nothing", nil, PkgUnknown},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h := newFakeHost()
			for _, b := range c.bins {
				h.bins[b] = "/usr/bin/" + b
			}
			if got := DetectPackageManager(h); got != c.want {
				t.Errorf("DetectPackageManager = %q, want %q", got, c.want)
			}
		})
	}
}

func TestInstallCmdUnknownManagerNamesThePackages(t *testing.T) {
	_, err := installCmd(PkgUnknown, "curl", "tar")
	if err == nil {
		t.Fatal("want an error with no package manager")
	}
	// The error has to tell the operator what to install by hand.
	for _, want := range []string{"curl", "tar", "apt/dnf/yum"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error missing %q: %v", want, err)
		}
	}
}

func TestPrepareProvisionsABareHost(t *testing.T) {
	h := aptHost()
	h.onRun = installEverything

	report := Run(h, Options{}, io.Discard)
	if report.Failed() {
		t.Fatalf("prepare failed on a bare host:\n%s", report.Summary())
	}

	// The identities and paths the daemon assumes at runtime must all exist.
	if !h.groups[ServiceGroup] || !h.users[ServiceUser] {
		t.Error("service user/group were not created")
	}
	for _, d := range []string{AppRoot, AppsDir, UploadsDir, TmpDir} {
		if !h.dirs[d] {
			t.Errorf("directory %s was not created", d)
		}
		if got := h.owners[d]; got != ServiceUser+":"+ServiceGroup {
			t.Errorf("%s owned by %q, want %s:%s", d, got, ServiceUser, ServiceGroup)
		}
	}
}

func TestPrepareInstallsEveryPackageManagerTheDaemonCanEmit(t *testing.T) {
	// The daemon writes ExecStart=/usr/local/bin/<pm> for whichever manager the
	// app declares. If one is missing, that app ships "successfully" and then
	// fails at unit start — the exact bug this closes. This test is the
	// provisioning half of the support matrix; it fails if they drift apart.
	h := aptHost()
	h.onRun = installEverything

	if report := Run(h, Options{}, io.Discard); report.Failed() {
		t.Fatalf("prepare failed:\n%s", report.Summary())
	}

	for _, pm := range []string{"node", "npm", "pnpm", "yarn", "bun"} {
		if !h.Exists(BinDir + "/" + pm) {
			t.Errorf("%s is not at %s/%s — an app using it would ship then fail to start", pm, BinDir, pm)
		}
	}
}

func TestPrepareIsIdempotent(t *testing.T) {
	h := aptHost()
	h.onRun = installEverything

	if report := Run(h, Options{}, io.Discard); report.Failed() {
		t.Fatalf("first run failed:\n%s", report.Summary())
	}
	firstRunCommands := len(h.ran)

	h.ran = nil
	report := Run(h, Options{}, io.Discard)
	if report.Failed() {
		t.Fatalf("second run failed:\n%s", report.Summary())
	}

	// Everything with a Done predicate must be skipped the second time. Only
	// the package-index refresh (no Done — it's inherently re-runnable) should
	// still execute.
	ok, skipped, _, _ := report.Counts()
	if skipped == 0 {
		t.Fatal("second run skipped nothing — check-before-change is not working")
	}
	if ok > 1 {
		t.Errorf("second run re-applied %d steps, want at most 1 (the index refresh):\n%s", ok, report.Summary())
	}
	if len(h.ran) >= firstRunCommands {
		t.Errorf("second run issued %d commands vs %d on a bare host — not converging", len(h.ran), firstRunCommands)
	}
}

func TestPrepareStopsAtFirstRequiredFailure(t *testing.T) {
	// Later steps assume earlier ones landed (the user must exist before its
	// directories are chowned), so pressing on would turn one clear error into
	// a cascade of confusing ones.
	h := aptHost()
	h.onRun = installEverything
	h.failCmd["groupadd"] = errors.New("groupadd: permission denied")

	report := Run(h, Options{}, io.Discard)
	if !report.Failed() {
		t.Fatal("a failed groupadd should fail the run")
	}
	if len(h.commandsMatching("useradd")) != 0 {
		t.Error("run continued to useradd after the group step failed")
	}
	last := report.Results[len(report.Results)-1]
	if last.Status != StatusFailed || !strings.Contains(last.Name, "group") {
		t.Errorf("last result should be the failed group step, got %+v", last)
	}
}

func TestPrepareOptionalFailureWarnsButContinues(t *testing.T) {
	// Bun is opt-in per app; a host without it still runs Node/pnpm apps.
	h := aptHost()
	h.onRun = installEverything
	h.bins["fail2ban-client"] = "/usr/bin/fail2ban-client" // isolate bun as the only warning
	h.failCmd["bun.sh/install"] = errors.New("network unreachable")

	report := Run(h, Options{}, io.Discard)
	if report.Failed() {
		t.Fatalf("an optional step failure must not fail the run:\n%s", report.Summary())
	}
	_, _, warned, _ := report.Counts()
	if warned != 1 {
		t.Errorf("warned = %d, want 1", warned)
	}
	// And the steps after it still ran.
	if !h.dirs[CaddyLogDir] {
		t.Error("run did not continue past the optional failure")
	}
}

func TestPrepareWritesHardeningConfigs(t *testing.T) {
	h := aptHost()
	h.onRun = installEverything
	h.bins["fail2ban-client"] = "/usr/bin/fail2ban-client"

	if report := Run(h, Options{}, io.Discard); report.Failed() {
		t.Fatalf("prepare failed:\n%s", report.Summary())
	}

	if _, ok := h.files["/etc/logrotate.d/caddy"]; !ok {
		t.Error("no logrotate policy — Caddy/Coraza logs would grow unbounded")
	}
	// The WAF jail is the one that closes the detect→ban gap: Coraza writes to
	// audit.log, which the access-log jail never reads.
	waf, ok := h.files["/etc/fail2ban/jail.d/caddy-waf.conf"]
	if !ok {
		t.Fatal("no Coraza WAF jail — WAF blocks would never reach fail2ban")
	}
	if !strings.Contains(string(waf), "/var/log/caddy/audit.log") {
		t.Errorf("WAF jail does not read audit.log:\n%s", waf)
	}
}

func TestPrepareInstallsFail2banRatherThanWarningAboutIt(t *testing.T) {
	// Previously the jails step only checked for fail2ban and warned when it was
	// absent, so on any host that didn't ship it the ban-on-abuse layer was
	// quietly off while prepare reported success.
	h := aptHost()
	h.onRun = installEverything // fail2ban-client is NOT pre-seeded

	report := Run(h, Options{}, io.Discard)
	if report.Failed() {
		t.Fatalf("prepare failed:\n%s", report.Summary())
	}
	if len(h.commandsMatching("fail2ban")) == 0 {
		t.Fatal("prepare never tried to install fail2ban")
	}
	if _, ok := h.files["/etc/fail2ban/jail.d/caddy-waf.conf"]; !ok {
		t.Error("jails were not installed after fail2ban was")
	}
	_, _, warned, _ := report.Counts()
	if warned != 0 {
		t.Errorf("a host where fail2ban installs cleanly should not warn:\n%s", report.Summary())
	}
}

func TestPrepareWarnsWhenFail2banCannotBeInstalled(t *testing.T) {
	// An unreachable package mirror must not fail the run: the host still
	// serves traffic without fail2ban.
	h := aptHost()
	h.onRun = installEverything
	h.failCmd["fail2ban"] = errors.New("could not reach the package mirror")

	report := Run(h, Options{}, io.Discard)
	if report.Failed() {
		t.Fatalf("a failed fail2ban install must not fail the run:\n%s", report.Summary())
	}
	_, _, warned, _ := report.Counts()
	if warned == 0 {
		t.Error("a failed fail2ban install should warn, not pass silently")
	}
}

func TestPrepareSkipOptions(t *testing.T) {
	t.Run("skip runtimes", func(t *testing.T) {
		h := aptHost()
		h.onRun = installEverything
		Run(h, Options{SkipRuntimes: true}, io.Discard)
		if len(h.commandsMatching("nodesource")) != 0 {
			t.Error("SkipRuntimes still installed Node")
		}
		if !h.dirs[CaddyLogDir] {
			t.Error("SkipRuntimes should not skip hardening")
		}
	})

	t.Run("skip hardening", func(t *testing.T) {
		h := aptHost()
		h.onRun = installEverything
		Run(h, Options{SkipHardening: true}, io.Discard)
		if _, ok := h.files["/etc/logrotate.d/caddy"]; ok {
			t.Error("SkipHardening still wrote the logrotate policy")
		}
		if len(h.commandsMatching("nodesource")) == 0 {
			t.Error("SkipHardening should not skip runtimes")
		}
	})
}

func TestPrepareOnHostWithNoPackageManager(t *testing.T) {
	h := newFakeHost() // no apt/dnf/yum
	report := Run(h, Options{}, io.Discard)
	if !report.Failed() {
		t.Fatal("a host with no package manager should fail, not silently half-provision")
	}
	if !strings.Contains(report.Summary(), "apt/dnf/yum") {
		t.Errorf("failure should name the supported managers:\n%s", report.Summary())
	}
}

func TestLinkIntoBinDirReportsMissingBinary(t *testing.T) {
	// A silent skip here would leave a systemd unit pointing at a path that
	// does not exist — the deferred failure this whole package exists to stop.
	h := newFakeHost()
	err := linkIntoBinDir(h, "pnpm")
	if err == nil {
		t.Fatal("linking a binary that isn't on PATH should error")
	}
	if !strings.Contains(err.Error(), BinDir) {
		t.Errorf("error should name the target dir: %v", err)
	}
}

func TestLinkIntoBinDirIsANoOpWhenAlreadyLinked(t *testing.T) {
	h := newFakeHost()
	h.files[BinDir+"/pnpm"] = []byte("symlink")
	if err := linkIntoBinDir(h, "pnpm"); err != nil {
		t.Fatalf("already-linked should be a no-op, got %v", err)
	}
	if len(h.ran) != 0 {
		t.Errorf("already-linked issued commands: %v", h.ran)
	}
}
