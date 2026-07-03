package nextcompile

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// C1 — action loaders must carry the dist-dir prefix so they resolve to the
// same <distDir>/server/... path pages use.
func TestRenderDispatchTable_ActionLoaderHasDistDirPrefix(t *testing.T) {
	actions := &ActionManifest{
		SchemaVersion: actionManifestSchemaVersion,
		Actions: map[string]Action{
			"abc123": {ID: "abc123", Module: "app/actions/page", Export: "createPost", Runtime: ActionRuntimeNode},
		},
	}

	src := renderDispatchTable(nil, actions, "../", ".next")
	if !strings.Contains(src, `import("../.next/server/app/actions/page.js")`) {
		t.Errorf("action loader missing dist-dir prefix:\n%s", src)
	}
	if strings.Contains(src, `import("../server/app/actions/page.js")`) {
		t.Errorf("action loader still drops the dist-dir segment:\n%s", src)
	}

	// Custom dist dir must be honored end-to-end.
	src2 := renderDispatchTable(nil, actions, "../", "dist")
	if !strings.Contains(src2, `import("../dist/server/app/actions/page.js")`) {
		t.Errorf("custom distDir not honored:\n%s", src2)
	}
}

// C2 — the root layout must appear exactly once in a page's layout chain.
func TestAttachLayoutChains_NoDuplicateRootLayout(t *testing.T) {
	refs := []ModuleRef{
		{Kind: RouteKindLayout, CompiledPath: ".next/server/app/layout.js"},
		{Kind: RouteKindLayout, CompiledPath: ".next/server/app/dashboard/layout.js"},
		{Kind: RouteKindPage, CompiledPath: ".next/server/app/dashboard/page.js"},
	}

	out := attachLayoutChains(refs, "/app", "/app/.next")

	var page *ModuleRef
	for i := range out {
		if out[i].Kind == RouteKindPage {
			page = &out[i]
		}
	}
	if page == nil {
		t.Fatal("page ref missing from output")
	}
	want := []string{
		".next/server/app/layout.js",
		".next/server/app/dashboard/layout.js",
	}
	if !reflect.DeepEqual(page.LayoutChain, want) {
		t.Errorf("LayoutChain = %v, want %v (root layout must appear exactly once)", page.LayoutChain, want)
	}
}

// C3 — a user route directory named "chunks" must be scanned; only the internal
// <serverDir>/chunks is skipped.
func TestCollectCompiledFiles_KeepsUserChunksRoute(t *testing.T) {
	dir := t.TempDir()
	mustWrite := func(rel, body string) {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	mustWrite("chunks/webpack-runtime.js", "// Next internal") // must be skipped
	mustWrite("app/chunks/page.js", "export default null")     // user route: keep
	mustWrite("app/media/page.js", "export default null")

	got, err := collectCompiledFiles(dir)
	if err != nil {
		t.Fatal(err)
	}
	has := func(suffix string) bool {
		for _, p := range got {
			if strings.HasSuffix(filepath.ToSlash(p), suffix) {
				return true
			}
		}
		return false
	}
	if !has("app/chunks/page.js") {
		t.Errorf("user route app/chunks/page.js was dropped; got %v", got)
	}
	if !has("app/media/page.js") {
		t.Errorf("app/media/page.js missing; got %v", got)
	}
	if has("chunks/webpack-runtime.js") {
		t.Errorf("internal <serverDir>/chunks was not skipped; got %v", got)
	}
}

// C6 — a read error on an existing manifest must surface, not degrade to "no actions".
func TestDetectServerActions_ReadErrorNotSwallowed(t *testing.T) {
	dir := t.TempDir()
	// Place a DIRECTORY where the manifest file is expected → os.ReadFile
	// returns a non-NotExist error on a path that "exists".
	blocker := filepath.Join(dir, ".next", "server", "server-reference-manifest.json")
	if err := os.MkdirAll(blocker, 0o750); err != nil {
		t.Fatal(err)
	}
	_, err := DetectServerActions(dir, ".next")
	if err == nil || errors.Is(err, ErrNoActionManifest) {
		t.Fatalf("expected a real read error, got %v", err)
	}
}

// C7 — version detection must climb to the repo-root package.json.
func TestReadDepVersions_ClimbsToRepoRoot(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "package.json"),
		[]byte(`{"dependencies":{"next":"14.2.3","react":"18.3.1"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	standalone := filepath.Join(root, ".next", "standalone")
	if err := os.MkdirAll(standalone, 0o750); err != nil {
		t.Fatal(err)
	}
	nv, _, err := readDepVersions(standalone)
	if err != nil {
		t.Fatalf("expected resolution from repo root, got %v", err)
	}
	if nv != "14.2.3" {
		t.Errorf("next version = %q, want 14.2.3", nv)
	}
}

// C8 — Features.After reflects UsesAfter on any scanned module.
func TestBuildFeatures_DetectsAfter(t *testing.T) {
	if !buildFeatures(Payload{}, []ModuleRef{{Kind: RouteKindPage, UsesAfter: true}}).After {
		t.Error("Features.After should be true when a module uses after()")
	}
	if buildFeatures(Payload{}, []ModuleRef{{Kind: RouteKindPage}}).After {
		t.Error("Features.After should be false with no after() usage")
	}
}

// C5 — hashBundle is deterministic across absolute roots and excludes the
// wall-clock GeneratedAt.
func TestHashBundle_DeterministicAcrossRoots(t *testing.T) {
	writeTree := func() string {
		d := t.TempDir()
		if err := os.WriteFile(filepath.Join(d, "a.js"), []byte("console.log(1)"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(d, "b.js"), []byte("console.log(2)"), 0o600); err != nil {
			t.Fatal(err)
		}
		return d
	}
	rootA, rootB := writeTree(), writeTree()

	// Same content, different GeneratedAt → identical hash.
	mA := Manifest{AppName: "app", GeneratedAt: "2020-01-01T00:00:00Z"}
	mB := Manifest{AppName: "app", GeneratedAt: "2099-12-31T23:59:59Z"}

	hA, _, err := hashBundle(rootA, mA, []string{filepath.Join(rootA, "a.js"), filepath.Join(rootA, "b.js")})
	if err != nil {
		t.Fatal(err)
	}
	hB, _, err := hashBundle(rootB, mB, []string{filepath.Join(rootB, "a.js"), filepath.Join(rootB, "b.js")})
	if err != nil {
		t.Fatal(err)
	}
	if hA != hB {
		t.Errorf("hashBundle not deterministic across roots/timestamps:\n  A=%s\n  B=%s", hA, hB)
	}
}
