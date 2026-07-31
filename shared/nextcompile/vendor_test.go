package nextcompile

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVendorRSC_ESMProduction(t *testing.T) {
	dir := t.TempDir()
	bundleDir := filepath.Join(dir, "out")

	pkgDir := writeRSCFixture(t, dir, "18.3.1")
	writeFile(t, filepath.Join(pkgDir, "esm", "react-server-dom-webpack-server.edge.production.js"),
		`// react 18.3.1 server.edge production build
export function renderToReadableStream(){}`)

	got, err := VendorRSC(dir, bundleDir)
	if err != nil {
		t.Fatalf("VendorRSC: %v", err)
	}
	if got.Version != "18.3.1" {
		t.Errorf("version: got %s", got.Version)
	}
	if got.BuildKind != "production" {
		t.Errorf("buildKind: got %s", got.BuildKind)
	}
	if got.Bytes == 0 {
		t.Error("zero bytes written")
	}

	// Target file must exist at the expected path.
	expPath := filepath.Join(bundleDir, "_nextdeploy", "runtime", "vendor",
		"react-server-dom-webpack", "server.edge.mjs")
	if got.TargetPath != expPath {
		t.Errorf("target path: got %s, want %s", got.TargetPath, expPath)
	}
	data, err := os.ReadFile(expPath) // #nosec G304
	if err != nil {
		t.Fatalf("vendored file unreadable: %v", err)
	}
	if len(data) == 0 {
		t.Error("vendored file is empty")
	}
}

func TestVendorRSC_PrefersProductionOverDev(t *testing.T) {
	dir := t.TempDir()
	bundleDir := filepath.Join(dir, "out")

	pkgDir := writeRSCFixture(t, dir, "19.0.0")
	// Both flavors present.
	writeFile(t, filepath.Join(pkgDir, "esm", "react-server-dom-webpack-server.edge.production.js"),
		`// prod`)
	writeFile(t, filepath.Join(pkgDir, "esm", "react-server-dom-webpack-server.edge.development.js"),
		`// dev`)

	got, err := VendorRSC(dir, bundleDir)
	if err != nil {
		t.Fatal(err)
	}
	if got.BuildKind != "production" {
		t.Errorf("expected production, got %s", got.BuildKind)
	}

	data, _ := os.ReadFile(got.TargetPath) // #nosec G304
	if string(data) != "// prod" {
		t.Errorf("wrong build vendored: %q", data)
	}
}

func TestVendorRSC_FallsBackToDevThenLegacy(t *testing.T) {
	dir := t.TempDir()
	bundleDir := filepath.Join(dir, "out")

	pkgDir := writeRSCFixture(t, dir, "18.0.0")
	// Only the legacy flat file exists for server.edge.
	writeFile(t, filepath.Join(pkgDir, "server.edge.js"), `// legacy`)

	got, err := VendorRSC(dir, bundleDir)
	if err != nil {
		t.Fatal(err)
	}
	if got.BuildKind != "legacy" {
		t.Errorf("expected legacy, got %s", got.BuildKind)
	}
}

func TestVendorRSC_AppRootFallback(t *testing.T) {
	// Standalone tree lacks node_modules but the app root above it has it.
	// pnpm-workspace-style layouts hit this path.
	root := t.TempDir()
	standalone := filepath.Join(root, ".next", "standalone")
	if err := os.MkdirAll(standalone, 0o755); err != nil {
		t.Fatal(err)
	}

	pkgDir := writeRSCFixture(t, root, "18.3.1")
	writeFile(t, filepath.Join(pkgDir, "esm", "react-server-dom-webpack-server.edge.production.js"),
		`// root-level`)

	got, err := VendorRSC(standalone, filepath.Join(root, "out"))
	if err != nil {
		t.Fatalf("expected fallback to succeed: %v", err)
	}
	data, _ := os.ReadFile(got.TargetPath) // #nosec G304
	if string(data) != "// root-level" {
		t.Errorf("unexpected content: %q", data)
	}
}

