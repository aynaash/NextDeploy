package nextcompile

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ErrRSCPackageNotFound signals that react-server-dom-webpack is not
// present in the standalone tree's node_modules. Callers match on this
// via errors.Is to branch:
//   - manifest.Features.RSC == true → surface to user with vendoring steps
//   - manifest.Features.RSC == false → silently skip, rsc.mjs will 501
//     anyway if ever reached
var ErrRSCPackageNotFound = errors.New("react-server-dom-webpack not found in node_modules")

// SSR companion sentinels. Distinct from ErrRSCPackageNotFound so
// maybeVendorRSC can name the exact missing build, and so a companion gap is
// only fatal when the app actually uses RSC.
var (
	// ErrRSCServerEdgeNotFound is distinct from ErrRSCPackageNotFound: the
	// package IS installed, but publishes no server.edge build we recognize.
	// That's a corrupt or unexpected publish, not a missing dependency.
	ErrRSCServerEdgeNotFound   = errors.New("react-server-dom-webpack server.edge build not found")
	ErrRSCClientEdgeNotFound   = errors.New("react-server-dom-webpack client.edge build not found")
	ErrReactDOMPackageNotFound = errors.New("react-dom not found in node_modules")
	ErrReactDOMServerNotFound  = errors.New("react-dom server.edge build not found")
)

// VendoredPackage records what VendorRSC copied into the bundle. The
// adapter logs this and includes it in CompileStats.
type VendoredPackage struct {
	Name       string
	Version    string
	SourcePath string
	TargetPath string
	Bytes      int64
	BuildKind  string // "production" | "development" | "legacy"

	// Format is the module system of the vendored file: "esm" or "cjs".
	// React 19 publishes react-dom and react-server-dom-webpack as CJS only,
	// so this is routinely "cjs" — see vendorEdgeBuild.
	Format string

	// ShimPath is the generated ESM re-export written next to a CJS payload
	// so importers can always use a stable `.mjs` specifier. Empty when the
	// payload is already ESM.
	ShimPath string

	// Extra holds the SSR companion files vendored alongside the primary
	// server.edge bundle: react-server-dom-webpack/client.edge (deserializes
	// a Flight stream on the worker) and react-dom/server.edge (renders the
	// resulting element tree to an HTML stream).
	Extra []VendoredPackage
}

// VendorRSC resolves react-server-dom-webpack from the standalone tree's
// node_modules and copies its server.edge ESM bundle into
// <bundleDir>/_nextdeploy/runtime/vendor/react-server-dom-webpack/server.edge.mjs.
//
// Lookup order (first hit wins):
//  1. <standaloneDir>/node_modules/react-server-dom-webpack
//  2. <standaloneDir>/../node_modules/react-server-dom-webpack    (app root)
//
// Returns ErrRSCPackageNotFound when neither location resolves. The CF
// Workers runtime has no npm at request time, so vendoring is the only
// way Server Components render without OpenNext.
func VendorRSC(standaloneDir, bundleDir string) (*VendoredPackage, error) {
	pkgDir, err := locateRSCPackage(standaloneDir)
	if err != nil {
		return nil, err
	}

	meta, err := readRSCPackageMeta(pkgDir)
	if err != nil {
		return nil, fmt.Errorf("read react-server-dom-webpack metadata: %w", err)
	}

	rscTargetDir := filepath.Join(bundleDir, "_nextdeploy", "runtime", "vendor", "react-server-dom-webpack")

	// Primary: server.edge — encodes the React tree into a Flight stream.
	primary, err := vendorEdgeBuild(pkgDir, rscTargetDir, "react-server-dom-webpack", "server.edge",
		"react-server-dom-webpack/server.edge", ErrRSCServerEdgeNotFound)
	if err != nil {
		return nil, err
	}
	primary.Name = meta.Name
	primary.Version = meta.Version

	// SSR companion 1 — client.edge deserializes that Flight stream back into
	// a React element on the worker. Same package, same target dir.
	clientPkg, err := vendorEdgeBuild(pkgDir, rscTargetDir, "react-server-dom-webpack", "client.edge",
		"react-server-dom-webpack/client.edge", ErrRSCClientEdgeNotFound)
	if err != nil {
		return nil, err
	}
	clientPkg.Version = meta.Version
	primary.Extra = append(primary.Extra, *clientPkg)

	// SSR companion 2 — react-dom/server.edge streams that element tree to
	// HTML. Different package, resolved from the same node_modules root.
	domDir, err := locateReactDOMPackage(standaloneDir)
	if err != nil {
		return nil, err
	}
	domMeta, err := readRSCPackageMeta(domDir)
	if err != nil {
		return nil, fmt.Errorf("read react-dom metadata: %w", err)
	}
	domTargetDir := filepath.Join(bundleDir, "_nextdeploy", "runtime", "vendor", "react-dom")
	domPkg, err := vendorEdgeBuild(domDir, domTargetDir, "react-dom", "server.edge",
		"react-dom/server.edge", ErrReactDOMServerNotFound)
	if err != nil {
		return nil, err
	}
	domPkg.Version = domMeta.Version
	primary.Extra = append(primary.Extra, *domPkg)

	return primary, nil
}

