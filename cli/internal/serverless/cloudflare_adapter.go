package serverless

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/aynaash/nextdeploy/shared"
	"github.com/aynaash/nextdeploy/shared/config"
	"github.com/aynaash/nextdeploy/shared/nextcompile"
	"github.com/aynaash/nextdeploy/shared/nextcore"
	"github.com/evanw/esbuild/pkg/api"
)

// BuildWorkerBundle adapts a Next.js standalone build into a single
// Cloudflare Worker module bundle.
//
// Pipeline:
//  1. Run nextcompile.Compile against the standalone tree + nextcore payload.
//     This produces <standaloneDir>/.nextdeploy-cf/_nextdeploy/{manifest.json,
//     dispatch.mjs, action_manifest.json, worker_entry.mjs}, extracts the
//     embedded JS runtime, and (when RSC is used) vendors
//     react-server-dom-webpack.
//  2. Log the capability summary so operators see what the bundle supports
//     before it deploys.
//  3. Invoke `npx esbuild` to bundle worker_entry.mjs + all compiled Next
//     modules it transitively imports into a single ESM Worker bundle.
//  4. Return the final bundle path.
//
// Requires Node + npx on PATH for step 3. Every other step is pure Go.
//
// `meta` may be nil for the static-export path (DeployCompute skips early in
// that case), but every Worker-deploy call path supplies it.
func BuildWorkerBundle(
	ctx context.Context,
	standaloneDir string,
	meta *nextcore.NextCorePayload,
	cfg *config.NextDeployConfig,
	log *shared.Logger,
) (string, error) {
	if _, err := os.Stat(standaloneDir); err != nil {
		return "", fmt.Errorf("standalone dir not found: %w", err)
	}

	outDir := filepath.Join(standaloneDir, ".nextdeploy-cf")

	// Blow away any previous build to guarantee a clean output tree. The
	// compile is cheap and reproducible; incremental reuse isn't worth the
	// risk of stale dispatch tables referencing removed routes.
	if err := os.RemoveAll(outDir); err != nil {
		return "", fmt.Errorf("clean prior build dir: %w", err)
	}

	payload := toCompilePayload(meta, cfg)
	prot, err := buildProtectionRuntime(cfg)
	if err != nil {
		return "", fmt.Errorf("protection config: %w", err)
	}
	var cfCfg *config.CloudflareConfig
	if cfg != nil && cfg.Serverless != nil {
		cfCfg = cfg.Serverless.Cloudflare
	}
	engine := nextcompile.Engine(cfCfg.ResolveEngine())

	bundle, err := nextcompile.Compile(ctx, nextcompile.CompileOpts{
		StandaloneDir: standaloneDir,
		Payload:       payload,
		OutDir:        outDir,
		Target:        nextcompile.TargetCloudflareWorker,
		Engine:        engine,
		Protection:    prot,
		Log:           log,
	})
	if err != nil {
		return "", fmt.Errorf("nextcompile: %w", err)
	}
	logBundleSummary(log, bundle)

	esbuildOut := filepath.Join(outDir, "worker.mjs")
	metaPath := filepath.Join(outDir, "_nextdeploy", "esbuild-meta.json")
	if bundle.ResolvedEngine == nextcompile.EngineNextServer {
		if err := runEsbuildNextServer(ctx, bundle.EntryPath, esbuildOut, metaPath, standaloneDir, log); err != nil {
			return "", fmt.Errorf("esbuild (nextserver): %w", err)
		}
	} else {
		if err := runEsbuild(ctx, bundle.EntryPath, esbuildOut, metaPath, standaloneDir, log); err != nil {
			return "", fmt.Errorf("esbuild: %w", err)
		}
	}

	info, err := os.Stat(esbuildOut)
	if err != nil {
		return "", fmt.Errorf("bundle output missing: %w", err)
	}
	log.Info("Worker bundle ready: %s (%.1f KB)", esbuildOut, float64(info.Size())/1024)
	logBundleTopContributors(log, metaPath, esbuildOut)
	return esbuildOut, nil
}

// logBundleSummary prints a one-screen capability report. Operators should
// be able to eyeball this and know whether the deploy will serve their app,
// without having to grep the manifest.
func logBundleSummary(log *shared.Logger, bundle *nextcompile.CompiledBundle) {
	f := bundleFeaturesFromManifest(bundle)
	log.Info("─────────── nextcompile bundle ───────────")
	log.Info("  Engine:         %s", bundle.ResolvedEngine)
	log.Info("  Next.js:        %s", bundle.DetectedVersion.Raw)
	log.Info("  React:          %s", bundle.DetectedReact.Raw)
	log.Info("  Routes:         %d (elided %d)", bundle.Stats.RouteCount, bundle.Stats.DeadRoutesElided)
	log.Info("  Server Actions: %d", bundle.Stats.ActionCount)
	log.Info("  Features:       %s", featureStatusLine(f))
	if bundle.VendoredRSC != nil {
		log.Info("  RSC runtime:    vendored %s (%s build, %s)",
			bundle.VendoredRSC.Version, bundle.VendoredRSC.BuildKind,
			humanBytes(bundle.VendoredRSC.Bytes))
	}
	log.Info("  Bundle hash:    %s", bundle.Stats.ContentHash[:12])
	log.Info("  Bundle bytes:   %s (pre-esbuild)", humanBytes(bundle.Stats.BundleBytes))
	log.Info("──────────────────────────────────────────")

	for _, hint := range bundle.SuggestedBindings {
		log.Debug("  binding hint: %s %q — %s", hint.Kind, hint.Name, hint.Reason)
	}
}