func TestVendorRSC_NotFound(t *testing.T) {
	dir := t.TempDir()
	_, err := VendorRSC(dir, filepath.Join(dir, "out"))
	if !errors.Is(err, ErrRSCPackageNotFound) {
		t.Errorf("expected ErrRSCPackageNotFound, got %v", err)
	}
}

func TestVendorRSC_PackageWithoutServerEdge(t *testing.T) {
	// Package is installed but no server.edge file — corrupt or unexpected
	// publish. We treat this as non-recoverable and surface the exact dir.
	dir := t.TempDir()
	writeRSCFixture(t, dir, "99.0.0")
	// Intentionally no server.edge build present.

	_, err := VendorRSC(dir, filepath.Join(dir, "out"))
	if err == nil {
		t.Fatal("expected error for missing server.edge")
	}
	if errors.Is(err, ErrRSCPackageNotFound) {
		t.Errorf("should NOT be ErrRSCPackageNotFound — package exists, build is missing")
	}
}

// writeRSCFixture lays down a react-server-dom-webpack package plus the SSR
// companions VendorRSC now requires (client.edge + react-dom/server.edge), so
// each test only has to express the variation it actually cares about.
func writeRSCFixture(t *testing.T, root, version string) string {
	t.Helper()
	pkgDir := filepath.Join(root, "node_modules", "react-server-dom-webpack")
	writeFile(t, filepath.Join(pkgDir, "package.json"),
		`{"name":"react-server-dom-webpack","version":"`+version+`"}`)
	writeFile(t, filepath.Join(pkgDir, "esm", "react-server-dom-webpack-client.edge.production.js"),
		`// client.edge prod
export function createFromReadableStream(){}`)
	domDir := filepath.Join(root, "node_modules", "react-dom")
	writeFile(t, filepath.Join(domDir, "package.json"), `{"name":"react-dom","version":"`+version+`"}`)
	writeFile(t, filepath.Join(domDir, "esm", "react-dom-server.edge.production.js"),
		`// react-dom server.edge prod
export function renderToReadableStream(){}`)
	return pkgDir
}

func TestVendorRSC_VendorsSSRCompanions(t *testing.T) {
	dir := t.TempDir()
	pkgDir := writeRSCFixture(t, dir, "19.0.0")
	writeFile(t, filepath.Join(pkgDir, "esm", "react-server-dom-webpack-server.edge.production.js"),
		`// server`)

	out := filepath.Join(dir, "out")
	pkg, err := VendorRSC(dir, out)
	if err != nil {
		t.Fatalf("VendorRSC: %v", err)
	}
	if len(pkg.Extra) != 2 {
		t.Fatalf("want 2 companions, got %d: %+v", len(pkg.Extra), pkg.Extra)
	}
	vendorRoot := filepath.Join(out, "_nextdeploy", "runtime", "vendor")
	for _, rel := range []string{
		"react-server-dom-webpack/server.edge.mjs",
		"react-server-dom-webpack/client.edge.mjs",
		"react-dom/server.edge.mjs",
	} {
		if _, err := os.Stat(filepath.Join(vendorRoot, filepath.FromSlash(rel))); err != nil {
			t.Errorf("companion not vendored: %s (%v)", rel, err)
		}
	}
	// ESM sources need no interop shim.
	for _, p := range append([]VendoredPackage{*pkg}, pkg.Extra...) {
		if p.Format != "esm" {
			t.Errorf("%s: format = %q, want esm", p.Name, p.Format)
		}
		if p.ShimPath != "" {
			t.Errorf("%s: unexpected shim %s for an ESM payload", p.Name, p.ShimPath)
		}
	}
}

