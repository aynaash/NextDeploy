package adapter

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aynaash/nextdeploy/shared"
	"github.com/aynaash/nextdeploy/shared/nextcompile"
	"github.com/aynaash/nextdeploy/shared/nextcore"
)

// TestAdapterPreEsbuildStateIsValid runs the nextcompile phase of
// BuildWorkerBundle against a synthetic standalone tree and asserts the
// output is exactly what the downstream esbuild invocation expects.
//
// The esbuild step itself is skipped (needs npx in PATH + network to
// install on first run) — but this test proves that everything up to
// esbuild is producing a correct, deployable bundle structure.
func TestAdapterPreEsbuildStateIsValid(t *testing.T) {
	dir := t.TempDir()

	// Synthetic standalone tree.
	writeTestFile(t, filepath.Join(dir, "package.json"), `{
		"name": "fullstack-demo",
		"dependencies": {"next":"15.0.0","react":"19.0.0"}
	}`)
	writeTestFile(t, filepath.Join(dir, ".next", "server", "app", "page.js"),
		`export default function Home(){ return "<h1>home</h1>" }`)
	writeTestFile(t, filepath.Join(dir, ".next", "server", "app", "api", "hello", "route.js"),
		`export async function GET(){ return Response.json({ok:true}) }`)
	writeTestFile(t, filepath.Join(dir, ".next", "server", "app", "actions", "page.js"),
		`export function doThing(){ return "done" }`)
	writeTestFile(t, filepath.Join(dir, ".next", "server", "proxy.js"),
		`export default function proxy(){}`)
	writeTestFile(t, filepath.Join(dir, ".next", "server", "middleware.js"),
		`export default function mw(){}`)
	// Server Action manifest
	writeTestFile(t, filepath.Join(dir, ".next", "server", "server-reference-manifest.json"),
		`{"node":{"actHash":{"workers":{"app/actions/page":"doThing"}}},"edge":{}}`)
	// Minimal React vendor fixture — the real Next 15 / React 19 build would
	// include these automatically. The SSR layer needs three edge builds:
	// server.edge (Flight encode), client.edge (Flight decode) and
	// react-dom/server.edge (element tree → HTML).
	writeTestFile(t, filepath.Join(dir, "node_modules", "react-server-dom-webpack", "package.json"),
		`{"name":"react-server-dom-webpack","version":"19.0.0"}`)
	writeTestFile(t, filepath.Join(dir, "node_modules", "react-server-dom-webpack",
		"esm", "react-server-dom-webpack-server.edge.production.js"),
		`export function renderToReadableStream(){}`)
	writeTestFile(t, filepath.Join(dir, "node_modules", "react-server-dom-webpack",
		"esm", "react-server-dom-webpack-client.edge.production.js"),
		`export function createFromReadableStream(){}`)
	writeTestFile(t, filepath.Join(dir, "node_modules", "react-dom", "package.json"),
		`{"name":"react-dom","version":"19.0.0"}`)
	writeTestFile(t, filepath.Join(dir, "node_modules", "react-dom",
		"esm", "react-dom-server.edge.production.js"),
		`export function renderToReadableStream(){}`)

	meta := &nextcore.NextCorePayload{
		AppName:    "fullstack-demo",
		DistDir:    ".next",
		OutputMode: nextcore.OutputModeStandalone,
		GitCommit:  "deadbeef",
		NextBuildMetadata: nextcore.NextBuildMetadata{
			BuildID:      "build-42",
			HasAppRouter: true,
		},
		RouteInfo: nextcore.RouteInfo{
			APIRoutes:        []string{"/api/hello"},
			MiddlewareRoutes: []string{"/"},
		},
		Middleware: &nextcore.MiddlewareConfig{
			Path:    "middleware.ts",
			Runtime: "edge",
		},
	}

	// Run nextcompile directly (skipping esbuild). This is the same code
	// path BuildWorkerBundle takes, minus the final exec.
	payload := toCompilePayload(meta, nil)
	bundle, err := nextcompile.Compile(context.Background(), nextcompile.CompileOpts{
		StandaloneDir: dir,
		Payload:       payload,
		OutDir:        filepath.Join(dir, ".nextdeploy-cf"),
		Target:        nextcompile.TargetCloudflareWorker,
	})
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}

	// Every artifact the adapter will feed to esbuild must exist.
	for name, p := range map[string]string{
		"entry":          bundle.EntryPath,
		"manifest":       bundle.ManifestPath,
		"dispatch":       bundle.DispatchPath,
		"actionManifest": bundle.ActionManifest,
	} {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("%s missing: %v", name, err)
		}
	}

	// Action manifest contains our planted action.
	actData, _ := os.ReadFile(bundle.ActionManifest) // #nosec G304
	var actMan nextcompile.ActionManifest
	if err := json.Unmarshal(actData, &actMan); err != nil {
		t.Fatalf("action manifest invalid: %v", err)
	}
	if actMan.Actions["actHash"].Export != "doThing" {
		t.Errorf("action not wired: %+v", actMan.Actions)
	}

	// Entry point must reference every dependency esbuild will follow.
	entry, _ := os.ReadFile(bundle.EntryPath) // #nosec G304
	entrySrc := string(entry)
	for _, need := range []string{
		`from "./runtime/dispatcher.mjs"`,
		`from "./dispatch.mjs"`,
		`from "./manifest.json"`,
		`from "./action_manifest.json"`,
		`actionLoaders`,
	} {
		if !strings.Contains(entrySrc, need) {
			t.Errorf("entry missing %q", need)
		}
	}

	// Dispatch table must include the API route, proxy, middleware, and
	// action loader — the full dispatch surface.
	dispatch, _ := os.ReadFile(bundle.DispatchPath) // #nosec G304
	dSrc := string(dispatch)
	for _, need := range []string{
		`"/api/hello":`,
		`export const proxyRef = { compiled:`,
		`export const middlewareRef = { compiled:`,
		`"app/actions/page": () => import(`,
	} {
		if !strings.Contains(dSrc, need) {
			t.Errorf("dispatch missing %q:\n%s", need, dSrc)
		}
	}

	// Runtime files must be extracted alongside.
	runtimeDir := filepath.Join(bundle.BundleDir, "_nextdeploy", "runtime")
	for _, name := range []string{
		"dispatcher.mjs", "serve.mjs", "route_match.mjs", "errors.mjs",
		"context.mjs", "rsc.mjs", "actions.mjs",
	} {
		if _, err := os.Stat(filepath.Join(runtimeDir, name)); err != nil {
			t.Errorf("runtime/%s missing: %v", name, err)
		}
	}

	// Vendored RSC bundle for React 19 must be present (the fixture has it).
	vendor := filepath.Join(runtimeDir, "vendor", "react-server-dom-webpack", "server.edge.mjs")
	if _, err := os.Stat(vendor); err != nil {
		t.Errorf("vendored RSC bundle missing: %v", err)
	}
	if bundle.VendoredRSC == nil || bundle.VendoredRSC.Version != "19.0.0" {
		t.Errorf("VendoredRSC metadata wrong: %+v", bundle.VendoredRSC)
	}
	// The SSR companions must land too — without them the worker can decode a
	// Flight stream but has nothing to render it to HTML with.
	for _, rel := range []string{
		"vendor/react-server-dom-webpack/client.edge.mjs",
		"vendor/react-dom/server.edge.mjs",
	} {
		if _, err := os.Stat(filepath.Join(runtimeDir, filepath.FromSlash(rel))); err != nil {
			t.Errorf("SSR companion missing: %s (%v)", rel, err)
		}
	}
	if n := len(bundle.VendoredRSC.Extra); n != 2 {
		t.Errorf("want 2 vendored companions, got %d", n)
	}

	// Stats must be populated — content hash non-empty, bytes > 0.
	if bundle.Stats.ContentHash == "" {
		t.Error("empty ContentHash")
	}
	if bundle.Stats.BundleBytes == 0 {
		t.Error("zero BundleBytes")
	}
	if bundle.Stats.ActionCount != 1 {
		t.Errorf("ActionCount: got %d, want 1", bundle.Stats.ActionCount)
	}

	// Now execute BuildWorkerBundle end-to-end with the in-process Go esbuild compiler.
	log := shared.PackageLogger("test", "TEST")
	workerOut, err := BuildWorkerBundle(context.Background(), dir, meta, nil, log)
	if err != nil {
		t.Fatalf("BuildWorkerBundle: %v", err)
	}
	info, err := os.Stat(workerOut)
	if err != nil {
		t.Fatalf("worker.mjs not found at %s: %v", workerOut, err)
	}
	if info.Size() == 0 {
		t.Fatalf("worker.mjs is empty")
	}
}