// bundleFeaturesFromManifest re-reads the emitted manifest to get the
// Features struct. Compile doesn't currently return it on the bundle; doing
// it here keeps the adapter independent of internal manifest structure.
func bundleFeaturesFromManifest(bundle *nextcompile.CompiledBundle) nextcompile.ManifestFeatures {
	var m nextcompile.Manifest
	data, err := os.ReadFile(bundle.ManifestPath) // #nosec G304 — reading our own output
	if err != nil {
		return nextcompile.ManifestFeatures{}
	}
	_ = json.Unmarshal(data, &m)
	return m.Features
}

func featureStatusLine(f nextcompile.ManifestFeatures) string {
	tag := func(on bool, label string) string {
		if on {
			return label + ":on"
		}
		return label + ":off"
	}
	return fmt.Sprintf("%s %s %s %s %s %s %s %s %s",
		tag(f.RSC, "rsc"),
		tag(f.ServerActions, "actions"),
		tag(f.Middleware, "middleware"),
		tag(f.Proxy, "proxy"),
		tag(f.ISR, "isr"),
		tag(f.ImageOptimize, "imgopt"),
		tag(f.I18n, "i18n"),
		tag(f.PPR, "ppr"),
		tag(f.After, "after"),
	)
}

func humanBytes(n int64) string {
	switch {
	case n < 1024:
		return fmt.Sprintf("%d B", n)
	case n < 1024*1024:
		return fmt.Sprintf("%.1f KB", float64(n)/1024)
	default:
		return fmt.Sprintf("%.1f MB", float64(n)/(1024*1024))
	}
}

// logBundleTopContributors parses esbuild's --metafile JSON and prints the
// modules contributing the most bytes to the final Worker bundle. This is
// the answer to "why is worker.mjs huge?" — operators get a one-screen
// view without needing to crack open the metafile by hand.
//
// Best-effort: a missing or malformed metafile only suppresses the report;
// it never fails the build. The metafile we read is the same one esbuild
// emits via --metafile in runEsbuild, located relative to the standalone
// dir.
func logBundleTopContributors(log *shared.Logger, metaPath, bundlePath string) {
	const topN = 10

	data, err := os.ReadFile(metaPath) // #nosec G304 — reading our own emitted metafile
	if err != nil {
		log.Debug("esbuild metafile not readable (%v) — skipping size report", err)
		return
	}
	var meta struct {
		Outputs map[string]struct {
			Bytes  int64 `json:"bytes"`
			Inputs map[string]struct {
				BytesInOutput int64 `json:"bytesInOutput"`
			} `json:"inputs"`
		} `json:"outputs"`
	}
	if err := json.Unmarshal(data, &meta); err != nil {
		log.Debug("esbuild metafile parse failed (%v) — skipping size report", err)
		return
	}

	// Find the entry corresponding to the Worker bundle. esbuild keys
	// outputs by path relative to its cwd (standaloneDir); the metafile
	// is loaded after the build, so we match by basename to stay robust
	// against absolute-vs-relative path differences.
	wantBase := filepath.Base(bundlePath)
	var inputs map[string]int64
	var totalBytes int64
	for outPath, out := range meta.Outputs {
		if filepath.Base(outPath) != wantBase {
			continue
		}
		totalBytes = out.Bytes
		inputs = make(map[string]int64, len(out.Inputs))
		for in, info := range out.Inputs {
			inputs[in] = info.BytesInOutput
		}
		break
	}
	if len(inputs) == 0 {
		return
	}

	type pair struct {
		path  string
		bytes int64
	}
	pairs := make([]pair, 0, len(inputs))
	for p, b := range inputs {
		pairs = append(pairs, pair{p, b})
	}
	sort.Slice(pairs, func(i, j int) bool { return pairs[i].bytes > pairs[j].bytes })

	if len(pairs) > topN {
		pairs = pairs[:topN]
	}

	log.Info("─────────── bundle top contributors ───────────")
	for _, p := range pairs {
		pct := 0.0
		if totalBytes > 0 {
			pct = float64(p.bytes) / float64(totalBytes) * 100
		}
		log.Info("  %8s  %5.1f%%  %s", humanBytes(p.bytes), pct, shortenInputPath(p.path))
	}
	log.Info("──────────────────────────────────────────────")
}

// shortenInputPath collapses esbuild's input keys to a more readable form.
// esbuild reports paths relative to its cwd, sometimes with "../" prefixes
// or workspace-internal redirects; the head of the path is rarely what an
// operator cares about — they want to know which compiled module / runtime
// piece is dominating.
func shortenInputPath(p string) string {
	// Strip leading "../" climbs — they're noise to a reader.
	for strings.HasPrefix(p, "../") {
		p = p[3:]
	}
	return p
}

