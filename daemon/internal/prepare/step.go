package prepare

import (
	"fmt"
	"io"
	"strings"
)

// Step is one idempotent unit of provisioning.
//
// Done is the check-before-change Ansible gave us for free and we now write by
// hand: it must be cheap, side-effect free, and honest. A Done that returns
// true when the state is only partially there is worse than no Done at all —
// the runner will skip the repair.
type Step struct {
	Name string
	// Done reports the step's effect is already in place. nil means "always
	// apply" (only correct for steps that are inherently idempotent).
	Done func(Host) bool
	// Apply performs the change.
	Apply func(Host) error
	// Optional marks a step whose failure degrades the install but must not
	// abort it (e.g. fail2ban hardening on a host without fail2ban).
	Optional bool
}

// Status is the outcome of one step.
type Status string

const (
	StatusOK      Status = "ok"      // applied successfully
	StatusSkipped Status = "skipped" // already satisfied
	StatusWarned  Status = "warn"    // optional step failed
	StatusFailed  Status = "failed"  // required step failed
)

// Result records what happened to one step.
type Result struct {
	Name   string
	Status Status
	Err    error
}

// Report is the outcome of a whole run.
type Report struct {
	Results []Result
}

// Failed reports whether any required step failed.
func (r *Report) Failed() bool {
	for _, res := range r.Results {
		if res.Status == StatusFailed {
			return true
		}
	}
	return false
}

// Counts summarizes the run for a one-line ending.
func (r *Report) Counts() (ok, skipped, warned, failed int) {
	for _, res := range r.Results {
		switch res.Status {
		case StatusOK:
			ok++
		case StatusSkipped:
			skipped++
		case StatusWarned:
			warned++
		case StatusFailed:
			failed++
		}
	}
	return
}

// Summary renders the run as a human-readable block.
func (r *Report) Summary() string {
	var b strings.Builder
	for _, res := range r.Results {
		switch res.Status {
		case StatusSkipped:
			fmt.Fprintf(&b, "  [skip] %s (already satisfied)\n", res.Name)
		case StatusOK:
			fmt.Fprintf(&b, "  [ ok ] %s\n", res.Name)
		case StatusWarned:
			fmt.Fprintf(&b, "  [warn] %s: %v\n", res.Name, res.Err)
		case StatusFailed:
			fmt.Fprintf(&b, "  [FAIL] %s: %v\n", res.Name, res.Err)
		}
	}
	ok, skipped, warned, failed := r.Counts()
	fmt.Fprintf(&b, "\n%d applied, %d already satisfied, %d warnings, %d failed\n", ok, skipped, warned, failed)
	return b.String()
}

// RunSteps executes steps in order, skipping satisfied ones and stopping at the
// first required failure. Optional failures are recorded and the run continues.
//
// Stopping at the first required failure is deliberate: later steps assume
// earlier ones landed (the app user must exist before its directories are
// chowned), so pressing on would turn one clear error into a cascade of
// confusing ones.
func RunSteps(h Host, steps []Step, out io.Writer) *Report {
	report := &Report{}
	for _, s := range steps {
		if s.Done != nil && s.Done(h) {
			report.Results = append(report.Results, Result{Name: s.Name, Status: StatusSkipped})
			fmt.Fprintf(out, "  [skip] %s\n", s.Name)
			continue
		}
		fmt.Fprintf(out, "  [ .. ] %s\n", s.Name)
		if err := s.Apply(h); err != nil {
			status := StatusFailed
			if s.Optional {
				status = StatusWarned
			}
			report.Results = append(report.Results, Result{Name: s.Name, Status: status, Err: err})
			fmt.Fprintf(out, "  [%s] %s: %v\n", map[Status]string{StatusFailed: "FAIL", StatusWarned: "warn"}[status], s.Name, err)
			if status == StatusFailed {
				return report
			}
			continue
		}
		report.Results = append(report.Results, Result{Name: s.Name, Status: StatusOK})
		fmt.Fprintf(out, "  [ ok ] %s\n", s.Name)
	}
	return report
}
