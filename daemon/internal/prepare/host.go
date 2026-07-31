// Package prepare provisions a server for NextDeploy from the daemon's own
// static binary — no Python, no Ansible, nothing pre-installed on the target
// beyond a shell.
//
// Ansible needed Python on the managed host because Ansible ships Python
// modules there and executes them; the provisioning work itself never did. On a
// small server that dependency is real cost, and a Python version mismatch
// between the control node and the target fails the run with an error that
// looks like a NextDeploy bug. nextdeployd is already a static Go binary that
// generates systemd units, writes Caddy config and sets socket permissions —
// this package moves provisioning into the language the system already speaks.
//
// The trade Ansible gave us for free was check-before-change. Here every step
// declares its own Done predicate, and the runner skips satisfied steps, so a
// re-run converges instead of redoing work.
package prepare

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"strings"
	"time"
)

// Host is every effect a step can have on the machine. Steps take a Host rather
// than calling os/exec directly so the whole provisioning plan is unit-testable
// against a fake — the thing hand-written idempotency most needs.
type Host interface {
	// Run executes a command and returns its combined output.
	Run(name string, args ...string) (string, error)
	// RunShell executes a shell script (for pipelines the package managers need).
	RunShell(script string) (string, error)
	// LookPath reports whether an executable is resolvable, and where.
	LookPath(name string) (string, bool)
	// Exists reports whether a path exists.
	Exists(path string) bool
	// ReadFile / WriteFile / MkdirAll mirror the os equivalents.
	ReadFile(path string) ([]byte, error)
	WriteFile(path string, data []byte, perm os.FileMode) error
	MkdirAll(path string, perm os.FileMode) error
	// Chown sets ownership by name, resolving uid/gid on the host.
	Chown(path, owner, group string) error
	// GroupExists / UserExists gate the create steps.
	GroupExists(name string) bool
	UserExists(name string) bool
	// UserInGroup reports secondary-group membership. Caddy serves assets out
	// of /opt/nextdeploy, which is 0750 nextdeploy:nextdeploy — group access is
	// the only way in.
	UserInGroup(username, group string) bool
}

// realHost is the production Host: it actually touches the machine.
type realHost struct {
	// timeout bounds any single command so a hung package manager can't wedge
	// the whole run with no output.
	timeout time.Duration
}

// NewHost returns a Host backed by the real machine.
func NewHost() Host { return &realHost{timeout: 10 * time.Minute} }

func (h *realHost) Run(name string, args ...string) (string, error) {
	// #nosec G204 — command names are literals from this package, not user input
	cmd := exec.Command(name, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

func (h *realHost) RunShell(script string) (string, error) {
	// #nosec G204 — scripts are literals from this package
	cmd := exec.Command("/bin/sh", "-c", script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("shell: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

func (h *realHost) LookPath(name string) (string, bool) {
	p, err := exec.LookPath(name)
	if err != nil {
		return "", false
	}
	return p, true
}

func (h *realHost) Exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func (h *realHost) ReadFile(path string) ([]byte, error) {
	return os.ReadFile(path) // #nosec G304 — paths are literals from this package
}

func (h *realHost) WriteFile(path string, data []byte, perm os.FileMode) error {
	return os.WriteFile(path, data, perm)
}

func (h *realHost) MkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}

func (h *realHost) Chown(path, owner, group string) error {
	// chown(8) rather than os.Chown so we don't hand-resolve uid/gid, and so
	// the same code path works whether the user was created moments ago.
	spec := owner
	if group != "" {
		spec = owner + ":" + group
	}
	_, err := h.Run("chown", spec, path)
	return err
}

func (h *realHost) GroupExists(name string) bool {
	_, err := user.LookupGroup(name)
	return err == nil
}

func (h *realHost) UserExists(name string) bool {
	_, err := user.Lookup(name)
	return err == nil
}

func (h *realHost) UserInGroup(username, group string) bool {
	u, err := user.Lookup(username)
	if err != nil {
		return false
	}
	g, err := user.LookupGroup(group)
	if err != nil {
		return false
	}
	gids, err := u.GroupIds()
	if err != nil {
		return false
	}
	for _, gid := range gids {
		if gid == g.Gid {
			return true
		}
	}
	return false
}