// optionalExternalPackages lists modules Next.js conditionally requires
// at runtime via try/catch'd `require()` calls. Each is loaded only when
// the host app opts in (e.g. by installing the peer dep). esbuild can't
// see the surrounding try/catch and errors out on the unresolved import,
// so we mark them external. The Worker runtime never resolves them
// (because they aren't installed); the require call throws inside Next's
// catch block and the fallback path runs.
//
// Keep this list narrow — every entry is a Worker that won't try to use
// the package's real implementation. If we ever need one of these to
// actually work, the fix is to install it as a real dependency, not
// remove it from this list.
var optionalExternalPackages = []string{
	"critters", // CSS inlining; only required when experimental.optimizeCss is on
	"next/dist/compiled/@ampproject/toolbox-optimizer", // AMP optimizer; only for AMP pages
	// Required inside Next's *pages* runtime (pages.runtime.prod.js) and its
	// externalized app-page runtime, neither of which executes on Workers.
	//
	// This governs only the BARE specifier. Our own SSR layer imports the
	// vendored copy by relative path (vendor/react-dom/server.edge.mjs), which
	// is a distinct module path, so this external does not shadow it — and
	// removing the entry would pull Next's never-run runtime into the bundle.
	// Safe to keep.
	"react-dom/server.edge",
}

// runEsbuild invokes `npx esbuild` against the generated entrypoint. Flags
// are tuned for Cloudflare Workers:
//
//   - --format=esm        Workers require ESM
//   - --bundle            resolve all transitively-imported modules
//   - --platform=node     use Node-style resolution (import "node:*")
//   - --conditions=workerd,worker,node  prefer "workerd" then "worker"
//     exports where present. The "workerd" condition is required for
//     packages like pg-cloudflare and cloudflare:sockets consumers that
//     ship runtime-specific entrypoints; without it they resolve to
//     empty stubs at bundle time and fail at runtime with cryptic
//     "X is not a constructor" errors.
//   - --external:node:*   let the Worker runtime provide Node polyfills
//   - --external:cloudflare:*   don't bundle CF platform modules
//   - --loader:.json=json   dispatch.mjs imports manifest.json with JSON assertion
//   - --minify-syntax + --minify-whitespace + --legal-comments=none:
//     shrink the output without renaming identifiers. Keeping names
//     readable matters because the same bundle is what shows up in
//     Worker stack traces; a 3–5× byte reduction is worth the slight
//     LOC cost vs. full --minify, which would mangle identifiers and
//     make tail-time debugging miserable.
//   - --metafile     dropped beside the bundle so we can report which
//     modules contributed the most bytes (logBundleTopContributors).
//
// cwd is the standalone dir so relative imports in the compiled Next tree
// resolve correctly.
func runEsbuild(ctx context.Context, entry, out, metaPath, cwd string, log *shared.Logger) error {
	// Aliases redirect Next's public module specifiers to our runtime
	// shims. Use absolute path to ensure esbuild resolves regardless of cwd.
	runtimeRoot := filepath.Join(cwd, ".nextdeploy-cf", "_nextdeploy", "runtime")

	asyncStorageShim := filepath.Join(runtimeRoot, "next_shims", "async_storages.mjs")
	fastSetImmediateShim := filepath.Join(runtimeRoot, "next_shims", "fast_set_immediate.mjs")
	alias := map[string]string{
		"next/cache":   filepath.Join(runtimeRoot, "next_shims", "cache.mjs"),
		"next/headers": filepath.Join(runtimeRoot, "next_shims", "headers.mjs"),
		"next/server":  filepath.Join(runtimeRoot, "next_shims", "server.mjs"),
		"next/dist/server/app-render/work-async-storage.external.js":                  asyncStorageShim,
		"next/dist/server/app-render/work-async-storage.external":                     asyncStorageShim,
		"next/dist/server/app-render/action-async-storage.external.js":                asyncStorageShim,
		"next/dist/server/app-render/action-async-storage.external":                   asyncStorageShim,
		"next/dist/server/app-render/after-task-async-storage.external.js":            asyncStorageShim,
		"next/dist/server/app-render/after-task-async-storage.external":               asyncStorageShim,
		"next/dist/server/node-environment-extensions/fast-set-immediate.external.js": fastSetImmediateShim,
		"next/dist/server/node-environment-extensions/fast-set-immediate.external":    fastSetImmediateShim,
	}

	var nodePaths []string
	if nm := filepath.Join(cwd, "node_modules"); nm != "" {
		if info, err := os.Stat(nm); err == nil && info.IsDir() {
			nodePaths = append(nodePaths, nm)
		}
	}
	if wd, err := os.Getwd(); err == nil {
		curr := wd
		for range 5 {
			nm := filepath.Join(curr, "node_modules")
			if info, err := os.Stat(nm); err == nil && info.IsDir() {
				nodePaths = append(nodePaths, nm)
			}
			parent := filepath.Dir(curr)
			if parent == curr {
				break
			}
			curr = parent
		}
	}

	var reactServerPath string
	for _, np := range nodePaths {
		if reactServerPath == "" {
			for _, rel := range []string{
				filepath.Join("react", "react.react-server.js"),
				filepath.Join("react", "cjs", "react.react-server.production.js"),
				filepath.Join("next", "dist", "compiled", "react", "react.react-server.js"),
				filepath.Join("next", "dist", "compiled", "react", "cjs", "react.react-server.production.js"),
			} {
				candidate := filepath.Join(np, rel)
				if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
					reactServerPath = candidate
					break
				}
			}
		}
		if _, ok := alias["react-dom"]; !ok {
			candidate := filepath.Join(np, "react-dom")
			if _, err := os.Stat(candidate); err == nil {
				alias["react-dom"] = candidate
			}
		}
		if _, ok := alias["@opentelemetry/api"]; !ok {
			candidate := filepath.Join(np, "@opentelemetry", "api")
			if _, err := os.Stat(candidate); err == nil {
				alias["@opentelemetry/api"] = candidate
			}
		}
	}
	if _, ok := alias["@opentelemetry/api"]; !ok {
		alias["@opentelemetry/api"] = "next/dist/compiled/@opentelemetry/api"
	}

	externals := append([]string{"node:*", "cloudflare:*"}, optionalExternalPackages...)

	log.Info("Bundling Worker via in-process esbuild (entry: %s)...", filepath.Base(entry))

	result := api.Build(api.BuildOptions{
		EntryPoints: []string{entry},
		Bundle:      true,
		Platform:    api.PlatformNode,
		Format:      api.FormatESModule,
		Target:      api.ESNext,
		MainFields:  []string{"module", "main"},
		Conditions:  []string{"react-server", "workerd", "worker", "node"},
		External:    externals,
		NodePaths:   nodePaths,
		Define: map[string]string{
			"process.env.NODE_ENV": `"production"`,
		},
		Loader: map[string]api.Loader{
			".node": api.LoaderCopy,
			".json": api.LoaderJSON,
		},
		MinifySyntax:     true,
		MinifyWhitespace: true,
		LegalComments:    api.LegalCommentsNone,
		Metafile:         true,
		Alias:            alias,
		Banner: map[string]string{
			"js": "var __dirname = \"/\"; var __filename = \"/worker.mjs\"; globalThis.__dirname = \"/\"; globalThis.__filename = \"/worker.mjs\"; process.env.NEXT_RUNTIME = process.env.NEXT_RUNTIME || \"edge\";",
		},
		Plugins: []api.Plugin{
			reactConditionPlugin(runtimeRoot, reactServerPath),
			nodeBuiltinsPlugin(),
		},
		AbsWorkingDir: cwd,
		Outfile:       out,
		Write:         true,
	})

	if len(result.Errors) > 0 {
		formatted := api.FormatMessages(result.Errors, api.FormatMessagesOptions{
			Kind:          api.ErrorMessage,
			Color:         false,
			TerminalWidth: 80,
		})
		return fmt.Errorf("esbuild failed:\n%s", strings.Join(formatted, "\n"))
	}

	if result.Metafile != "" {
		if err := os.MkdirAll(filepath.Dir(metaPath), 0o750); err == nil {
			_ = os.WriteFile(metaPath, []byte(result.Metafile), 0o640)
		}
	}

	return nil
}

