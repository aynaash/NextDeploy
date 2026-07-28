package daemon

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func newTestCaddyManager(t *testing.T) *CaddyManager {
	t.Helper()
	return &CaddyManager{configDir: t.TempDir()}
}

func writeFragment(t *testing.T, cm *CaddyManager, appName, body string) string {
	t.Helper()
	p := filepath.Join(cm.configDir, appName+".caddy")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestSiteAddrs(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []string
	}{
		{"single", "a.com {\n\troot * /x\n}\n", []string{"a.com"}},
		{"comma list", "a.com, www.a.com {\n}\n", []string{"a.com", "www.a.com"}},
		{"leading newline", "\na.com,www.a.com {\n}\n", []string{"a.com", "www.a.com"}},
		{"tabs", "a.com,\twww.a.com\t{\n}\n", []string{"a.com", "www.a.com"}},
		{"no brace", "a.com", []string{"a.com"}},
		{"empty", "", nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := siteAddrs([]byte(c.in)); !reflect.DeepEqual(got, c.want) {
				t.Errorf("siteAddrs(%q) = %v, want %v", c.in, got, c.want)
			}
		})
	}
}

func TestSupersedeConflictsRemovesStaleFragment(t *testing.T) {
	cm := newTestCaddyManager(t)
	stale := writeFragment(t, cm, "old", "a.com, www.a.com {\n\troot * /old\n}\n")
	unrelated := writeFragment(t, cm, "other", "b.com {\n\troot * /other\n}\n")

	superseded, err := cm.supersedeConflicts("new", []string{"a.com", "www.a.com"})
	if err != nil {
		t.Fatalf("supersedeConflicts: %v", err)
	}
	if !reflect.DeepEqual(superseded, []string{"old"}) {
		t.Errorf("superseded = %v, want [old]", superseded)
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Error("stale fragment was not removed")
	}
	if _, err := os.Stat(unrelated); err != nil {
		t.Errorf("unrelated fragment was removed: %v", err)
	}
}

func TestSupersedeConflictsPartialOverlapStillCollides(t *testing.T) {
	// Caddy's constraint is per site address, so sharing even ONE address is a
	// collision — the old fragment must go even if its other address differs.
	cm := newTestCaddyManager(t)
	writeFragment(t, cm, "old", "a.com, legacy.a.com {\n}\n")

	superseded, err := cm.supersedeConflicts("new", []string{"a.com", "www.a.com"})
	if err != nil {
		t.Fatalf("supersedeConflicts: %v", err)
	}
	if !reflect.DeepEqual(superseded, []string{"old"}) {
		t.Errorf("superseded = %v, want [old]", superseded)
	}
}

func TestSupersedeConflictsSpareOwnFragment(t *testing.T) {
	// Re-deploying the same app must not delete its own fragment — the commit
	// step overwrites it atomically, and removing it first would open a window
	// where the site is unrouted.
	cm := newTestCaddyManager(t)
	own := writeFragment(t, cm, "myapp", "a.com {\n}\n")

	superseded, err := cm.supersedeConflicts("myapp", []string{"a.com"})
	if err != nil {
		t.Fatalf("supersedeConflicts: %v", err)
	}
	if len(superseded) != 0 {
		t.Errorf("superseded = %v, want none", superseded)
	}
	if _, err := os.Stat(own); err != nil {
		t.Errorf("own fragment was removed: %v", err)
	}
}

func TestSupersedeConflictsNoOpWhenNoOverlap(t *testing.T) {
	cm := newTestCaddyManager(t)
	keep := writeFragment(t, cm, "other", "b.com {\n}\n")

	superseded, err := cm.supersedeConflicts("new", []string{"a.com"})
	if err != nil {
		t.Fatalf("supersedeConflicts: %v", err)
	}
	if len(superseded) != 0 {
		t.Errorf("superseded = %v, want none", superseded)
	}
	if _, err := os.Stat(keep); err != nil {
		t.Errorf("unrelated fragment was removed: %v", err)
	}
}

func TestSupersedeConflictsIgnoresNonFragments(t *testing.T) {
	cm := newTestCaddyManager(t)
	notes := filepath.Join(cm.configDir, "notes.txt")
	if err := os.WriteFile(notes, []byte("a.com {\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(cm.configDir, "sub.caddy"), 0o755); err != nil {
		t.Fatal(err)
	}

	superseded, err := cm.supersedeConflicts("new", []string{"a.com"})
	if err != nil {
		t.Fatalf("supersedeConflicts: %v", err)
	}
	if len(superseded) != 0 {
		t.Errorf("superseded = %v, want none", superseded)
	}
	if _, err := os.Stat(notes); err != nil {
		t.Error("a non-.caddy file was removed")
	}
}

func TestSupersedeConflictsMissingDirIsNotAnError(t *testing.T) {
	cm := &CaddyManager{configDir: filepath.Join(t.TempDir(), "absent")}
	superseded, err := cm.supersedeConflicts("new", []string{"a.com"})
	if err != nil {
		t.Fatalf("supersedeConflicts on a missing dir: %v", err)
	}
	if len(superseded) != 0 {
		t.Errorf("superseded = %v, want none", superseded)
	}
}

func TestFragmentsClaimingNamesTheConflict(t *testing.T) {
	// This is what turns the opaque "ambiguous site definition" JSON dump into
	// an answer: the daemon already knows which app owns the domain.
	cm := newTestCaddyManager(t)
	writeFragment(t, cm, "ressencesystems-old", "ressencesystems.com {\n}\n")
	writeFragment(t, cm, "elsewhere", "other.com {\n}\n")

	claiming, err := cm.fragmentsClaiming([]string{"ressencesystems.com"}, "ressencesystems")
	if err != nil {
		t.Fatalf("fragmentsClaiming: %v", err)
	}
	if !reflect.DeepEqual(claiming, []string{"ressencesystems-old"}) {
		t.Fatalf("claiming = %v, want [ressencesystems-old]", claiming)
	}

	// And the name is what an operator would search for.
	if !strings.Contains(strings.Join(claiming, ","), "ressencesystems-old") {
		t.Error("conflict name not usable in an error message")
	}
}
