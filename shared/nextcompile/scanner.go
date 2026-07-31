package nextcompile

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"golang.org/x/sync/errgroup"
)

// scanConcurrency caps parallel file reads. Standalone trees for large apps
// can contain 50k+ files; uncapped errgroup Go routines thrash open-file
// limits on macOS (default 256). 32 is empirically a sweet spot across
// common dev hardware.
const scanConcurrency = 32

// Compiled-path prefixes that classify where a module lives in Next's
// standalone output. `app/` for App Router, `pages/` for Pages Router.
const (
	appRouterPrefix   = "app/"
	pagesRouterPrefix = "pages/"
)

// ScanCompiledServer walks the server subtree of a Next standalone build
// in parallel and returns a ModuleRef per compiled route/handler/middleware.
// Non-server assets (client chunks, static files) are skipped — those flow
// to the CDN via packaging.S3Assets, not into the Worker bundle.
//
// The caller's Payload supplies route classification; the scanner's job is
// to attach compiled file paths to each classified route and extract the
// static-analysis facts (env refs, fetch targets, RSC markers) that later
// phases consume.
func ScanCompiledServer(ctx context.Context, standaloneDir string, payload Payload) ([]ModuleRef, error) {
	serverDir := filepath.Join(standaloneDir, payloadDistDir(payload), "server")
	if _, err := os.Stat(serverDir); err != nil {
		return nil, fmt.Errorf("server dir not found at %s: %w", serverDir, err)
	}

	paths, err := collectCompiledFiles(serverDir)
	if err != nil {
		return nil, err
	}

	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(scanConcurrency)

	// classifyRoot is the parent of `server/` — i.e. the .next directory.
	// Paths relative to classifyRoot start with "server/..." which is what
	// routePathFromCompiled / kindFromCompiled expect.
	classifyRoot := filepath.Dir(serverDir)

	refs := make([]ModuleRef, len(paths))
	for i, p := range paths {
		g.Go(func() error {
			if err := gctx.Err(); err != nil {
				return err
			}
			ref, err := analyzeModule(standaloneDir, classifyRoot, p)
			if err != nil {
				return fmt.Errorf("analyze %s: %w", p, err)
			}
			refs[i] = ref
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, err
	}

	// Tag each ref against the route classification. This is the bit that
	// distinguishes an /api/users handler from a /dashboard page — the
	// compiled path alone is ambiguous (both land under server/app/...).
	refs = classifyRefs(refs, payload)

	// Second pass: attach client reference manifests + layout chains to
	// page refs. Cheap — each is a stat(). Done after classification so we
	// only walk up the tree for confirmed page kinds.
	refs = attachClientManifests(refs, standaloneDir, classifyRoot)
	refs = attachLayoutChains(refs, standaloneDir, classifyRoot)
	refs = attachBootstrapChunks(refs, classifyRoot)
	refs = attachStylesheets(refs, standaloneDir)

	sort.Slice(refs, func(i, j int) bool {
		return refs[i].RoutePath < refs[j].RoutePath
	})
	return refs, nil
}

// attachClientManifests looks for the Next-emitted
// `page_client-reference-manifest.json` sibling next to each page.js and
// attaches its relative path to the ModuleRef. Next 14+ emits one per RSC
// page; its contents are the bundlerConfig the Flight encoder needs to
// serialize "use client" boundaries.
func attachClientManifests(refs []ModuleRef, standaloneDir, classifyRoot string) []ModuleRef {
	for i := range refs {
		r := &refs[i]
		if r.Kind != RouteKindPage {
			continue
		}
		// Sibling convention: for .../<route>/page.js, look for
		// .../<route>/page_client-reference-manifest.{json,js}.
		abs := filepath.Join(standaloneDir, r.CompiledPath)
		dir := filepath.Dir(abs)
		base := filepath.Base(abs)
		stem := strings.TrimSuffix(base, filepath.Ext(base))
		jsonPath := filepath.Join(dir, stem+"_client-reference-manifest.json")

		// Older Next emits a .json we can import as data directly. Next 15 instead
		// emits a side-effecting .js module that assigns the manifest to a global:
		//   globalThis.__RSC_MANIFEST["<key>"]={...json...}
		// When only the .js exists, extract its JSON payload into the .json
		// sibling so the runtime's `import(..., {type:"json"})` keeps working.
		// Without this the manifest is never wired (loadClientManifest: null) and
		// "use client" boundaries fail to hydrate — a client-side exception on
		// every page that ships interactive components.
		if _, err := os.Stat(jsonPath); err != nil {
			jsPath := filepath.Join(dir, stem+"_client-reference-manifest.js")
			data, readErr := os.ReadFile(jsPath) // #nosec G304 — reading our own build output
			if readErr != nil {
				continue
			}
			payload, ok := extractRSCManifestJSON(data)
			if !ok {
				continue
			}
			if writeErr := os.WriteFile(jsonPath, payload, 0o600); writeErr != nil {
				continue
			}
		}

		rel, err := filepath.Rel(standaloneDir, jsonPath)
		if err != nil {
			continue
		}
		r.ClientManifestPath = filepath.ToSlash(rel)
	}
	return refs
}

// attachStylesheets fills ModuleRef.StylesheetChunks from the per-route
// client-reference-manifest's entryCSSFiles map.
//
// Runs after attachClientManifests, which is what guarantees
// ClientManifestPath points at a readable .json (it materializes one from
// Next 15's .js form when needed).
//
// Non-fatal throughout, like its sibling passes: a page with no resolvable
// stylesheet list renders unstyled, which beats failing the compile.
func attachStylesheets(refs []ModuleRef, standaloneDir string) []ModuleRef {
	for i := range refs {
		r := &refs[i]
		if r.Kind != RouteKindPage || r.ClientManifestPath == "" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(standaloneDir, r.ClientManifestPath)) // #nosec G304 — our own build output
		if err != nil {
			continue
		}
		css, err := parseEntryCSSFiles(data)
		if err != nil {
			continue
		}
		r.StylesheetChunks = css
	}
	return refs
}

// entryCSSManifest is the sliver of the client-reference-manifest we read for
// stylesheets. RawMessage because entryCSSFiles' key ORDER is load-bearing
// (CSS cascade) and unmarshaling into a map would discard it.
type entryCSSManifest struct {
	EntryCSSFiles json.RawMessage `json:"entryCSSFiles"`
}

// parseEntryCSSFiles flattens entryCSSFiles into one ordered, deduped list of
// dist-relative CSS paths.
//
// Shape (Next 15):
//
//	"entryCSSFiles": {
//	  "<appRoot>/":            [],
//	  "<appRoot>/app/layout":  ["static/css/root.css"],
//	  "<appRoot>/app/page":    [{"inlined":false,"path":"static/css/page.css"}]
//	}
//
// Two things make this fiddly and are handled deliberately:
//
//   - Keys are ABSOLUTE source paths on the build machine, so they can't be
//     matched against our compiled paths. We don't try: the manifest is
//     emitted per route and already contains exactly that route's chain
//     (root → layout → page), so flattening every value in key order is both
//     simpler and more robust than path matching.
//   - Values are strings on some Next 15 minors and {inlined, path} objects on
//     others (the latter arrived with inline-CSS support). cssEntry accepts
//     both rather than betting on one.
func parseEntryCSSFiles(manifestJSON []byte) ([]string, error) {
	var m entryCSSManifest
	if err := json.Unmarshal(manifestJSON, &m); err != nil {
		return nil, err
	}
	if len(m.EntryCSSFiles) == 0 {
		return nil, nil
	}

	dec := json.NewDecoder(bytes.NewReader(m.EntryCSSFiles))
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	if delim, ok := tok.(json.Delim); !ok || delim != '{' {
		return nil, fmt.Errorf("entryCSSFiles: want object, got %v", tok)
	}

	seen := map[string]struct{}{}
	var out []string
	for dec.More() {
		if _, err := dec.Token(); err != nil { // the key; position matters, value doesn't
			return nil, err
		}
		var entries []cssEntry
		if err := dec.Decode(&entries); err != nil {
			return nil, err
		}
		for _, e := range entries {
			if e.Path == "" {
				continue
			}
			if _, dup := seen[e.Path]; dup {
				continue
			}
			seen[e.Path] = struct{}{}
			out = append(out, e.Path)
		}
	}
	return out, nil
}

// cssEntry decodes either "static/css/a.css" or {"inlined":false,"path":"…"}.
type cssEntry struct {
	Path string
}

func (c *cssEntry) UnmarshalJSON(b []byte) error {
	trimmed := bytes.TrimSpace(b)
	if len(trimmed) > 0 && trimmed[0] == '"' {
		return json.Unmarshal(trimmed, &c.Path)
	}
	var obj struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(trimmed, &obj); err != nil {
		return err
	}
	c.Path = obj.Path
	return nil
}

// extractRSCManifestJSON pulls the JSON manifest object out of a Next.js 15
// page_client-reference-manifest.js, whose shape is:
//
//	globalThis.__RSC_MANIFEST=(globalThis.__RSC_MANIFEST||{});globalThis.__RSC_MANIFEST["<key>"]={...}
//
// The JSON object is everything after the first `]=` (the key bracket close).
// JSON has no `=` outside string values, so the first `]=` reliably marks the
// start of the payload.
func extractRSCManifestJSON(js []byte) ([]byte, bool) {
	s := string(js)
	_, after, ok := strings.Cut(s, "]=")
	if !ok {
		return nil, false
	}
	payload := strings.TrimSpace(after)
	payload = strings.TrimSuffix(strings.TrimSpace(payload), ";")
	payload = strings.TrimSpace(payload)
	if !json.Valid([]byte(payload)) {
		return nil, false
	}
	return []byte(payload), true
}

// attachLayoutChains walks the App Router tree for each page ref and
// records every layout.js on the path from root to the page's containing
// directory. classifyRoot lets us compute directory structure without
// re-parsing the standalone tree.
func attachLayoutChains(refs []ModuleRef, standaloneDir, classifyRoot string) []ModuleRef {
	// Index layouts by their enclosing directory (relative to classifyRoot).
	// A layout at `server/app/dashboard/layout.js` applies to everything
	// under `server/app/dashboard/**`.
	layoutByDir := map[string]string{}
	for _, r := range refs {
		if r.Kind != RouteKindLayout {
			continue
		}
		// r.CompiledPath is relative to standaloneDir; we want it relative
		// to classifyRoot for comparison with page dirs.
		layoutAbs := filepath.Join(standaloneDir, r.CompiledPath)
		classifyRel, err := filepath.Rel(classifyRoot, layoutAbs)
		if err != nil {
			continue
		}
		dir := filepath.Dir(filepath.ToSlash(classifyRel))
		layoutByDir[dir] = r.CompiledPath
	}

	for i := range refs {
		r := &refs[i]
		if r.Kind != RouteKindPage {
			continue
		}
		pageAbs := filepath.Join(standaloneDir, r.CompiledPath)
		pageRel, err := filepath.Rel(classifyRoot, pageAbs)
		if err != nil {
			continue
		}
		pageDir := filepath.Dir(filepath.ToSlash(pageRel))

		// Walk from the page's directory up to the app root, collecting each
		// ancestor's layout.js. The loop already visits server/app (it runs
		// while dir != "." && dir != "/"), so there's no separate top-level
		// append — that double-counted the root layout. A seen-set guarantees
		// no path shape can reintroduce a duplicate. Ordered page-nearest to
		// root-nearest; we reverse below so the runtime applies root-first.
		var chain []string
		seen := map[string]struct{}{}
		for dir := pageDir; dir != "." && dir != "/"; dir = filepath.Dir(dir) {
			layoutPath, ok := layoutByDir[dir]
			if !ok {
				continue
			}
			if _, dup := seen[layoutPath]; dup {
				continue
			}
			seen[layoutPath] = struct{}{}
			chain = append(chain, layoutPath)
		}

		// Reverse to root→leaf ordering.
		for l, rr := 0, len(chain)-1; l < rr; l, rr = l+1, rr-1 {
			chain[l], chain[rr] = chain[rr], chain[l]
		}
		r.LayoutChain = chain
	}
	return refs
}

// payloadDistDir returns the configured dist directory or the Next default.
func payloadDistDir(p Payload) string {
	if p.DistDir != "" {
		return p.DistDir
	}
	return ".next"
}

// collectCompiledFiles walks serverDir and returns every .js / .mjs file
// below it. Ordered deterministically for reproducible builds.
func collectCompiledFiles(serverDir string) ([]string, error) {
	var out []string
	err := filepath.WalkDir(serverDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			// Skip ONLY Next's internal build dir at <serverDir>/chunks — never
			// a user route directory that happens to be named "chunks" (e.g.
			// app/chunks/page.tsx compiles to server/app/chunks/page.js and must
			// be scanned). Matching the name at any depth silently dropped those.
			if rel, err := filepath.Rel(serverDir, path); err == nil && filepath.ToSlash(rel) == "chunks" {
				return fs.SkipDir
			}
			return nil
		}
		ext := filepath.Ext(path)
		if ext == ".js" || ext == ".mjs" {
			out = append(out, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(out)
	return out, nil
}

// analyzeModule reads a single compiled file and extracts the facts the
// compiler needs for dispatch + binding derivation. It does NOT parse JS —
// Next's compiled output is regular enough that regex grep on the source
// is accurate and orders of magnitude faster than a full AST pass. If
// that assumption ever breaks we'll swap in esbuild's parser here.
//
// standaloneDir is the bundle root (used for CompiledPath display).
// classifyRoot is the directory whose children are `server/`, `static/`,
// etc. — normally the Next dist dir. Paths relative to classifyRoot are
// what routePathFromCompiled / kindFromCompiled are calibrated for.
func analyzeModule(standaloneDir, classifyRoot, absPath string) (ModuleRef, error) {
	displayRel, err := filepath.Rel(standaloneDir, absPath)
	if err != nil {
		return ModuleRef{}, err
	}
	classifyRel, err := filepath.Rel(classifyRoot, absPath)
	if err != nil {
		return ModuleRef{}, err
	}
	info, err := os.Stat(absPath)
	if err != nil {
		return ModuleRef{}, err
	}

	data, err := os.ReadFile(absPath) // #nosec G304 — compiler reads its own output tree
	if err != nil {
		return ModuleRef{}, err
	}
	src := string(data)

	classifySlash := filepath.ToSlash(classifyRel)
	ref := ModuleRef{
		CompiledPath: filepath.ToSlash(displayRel),
		RoutePath:    routePathFromCompiled(classifySlash),
		Kind:         kindFromCompiled(classifySlash),
		ByteSize:     info.Size(),
		EnvRefs:      uniqueStrings(envRefPattern.FindAllStringSubmatch(src, -1), 1),
		FetchTargets: uniqueStrings(fetchURLPattern.FindAllStringSubmatch(src, -1), 1),
		UsesRSC:      usesRSCPattern.MatchString(src),
		HasActions:   hasActionsPattern.MatchString(src),
		PPREnabled:   pprPattern.MatchString(src),
		UsesAfter:    afterPattern.MatchString(src),
	}
	return ref, nil
}

// ── Regex surface ────────────────────────────────────────────────────────────
//
// These are tuned against Next 13/14/15 compiled output. Patterns are
// conservative — false negatives (missing a ref) are cheaper than false
// positives (suggesting a binding that doesn't exist). When a new Next
// version lands, the CI fixture matrix (testdata/fixtures/*) will flag
// any pattern that drifts.

var (
	// process.env.FOO_BAR — minified output preserves the exact form because
	// Next explicitly doesn't rename env references (they're static-replaced
	// by webpack/turbopack in some paths, left intact in others).
	envRefPattern = regexp.MustCompile(`process\.env\.([A-Z][A-Z0-9_]*)`)

	// fetch("https://...") or fetch('https://...') — double OR single quote,
	// url-ish body up to the next quote. Template literals are intentionally
	// not matched here; they'd produce too many false positives.
	fetchURLPattern = regexp.MustCompile(`\bfetch\(\s*['"]([a-z][a-z0-9+.-]*://[^'"]+)['"]`)

	// Flight / RSC markers. Two tells:
	//   - "use client" pragma preserved in compiled output
	//   - imports from react-server-dom-* runtime modules
	usesRSCPattern = regexp.MustCompile(`"use client"|react-server-dom-`)

	// Server Actions: compiled modules tag their action exports with a
	// specific comment marker plus a $$typeof reference. We look for either.
	hasActionsPattern = regexp.MustCompile(`"use server"|__next_internal_action_entry_do_not_use__`)

	// PPR opt-in markers. Next 14+ exports `experimental_ppr = true` from
	// pages that opt into Partial Prerendering. Compiled output preserves
	// the identifier. `__NEXT_PPR_STATIC_` is a second tell observed in
	// some Next 15 canary builds where the static shell has already been
	// materialized at build time.
	pprPattern = regexp.MustCompile(`experimental_ppr\s*=\s*true|__NEXT_PPR_STATIC_`)

	// after() — Next's post-response work API (next/server). "unstable_after"
	// (Next 14.x) survives verbatim in compiled output; the stable "after"
	// (Next 15) is too common a word to match bare, so we also accept a
	// __next_after tell. Validate against a real Next 15 build before relying
	// on this signal — compiled output is the ground truth.
	afterPattern = regexp.MustCompile(`\bunstable_after\b|__next_after__`)
)

// uniqueStrings deduplicates a regex FindAllStringSubmatch result by the
// capture group at `group` (1 = first paren group). Sorted for determinism.
func uniqueStrings(matches [][]string, group int) []string {
	if len(matches) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(matches))
	for _, m := range matches {
		if len(m) <= group {
			continue
		}
		seen[m[group]] = struct{}{}
	}
	out := make([]string, 0, len(seen))
	for s := range seen {
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

// ── Path classification ─────────────────────────────────────────────────────

// routePathFromCompiled translates a compiled path into the URL route.
// Examples:
//
//	server/app/dashboard/page.js       → /dashboard
//	server/app/api/users/route.js      → /api/users
//	server/pages/index.js              → /
//	server/pages/blog/[slug].js        → /blog/[slug]
//	server/middleware.js               → /_middleware
//	server/proxy.js                    → /_proxy
//	server/app/layout.js               → /_root_layout
func routePathFromCompiled(rel string) string {
	rel = strings.TrimPrefix(rel, "server/")

	switch {
	case rel == "middleware.js" || rel == "middleware.mjs":
		return "/_middleware"
	case rel == "proxy.js" || rel == "proxy.mjs":
		return "/_proxy"
	case strings.HasPrefix(rel, appRouterPrefix):
		return appRouteFromPath(strings.TrimPrefix(rel, appRouterPrefix))
	case strings.HasPrefix(rel, pagesRouterPrefix):
		return pagesRouteFromPath(strings.TrimPrefix(rel, pagesRouterPrefix))
	}
	return ""
}

// appRouteFromPath handles App Router conventions. page.js / route.js /
// layout.js are all nested under directories that name the route segment.
// Route groups like (marketing) are stripped. Intercepting routes
// ((..)) are preserved verbatim — the dispatcher resolves them.
func appRouteFromPath(rel string) string {
	rel = strings.TrimSuffix(rel, filepath.Ext(rel))
	base := filepath.Base(rel)
	dir := filepath.Dir(rel)

	// Special files that don't contribute their filename to the URL.
	switch base {
	case "page", "route":
		// fall through — directory is the URL
	case "layout":
		if dir == "." {
			return "/_root_layout"
		}
		return "/" + stripRouteGroups(dir) + "/_layout"
	default:
		// Other compiled files (e.g. not-found.js, loading.js) — tag by name.
		return "/" + stripRouteGroups(dir) + "/_" + base
	}

	if dir == "." {
		return "/"
	}
	return "/" + stripRouteGroups(dir)
}

// pagesRouteFromPath handles the older Pages Router. index.js collapses to /,
// other files become their path. API routes under pages/api/ are flagged by
// kindFromCompiled, not here.
func pagesRouteFromPath(rel string) string {
	rel = strings.TrimSuffix(rel, filepath.Ext(rel))
	if rel == "index" {
		return "/"
	}
	if before, ok := strings.CutSuffix(rel, "/index"); ok {
		rel = before
	}
	return "/" + rel
}

// stripRouteGroups removes Next.js route group segments like (marketing)
// from a path. Route groups are directory-only organization; they never
// appear in URLs.
func stripRouteGroups(p string) string {
	parts := strings.Split(p, "/")
	out := parts[:0]
	for _, part := range parts {
		if strings.HasPrefix(part, "(") && strings.HasSuffix(part, ")") {
			continue
		}
		out = append(out, part)
	}
	return strings.Join(out, "/")
}

// kindFromCompiled classifies a compiled path without reading the source.
// The source-based checks (UsesRSC, HasActions) refine Kind later in
// classifyRefs — this function just gets us to a reasonable default.
func kindFromCompiled(rel string) RouteKind {
	rel = strings.TrimPrefix(rel, "server/")

	switch {
	case rel == "middleware.js" || rel == "middleware.mjs":
		return RouteKindMiddleware
	case rel == "proxy.js" || rel == "proxy.mjs":
		return RouteKindProxy
	case strings.HasSuffix(rel, "/route.js") || strings.HasSuffix(rel, "/route.mjs"):
		return RouteKindAPI
	case strings.HasPrefix(rel, "pages/api/"):
		return RouteKindAPI
	case strings.HasSuffix(rel, "/page.js") || strings.HasSuffix(rel, "/page.mjs"):
		return RouteKindPage
	case strings.HasSuffix(rel, "/layout.js") || strings.HasSuffix(rel, "/layout.mjs"):
		return RouteKindLayout
	case strings.HasPrefix(rel, pagesRouterPrefix):
		return RouteKindPage
	}
	return RouteKindUnknown
}

// classifyRefs refines Kind against the Payload.Routes classification.
// The compiled-path heuristic in kindFromCompiled gets the common cases
// (middleware.js, route.js, page.js); the manifest overrides only for
// edge cases like a custom API route declared outside server/api/.
//
// Intentionally not consulted here:
//   - RouteInfo.MiddlewareRoutes — this is "paths middleware APPLIES TO",
//     not "modules that ARE middleware". Mixing them reclassifies pages.
//   - HasActions — action promotion is confirmed by actions.go against
//     Next's server-reference-manifest, not the path shape alone.
func classifyRefs(refs []ModuleRef, payload Payload) []ModuleRef {
	apiSet := sliceToSet(payload.Routes.APIRoutes)

	for i := range refs {
		r := &refs[i]
		// File-name heuristics already identified these special modules;
		// don't let a RoutePath coincidence reclassify them.
		if r.Kind == RouteKindMiddleware || r.Kind == RouteKindProxy {
			continue
		}
		if _, ok := apiSet[r.RoutePath]; ok {
			r.Kind = RouteKindAPI
		}
	}
	return refs
}

func sliceToSet(ss []string) map[string]struct{} {
	out := make(map[string]struct{}, len(ss))
	for _, s := range ss {
		out[s] = struct{}{}
	}
	return out
}

// appBuildManifest is the subset of .next/app-build-manifest.json we consume:
// app-path entry → ordered client chunk list. Keys look like "/page",
// "/layout", "/blog/[id]/page", "/staff/(authed)/dashboard/page".
type appBuildManifest struct {
	Pages map[string][]string `json:"pages"`
}

// attachBootstrapChunks fills ModuleRef.BootstrapChunks for every page ref from
// app-build-manifest.json. classifyRoot is the .next dir (parent of server/).
//
// An absent or unreadable manifest is non-fatal, matching its sibling passes: a
// production build always emits one, but a partial or exotic build should
// degrade to the current no-bootstrap shell rather than fail the compile.
func attachBootstrapChunks(refs []ModuleRef, classifyRoot string) []ModuleRef {
	manifestPath := filepath.Join(classifyRoot, "app-build-manifest.json")
	data, err := os.ReadFile(manifestPath) // #nosec G304 — reading our own build output
	if err != nil {
		return refs
	}
	var m appBuildManifest
	if err := json.Unmarshal(data, &m); err != nil || m.Pages == nil {
		return refs
	}

	for i := range refs {
		r := &refs[i]
		if r.Kind != RouteKindPage {
			continue
		}
		r.BootstrapChunks = bootstrapChunksForRoute(r.CompiledPath, m.Pages)
	}
	return refs
}

// bootstrapChunksForRoute concatenates, in load order, the chunk lists for the
// route's ancestor layouts (root → leaf) then its own page entry, deduping
// while preserving first-seen order. Absent keys are skipped.
//
// compiledPath is the page's path relative to standaloneDir, e.g.
// "server/app/staff/(authed)/dashboard/page.js". We key off it rather than off
// ModuleRef.RoutePath deliberately: Next's manifest keys preserve route-group
// segments ("(authed)"), parallel-route slots ("@modal") and intercepting
// markers, while the user-facing URL has already dropped them. Deriving the key
// from the URL yields an empty chunk list for every route-grouped page — a page
// that renders but never hydrates. See testdata/fixtures/next15-appdir.
func bootstrapChunksForRoute(compiledPath string, pages map[string][]string) []string {
	pageKey, ok := appManifestKey(compiledPath)
	if !ok {
		return nil // Pages Router or a non-app module: different manifest
	}

	// Ancestor layouts, root first. Layout chunk entries are keyed by the same
	// app-path prefixes as the page, so walk the page key's own directory
	// prefixes rather than consulting LayoutChain — a production build inlines
	// layouts and emits no server/app/**/layout.js for it to find.
	var keys []string
	for _, prefix := range ancestorPrefixes(pageKey) {
		keys = append(keys, prefix+"/layout")
	}
	keys = append(keys, pageKey)

	seen := map[string]struct{}{}
	var out []string
	for _, k := range keys {
		for _, chunk := range pages[k] { // nil for an absent key → no-op
			if _, dup := seen[chunk]; dup {
				continue
			}
			seen[chunk] = struct{}{}
			out = append(out, chunk)
		}
	}
	return out
}

// appManifestKey maps a compiled App Router module path to its
// app-build-manifest key: "server/app/page.js" → "/page",
// "server/app/blog/[id]/page.js" → "/blog/[id]/page". Returns ok=false for
// anything outside server/app (Pages Router, middleware, instrumentation).
func appManifestKey(compiledPath string) (string, bool) {
	p := filepath.ToSlash(compiledPath)
	const appDir = "server/" + appRouterPrefix // "server/app/"
	idx := strings.Index(p, appDir)
	if idx == -1 {
		return "", false
	}
	rel := p[idx+len(appDir):]
	rel = strings.TrimSuffix(rel, filepath.Ext(rel))
	if rel == "" {
		return "", false
	}
	return "/" + rel, true
}

// ancestorPrefixes returns the cumulative app-path prefixes whose layouts wrap
// the given page key, root first. "/page" → [""]; "/blog/[id]/page" → ["",
// "/blog", "/blog/[id]"]. Suffixed with "/layout" these become the manifest
// keys "/layout", "/blog/layout", "/blog/[id]/layout".
func ancestorPrefixes(pageKey string) []string {
	prefixes := []string{""} // root layout, key "/layout"
	cut := strings.LastIndex(pageKey, "/")
	if cut < 0 {
		return prefixes
	}
	dir := strings.Trim(pageKey[:cut], "/")
	if dir == "" {
		return prefixes
	}
	cur := ""
	for _, seg := range strings.Split(dir, "/") {
		cur += "/" + seg
		prefixes = append(prefixes, cur)
	}
	return prefixes
}
