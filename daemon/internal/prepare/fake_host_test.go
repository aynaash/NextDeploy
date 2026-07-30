package prepare

import (
	"fmt"
	"os"
	"strings"
)

// fakeHost is an in-memory Host. It exists so the whole provisioning plan can
// be exercised without a machine — which is the point of moving provisioning
// into Go: hand-written idempotency is only trustworthy if it's tested.
type fakeHost struct {
	bins    map[string]string // name -> resolved path
	files   map[string][]byte
	dirs    map[string]bool
	groups  map[string]bool
	users   map[string]bool
	owners  map[string]string // path -> "user:group"
	ran     []string          // every command/script, in order
	failCmd map[string]error  // substring -> error to return
	// outputs lets a command report something back (substring -> stdout), which
	// Done predicates that interrogate the host need — `caddy list-modules` is
	// only meaningful if the fake can answer it.
	outputs map[string]string
	// onRun lets a test mutate host state as a side effect of a command,
	// modelling what a real installer would do.
	onRun func(h *fakeHost, cmd string)
}

func newFakeHost() *fakeHost {
	return &fakeHost{
		bins:    map[string]string{},
		files:   map[string][]byte{},
		dirs:    map[string]bool{},
		groups:  map[string]bool{},
		users:   map[string]bool{},
		owners:  map[string]string{},
		failCmd: map[string]error{},
		outputs: map[string]string{},
	}
}

// aptHost is a plausible bare Debian box: a package manager and a shell, and
// nothing NextDeploy needs.
func aptHost() *fakeHost {
	h := newFakeHost()
	h.bins["apt-get"] = "/usr/bin/apt-get"
	h.bins["sh"] = "/bin/sh"
	return h
}

func (h *fakeHost) record(cmd string) (string, error) {
	h.ran = append(h.ran, cmd)
	for frag, err := range h.failCmd {
		if strings.Contains(cmd, frag) {
			return "", err
		}
	}
	if h.onRun != nil {
		h.onRun(h, cmd)
	}
	for frag, out := range h.outputs {
		if strings.Contains(cmd, frag) {
			return out, nil
		}
	}
	return "", nil
}

func (h *fakeHost) Run(name string, args ...string) (string, error) {
	return h.record(strings.TrimSpace(name + " " + strings.Join(args, " ")))
}

func (h *fakeHost) RunShell(script string) (string, error) {
	return h.record(script)
}

func (h *fakeHost) LookPath(name string) (string, bool) {
	p, ok := h.bins[name]
	return p, ok
}

func (h *fakeHost) Exists(path string) bool {
	if h.dirs[path] {
		return true
	}
	_, ok := h.files[path]
	return ok
}

func (h *fakeHost) ReadFile(path string) ([]byte, error) {
	d, ok := h.files[path]
	if !ok {
		return nil, fmt.Errorf("no such file: %s", path)
	}
	return d, nil
}

func (h *fakeHost) WriteFile(path string, data []byte, _ os.FileMode) error {
	h.files[path] = data
	return nil
}

func (h *fakeHost) MkdirAll(path string, _ os.FileMode) error {
	h.dirs[path] = true
	return nil
}

func (h *fakeHost) Chown(path, owner, group string) error {
	if !h.Exists(path) {
		return fmt.Errorf("chown: no such path: %s", path)
	}
	h.owners[path] = owner + ":" + group
	return nil
}

func (h *fakeHost) GroupExists(name string) bool { return h.groups[name] }
func (h *fakeHost) UserExists(name string) bool  { return h.users[name] }

// commandsMatching returns every recorded command containing frag.
func (h *fakeHost) commandsMatching(frag string) []string {
	var out []string
	for _, c := range h.ran {
		if strings.Contains(c, frag) {
			out = append(out, c)
		}
	}
	return out
}