// TestBuildWorkerBundle_ProductionDefinesAndReactDirFallback guards against two
// real-world React 19 standalone bundling regressions:
//  1. node_modules/react without react.react-server.js must NOT resolve to the
//     react directory itself inside reactConditionPlugin ("is a directory").
//  2. node_modules/react-dom/index.js with a NODE_ENV branch referencing a
//     stripped ./cjs/react-dom.development.js must fold to production.
func TestBuildWorkerBundle_ProductionDefinesAndReactDirFallback(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "package.json"), `{
		"name": "react19-demo",
		"dependencies": {"next":"16.0.0","react":"19.2.0"}
	}`)
	writeTestFile(t, filepath.Join(dir, ".next", "server", "app", "page.js"),
		`if (process.env.__NEXT_DEV_SERVER) { require("./missing-dev-only-module"); }
const React = require("react"); const ReactDOM = require("react-dom"); export default function Page(){ return [React, ReactDOM]; }`)

	// Minimal node_modules/react without react.react-server.js (CJS prod only).
	writeTestFile(t, filepath.Join(dir, "node_modules", "react", "package.json"),
		`{"name":"react","version":"19.2.0","main":"index.js"}`)
	writeTestFile(t, filepath.Join(dir, "node_modules", "react", "index.js"),
		`if (process.env.NODE_ENV === 'production') { module.exports = require('./cjs/react.production.js'); } else { module.exports = require('./cjs/react.development.js'); }`)
	writeTestFile(t, filepath.Join(dir, "node_modules", "react", "cjs", "react.production.js"),
		`'use strict'; exports.use = function(){}; exports.createElement = function(){};`)

	// Minimal node_modules/react-dom with stripped development build.
	writeTestFile(t, filepath.Join(dir, "node_modules", "react-dom", "package.json"),
		`{"name":"react-dom","version":"19.2.0","main":"index.js"}`)
	writeTestFile(t, filepath.Join(dir, "node_modules", "react-dom", "index.js"),
		`if (process.env.NODE_ENV === 'production') { module.exports = require('./cjs/react-dom.production.js'); } else { module.exports = require('./cjs/react-dom.development.js'); }`)
	writeTestFile(t, filepath.Join(dir, "node_modules", "react-dom", "cjs", "react-dom.production.js"),
		`'use strict'; exports.version = '19.2.0';`)
	writeTestFile(t, filepath.Join(dir, "node_modules", "react-dom", "cjs", "react-dom-server.edge.production.js"),
		`'use strict'; const React = require('react'); exports.renderToReadableStream = function(){ return React; };`)

	// Minimal react-server-dom-webpack requiring bare "react".
	writeTestFile(t, filepath.Join(dir, "node_modules", "react-server-dom-webpack", "package.json"),
		`{"name":"react-server-dom-webpack","version":"19.2.0"}`)
	writeTestFile(t, filepath.Join(dir, "node_modules", "react-server-dom-webpack", "cjs", "react-server-dom-webpack-server.edge.production.js"),
		`'use strict'; const React = require('react'); exports.renderToReadableStream = function(){ return React; };`)
	writeTestFile(t, filepath.Join(dir, "node_modules", "react-server-dom-webpack", "cjs", "react-server-dom-webpack-client.edge.production.js"),
		`'use strict'; const React = require('react'); exports.createFromReadableStream = function(){ return React; };`)

	meta := &nextcore.NextCorePayload{
		AppName:    "react19-demo",
		DistDir:    ".next",
		OutputMode: nextcore.OutputModeStandalone,
		NextBuildMetadata: nextcore.NextBuildMetadata{
			BuildID:      "build-99",
			HasAppRouter: true,
		},
	}

	log := shared.PackageLogger("test", "TEST")
	workerOut, err := BuildWorkerBundle(context.Background(), dir, meta, nil, log)
	if err != nil {
		t.Fatalf("BuildWorkerBundle failed: %v", err)
	}
	info, err := os.Stat(workerOut)
	if err != nil || info.Size() == 0 {
		t.Fatalf("expected non-empty worker bundle at %s (err=%v)", workerOut, err)
	}
}