// React 19 publishes react-dom and react-server-dom-webpack as CJS ONLY: no
// esm/ dir, and the flat `<entry>.js` is a shim that `require`s a sibling in
// cjs/. Vendoring that shim copies a module whose relative require dangles, so
// we must pick the concrete cjs/ implementation and put an ESM facade in front
// of it. This test pins the real React 19 on-disk layout.
func TestVendorRSC_React19CJSLayout(t *testing.T) {
	dir := t.TempDir()

	rsc := filepath.Join(dir, "node_modules", "react-server-dom-webpack")
	writeFile(t, filepath.Join(rsc, "package.json"), `{"name":"react-server-dom-webpack","version":"19.0.0"}`)
	writeFile(t, filepath.Join(rsc, "server.edge.js"),
		`'use strict';
module.exports = require('./cjs/react-server-dom-webpack-server.edge.production.js');`)
	writeFile(t, filepath.Join(rsc, "client.edge.js"),
		`'use strict';
module.exports = require('./cjs/react-server-dom-webpack-client.edge.production.js');`)
	writeFile(t, filepath.Join(rsc, "cjs", "react-server-dom-webpack-server.edge.production.js"),
		`'use strict'; exports.renderToReadableStream = function(){};`)
	writeFile(t, filepath.Join(rsc, "cjs", "react-server-dom-webpack-client.edge.production.js"),
		`'use strict'; exports.createFromReadableStream = function(){};`)

	dom := filepath.Join(dir, "node_modules", "react-dom")
	writeFile(t, filepath.Join(dom, "package.json"), `{"name":"react-dom","version":"19.0.0"}`)
	writeFile(t, filepath.Join(dom, "server.edge.js"),
		`'use strict'; module.exports = require('./cjs/react-dom-server.edge.production.js');`)
	writeFile(t, filepath.Join(dom, "cjs", "react-dom-server.edge.production.js"),
		`'use strict'; exports.renderToReadableStream = function(){};`)

	out := filepath.Join(dir, "out")
	pkg, err := VendorRSC(dir, out)
	if err != nil {
		t.Fatalf("VendorRSC: %v", err)
	}

	all := append([]VendoredPackage{*pkg}, pkg.Extra...)
	for _, p := range all {
		if p.Format != "cjs" {
			t.Errorf("%s: format = %q, want cjs", p.Name, p.Format)
		}
		if p.BuildKind != "production" {
			t.Errorf("%s: buildKind = %q, want production (the concrete cjs/ impl, not the shim)",
				p.Name, p.BuildKind)
		}
		// The concrete implementation must be vendored — never the shim,
		// whose relative require would dangle.
		body, err := os.ReadFile(p.TargetPath) // #nosec G304
		if err != nil {
			t.Fatalf("%s: %v", p.Name, err)
		}
		if strings.Contains(string(body), "require('./cjs/") {
			t.Errorf("%s: vendored the shim, not the implementation:\n%s", p.Name, body)
		}
		// A stable .mjs specifier must exist for every payload.
		if p.ShimPath == "" {
			t.Fatalf("%s: CJS payload has no interop shim", p.Name)
		}
		if filepath.Ext(p.TargetPath) != ".cjs" {
			t.Errorf("%s: CJS payload should be named .cjs, got %s", p.Name, p.TargetPath)
		}
		shim, err := os.ReadFile(p.ShimPath) // #nosec G304
		if err != nil {
			t.Fatalf("%s shim: %v", p.Name, err)
		}
		want := "./" + filepath.Base(p.TargetPath)
		if !strings.Contains(string(shim), want) {
			t.Errorf("%s: shim does not re-export %s:\n%s", p.Name, want, shim)
		}
	}

	// rsc.mjs imports ./vendor/react-server-dom-webpack/server.edge.mjs
	// unconditionally — that specifier must resolve regardless of how React
	// happened to be published.
	if _, err := os.Stat(filepath.Join(out, "_nextdeploy", "runtime", "vendor",
		"react-server-dom-webpack", "server.edge.mjs")); err != nil {
		t.Errorf("stable .mjs specifier missing: %v", err)
	}
}

func TestVendorRSC_MissingClientEdgeIsSentinel(t *testing.T) {
	dir := t.TempDir()
	rsc := filepath.Join(dir, "node_modules", "react-server-dom-webpack")
	writeFile(t, filepath.Join(rsc, "package.json"), `{"name":"react-server-dom-webpack","version":"19.0.0"}`)
	writeFile(t, filepath.Join(rsc, "esm", "react-server-dom-webpack-server.edge.production.js"), `// server`)
	// No client.edge, no react-dom.
	_, err := VendorRSC(dir, filepath.Join(dir, "out"))
	if !errors.Is(err, ErrRSCClientEdgeNotFound) {
		t.Fatalf("want ErrRSCClientEdgeNotFound, got %v", err)
	}
}