func collectNodePaths(cwd string) []string {
	var nodePaths []string
	if nm := filepath.Join(cwd, "node_modules"); nm != "" {
		if info, err := os.Stat(nm); err == nil && info.IsDir() {
			nodePaths = append(nodePaths, nm)
		}
	}
	if wd, err := os.Getwd(); err == nil {
		curr := wd
		for range 5 {
			nm := filepath.Join(curr, "node_modules")
			if info, err := os.Stat(nm); err == nil && info.IsDir() {
				nodePaths = append(nodePaths, nm)
			}
			parent := filepath.Dir(curr)
			if parent == curr {
				break
			}
			curr = parent
		}
	}
	return nodePaths
}

// runEsbuildNextServer bundles the NextNodeServer-backed Worker entrypoint
// (supports both Turbopack and Webpack Next.js standalone outputs).
func runEsbuildNextServer(_ context.Context, entry, out, metaPath, cwd string, log *shared.Logger) error {
	runtimeRoot := filepath.Join(cwd, ".nextdeploy-cf", "_nextdeploy", "runtime")
	nodePaths := collectNodePaths(cwd)
	externals := append([]string{"node:*", "cloudflare:*"}, optionalExternalPackages...)

	alias := map[string]string{
		"@opentelemetry/api": "next/dist/compiled/@opentelemetry/api",
	}
	for _, np := range nodePaths {
		if candidate := filepath.Join(np, "@opentelemetry", "api"); fileOrDirExists(candidate) {
			alias["@opentelemetry/api"] = candidate
			break
		}
	}

	log.Info("Bundling NextServer Worker via in-process esbuild (entry: %s)...", filepath.Base(entry))

	result := api.Build(api.BuildOptions{
		EntryPoints: []string{entry},
		Bundle:      true,
		Platform:    api.PlatformNode,
		Format:      api.FormatESModule,
		Target:      api.ESNext,
		MainFields:  []string{"module", "main"},
		Conditions:  []string{"workerd", "worker", "node", "default"},
		External:    externals,
		NodePaths:   nodePaths,
		Alias:       alias,
		Define: map[string]string{
			"process.env.NODE_ENV": `"production"`,
		},
		Loader: map[string]api.Loader{
			".node": api.LoaderCopy,
			".json": api.LoaderJSON,
		},
		MinifySyntax:     true,
		MinifyWhitespace: true,
		LegalComments:    api.LegalCommentsNone,
		Metafile:         true,
		Banner: map[string]string{
			"js": "var __dirname = \"/\"; var __filename = \"/worker.mjs\"; globalThis.__dirname = \"/\"; globalThis.__filename = \"/worker.mjs\"; if (typeof process !== \"undefined\" && process.env) { process.env.NODE_ENV = \"production\"; process.env.NEXT_RUNTIME = process.env.NEXT_RUNTIME || \"nodejs\"; }",
		},
		Plugins: []api.Plugin{
			vfsFsPlugin(runtimeRoot),
			nextServerTrimPlugin(runtimeRoot, nodePaths),
			dynamicRequirePatchPlugin(),
			nodeBuiltinsPlugin(),
		},
		AbsWorkingDir: cwd,
		Outfile:       out,
		Write:         true,
	})

	if len(result.Errors) > 0 {
		formatted := api.FormatMessages(result.Errors, api.FormatMessagesOptions{
			Kind:          api.ErrorMessage,
			Color:         false,
			TerminalWidth: 80,
		})
		return fmt.Errorf("esbuild failed:\n%s", strings.Join(formatted, "\n"))
	}

	if result.Metafile != "" {
		if err := os.MkdirAll(filepath.Dir(metaPath), 0o750); err == nil {
			_ = os.WriteFile(metaPath, []byte(result.Metafile), 0o600)
		}
	}
	return nil
}

