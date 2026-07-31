package nextcompile

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// extractRSCManifestJSON must pull the JSON payload out of Next 15's
// side-effecting .js client-reference-manifest. If it returns nothing, the
// manifest is never wired and "use client" pages throw a client-side exception.
func TestExtractRSCManifestJSON(t *testing.T) {
	js := []byte(`globalThis.__RSC_MANIFEST=(globalThis.__RSC_MANIFEST||{});` +
		`globalThis.__RSC_MANIFEST["/(marketing)/page"]={"clientModules":{"x":1},"ssrModuleMapping":{}};`)
	got, ok := extractRSCManifestJSON(js)
	if !ok {
		t.Fatal("expected to extract JSON from a Next 15 .js manifest")
	}
	if string(got) != `{"clientModules":{"x":1},"ssrModuleMapping":{}}` {
		t.Errorf("payload = %s", got)
	}

	if _, ok := extractRSCManifestJSON([]byte("module.exports = {}")); ok {
		t.Error("expected no extraction from a non-manifest module")
	}
}

// attachClientManifests must wire the manifest from a Next 15 .js file by
// materializing the .json the runtime imports — the fix for the client-side
// hydration exception on Cloudflare.
func TestAttachClientManifests_FromJS(t *testing.T) {
	dir := t.TempDir()
	pageDir := filepath.Join(dir, "server", "app", "(marketing)")
	if err := os.MkdirAll(pageDir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pageDir, "page.js"), []byte("//page"), 0o600); err != nil {
		t.Fatal(err)
	}
	manifest := `globalThis.__RSC_MANIFEST=(globalThis.__RSC_MANIFEST||{});` +
		`globalThis.__RSC_MANIFEST["/(marketing)/page"]={"clientModules":{}}`
	if err := os.WriteFile(filepath.Join(pageDir, "page_client-reference-manifest.js"), []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}

	refs := []ModuleRef{{
		Kind:         RouteKindPage,
		CompiledPath: filepath.ToSlash(filepath.Join("server", "app", "(marketing)", "page.js")),
	}}
	out := attachClientManifests(refs, dir, filepath.Join(dir, "server"))
	if out[0].ClientManifestPath == "" {
		t.Fatal("ClientManifestPath not set from a .js manifest — hydration would break")
	}
	if _, err := os.Stat(filepath.Join(dir, out[0].ClientManifestPath)); err != nil {
		t.Errorf("materialized manifest not found at %s: %v", out[0].ClientManifestPath, err)
	}
}

func TestRoutePathFromCompiled(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"server/app/page.js", "/"},
		{"server/app/dashboard/page.js", "/dashboard"},
		{"server/app/api/users/route.js", "/api/users"},
		{"server/app/blog/[slug]/page.js", "/blog/[slug]"},
		{"server/app/(marketing)/about/page.js", "/about"},
		{"server/app/layout.js", "/_root_layout"},
		{"server/app/dashboard/layout.js", "/dashboard/_layout"},
		{"server/pages/index.js", "/"},
		{"server/pages/blog/[slug].js", "/blog/[slug]"},
		{"server/pages/api/users.js", "/api/users"},
		{"server/middleware.js", "/_middleware"},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			got := routePathFromCompiled(tc.in)
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestKindFromCompiled(t *testing.T) {
	cases := []struct {
		in   string
		want RouteKind
	}{
		{"server/app/page.js", RouteKindPage},
		{"server/app/api/users/route.js", RouteKindAPI},
		{"server/pages/api/users.js", RouteKindAPI},
		{"server/pages/index.js", RouteKindPage},
		{"server/app/layout.js", RouteKindLayout},
		{"server/middleware.js", RouteKindMiddleware},
		{"server/proxy.js", RouteKindProxy},
		{"server/proxy.mjs", RouteKindProxy},
		{"server/chunks/some-chunk.js", RouteKindUnknown},
	}
	for _, tc := range cases {
		got := kindFromCompiled(tc.in)
		if got != tc.want {
			t.Errorf("%s: got %s, want %s", tc.in, got, tc.want)
		}
	}
}

