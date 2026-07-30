package prepare

import (
	"errors"
	"io"
	"regexp"
	"strings"
	"testing"
	"time"
)

// dnfHost is a plausible bare RHEL-family box.
func dnfHost() *fakeHost {
	h := newFakeHost()
	h.bins["dnf"] = "/usr/bin/dnf"
	h.bins["sh"] = "/bin/sh"
	return h
}

// TestPrepareProvisionsTheEdgeAndControlPlane is the regression test for the
// gap this file closes: the plan installed runtimes and directories but neither
// Caddy nor the daemon, so `nextdeploy ship` got all the way to the Caddy stage
// of activateRelease before discovering the host was never fully provisioned.
func TestPrepareProvisionsTheEdgeAndControlPlane(t *testing.T) {
	h := aptHost()
	h.onRun = installEverything

	report := Run(h, Options{}, io.Discard)
	if report.Failed() {
		t.Fatalf("prepare failed on a bare host:\n%s", report.Summary())
	}

	if _, ok := h.LookPath("caddy"); !ok {
		t.Error("Caddy was never installed — every deploy would fail at the Caddy stage")
	}
	if _, ok := h.files[CaddyfilePath]; !ok {
		t.Errorf("%s was not seeded — Caddy has nothing valid to start against", CaddyfilePath)
	}
	if !h.dirs[CaddyFragmentDir] {
		t.Errorf("%s was not created — the import directive would dangle", CaddyFragmentDir)
	}
	if _, ok := h.files[DaemonConfPath]; !ok {
		t.Errorf("%s was not written — the daemon would have no HMAC secret", DaemonConfPath)
	}
	if _, ok := h.files[DaemonUnitPath]; !ok {
		t.Errorf("%s was not written — nextdeployd would never run as a service", DaemonUnitPath)
	}
}

func TestSeededCaddyfileMatchesWhatTheDaemonExpects(t *testing.T) {
	// The daemon's EnsureMainCaddyfile converges on this exact shape, and every
	// generated fragment needs `order coraza_waf first` to parse. If the seed
	// disagrees, Caddy fails to start before the first deploy ever runs.
	h := aptHost()
	h.onRun = installEverything
	if report := Run(h, Options{}, io.Discard); report.Failed() {
		t.Fatalf("prepare failed:\n%s", report.Summary())
	}

	body := string(h.files[CaddyfilePath])
	for _, want := range []string{"order coraza_waf first", "import " + CaddyFragmentDir + "/*.caddy"} {
		if !strings.Contains(body, want) {
			t.Errorf("seeded Caddyfile missing %q:\n%s", want, body)
		}
	}
}

func TestCaddyfileSeedDoesNotClobberAnExistingValidFile(t *testing.T) {
	h := aptHost()
	h.onRun = installEverything
	custom := "{\n\torder coraza_waf first\n}\n\nimport " + CaddyFragmentDir + "/*.caddy\n# operator's note\n"
	h.files[CaddyfilePath] = []byte(custom)

	if report := Run(h, Options{}, io.Discard); report.Failed() {
		t.Fatalf("prepare failed:\n%s", report.Summary())
	}
	if got := string(h.files[CaddyfilePath]); got != custom {
		t.Errorf("prepare overwrote a Caddyfile that already satisfied the contract:\ngot:\n%s\nwant:\n%s", got, custom)
	}
}

func TestCorazaModuleIsInstalledWhenMissingAndSkippedWhenPresent(t *testing.T) {
	t.Run("missing: fetches a plugin build", func(t *testing.T) {
		h := aptHost()
		h.onRun = installEverything // list-modules reports nothing until the fetch
		if report := Run(h, Options{}, io.Discard); report.Failed() {
			t.Fatalf("prepare failed:\n%s", report.Summary())
		}
		got := h.commandsMatching("caddyserver.com/api/download")
		if len(got) == 0 {
			t.Fatal("Coraza module was never fetched — every generated fragment would fail to parse")
		}
		if !strings.Contains(got[0], corazaPlugin) {
			t.Errorf("download did not request %s:\n%s", corazaPlugin, got[0])
		}
	})

	t.Run("present: does not rebuild", func(t *testing.T) {
		h := aptHost()
		h.onRun = installEverything
		h.outputs["caddy list-modules"] = "http.handlers.waf\n"

		if report := Run(h, Options{}, io.Discard); report.Failed() {
			t.Fatalf("prepare failed:\n%s", report.Summary())
		}
		if got := h.commandsMatching("caddyserver.com/api/download"); len(got) != 0 {
			t.Errorf("refetched the plugin build on a host that already has it: %v", got)
		}
	})
}