func TestVendorRSC_MissingReactDOMIsSentinel(t *testing.T) {
	dir := t.TempDir()
	rsc := filepath.Join(dir, "node_modules", "react-server-dom-webpack")
	writeFile(t, filepath.Join(rsc, "package.json"), `{"name":"react-server-dom-webpack","version":"19.0.0"}`)
	writeFile(t, filepath.Join(rsc, "esm", "react-server-dom-webpack-server.edge.production.js"), `// server`)
	writeFile(t, filepath.Join(rsc, "esm", "react-server-dom-webpack-client.edge.production.js"), `// client`)
	// react-dom absent entirely.
	_, err := VendorRSC(dir, filepath.Join(dir, "out"))
	if !errors.Is(err, ErrReactDOMPackageNotFound) {
		t.Fatalf("want ErrReactDOMPackageNotFound, got %v", err)
	}
}

func TestVendorRSC_ReactDOMWithoutServerEdgeIsSentinel(t *testing.T) {
	dir := t.TempDir()
	rsc := filepath.Join(dir, "node_modules", "react-server-dom-webpack")
	writeFile(t, filepath.Join(rsc, "package.json"), `{"name":"react-server-dom-webpack","version":"19.0.0"}`)
	writeFile(t, filepath.Join(rsc, "esm", "react-server-dom-webpack-server.edge.production.js"), `// server`)
	writeFile(t, filepath.Join(rsc, "esm", "react-server-dom-webpack-client.edge.production.js"), `// client`)
	dom := filepath.Join(dir, "node_modules", "react-dom")
	writeFile(t, filepath.Join(dom, "package.json"), `{"name":"react-dom","version":"19.0.0"}`)
	// Installed, but publishes no server.edge build we recognize.
	_, err := VendorRSC(dir, filepath.Join(dir, "out"))
	if !errors.Is(err, ErrReactDOMServerNotFound) {
		t.Fatalf("want ErrReactDOMServerNotFound, got %v", err)
	}
}

func TestVendorRSC_MissingServerEdgeIsItsOwnSentinel(t *testing.T) {
	dir := t.TempDir()
	writeRSCFixture(t, dir, "19.0.0") // client.edge + react-dom present, server.edge absent
	_, err := VendorRSC(dir, filepath.Join(dir, "out"))
	if !errors.Is(err, ErrRSCServerEdgeNotFound) {
		t.Fatalf("want ErrRSCServerEdgeNotFound, got %v", err)
	}
	if errors.Is(err, ErrRSCPackageNotFound) {
		t.Error("must not be ErrRSCPackageNotFound — the package is installed")
	}
}

func TestDetectModuleFormat(t *testing.T) {
	dir := t.TempDir()
	// Directory convention decides when React ships both flavors.
	esm := filepath.Join(dir, "esm", "x.js")
	writeFile(t, esm, `module.exports = 1`) // contents deliberately contradict
	if got, _ := detectModuleFormat(esm); got != "esm" {
		t.Errorf("esm/ dir should win, got %q", got)
	}
	cjs := filepath.Join(dir, "cjs", "y.js")
	writeFile(t, cjs, `export const y = 1`)
	if got, _ := detectModuleFormat(cjs); got != "cjs" {
		t.Errorf("cjs/ dir should win, got %q", got)
	}
	// Flat files fall back to a content sniff.
	flat := filepath.Join(dir, "flat.js")
	writeFile(t, flat, `'use strict'; module.exports = require('./cjs/impl.js')`)
	if got, _ := detectModuleFormat(flat); got != "cjs" {
		t.Errorf("flat shim should sniff as cjs, got %q", got)
	}
	flatESM := filepath.Join(dir, "flat-esm.js")
	writeFile(t, flatESM, `export function renderToReadableStream(){}`)
	if got, _ := detectModuleFormat(flatESM); got != "esm" {
		t.Errorf("flat esm should sniff as esm, got %q", got)
	}
}
