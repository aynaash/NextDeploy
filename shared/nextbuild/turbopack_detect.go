package nextbuild

import (
	"os"
	"path/filepath"
	"strings"
)

// IsTurbopackOutput reports whether the given Next.js standalone tree
// was produced by Turbopack rather than Webpack.
//
// Detection signal: Turbopack emits chunks named `[turbopack]_*.js` and
// `[root-of-the-server]__*._.js` under `.next/server/chunks/ssr/`.
// Webpack-mode standalone output never produces these filenames.
//
// standaloneDir is the path to the extracted standalone tree (the
// directory containing server.js + .next/). For a not-yet-extracted
// build, point at the project root and the function will look at
// `.next/server/chunks/`.
//
// Returns false (no error) when the tree doesn't exist — callers that
// need to distinguish "not built" from "built with Webpack" should stat
// the path themselves.
func IsTurbopackOutput(standaloneDir string) bool {
	candidates := []string{
		filepath.Join(standaloneDir, ".next", "server", "chunks"),
		filepath.Join(standaloneDir, ".next", "server", "chunks", "ssr"),
	}
	for _, dir := range candidates {
		if hasTurbopackMarker(dir) {
			return true
		}
	}
	return false
}

// hasTurbopackMarker shallow-scans dir for any file whose name starts
// with "[turbopack]" or "[root-of-the-server]". Returns true on first
// hit. Missing directory → false (not Turbopack, just absent).
func hasTurbopackMarker(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, "[turbopack]") ||
			strings.HasPrefix(name, "[root-of-the-server]") {
			return true
		}
	}
	return false
}