// vfsFsPlugin routes `fs` / `node:fs` and `fs/promises` / `node:fs/promises`
// to our Virtual Filesystem shim so NextNodeServer's manifest reads succeed
// inside Cloudflare Workers without a disk.
func vfsFsPlugin(runtimeRoot string) api.Plugin {
	fsShim := filepath.Join(runtimeRoot, "nextserver", "vfs_fs_shim.mjs")
	fsPromisesShim := filepath.Join(runtimeRoot, "nextserver", "vfs_fs_promises_shim.mjs")
	return api.Plugin{
		Name: "nextserver-vfs-fs",
		Setup: func(build api.PluginBuild) {
			build.OnResolve(api.OnResolveOptions{Filter: `^(node:)?fs$`}, func(_ api.OnResolveArgs) (api.OnResolveResult, error) {
				return api.OnResolveResult{Path: fsShim}, nil
			})
			build.OnResolve(api.OnResolveOptions{Filter: `^(node:)?fs/promises$`}, func(_ api.OnResolveArgs) (api.OnResolveResult, error) {
				return api.OnResolveResult{Path: fsPromisesShim}, nil
			})
		},
	}
}

func fileOrDirExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// resolveNextCompiledReactServer maps Next.js internal bare requires like
// `react-server-dom-webpack/client` or `react-server-dom-turbopack/server` to
// Next's bundled copies under `next/dist/compiled/...` when present.
func resolveNextCompiledReactServer(spec string, nodePaths []string) string {
	for _, np := range nodePaths {
		base := filepath.Join(np, "next", "dist", "compiled", filepath.FromSlash(spec))
		for _, cand := range []string{base, base + ".js", base + ".edge.js", base + ".node.js"} {
			if info, err := os.Stat(cand); err == nil && !info.IsDir() {
				return cand
			}
		}
	}
	return ""
}