// locateRSCPackage walks upward from standaloneDir looking for a
// node_modules/react-server-dom-webpack. Next's standalone build lands at
// <project>/.next/standalone, so the package may be two levels up in a
// workspace or monorepo. Cap the walk at 5 levels to avoid unbounded
// upward search in pathological filesystems.
//
// Symlink-transparent by design — pnpm's flat .pnpm store resolves
// because os.Stat follows symlinks.
func locateRSCPackage(standaloneDir string) (string, error) {
	current := standaloneDir
	for range 5 {
		candidate := filepath.Join(current, "node_modules", "react-server-dom-webpack")
		if _, err := os.Stat(filepath.Join(candidate, "package.json")); err == nil {
			return candidate, nil
		}
		parent := filepath.Dir(current)
		if parent == current {
			break // hit root
		}
		current = parent
	}
	return "", ErrRSCPackageNotFound
}

type rscPackageMeta struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

func readRSCPackageMeta(pkgDir string) (rscPackageMeta, error) {
	data, err := os.ReadFile(filepath.Join(pkgDir, "package.json")) // #nosec G304
	if err != nil {
		return rscPackageMeta{}, err
	}
	var m rscPackageMeta
	if err := json.Unmarshal(data, &m); err != nil {
		return rscPackageMeta{}, err
	}
	return m, nil
}

// findEdgeBuild locates a React edge build inside an installed package and
// returns its path plus build flavor.
//
// pkgName is the npm package name used in React's file-naming convention
// ("react-server-dom-webpack", "react-dom"); entry is the export stem
// ("server.edge", "client.edge").
//
// Ordering rationale: prefer a CONCRETE production implementation (smallest,
// no dev warnings), ESM before CJS since ESM needs no interop shim. The flat
// `<entry>.js` is deliberately LAST: in React 19 that file is a conditional
// shim whose body is `require("./cjs/<pkg>-<entry>.production.js")`. Vendoring
// the shim alone copies a module whose relative require points at a file that
// was never copied — it resolves to nothing at bundle time. Ranking the cjs/
// implementation above it is what makes React 19 work at all.
func findEdgeBuild(pkgDir, pkgName, entry string) (string, string, error) {
	candidates := []struct {
		rel       string
		buildKind string
	}{
		{fmt.Sprintf("esm/%s-%s.production.js", pkgName, entry), "production"},
		{fmt.Sprintf("%s.production.js", entry), "production"},
		{fmt.Sprintf("cjs/%s-%s.production.js", pkgName, entry), "production"},
		{fmt.Sprintf("esm/%s-%s.development.js", pkgName, entry), "development"},
		{fmt.Sprintf("%s.development.js", entry), "development"},
		{fmt.Sprintf("cjs/%s-%s.development.js", pkgName, entry), "development"},
		{fmt.Sprintf("%s.js", entry), "legacy"},
	}
	for _, c := range candidates {
		p := filepath.Join(pkgDir, c.rel)
		if _, err := os.Stat(p); err == nil {
			return p, c.buildKind, nil
		}
	}
	return "", "", fmt.Errorf("no %s build found in %s (tried esm/, cjs/, and flat layouts)", entry, pkgDir)
}

