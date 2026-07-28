package prepare

import (
	"fmt"
	"io"
	"os"
)

// Run provisions this machine and returns the report. It never panics on a
// failed step — the report carries the outcome so the caller can decide the
// exit code and print a summary.
func Run(h Host, opts Options, out io.Writer) *Report {
	fmt.Fprintf(out, "nextdeployd prepare — provisioning this host\n")
	if pm := DetectPackageManager(h); pm == PkgUnknown {
		fmt.Fprintf(out, "  package manager: none detected (apt/dnf/yum) — package steps will fail\n")
	} else {
		fmt.Fprintf(out, "  package manager: %s\n", pm)
	}
	fmt.Fprintln(out)

	report := RunSteps(h, Plan(h, opts), out)

	fmt.Fprintln(out)
	fmt.Fprint(out, report.Summary())
	return report
}

// RequireRoot reports an actionable error when prepare isn't running with the
// privileges it needs. Every step writes under /opt, /etc or /usr/local and
// creates a system user, so failing here beats failing halfway through with a
// pile of permission errors.
func RequireRoot() error {
	if os.Geteuid() == 0 {
		return nil
	}
	return fmt.Errorf(
		"nextdeployd prepare must run as root — it creates a system user and writes to /opt, /etc and /usr/local.\n" +
			"Re-run with: sudo nextdeployd prepare")
}