// TestBuildWorkerBundle_NextServerTurbopackEndToEnd verifies the full
// NextNodeServer + Virtual FS + Turbopack chunk-registry pipeline end to end:
// compiles a synthetic Turbopack Next 16 standalone tree via BuildWorkerBundle,
// then executes the bundled worker.mjs in Node to assert VFS reads, dynamic
// Turbopack chunk loading, and streaming HTTP responses all work together.
func TestBuildWorkerBundle_NextServerTurbopackEndToEnd(t *testing.T) {
	dir := t.TempDir()

	writeTestFile(t, filepath.Join(dir, "package.json"), `{
		"name": "turbo-e2e",
		"dependencies": {"next":"16.0.10","react":"19.2.0"}
	}`)
	writeTestFile(t, filepath.Join(dir, ".next", "BUILD_ID"), `turbo-build-42`)
	writeTestFile(t, filepath.Join(dir, ".next", "routes-manifest.json"), `{"version":3,"basePath":""}`)
	writeTestFile(t, filepath.Join(dir, ".next", "required-server-files.json"), `{"config":{"poweredByHeader":false}}`)

	// Turbopack chunk runtime + SSR chunk loaded dynamically via require(path.join(...)).
	writeTestFile(t, filepath.Join(dir, ".next", "server", "chunks", "ssr", "[turbopack]_runtime.js"), `
const path = require("path");
exports.loadChunk = function(chunkRel) {
  return require(path.join(".next/server", chunkRel));
};
`)
	writeTestFile(t, filepath.Join(dir, ".next", "server", "chunks", "ssr", "page_chunk_1.js"), `
exports.greeting = "Hello from Turbopack NextServer!";
`)
	writeTestFile(t, filepath.Join(dir, ".next", "server", "app", "page.js"), `
const rt = require("../chunks/ssr/[turbopack]_runtime.js");
exports.render = function() {
  const chunk = rt.loadChunk("chunks/ssr/page_chunk_1.js");
  return chunk.greeting;
};
`)

	// Minimal NextNodeServer + require.js matching Next.js's real runtime contracts:
	// reads BUILD_ID via fs.readFileSync (VFS) and dynamically requires page.js.
	writeTestFile(t, filepath.Join(dir, "node_modules", "next", "dist", "server", "require.js"), `
exports.requirePage = function(pagePath) {
  return require(pagePath);
};
`)
	writeTestFile(t, filepath.Join(dir, "node_modules", "next", "dist", "client", "components", "app-router.js"), `
if (process.env.__NEXT_DEV_SERVER) {
  require("../dev/hot-reloader/app/hot-reloader-app");
}
exports.AppRouter = function() { return "ok"; };
`)
	writeTestFile(t, filepath.Join(dir, "node_modules", "next", "dist", "server", "next-server.js"), `
const fs = require("fs");
const { requirePage } = require("./require.js");
require("../client/components/app-router.js");

class NextNodeServer {
  constructor(opts) {
    this.opts = opts;
  }
  async prepare() {
    this.buildId = fs.readFileSync("/.next/BUILD_ID", "utf8").trim();
  }
  getRequestHandler() {
    return async (req, res) => {
      const page = requirePage(".next/server/app/page.js");
      res.statusCode = 200;
      res.setHeader("content-type", "text/html; charset=utf-8");
      res.setHeader("x-build-id", this.buildId);
      res.write("<h1>" + page.render() + "</h1>");
      res.end("<p>url=" + req.url + "</p>");
    };
  }
}
module.exports = NextNodeServer;
`)

	meta := &nextcore.NextCorePayload{
		AppName:    "turbo-e2e",
		DistDir:    ".next",
		OutputMode: nextcore.OutputModeStandalone,
		NextBuildMetadata: nextcore.NextBuildMetadata{
			BuildID:      "turbo-build-42",
			HasAppRouter: true,
		},
	}

	log := shared.PackageLogger("test", "TEST")
	workerOut, err := BuildWorkerBundle(context.Background(), dir, meta, nil, log)
	if err != nil {
		t.Fatalf("BuildWorkerBundle (nextserver/turbopack): %v", err)
	}

	nodeBin, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not on PATH; skipping runtime execution check")
	}

	script := `
import worker from ` + `"` + filepath.ToSlash(workerOut) + `"` + `;
const res = await worker.fetch(new Request("https://app.example.com/docs?tab=turbo"), {}, { waitUntil() {} });
const body = await res.text();
if (res.status !== 200) throw new Error("status=" + res.status + " body=" + body);
if (res.headers.get("x-build-id") !== "turbo-build-42") throw new Error("bad build id: " + res.headers.get("x-build-id"));
if (!body.includes("Hello from Turbopack NextServer!")) throw new Error("unexpected body: " + body);
if (!body.includes("url=/docs?tab=turbo")) throw new Error("missing url in body: " + body);
console.log("OK:" + body);
`
	cmd := exec.CommandContext(t.Context(), nodeBin, "--input-type=module", "-e", script) // #nosec G204
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("executing compiled Turbopack worker.mjs failed: %v\noutput:\n%s", err, string(out))
	}
	if !strings.Contains(string(out), "OK:<h1>Hello from Turbopack NextServer!</h1><p>url=/docs?tab=turbo</p>") {
		t.Fatalf("unexpected worker output: %s", string(out))
	}
}