// vendorEdgeBuild copies one React edge build into targetDir, naming it by the
// module system it actually uses: <entry>.mjs for ESM, <entry>.cjs for CJS.
// When the payload is CJS it also writes an <entry>.mjs re-export shim, so
// every importer can use the same stable `.mjs` specifier regardless of how
// the installed React happens to be published.
func vendorEdgeBuild(pkgDir, targetDir, pkgName, entry, displayName string, notFound error) (*VendoredPackage, error) {
	src, buildKind, err := findEdgeBuild(pkgDir, pkgName, entry)
	if err != nil {
		return nil, notFound
	}
	format, err := detectModuleFormat(src)
	if err != nil {
		return nil, fmt.Errorf("classify %s: %w", src, err)
	}
	if err := os.MkdirAll(targetDir, 0o750); err != nil {
		return nil, fmt.Errorf("mkdir vendor %s: %w", displayName, err)
	}

	ext := ".mjs"
	if format == "cjs" {
		ext = ".cjs"
	}
	dst := filepath.Join(targetDir, entry+ext)
	n, err := copyFile(src, dst)
	if err != nil {
		return nil, fmt.Errorf("copy %s → %s: %w", src, dst, err)
	}

	out := &VendoredPackage{
		Name:       displayName,
		SourcePath: src,
		TargetPath: dst,
		Bytes:      n,
		BuildKind:  buildKind,
		Format:     format,
	}

	if format == "cjs" {
		shim := filepath.Join(targetDir, entry+".mjs")
		if err := writeInteropShim(shim, entry+ext); err != nil {
			return nil, err
		}
		out.ShimPath = shim
	}
	return out, nil
}

// writeInteropShim emits the ESM facade over a vendored CJS payload. Keeping
// the `.mjs` specifier stable means runtime modules never have to branch on
// how React was published. esbuild statically analyzes React's
// `exports.foo = …` assignments, and Node's cjs-module-lexer does the same for
// the `node --test` path, so the star re-export resolves named bindings in
// both contexts.
func writeInteropShim(shimPath, payloadFile string) error {
	body := fmt.Sprintf(`// Generated by nextcompile — do not edit.
// CJS→ESM interop facade: the installed React ships this build as CommonJS,
// so the implementation lives in %s and this file re-exports it.
export * from %q;
export { default } from %q;
`, payloadFile, "./"+payloadFile, "./"+payloadFile)
	if err := os.WriteFile(shimPath, []byte(body), 0o640); err != nil {
		return fmt.Errorf("write interop shim %s: %w", shimPath, err)
	}
	return nil
}

// detectModuleFormat classifies a vendored file as "esm" or "cjs". Directory
// convention decides when React published both flavors; otherwise the file's
// own contents do, which correctly catches the flat `<entry>.js` shims.
func detectModuleFormat(path string) (string, error) {
	slash := filepath.ToSlash(path)
	switch {
	case strings.Contains(slash, "/esm/"):
		return "esm", nil
	case strings.Contains(slash, "/cjs/"):
		return "cjs", nil
	}
	data, err := os.ReadFile(path) // #nosec G304 — reading a package we resolved ourselves
	if err != nil {
		return "", err
	}
	if bytes.Contains(data, []byte("module.exports")) || bytes.Contains(data, []byte("exports.")) {
		return "cjs", nil
	}
	return "esm", nil
}

// locateReactDOMPackage mirrors locateRSCPackage for the react-dom package.
func locateReactDOMPackage(standaloneDir string) (string, error) {
	current := standaloneDir
	for range 5 {
		candidate := filepath.Join(current, "node_modules", "react-dom")
		if _, err := os.Stat(filepath.Join(candidate, "package.json")); err == nil {
			return candidate, nil
		}
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}
	return "", ErrReactDOMPackageNotFound
}

// copyFile byte-copies src to dst and returns the number of bytes written.
// Uses 0640 on the target — worker bundles contain code that can reference
// secrets via bindings, so conservative permissions are correct.
func copyFile(src, dst string) (int64, error) {
	in, err := os.Open(src) // #nosec G304
	if err != nil {
		return 0, err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o640) // #nosec G304
	if err != nil {
		return 0, err
	}
	defer out.Close()
	return io.Copy(out, in)
}