func TestStripRouteGroups(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"(marketing)/about", "about"},
		{"(group1)/(group2)/deep", "deep"},
		{"plain/path", "plain/path"},
		{"blog/[slug]", "blog/[slug]"},
	}
	for _, tc := range cases {
		got := stripRouteGroups(tc.in)
		if got != tc.want {
			t.Errorf("%q → %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestAnalyzeModule_EnvAndFetch(t *testing.T) {
	dir := t.TempDir()
	src := `
export async function GET() {
  const key = process.env.ALGOLIA_KEY;
  const other = process.env.DATABASE_URL;
  const dup = process.env.ALGOLIA_KEY;
  const r = await fetch("https://api.example.com/v1/data");
  const r2 = await fetch('https://other.example.com/q');
  return Response.json({key, other});
}
`
	path := filepath.Join(dir, "server", "app", "api", "users", "route.js")
	writeFile(t, path, src)

	ref, err := analyzeModule(dir, dir, path)
	if err != nil {
		t.Fatalf("analyzeModule: %v", err)
	}

	if ref.RoutePath != "/api/users" {
		t.Errorf("RoutePath: got %q, want /api/users", ref.RoutePath)
	}
	if ref.Kind != RouteKindAPI {
		t.Errorf("Kind: got %s, want api", ref.Kind)
	}
	if len(ref.EnvRefs) != 2 {
		t.Errorf("EnvRefs: got %v, want 2 unique entries", ref.EnvRefs)
	}
	if !contains(ref.EnvRefs, "ALGOLIA_KEY") || !contains(ref.EnvRefs, "DATABASE_URL") {
		t.Errorf("EnvRefs missing expected keys: %v", ref.EnvRefs)
	}
	if len(ref.FetchTargets) != 2 {
		t.Errorf("FetchTargets: got %v, want 2", ref.FetchTargets)
	}
}

func TestAnalyzeModule_RSCAndActions(t *testing.T) {
	dir := t.TempDir()
	src := `
"use client";
import { fn } from "react-server-dom-webpack/client";
// __next_internal_action_entry_do_not_use__ [["action123","default"]]
"use server";
export default function Page() { return null }
`
	path := filepath.Join(dir, "server", "app", "page.js")
	writeFile(t, path, src)

	ref, err := analyzeModule(dir, dir, path)
	if err != nil {
		t.Fatalf("analyzeModule: %v", err)
	}
	if !ref.UsesRSC {
		t.Error("expected UsesRSC=true")
	}
	if !ref.HasActions {
		t.Error("expected HasActions=true")
	}
}

func TestScanCompiledServer_Minimal(t *testing.T) {
	dir := t.TempDir()

	writeFile(t, filepath.Join(dir, ".next", "server", "app", "page.js"), `
export default function Home() { return null }
`)
	writeFile(t, filepath.Join(dir, ".next", "server", "app", "api", "users", "route.js"), `
export async function GET() { return new Response(process.env.X) }
`)
	writeFile(t, filepath.Join(dir, ".next", "server", "middleware.js"), `
export default function mw() {}
`)
	// A chunk that should be skipped.
	writeFile(t, filepath.Join(dir, ".next", "server", "chunks", "util.js"), `export const x = 1`)

	payload := Payload{
		DistDir: ".next",
		Routes: RouteInfo{
			APIRoutes: []string{"/api/users"},
		},
	}

	refs, err := ScanCompiledServer(context.Background(), dir, payload)
	if err != nil {
		t.Fatalf("ScanCompiledServer: %v", err)
	}
	if len(refs) != 3 {
		t.Fatalf("got %d refs, want 3: %+v", len(refs), refs)
	}

	byPath := map[string]ModuleRef{}
	for _, r := range refs {
		byPath[r.RoutePath] = r
	}
	if byPath["/"].Kind != RouteKindPage {
		t.Errorf("/ kind: got %s", byPath["/"].Kind)
	}
	if byPath["/api/users"].Kind != RouteKindAPI {
		t.Errorf("/api/users kind: got %s", byPath["/api/users"].Kind)
	}
	if !contains(byPath["/api/users"].EnvRefs, "X") {
		t.Errorf("/api/users env refs: %v", byPath["/api/users"].EnvRefs)
	}
	if byPath["/_middleware"].Kind != RouteKindMiddleware {
		t.Errorf("middleware kind: got %s", byPath["/_middleware"].Kind)
	}
}

func contains(xs []string, want string) bool {
	return slices.Contains(xs, want)
}

// --- SSR M0: bootstrap chunk extraction -------------------------------------

func TestAppManifestKey(t *testing.T) {
	cases := []struct {
		compiled string
		want     string
		ok       bool
	}{
		{"server/app/page.js", "/page", true},
		{"server/app/layout.js", "/layout", true},
		{"server/app/blog/[id]/page.js", "/blog/[id]/page", true},
		// Route groups / parallel slots survive because we key off the
		// compiled path, not the URL.
		{"server/app/staff/(authed)/dashboard/page.js", "/staff/(authed)/dashboard/page", true},
		{"server/app/@modal/(.)photo/page.js", "/@modal/(.)photo/page", true},
		// Non-app modules belong to a different manifest.
		{"server/pages/index.js", "", false},
		{"server/middleware.js", "", false},
	}
	for _, c := range cases {
		got, ok := appManifestKey(c.compiled)
		if ok != c.ok || got != c.want {
			t.Errorf("appManifestKey(%q) = (%q, %v), want (%q, %v)", c.compiled, got, ok, c.want, c.ok)
		}
	}
}

func TestAncestorPrefixes(t *testing.T) {
	cases := map[string][]string{
		"/page":                          {""},
		"/blog/page":                     {"", "/blog"},
		"/blog/[id]/page":                {"", "/blog", "/blog/[id]"},
		"/staff/(authed)/dashboard/page": {"", "/staff", "/staff/(authed)", "/staff/(authed)/dashboard"},
	}
	for in, want := range cases {
		if got := ancestorPrefixes(in); !slices.Equal(got, want) {
			t.Errorf("ancestorPrefixes(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestBootstrapChunksForRoute_UnionsLayoutAndPageInOrder(t *testing.T) {
	pages := map[string][]string{
		"/layout": {"webpack.js", "framework.js", "shared.js", "main-app.js", "layout.js"},
		"/page":   {"webpack.js", "framework.js", "shared.js", "main-app.js", "page.js"},
	}
	got := bootstrapChunksForRoute("server/app/page.js", pages)
	want := []string{"webpack.js", "framework.js", "shared.js", "main-app.js", "layout.js", "page.js"}
	if !slices.Equal(got, want) {
		t.Fatalf("bootstrap = %v\nwant %v", got, want)
	}
}

func TestBootstrapChunksForRoute_NestedAncestors(t *testing.T) {
	pages := map[string][]string{
		"/layout":         {"w.js", "root-layout.js"},
		"/blog/layout":    {"w.js", "blog-layout.js"},
		"/blog/[id]/page": {"w.js", "post.js"},
	}
	got := bootstrapChunksForRoute("server/app/blog/[id]/page.js", pages)
	want := []string{"w.js", "root-layout.js", "blog-layout.js", "post.js"}
	if !slices.Equal(got, want) {
		t.Fatalf("nested = %v\nwant %v", got, want)
	}
}

func TestBootstrapChunksForRoute_MissingEntryIsEmpty(t *testing.T) {
	if got := bootstrapChunksForRoute("server/app/ghost/page.js", map[string][]string{}); len(got) != 0 {
		t.Fatalf("want empty for absent route, got %v", got)
	}
}

func TestBootstrapChunksForRoute_PagesRouterIsSkipped(t *testing.T) {
	pages := map[string][]string{"/page": {"w.js"}}
	if got := bootstrapChunksForRoute("server/pages/index.js", pages); got != nil {
		t.Fatalf("Pages Router module must yield nil, got %v", got)
	}
}

// End-to-end against the committed production Next 15 build.
func TestAttachBootstrapChunks_RealFixture(t *testing.T) {
	root := filepath.Join("testdata", "fixtures", "next15-appdir") // = the ".next" dir
	refs := []ModuleRef{
		{RoutePath: "/", Kind: RouteKindPage, CompiledPath: "server/app/page.js"},
		{RoutePath: "/blog/[id]", Kind: RouteKindPage, CompiledPath: "server/app/blog/[id]/page.js"},
		// The route group "(authed)" is absent from the URL but present in the
		// manifest key — the case that motivates keying off CompiledPath.
		{RoutePath: "/staff/dashboard", Kind: RouteKindPage, CompiledPath: "server/app/staff/(authed)/dashboard/page.js"},
		{RoutePath: "/api/x", Kind: RouteKindAPI, CompiledPath: "server/app/api/x/route.js"},
	}
	refs = attachBootstrapChunks(refs, root)

	// "/" = root layout ++ page, deduped: 4 shared + layout + page.
	home := refs[0].BootstrapChunks
	if len(home) != 6 {
		t.Fatalf("want 6 deduped chunks for /, got %d: %v", len(home), home)
	}
	if !strings.HasPrefix(home[0], "static/chunks/webpack-") {
		t.Errorf("first chunk = %q, want the webpack runtime", home[0])
	}
	if !strings.HasPrefix(home[len(home)-1], "static/chunks/app/page-") {
		t.Errorf("last chunk = %q, want the page chunk", home[len(home)-1])
	}
	if !strings.HasPrefix(home[len(home)-2], "static/chunks/app/layout-") {
		t.Errorf("penultimate chunk = %q, want the root layout chunk", home[len(home)-2])
	}

	// "/blog/[id]" adds the nested blog layout: 4 shared + 2 layouts + page.
	blog := refs[1].BootstrapChunks
	if len(blog) != 7 {
		t.Fatalf("want 7 chunks for /blog/[id], got %d: %v", len(blog), blog)
	}
	if !strings.HasPrefix(blog[len(blog)-1], "static/chunks/app/blog/[id]/page-") {
		t.Errorf("last chunk = %q, want the blog page chunk", blog[len(blog)-1])
	}

	// The regression this pass exists to prevent: a route-grouped page must
	// resolve its group's layout chunk, not come back empty.
	// 4 shared + root layout + the group's own layout + the page.
	dash := refs[2].BootstrapChunks
	if len(dash) != 7 {
		t.Fatalf("route-grouped page got %d chunks, want 7: %v", len(dash), dash)
	}
	var sawGroupLayout bool
	for _, c := range dash {
		if strings.Contains(c, "app/staff/(authed)/layout-") {
			sawGroupLayout = true
		}
	}
	if !sawGroupLayout {
		t.Errorf("route-group layout chunk missing from %v", dash)
	}

	// Non-page kinds are left alone.
	if refs[3].BootstrapChunks != nil {
		t.Errorf("API route should carry no bootstrap chunks, got %v", refs[3].BootstrapChunks)
	}

	// No duplicates survived in any list.
	for _, r := range refs {
		seen := map[string]bool{}
		for _, c := range r.BootstrapChunks {
			if seen[c] {
				t.Fatalf("duplicate chunk %q for %s: %v", c, r.RoutePath, r.BootstrapChunks)
			}
			seen[c] = true
		}
	}
}

func TestAttachBootstrapChunks_MissingManifestIsNonFatal(t *testing.T) {
	refs := []ModuleRef{{RoutePath: "/", Kind: RouteKindPage, CompiledPath: "server/app/page.js"}}
	refs = attachBootstrapChunks(refs, t.TempDir()) // no app-build-manifest.json
	if refs[0].BootstrapChunks != nil {
		t.Fatalf("want nil chunks when manifest absent, got %v", refs[0].BootstrapChunks)
	}
}

func TestAttachBootstrapChunks_MalformedManifestIsNonFatal(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "app-build-manifest.json"), []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	refs := []ModuleRef{{RoutePath: "/", Kind: RouteKindPage, CompiledPath: "server/app/page.js"}}
	refs = attachBootstrapChunks(refs, dir)
	if refs[0].BootstrapChunks != nil {
		t.Fatalf("want nil chunks for malformed manifest, got %v", refs[0].BootstrapChunks)
	}
}

// --- SSR M2: stylesheet extraction ------------------------------------------
//
// CAVEAT on fixture coverage: the committed next15-appdir fixture was built
// from an app with no CSS, so every entryCSSFiles value in it is []. It pins
// the KEY shape (absolute source paths) and proves the empty case, but the
// populated element shape below is reconstructed from Next's emitters rather
// than captured. Both known encodings are accepted for that reason; if a real
// build ever disagrees, these tests are the place to correct it.

func TestParseEntryCSSFiles_StringForm(t *testing.T) {
	got, err := parseEntryCSSFiles([]byte(`{"entryCSSFiles":{
		"/abs/app/layout": ["static/css/root.css"],
		"/abs/app/page":   ["static/css/page.css"]
	}}`))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"static/css/root.css", "static/css/page.css"}
	if !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestParseEntryCSSFiles_ObjectForm(t *testing.T) {
	got, err := parseEntryCSSFiles([]byte(`{"entryCSSFiles":{
		"/abs/app/layout": [{"inlined":false,"path":"static/css/root.css"}],
		"/abs/app/page":   [{"inlined":true,"path":"static/css/page.css"}]
	}}`))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"static/css/root.css", "static/css/page.css"}
	if !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

// Cascade order is a correctness property, not cosmetics: the root layout's
// sheet must precede the page's or later rules lose. A map-based decode would
// randomize this, which is why the parser walks JSON tokens.
func TestParseEntryCSSFiles_PreservesManifestOrder(t *testing.T) {
	src := []byte(`{"entryCSSFiles":{
		"/abs/":                  [],
		"/abs/app/layout":        ["a.css"],
		"/abs/app/blog/layout":   ["b.css"],
		"/abs/app/blog/[id]/page":["c.css"]
	}}`)
	want := []string{"a.css", "b.css", "c.css"}
	// Repeat: a map-based implementation would pass this only by luck.
	for i := range 20 {
		got, err := parseEntryCSSFiles(src)
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(got, want) {
			t.Fatalf("iteration %d: got %v, want %v", i, got, want)
		}
	}
}

func TestParseEntryCSSFiles_DedupesSharedSheets(t *testing.T) {
	got, err := parseEntryCSSFiles([]byte(`{"entryCSSFiles":{
		"/abs/app/layout": ["shared.css"],
		"/abs/app/page":   ["shared.css", "page.css"]
	}}`))
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"shared.css", "page.css"}; !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestParseEntryCSSFiles_EmptyAndAbsent(t *testing.T) {
	cases := map[string]string{
		"all entries empty": `{"entryCSSFiles":{"/abs/app/page":[]}}`,
		"key absent":        `{"clientModules":{}}`,
		"empty object":      `{"entryCSSFiles":{}}`,
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := parseEntryCSSFiles([]byte(src))
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != 0 {
				t.Errorf("want no stylesheets, got %v", got)
			}
		})
	}
}

func TestParseEntryCSSFiles_MalformedIsAnError(t *testing.T) {
	if _, err := parseEntryCSSFiles([]byte(`{"entryCSSFiles": "not-an-object"}`)); err == nil {
		t.Error("want an error for a non-object entryCSSFiles")
	}
	if _, err := parseEntryCSSFiles([]byte(`{`)); err == nil {
		t.Error("want an error for malformed JSON")
	}
}

func TestAttachStylesheets_ReadsThePerRouteManifest(t *testing.T) {
	dir := t.TempDir()
	manifestRel := "server/app/page_client-reference-manifest.json"
	if err := os.MkdirAll(filepath.Join(dir, "server", "app"), 0o750); err != nil {
		t.Fatal(err)
	}
	body := `{"entryCSSFiles":{"/abs/app/layout":["static/css/root.css"],"/abs/app/page":["static/css/page.css"]}}`
	if err := os.WriteFile(filepath.Join(dir, manifestRel), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	refs := []ModuleRef{
		{RoutePath: "/", Kind: RouteKindPage, CompiledPath: "server/app/page.js", ClientManifestPath: manifestRel},
		// No manifest wired → left alone rather than failing the pass.
		{RoutePath: "/other", Kind: RouteKindPage, CompiledPath: "server/app/other/page.js"},
		// API routes never carry stylesheets.
		{RoutePath: "/api/x", Kind: RouteKindAPI, CompiledPath: "server/app/api/x/route.js", ClientManifestPath: manifestRel},
	}
	refs = attachStylesheets(refs, dir)

	if want := []string{"static/css/root.css", "static/css/page.css"}; !slices.Equal(refs[0].StylesheetChunks, want) {
		t.Errorf("page css = %v, want %v", refs[0].StylesheetChunks, want)
	}
	if refs[1].StylesheetChunks != nil {
		t.Errorf("want nil for a page with no manifest, got %v", refs[1].StylesheetChunks)
	}
	if refs[2].StylesheetChunks != nil {
		t.Errorf("want nil for an API route, got %v", refs[2].StylesheetChunks)
	}
}

func TestAttachStylesheets_UnreadableManifestIsNonFatal(t *testing.T) {
	refs := []ModuleRef{{
		RoutePath: "/", Kind: RouteKindPage, CompiledPath: "server/app/page.js",
		ClientManifestPath: "server/app/does-not-exist.json",
	}}
	if refs = attachStylesheets(refs, t.TempDir()); refs[0].StylesheetChunks != nil {
		t.Errorf("want nil chunks for a missing manifest, got %v", refs[0].StylesheetChunks)
	}
}

// The committed fixture's arrays are all empty — assert exactly that, so if a
// future fixture regen captures an app WITH css this test fails loudly and
// gets updated rather than silently continuing to prove nothing.
func TestAttachStylesheets_RealFixtureHasNoCSS(t *testing.T) {
	root := filepath.Join("testdata", "fixtures", "next15-appdir")
	rel, err := filepath.Rel(root, filepath.Join(root, "server", "app", "page_client-reference-manifest.js"))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		t.Skipf("fixture manifest unreadable: %v", err)
	}
	payload, ok := extractRSCManifestJSON(raw)
	if !ok {
		t.Fatal("could not extract JSON from the fixture manifest")
	}
	got, err := parseEntryCSSFiles(payload)
	if err != nil {
		t.Fatalf("fixture entryCSSFiles did not parse: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("fixture now HAS css (%v) — populated-shape assumptions above are testable for real now; update them", got)
	}
}