// nextServerTrimPlugin stubs build-time / native-binary packages conditionally
// referenced by NextNodeServer (sharp, swc, webpack, terser, telemetry, etc.)
// and resolves our internal Cloudflare CacheHandler specifier + Next's compiled
// react-server-dom-{webpack,turbopack} modules.
func nextServerTrimPlugin(runtimeRoot string, nodePaths []string) api.Plugin {
	cacheHandlerPath := filepath.Join(runtimeRoot, "nextserver", "cf_cache_handler.mjs")
	trimFilter := `^(sharp|webpack|webpack/.*|terser|typescript|@next/swc-.*|@babel/.*|styled-jsx/macro|caniuse-lite|caniuse-lite/.*|browserslist|@vercel/og|react-dom/static|next/dist/compiled/webpack/.*|next/dist/compiled/edge-runtime.*|next/dist/compiled/next-server/.*-experimental\.runtime\..*|next/dist/compiled/next-server/.*\.runtime\.dev\.js|next/dist/telemetry/.*|next/dist/trace/.*|next/dist/build/.*)$`
	return api.Plugin{
		Name: "nextserver-trim",
		Setup: func(build api.PluginBuild) {
			build.OnResolve(api.OnResolveOptions{Filter: `^__nextdeploy_cf_cache_handler__$`}, func(_ api.OnResolveArgs) (api.OnResolveResult, error) {
				return api.OnResolveResult{Path: cacheHandlerPath}, nil
			})
			build.OnResolve(api.OnResolveOptions{Filter: `^react-server-dom-(webpack|turbopack)/`}, func(args api.OnResolveArgs) (api.OnResolveResult, error) {
				if resolved := resolveNextCompiledReactServer(args.Path, nodePaths); resolved != "" {
					return api.OnResolveResult{Path: resolved}, nil
				}
				return api.OnResolveResult{
					Path:      args.Path,
					Namespace: "next-trim-stub",
				}, nil
			})
			build.OnResolve(api.OnResolveOptions{Filter: trimFilter}, func(args api.OnResolveArgs) (api.OnResolveResult, error) {
				return api.OnResolveResult{
					Path:      args.Path,
					Namespace: "next-trim-stub",
				}, nil
			})
			build.OnLoad(api.OnLoadOptions{Filter: `.*`, Namespace: "next-trim-stub"}, func(_ api.OnLoadArgs) (api.OnLoadResult, error) {
				contents := `
const stub = new Proxy(function() {}, {
	get: (_t, prop) => {
		if (prop === "__esModule") return true;
		if (prop === "default") return stub;
		return stub;
	},
	apply: () => stub,
	construct: () => stub,
});
module.exports = stub;
`
				return api.OnLoadResult{
					Contents: &contents,
					Loader:   api.LoaderJS,
				}, nil
			})
		},
	}
}

var dynamicRequireExprRe = regexp.MustCompile(`(^|[^\w.$])require\(\s*([^'"` + "`" + `\s][^)]*)\)`)

const dynamicRequireHelper = `
const __ndDynRequire = (spec) => {
  if (typeof spec === "string" && spec.includes("__nextdeploy_cf_cache_handler__") && globalThis.__nextdeployCacheHandler) {
    return globalThis.__nextdeployCacheHandler;
  }
  const loaded = globalThis.__nextdeployLoadModule?.(spec);
  if (loaded !== undefined) return loaded;
  if (typeof spec === "string" && spec.endsWith(".json") && globalThis.__nextdeployVFS?.files) {
    const clean = spec.replace(/\\/g, "/").replace(/^\/+/, "");
    const nextIdx = clean.indexOf(".next/");
    const key = nextIdx >= 0 ? clean.slice(nextIdx) : clean;
    if (Object.prototype.hasOwnProperty.call(globalThis.__nextdeployVFS.files, key)) {
      return JSON.parse(globalThis.__nextdeployVFS.files[key]);
    }
  }
  const err = new Error("Cannot find module '" + spec + "'");
  err.code = "MODULE_NOT_FOUND";
  throw err;
};
const __ndDynImport = async (spec) => {
  if (typeof spec === "string" && spec.includes("__nextdeploy_cf_cache_handler__") && globalThis.__nextdeployCacheHandler) {
    return { default: globalThis.__nextdeployCacheHandler };
  }
  const loaded = globalThis.__nextdeployLoadModule?.(spec);
  if (loaded !== undefined) return loaded;
  return import(spec);
};
`

func patchDynamicRequires(src string) string {
	out := dynamicRequireExprRe.ReplaceAllStringFunc(src, func(match string) string {
		// Never rewrite `function require(...)` declarations.
		trimmed := strings.TrimSpace(match)
		if strings.HasPrefix(trimmed, "require(") {
			idx := strings.Index(src, match)
			if idx >= 9 && strings.HasSuffix(strings.TrimSpace(src[:idx]), "function") {
				return match
			}
		}
		return dynamicRequireExprRe.ReplaceAllString(match, "${1}__ndDynRequire(${2})")
	})
	out = strings.ReplaceAll(out, "await dynamicImportEsmDefault(", "await __ndDynImport(")
	return out
}