func TestDaemonSecretIsRandomAndStable(t *testing.T) {
	hexRe := regexp.MustCompile(`"security_secret": "([0-9a-f]{64})"`)

	first := aptHost()
	first.onRun = installEverything
	if report := Run(first, Options{}, io.Discard); report.Failed() {
		t.Fatalf("prepare failed:\n%s", report.Summary())
	}
	m1 := hexRe.FindSubmatch(first.files[DaemonConfPath])
	if m1 == nil {
		t.Fatalf("no 32-byte hex secret in the daemon config:\n%s", first.files[DaemonConfPath])
	}

	// A second run must not churn the secret: the CLI subcommand and the running
	// daemon both read this file, and rotating it mid-life invalidates every
	// in-flight signed command.
	before := string(first.files[DaemonConfPath])
	if report := Run(first, Options{}, io.Discard); report.Failed() {
		t.Fatalf("second run failed:\n%s", report.Summary())
	}
	if got := string(first.files[DaemonConfPath]); got != before {
		t.Error("re-running prepare regenerated the HMAC secret")
	}

	// And two hosts must not get the same one.
	second := aptHost()
	second.onRun = installEverything
	if report := Run(second, Options{}, io.Discard); report.Failed() {
		t.Fatalf("prepare failed on the second host:\n%s", report.Summary())
	}
	m2 := hexRe.FindSubmatch(second.files[DaemonConfPath])
	if m2 == nil {
		t.Fatal("no secret on the second host")
	}
	if string(m1[1]) == string(m2[1]) {
		t.Error("two hosts were provisioned with an identical HMAC secret")
	}
}

func TestDaemonUnitCarriesTheRuntimeDirectory(t *testing.T) {
	// RuntimeDirectory is what creates /run/nextdeployd before ExecStart. Without
	// it the daemon starts, fails to bind its socket in a directory that doesn't
	// exist, and every CLI command reports "is nextdeployd running?" against a
	// process that is running.
	for _, want := range []string{
		"RuntimeDirectory=nextdeployd",
		"RuntimeDirectoryGroup=" + ServiceGroup,
		"LogsDirectory=nextdeployd",
		"ExecStart=" + BinDir + "/nextdeployd --foreground=true",
	} {
		if !strings.Contains(daemonUnit, want) {
			t.Errorf("daemon unit missing %q", want)
		}
	}
}

func TestControlSocketGetsTheOwnershipTheCLIExpects(t *testing.T) {
	// 0660 root:nextdeploy IS the local authorization model — HandleCommand
	// skips the IP allow-list for unix peers precisely because these
	// permissions already gate them.
	h := aptHost()
	h.onRun = installEverything

	if report := Run(h, Options{}, io.Discard); report.Failed() {
		t.Fatalf("prepare failed:\n%s", report.Summary())
	}
	if got := h.owners[DaemonSockPath]; got != "root:"+ServiceGroup {
		t.Errorf("socket owned by %q, want root:%s", got, ServiceGroup)
	}
	if len(h.commandsMatching("chmod 0660 "+DaemonSockPath)) == 0 {
		t.Errorf("socket was never chmod'd to 0660; commands were %v", h.ran)
	}
}

