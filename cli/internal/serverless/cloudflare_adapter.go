package serverless

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/aynaash/nextdeploy/shared"
	"github.com/aynaash/nextdeploy/shared/config"
	"github.com/aynaash/nextdeploy/shared/nextcompile"
	"github.com/aynaash/nextdeploy/shared/nextcore"
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
	if _, err := exec.LookPath("npx"); err != nil {
		return "", fmt.Errorf("npx not found on PATH (install Node.js): %w", err)
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
	bundle, err := nextcompile.Compile(ctx, nextcompile.CompileOpts{
		StandaloneDir: standaloneDir,
		Payload:       payload,
		OutDir:        outDir,
		Target:        nextcompile.TargetCloudflareWorker,
		Protection:    prot,
		Log:           log,
	})
	if err != nil {
		return "", fmt.Errorf("nextcompile: %w", err)
	}
	logBundleSummary(log, bundle)

	esbuildOut := filepath.Join(outDir, "worker.mjs")
	metaPath := filepath.Join(outDir, "_nextdeploy", "esbuild-meta.json")
	if err := runEsbuild(ctx, bundle.EntryPath, esbuildOut, metaPath, standaloneDir, log); err != nil {
		return "", fmt.Errorf("esbuild: %w", err)
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
	"@opentelemetry/api", // Next's tracer fallback when tracing isn't enabled
	"critters",           // CSS inlining; only required when experimental.optimizeCss is on
	"next/dist/compiled/@ampproject/toolbox-optimizer", // AMP optimizer; only for AMP pages
	// Required inside Next's *pages* runtime (pages.runtime.prod.js). App-Router
	// apps never invoke that runtime, and the matching export isn't resolvable
	// under the workerd/worker conditions anyway. The app-page runtime resolves
	// react-dom/server.edge on its own, so externalizing here is safe.
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
	// shims. Path is relative to the standalone dir (esbuild's cwd).
	runtimeRoot := ".nextdeploy-cf/_nextdeploy/runtime"

	// metaPath is given to us as absolute; esbuild resolves --metafile
	// relative to its cwd. Make it relative so the path lands where the
	// caller expects (and stays readable in the esbuild log line).
	relMeta, err := filepath.Rel(cwd, metaPath)
	if err != nil {
		relMeta = metaPath
	}

	args := []string{
		"--yes",
		"esbuild@latest",
		entry,
		"--bundle",
		"--platform=node",
		"--format=esm",
		"--target=esnext",
		"--main-fields=module,main",
		"--conditions=workerd,worker,node",
		"--external:node:*",
		"--external:cloudflare:*",
		// __dirname/__filename are CJS-only globals that don't exist in ESM
		// output. Bundled code (e.g. next/dist/compiled/@opentelemetry/api via
		// Next's tracer) references __dirname at module-eval time, which throws
		// `ReferenceError: __dirname is not defined` on the Workers runtime and
		// 500s every dynamically-rendered page. Filesystem paths are meaningless
		// in the Worker sandbox, so define them as inert string literals.
		`--define:__dirname="/"`,
		`--define:__filename="/worker.mjs"`,
		"--loader:.node=copy",
		"--loader:.json=json",
		"--minify-syntax",
		"--minify-whitespace",
		"--legal-comments=none",
		"--metafile=" + relMeta,
		"--alias:next/cache=" + filepath.Join(runtimeRoot, "next_shims", "cache.mjs"),
		"--alias:next/headers=" + filepath.Join(runtimeRoot, "next_shims", "headers.mjs"),
		"--alias:next/server=" + filepath.Join(runtimeRoot, "next_shims", "server.mjs"),
		"--outfile=" + out,
	}
	// Optional Next.js peer dependencies that aren't part of the
	// standard bundle and are imported via try/catch'd require() calls
	// in Next's source. Marking them external lets the bundle complete;
	// at runtime the `require()` throws and Next's catch fallback runs.
	//
	// Add new entries here as we encounter them — each is one user-deploy
	// failure away from being added.
	for _, pkg := range optionalExternalPackages {
		args = append(args, "--external:"+pkg)
	}

	log.Info("Bundling Worker via esbuild (entry: %s)...", filepath.Base(entry))
	// NOSONAR: spawning `npx` is intentional and every argument here is
	// either a static flag or a path the compiler just emitted into
	// OutDir. No user-supplied string reaches argv; there is no shell
	// interpolation — exec.CommandContext takes argv directly, not a
	// shell string.
	cmd := exec.CommandContext(ctx, "npx", args...)
	cmd.Dir = cwd
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