// dynamicRequirePatchPlugin rewrites dynamic `require(expr)` calls inside
// Next.js's server internals and Turbopack's server chunk runtime so they
// resolve synchronously against the build-time static chunk registry.
func dynamicRequirePatchPlugin() api.Plugin {
	return api.Plugin{
		Name: "nextserver-dynamic-require-patch",
		Setup: func(build api.PluginBuild) {
			build.OnLoad(api.OnLoadOptions{Filter: `\.(c?js)$`}, func(args api.OnLoadArgs) (api.OnLoadResult, error) {
				slash := filepath.ToSlash(args.Path)
				// require-hook.js monkey-patches Node's Module.prototype.require
				// which does not exist in Cloudflare Workers / ESM bundles.
				if strings.HasSuffix(slash, "/next/dist/server/require-hook.js") {
					contents := `
const hookPropertyMap = new Map();
const defaultOverrides = {};
function addHookAliases(aliases = []) {
  for (const [k, v] of aliases) hookPropertyMap.set(k, v);
}
module.exports = { addHookAliases, defaultOverrides, hookPropertyMap };
`
					return api.OnLoadResult{
						Contents:   &contents,
						Loader:     api.LoaderJS,
						ResolveDir: filepath.Dir(args.Path),
					}, nil
				}
				// node-environment.js loads Node-only process/console/crypto monkey-patches
				// (e.g. unhandled-rejection.js calling process.listeners, which does not
				// exist in Cloudflare workerd). Keep only the safe AsyncLocalStorage baseline.
				if strings.HasSuffix(slash, "/next/dist/server/node-environment.js") {
					contents := `
if (typeof globalThis.AsyncLocalStorage !== "function") {
  const { AsyncLocalStorage } = require("node:async_hooks");
  globalThis.AsyncLocalStorage = AsyncLocalStorage;
}
module.exports = {};
`
					return api.OnLoadResult{
						Contents:   &contents,
						Loader:     api.LoaderJS,
						ResolveDir: filepath.Dir(args.Path),
					}, nil
				}
				// load-manifest.external.js uses vm.runInNewContext for evalManifest,
				// which is forbidden in Cloudflare Workers. Execute the pre-bundled
				// manifest module directly via our static chunk registry instead.
				if strings.HasSuffix(slash, "/next/dist/server/load-manifest.external.js") {
					contents := `
const fs = require("fs");
const pathMod = require("path");
const sharedCache = new Map();
function loadManifest(manifestPath, shouldCache = true, cache = sharedCache, skipParse = false) {
  const cached = shouldCache && cache && cache.get(manifestPath);
  if (cached) return cached;
  let manifest = fs.readFileSync(manifestPath, "utf8");
  if (!skipParse) {
    manifest = JSON.parse(manifest);
  }
  if (shouldCache && cache) cache.set(manifestPath, manifest);
  return manifest;
}
function evalManifest(manifestPath, shouldCache = true, cache = sharedCache) {
  const cached = shouldCache && cache && cache.get(manifestPath);
  if (cached) return cached;
  globalThis.__RSC_MANIFEST = globalThis.__RSC_MANIFEST || {};
  const loaded = globalThis.__nextdeployLoadModule?.(manifestPath);
  if (loaded === undefined) {
    const err = new Error("ENOENT: no such manifest '" + manifestPath + "'");
    err.code = "ENOENT";
    throw err;
  }
  const result = { __RSC_MANIFEST: globalThis.__RSC_MANIFEST };
  if (shouldCache && cache) cache.set(manifestPath, result);
  return result;
}
function loadManifestFromRelativePath({ projectDir, distDir, manifest, shouldCache, cache, skipParse, handleMissing, useEval }) {
  try {
    const manifestPath = pathMod.join(projectDir || "/", distDir || ".next", manifest);
    if (useEval) {
      return evalManifest(manifestPath, shouldCache, cache);
    }
    return loadManifest(manifestPath, shouldCache, cache, skipParse);
  } catch (err) {
    if (handleMissing) return {};
    throw err;
  }
}
function clearManifestCache(manifestPath, cache = sharedCache) {
  return cache ? cache.delete(manifestPath) : false;
}
module.exports = { loadManifest, evalManifest, loadManifestFromRelativePath, clearManifestCache };
`
					return api.OnLoadResult{
						Contents:   &contents,
						Loader:     api.LoaderJS,
						ResolveDir: filepath.Dir(args.Path),
					}, nil
				}
				needsPatch := strings.Contains(slash, "/[turbopack]") ||
					(strings.Contains(slash, "/next/dist/server/") && !strings.Contains(slash, "/compiled/"))
				if !needsPatch {
					return api.OnLoadResult{}, nil
				}
				raw, err := os.ReadFile(args.Path) // #nosec G304 — esbuild resolved module path
				if err != nil {
					return api.OnLoadResult{}, err
				}
				src := string(raw)
				patched := patchDynamicRequires(src)
				if patched == src {
					return api.OnLoadResult{}, nil
				}
				contents := dynamicRequireHelper + "\n" + patched
				return api.OnLoadResult{
					Contents:   &contents,
					Loader:     api.LoaderJS,
					ResolveDir: filepath.Dir(args.Path),
				}, nil
			})
		},
	}
}

// reactConditionPlugin resolves bare `react` imports per importer so a single
// Worker bundle can hold both react.react-server (for RSC encoding) and the
// standard React build (for react-dom/server.edge and Suspense use() streaming).
func reactConditionPlugin(runtimeRoot, reactServerPath string) api.Plugin {
	ssrReact := filepath.Join(runtimeRoot, "vendor", "react", "index.mjs")
	return api.Plugin{
		Name: "react-dual-condition",
		Setup: func(build api.PluginBuild) {
			build.OnResolve(api.OnResolveOptions{Filter: `^react$`}, func(args api.OnResolveArgs) (api.OnResolveResult, error) {
				imp := filepath.ToSlash(args.Importer)
				isSSRImporter := strings.Contains(imp, "/vendor/react-dom/") ||
					strings.Contains(imp, "/vendor/react-server-dom-webpack/client.edge") ||
					strings.HasSuffix(imp, "/runtime/ssr.mjs")
				if isSSRImporter {
					if info, err := os.Stat(ssrReact); err == nil && !info.IsDir() {
						return api.OnResolveResult{Path: ssrReact}, nil
					}
				}
				if reactServerPath != "" {
					return api.OnResolveResult{Path: reactServerPath}, nil
				}
				return api.OnResolveResult{}, nil
			})
		},
	}
}