func TestPrepareFailsWhenTheDaemonNeverBindsItsSocket(t *testing.T) {
	// A host that finishes prepare without a listening daemon is the exact
	// silent-success this package exists to prevent.
	restore := socketWaitTimeout
	socketWaitTimeout = 50 * time.Millisecond
	defer func() { socketWaitTimeout = restore }()

	h := aptHost()
	h.onRun = func(hh *fakeHost, cmd string) {
		installEverything(hh, cmd)
		delete(hh.files, DaemonSockPath) // service "starts" but never listens
	}

	report := Run(h, Options{}, io.Discard)
	if !report.Failed() {
		t.Fatalf("a daemon that never bound its socket should fail the run:\n%s", report.Summary())
	}
	if !strings.Contains(report.Summary(), "journalctl -u nextdeployd") {
		t.Errorf("failure should point at the daemon's logs:\n%s", report.Summary())
	}
}

func TestStaticCaddyFallbackBackfillsUserAndUnit(t *testing.T) {
	// The COPR/apt packages ship a caddy user and a unit file. The static-binary
	// fallback does not, so prepare has to provide both or Caddy has nothing to
	// run under.
	h := dnfHost()
	h.onRun = func(hh *fakeHost, cmd string) {
		installEverything(hh, cmd)
		if strings.Contains(cmd, "copr enable") {
			// Model the fallback: the binary lands, the package does not.
			delete(hh.files, "/lib/systemd/system/caddy.service")
		}
	}

	if report := Run(h, Options{}, io.Discard); report.Failed() {
		t.Fatalf("prepare failed on an rpm host:\n%s", report.Summary())
	}
	if !h.groups["caddy"] || !h.users["caddy"] {
		t.Error("static fallback did not create the caddy service account")
	}
	if _, ok := h.files[CaddyUnitPath]; !ok {
		t.Errorf("static fallback did not write %s", CaddyUnitPath)
	}
}

func TestPackagedCaddyUnitIsLeftAlone(t *testing.T) {
	h := aptHost()
	h.onRun = installEverything

	if report := Run(h, Options{}, io.Discard); report.Failed() {
		t.Fatalf("prepare failed:\n%s", report.Summary())
	}
	if _, ok := h.files[CaddyUnitPath]; ok {
		t.Errorf("prepare wrote %s over a packaged unit at /lib/systemd/system", CaddyUnitPath)
	}
}

func TestSkipRuntimesStillProvisionsTheEdgeAndControlPlane(t *testing.T) {
	// --skip-runtimes means "the host manages Node/Bun separately", not "skip
	// the parts of NextDeploy that make a deploy possible".
	h := aptHost()
	h.onRun = installEverything

	report := Run(h, Options{SkipRuntimes: true}, io.Discard)
	if report.Failed() {
		t.Fatalf("prepare failed:\n%s", report.Summary())
	}
	if _, ok := h.files[DaemonUnitPath]; !ok {
		t.Error("SkipRuntimes skipped the daemon unit")
	}
	if _, ok := h.files[CaddyfilePath]; !ok {
		t.Error("SkipRuntimes skipped the Caddy seed")
	}
	if len(h.commandsMatching("cli.doppler.com")) != 0 {
		t.Error("SkipRuntimes should skip the Doppler CLI")
	}
}

func TestDopplerFailureDoesNotBlockTheDeploy(t *testing.T) {
	// Doppler is opt-in per deploy, same as Bun: the daemon only emits
	// `doppler run --` when a deploy carries a token.
	h := aptHost()
	h.onRun = installEverything
	h.failCmd["cli.doppler.com"] = errors.New("network unreachable")

	report := Run(h, Options{}, io.Discard)
	if report.Failed() {
		t.Fatalf("an unreachable Doppler installer must not fail the run:\n%s", report.Summary())
	}
	if _, ok := h.files[DaemonUnitPath]; !ok {
		t.Error("run did not continue past the optional Doppler failure")
	}
}

func TestCaddyInstallScriptRefusesAnUnknownPackageManager(t *testing.T) {
	if _, err := caddyInstallScript(PkgUnknown); err == nil {
		t.Fatal("want an error with no package manager")
	}
}
