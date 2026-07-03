package nextcompile

import (
	"os"
	"path/filepath"
	"testing"
)

// C4 — parse the REAL server-reference-manifest.json captured from Next 14.2 and
// Next 15.1 builds (testdata/fixtures). Next 14's `workers` value is a bare
// string moduleId; Next 15's is a {moduleId, async} object. Both must parse
// (the old map[string]string aborted on the Next 15 object), and both must set
// Module to the module path and Export to the moduleId.
func TestParseUpstreamManifest_RealFixtures(t *testing.T) {
	cases := []struct {
		dir          string
		wantModuleID string
	}{
		{"next14", "9459"},
		{"next15", "1149"},
	}
	for _, tc := range cases {
		t.Run(tc.dir, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join("testdata", "fixtures", tc.dir, "server-reference-manifest.json"))
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}
			m, err := parseUpstreamManifest(data)
			if err != nil {
				t.Fatalf("parse %s manifest: %v", tc.dir, err)
			}
			if len(m.Actions) == 0 {
				t.Fatalf("%s: expected at least one action, got none", tc.dir)
			}
			for id, a := range m.Actions {
				if a.Module != "app/page" {
					t.Errorf("%s action %s: Module = %q, want the module path \"app/page\"", tc.dir, id, a.Module)
				}
				if a.Export != tc.wantModuleID {
					t.Errorf("%s action %s: Export(moduleId) = %q, want %q", tc.dir, id, a.Export, tc.wantModuleID)
				}
				if a.Runtime != ActionRuntimeNode {
					t.Errorf("%s action %s: Runtime = %q, want node", tc.dir, id, a.Runtime)
				}
			}
		})
	}
}