var nativeNodeModules = map[string]bool{
	"_stream_duplex":      true,
	"_stream_passthrough": true,
	"_stream_readable":    true,
	"_stream_transform":   true,
	"_stream_writable":    true,
	"_tls_common":         true,
	"_tls_wrap":           true,
	"assert":              true,
	"assert/strict":       true,
	"async_hooks":         true,
	"buffer":              true,
	"constants":           true,
	"crypto":              true,
	"diagnostics_channel": true,
	"dns":                 true,
	"dns/promises":        true,
	"events":              true,
	"net":                 true,
	"path":                true,
	"path/posix":          true,
	"path/win32":          true,
	"querystring":         true,
	"module":              true,
	"stream":              true,
	"stream/consumers":    true,
	"stream/promises":     true,
	"stream/web":          true,
	"string_decoder":      true,
	"sys":                 true,
	"timers":              true,
	"timers/promises":     true,
	"tls":                 true,
	"url":                 true,
	"util":                true,
	"util/types":          true,
	"zlib":                true,
}

var stubNodeModules = map[string]bool{
	"child_process":      true,
	"cluster":            true,
	"console":            true,
	"dgram":              true,
	"domain":             true,
	"fs":                 true,
	"fs/promises":        true,
	"http":               true,
	"http2":              true,
	"https":              true,
	"inspector":          true,
	"inspector/promises": true,
	"os":                 true,
	"perf_hooks":         true,
	"punycode":           true,
	"readline":           true,
	"readline/promises":  true,
	"repl":               true,
	"trace_events":       true,
	"tty":                true,
	"v8":                 true,
	"vm":                 true,
	"wasi":               true,
	"worker_threads":     true,
}

// nodeBuiltinsPlugin handles Node.js built-ins in the Worker bundle:
//  1. Modules natively supported by Cloudflare Workers (workerd) via nodejs_compat
//     (like util, path, crypto, buffer, http) are mapped to ESM bridge modules importing "node:<mod>".
//  2. Modules not available in workerd (like fs, child_process) are stubbed with empty modules
//     so require() calls inside dead code / try-catch do not break Cloudflare's upload validation.
func nodeBuiltinsPlugin() api.Plugin {
	return api.Plugin{
		Name: "node-builtins",
		Setup: func(build api.PluginBuild) {
			build.OnResolve(api.OnResolveOptions{Filter: `^(node:|[a-z0-9_/-]+$)`}, func(args api.OnResolveArgs) (api.OnResolveResult, error) {
				specifier := strings.TrimPrefix(args.Path, "node:")
				if nativeNodeModules[specifier] {
					prefixed := "node:" + specifier
					if args.Kind == api.ResolveJSRequireCall {
						return api.OnResolveResult{
							Path:      prefixed,
							Namespace: "node-native",
						}, nil
					}
					return api.OnResolveResult{
						Path:     prefixed,
						External: true,
					}, nil
				}
				if stubNodeModules[specifier] {
					return api.OnResolveResult{
						Path:      specifier,
						Namespace: "node-stub",
					}, nil
				}
				return api.OnResolveResult{}, nil
			})

			build.OnLoad(api.OnLoadOptions{Filter: `.*`, Namespace: "node-native"}, func(args api.OnLoadArgs) (api.OnLoadResult, error) {
				contents := fmt.Sprintf("import * as mod from %q;\nexport default (mod.default ?? mod);\nexport * from %q;\n", args.Path, args.Path)
				return api.OnLoadResult{
					Contents: &contents,
					Loader:   api.LoaderJS,
				}, nil
			})

			build.OnLoad(api.OnLoadOptions{Filter: `.*`, Namespace: "node-stub"}, func(_ api.OnLoadArgs) (api.OnLoadResult, error) {
				contents := `
const noop = () => {};
const overrides = {
	__esModule: true,
	STATUS_CODES: {},
	METHODS: [],
	isatty: () => false,
	EOL: "\n",
	platform: () => "linux",
	arch: () => "x64",
	tmpdir: () => "/tmp",
	homedir: () => "/",
	hostname: () => "cloudflare-worker",
	cpus: () => [{ model: "workerd", speed: 1000 }],
	totalmem: () => 134217728,
	freemem: () => 67108864,
	networkInterfaces: () => ({}),
	constants: {},
	performance: globalThis.performance,
	PerformanceObserver: globalThis.PerformanceObserver,
	console: globalThis.console,
};
const stub = new Proxy(function() {}, {
	get: (_target, prop) => {
		if (prop === "default") return stub;
		if (Object.prototype.hasOwnProperty.call(overrides, prop)) return overrides[prop];
		return stub;
	},
	apply: () => stub,
	construct: () => stub,
});
module.exports = stub;
`
				return api.OnLoadResult{
					Contents: &contents,
					Loader:   api.LoaderJS,
				}, nil
			})
		},
	}
}
