package packaging

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/aynaash/nextdeploy/shared/nextcore"
)

func TestAddPrerenderedAsset_QueuesHTMLRSCAndSegmentRSC(t *testing.T) {
	projectRoot := t.TempDir()
	buildDir := filepath.Join(projectRoot, ".next")
	standaloneDir := filepath.Join(buildDir, "standalone")

	writePkgFile(t, filepath.Join(standaloneDir, ".next", "server", "app", "docs.html"), "<html>docs</html>")
	writePkgFile(t, filepath.Join(standaloneDir, ".next", "server", "app", "docs.rsc"), "flight-docs")
	writePkgFile(t, filepath.Join(standaloneDir, ".next", "server", "app", "docs.segments", "_tree.segment.rsc"), "tree-seg")
	writePkgFile(t, filepath.Join(standaloneDir, ".next", "server", "app", "docs.segments", "docs", "__PAGE__.segment.rsc"), "page-seg")

	// Fallback test: route files present in .next/server/app even if omitted from standalone.
	writePkgFile(t, filepath.Join(buildDir, "server", "app", "routes.html"), "<html>routes</html>")
	writePkgFile(t, filepath.Join(buildDir, "server", "app", "routes.segments", "_tree.segment.rsc"), "routes-tree")

	p, err := NewPackager(projectRoot, &nextcore.NextCorePayload{
		DistDir: ".next",
		RouteInfo: nextcore.RouteInfo{
			SSGRoutes: map[string]string{
				"/docs":   "docs.html",
				"/routes": "routes.html",
			},
		},
	})
	if err != nil {
		t.Fatalf("NewPackager: %v", err)
	}
	defer p.Cleanup()

	var assets []S3Asset
	p.addPrerenderedAsset(&assets, "/docs")
	p.addPrerenderedAsset(&assets, "/routes")

	byKey := make(map[string]S3Asset, len(assets))
	for _, a := range assets {
		if _, dup := byKey[a.S3Key]; dup {
			t.Fatalf("duplicate S3Key queued: %s", a.S3Key)
		}
		byKey[a.S3Key] = a
	}

	wants := map[string]string{
		"docs.html":                       "text/html; charset=utf-8",
		"docs.rsc":                        "text/x-component",
		"docs.segments/_tree.segment.rsc": "text/x-component",
		"docs.segments/docs/__PAGE__.segment.rsc": "text/x-component",
		"routes.html":                       "text/html; charset=utf-8",
		"routes.segments/_tree.segment.rsc": "text/x-component",
	}
	for key, wantCT := range wants {
		got, ok := byKey[key]
		if !ok {
			t.Errorf("missing expected S3Asset key %q; got keys=%v", key, keysOf(byKey))
			continue
		}
		if got.ContentType != wantCT {
			t.Errorf("asset %q ContentType = %q, want %q", key, got.ContentType, wantCT)
		}
	}
}

func writePkgFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func keysOf(m map[string]S3Asset) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