// TestBuildWorkerBundle_RealStandaloneSmoke lets developers verify the
// nextserver engine against any real Next.js standalone tree on disk:
//
//	NEXTDEPLOY_CF_ENGINE=nextserver NEXTDEPLOY_TEST_STANDALONE=/path/to/app/.next/standalone \
//	  go test -v ./cli/internal/serverless -run TestBuildWorkerBundle_RealStandaloneSmoke
func TestBuildWorkerBundle_RealStandaloneSmoke(t *testing.T) {
	standaloneDir := os.Getenv("NEXTDEPLOY_TEST_STANDALONE")
	if standaloneDir == "" {
		t.Skip("NEXTDEPLOY_TEST_STANDALONE not set")
	}
	meta := &nextcore.NextCorePayload{
		AppName:    "real-smoke",
		DistDir:    ".next",
		OutputMode: nextcore.OutputModeStandalone,
		NextBuildMetadata: nextcore.NextBuildMetadata{
			HasAppRouter: true,
		},
	}
	log := shared.PackageLogger("test", "TEST")
	workerOut, err := BuildWorkerBundle(context.Background(), standaloneDir, meta, nil, log)
	if err != nil {
		t.Fatalf("BuildWorkerBundle(%s): %v", standaloneDir, err)
	}
	info, err := os.Stat(workerOut)
	if err != nil {
		t.Fatalf("stat %s: %v", workerOut, err)
	}
	t.Logf("Compiled worker.mjs: %s (%.1f KB)", workerOut, float64(info.Size())/1024)

	nodeBin, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not on PATH")
	}
	script := `
import worker from "` + filepath.ToSlash(workerOut) + `";
for (const p of ["/api/health", "/", "/docs"]) {
  const res = await worker.fetch(new Request("https://app.example.com" + p), {}, { waitUntil() {} });
  const text = await res.text();
  console.log("ROUTE", p, "STATUS", res.status, "CT", res.headers.get("content-type"), "LEN", text.length, "SNIPPET", JSON.stringify(text.slice(0, 400)));
}
`
	cmd := exec.CommandContext(t.Context(), nodeBin, "--input-type=module", "-e", script) // #nosec G204
	out, err := cmd.CombinedOutput()
	t.Logf("Node execution output:\n%s", string(out))
	if err != nil {
		t.Fatalf("worker execution failed: %v", err)
	}
}

// writeTestFile mirrors the test helper in nextcompile — duplicated here
// because Go test helpers don't cross package boundaries.
func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
