package nextcompile

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// Compile runs the full build-side pipeline against a Next.js standalone
// tree and produces a CompiledBundle ready for the adapter's esbuild step.
//
// This is the single public entry point; callers should not invoke the
// phase functions directly. Phase ordering and error policy are documented
// inline — deviations will break reproducibility guarantees.
//
// Contract:
//   - On error, no guarantee that OutDir has been cleaned. The adapter's
//     build-bundle step should treat OutDir as disposable (blow it away
//     and re-run on retry) rather than expect partial success.
//   - On success, every path in the returned CompiledBundle exists.
//   - CompileStats.ContentHash is deterministic for identical input.
func Compile(ctx context.Context, opts CompileOpts) (*CompiledBundle, error) {
	opts = normalizeOpts(opts)
	log := opts.Log

	start := time.Now()
	log.Info("nextcompile: starting (target=%s, standalone=%s)", opts.Target, opts.StandaloneDir)

	// Phase 1 — detect Next + React versions.
	nextVer, reactVer, err := DetectVersions(opts.StandaloneDir)
	if err != nil {
		return nil, fmt.Errorf("detect versions: %w", err)
	}
	log.Info("nextcompile: detected next=%s react=%s variant=%s",
		nextVer.Raw, reactVer.Raw, nextVer.RuntimeVariant())

	// Phase 2 — scan compiled server tree in parallel.
	refs, err := ScanCompiledServer(ctx, opts.StandaloneDir, opts.Payload)
	if err != nil {
		return nil, fmt.Errorf("scan compiled server: %w", err)
	}
	log.Info("nextcompile: scanned %d compiled modules", len(refs))

	// Phase 3 — server actions. Parse Next's server-reference-manifest.json
	// from the standalone tree. Missing manifest → no actions (fine).
	actionManifest, err := DetectServerActions(opts.StandaloneDir, opts.Payload.DistDir)
	if err != nil && !errors.Is(err, ErrNoActionManifest) {
		// A PARSE failure must NOT abort the whole deploy — degrade to "actions
		// unavailable" so a manifest-shape drift in a new Next version doesn't
		// brick deploys that don't use actions.
		log.Warn("nextcompile: server-reference-manifest parse failed (%v); "+
			"Server Actions will be unavailable in this deploy", err)
		actionManifest = nil
	}
	if actionManifest != nil && len(actionManifest.Actions) > 0 {
		log.Info("nextcompile: detected %d Server Actions", len(actionManifest.Actions))
	}

	// Phase 4 — binding hints (stub until bindings.go lands).
	hints := deriveBindingsStub(refs, opts.Payload)

	// Phase 5 — elide dead routes. Deliberately a no-op; see the function doc.
	kept, elided := elideDeadRoutes(refs, opts.Payload.Routes)

	// Phase 6 — ensure OutDir exists. Every emit below writes under
	// <OutDir>/_nextdeploy/ so this covers them all.
	if err := ensureOutDir(opts.OutDir); err != nil {
		return nil, fmt.Errorf("prepare out dir: %w", err)
	}

	// Phase 7 — build + emit manifest.json.
	generatedAt := time.Now()
	manifest := BuildManifest(opts.Payload, nextVer, reactVer, kept, generatedAt)
	manifestPath, err := EmitManifest(manifest, opts.OutDir)
	if err != nil {
		return nil, fmt.Errorf("emit manifest: %w", err)
	}

	// Phase 8 — emit dispatch.mjs (build-time route table + action loaders).
	// The compiled paths in `kept` are relative to opts.StandaloneDir, and
	// dispatch.mjs lives at <opts.OutDir>/_nextdeploy/dispatch.mjs; when
	// OutDir is nested inside StandaloneDir (as the cloudflare adapter
	// does for isolation), the import-prefix has to walk back through the
	// nesting to reach the standalone root.
	dispatchPath, err := EmitDispatchTable(kept, actionManifest, opts.OutDir, opts.StandaloneDir, payloadDistDir(opts.Payload))
	if err != nil {
		return nil, fmt.Errorf("emit dispatch: %w", err)
	}

	// Phase 9 — extract embedded JS runtime (dispatcher + serve + route_match + errors + context + rsc).
	runtimeFiles, err := ExtractRuntime(opts.OutDir)
	if err != nil {
		return nil, fmt.Errorf("extract runtime: %w", err)
	}
	log.Debug("nextcompile: extracted %d runtime files", len(runtimeFiles))

	// Phase 10 — vendor RSC runtime when the target requires it.
	vendored, err := maybeVendorRSC(opts, manifest.Features, reactVer, log)
	if err != nil {
		return nil, err
	}

	// Phase 11 — emit action_manifest.json. Always emitted (empty when app
	// has no actions) so the runtime's import is never missing.
	actionManifestPath, err := EmitActionManifest(actionManifest, opts.OutDir)
	if err != nil {
		return nil, fmt.Errorf("emit action manifest: %w", err)
	}

	// Phase 12 — emit protection.json (edge-guard policy; `null` when unset).
	protectionPath, err := EmitProtectionConfig(opts.Protection, opts.OutDir)
	if err != nil {
		return nil, fmt.Errorf("emit protection config: %w", err)
	}

	// Phase 13 — emit worker_entry.mjs (the esbuild entrypoint).
	entryPath, err := EmitWorkerEntry(opts.OutDir)
	if err != nil {
		return nil, fmt.Errorf("emit worker entry: %w", err)
	}

	// Phase 14 — compute content hash over every generated/extracted file in
	// deterministic order. Adapter uses this to skip no-op redeploys.
	// manifestPath is intentionally excluded from the file list — its on-disk
	// bytes embed the wall-clock GeneratedAt; hashBundle folds in a normalized
	// (timestamp-zeroed) manifest instead so identical input hashes identically.
	hashInputs := append([]string{dispatchPath, actionManifestPath, protectionPath, entryPath}, runtimeFiles...)
	if vendored != nil {
		hashInputs = append(hashInputs, vendored.TargetPath)
		if vendored.ShimPath != "" {
			hashInputs = append(hashInputs, vendored.ShimPath)
		}
		// Every byte that ships must be in the hash. If a companion were
		// omitted, bumping react-dom 19.0 → 19.1 would leave the hash
		// unchanged, the adapter would skip the redeploy, and the Worker
		// would keep serving the old React.
		for _, ex := range vendored.Extra {
			hashInputs = append(hashInputs, ex.TargetPath)
			if ex.ShimPath != "" {
				hashInputs = append(hashInputs, ex.ShimPath)
			}
		}
	}
	sort.Strings(hashInputs)
	contentHash, totalBytes, err := hashBundle(opts.OutDir, &manifest, hashInputs)
	if err != nil {
		return nil, fmt.Errorf("hash bundle: %w", err)
	}

	actionCount := 0
	if actionManifest != nil {
		actionCount = len(actionManifest.Actions)
	}
	stats := CompileStats{
		RouteCount:       len(kept),
		ActionCount:      actionCount,
		DeadRoutesElided: elided,
		BundleBytes:      totalBytes,
		Duration:         time.Since(start),
		ContentHash:      contentHash,
	}

	log.Info("nextcompile: complete in %s (routes=%d actions=%d elided=%d bytes=%d hash=%s)",
		stats.Duration.Round(time.Millisecond),
		stats.RouteCount, stats.ActionCount, stats.DeadRoutesElided,
		stats.BundleBytes, stats.ContentHash[:12])

	return &CompiledBundle{
		BundleDir:         opts.OutDir,
		EntryPath:         entryPath,
		ManifestPath:      manifestPath,
		DispatchPath:      dispatchPath,
		ActionManifest:    actionManifestPath,
		DetectedVersion:   nextVer,
		DetectedReact:     reactVer,
		SuggestedBindings: hints,
		VendoredRSC:       vendored,
		Stats:             stats,
	}, nil
}

// maybeVendorRSC runs VendorRSC only for targets that can't install npm
// packages at runtime (Workers). Other targets ship node_modules and
// resolve at runtime, so vendoring is unnecessary.
//
// Failure policy:
//   - package missing + features.RSC == true  → fatal, clear error
//   - package missing + features.RSC == false → debug log, proceed
//   - package missing, any other error        → bubble up unchanged
func maybeVendorRSC(opts CompileOpts, features ManifestFeatures, react ReactVersion, log Logger) (*VendoredPackage, error) {
	if opts.Target != TargetCloudflareWorker {
		return nil, nil
	}

	vendored, err := VendorRSC(opts.StandaloneDir, opts.OutDir)
	if err == nil {
		log.Info("nextcompile: vendored %s@%s (%s %s build, %d bytes) + %d SSR companion(s)",
			vendored.Name, vendored.Version, vendored.BuildKind, vendored.Format,
			vendored.Bytes, len(vendored.Extra))
		for _, ex := range vendored.Extra {
			log.Debug("nextcompile:   companion %s (%s %s, %d bytes)",
				ex.Name, ex.BuildKind, ex.Format, ex.Bytes)
		}
		return vendored, nil
	}

	// A missing primary OR SSR companion is fatal only when the app uses RSC.
	// %w on each sentinel keeps errors.Is working for callers above and lets
	// the message name the exact build that's absent.
	if errors.Is(err, ErrRSCPackageNotFound) ||
		errors.Is(err, ErrRSCServerEdgeNotFound) ||
		errors.Is(err, ErrRSCClientEdgeNotFound) ||
		errors.Is(err, ErrReactDOMPackageNotFound) ||
		errors.Is(err, ErrReactDOMServerNotFound) {
		if features.RSC {
			return nil, fmt.Errorf(
				"RSC/SSR pages detected but a required React edge build is missing (%w).\n"+
					"Detected React: %[2]s.\n"+
					"Fix: at the app level, run `pnpm add react-dom@%[2]s react-server-dom-webpack@%[2]s`\n"+
					"(matching your React version), or ensure the standalone build includes\n"+
					"react, react-dom and react-server-dom-webpack under node_modules.\n"+
					"See _nextdeploy/runtime/vendor/README.md for details",
				err, react.Raw)
		}
		log.Debug("nextcompile: RSC/SSR edge builds not installed; app does not use RSC, skipping vendoring")
		return nil, nil
	}
	return nil, fmt.Errorf("vendor RSC/SSR: %w", err)
}

// normalizeOpts fills in defensible defaults so the rest of the pipeline
// can assume a fully-populated CompileOpts.
func normalizeOpts(opts CompileOpts) CompileOpts {
	if opts.Target == "" {
		opts.Target = TargetCloudflareWorker
	}
	if opts.OutDir == "" {
		opts.OutDir = filepath.Join(opts.StandaloneDir, ".nextdeploy-build")
	}
	if opts.Log == nil {
		opts.Log = nopLogger{}
	}
	return opts
}

func ensureOutDir(dir string) error {
	// Restrictive perms — this tree ends up in a worker bundle and
	// may contain compiled secrets-by-reference. 0o750 (rwxr-x---)
	// prevents other users on shared build hosts from reading the
	// pre-bundle artifacts. NOSONAR: chosen intentionally for this
	// reason; do not widen to 0o755.
	return os.MkdirAll(dir, 0o750)
}

// hashBundle computes the deploy-skip digest deterministically for identical
// input: a NORMALIZED manifest (GeneratedAt zeroed — it's a wall-clock stamp)
// plus every other emitted file, each hashed by its baseDir-RELATIVE path
// (never the absolute path, which varies per machine/checkout) and content.
// Also returns the total byte count for CompileStats.BundleBytes.
func hashBundle(baseDir string, manifest *Manifest, paths []string) (hash string, total int64, err error) {
	norm := *manifest
	norm.GeneratedAt = "" // exclude the wall-clock stamp from the digest
	mb, err := json.Marshal(norm)
	if err != nil {
		return "", 0, fmt.Errorf("marshal manifest for hash: %w", err)
	}

	h := sha256.New()
	h.Write([]byte("manifest\x00"))
	h.Write(mb)
	total = int64(len(mb))

	for _, p := range paths {
		data, err := os.ReadFile(p) // #nosec G304 — hashing the compiler's own output
		if err != nil {
			return "", 0, fmt.Errorf("read %s: %w", p, err)
		}
		rel, err := filepath.Rel(baseDir, p)
		if err != nil {
			rel = filepath.Base(p)
		}
		h.Write([]byte(filepath.ToSlash(rel)))
		h.Write([]byte{0})
		h.Write(data)
		total += int64(len(data))
	}
	return hex.EncodeToString(h.Sum(nil)), total, nil
}

// ── Stub phases (until dedicated files land) ─────────────────────────────────

func deriveBindingsStub(refs []ModuleRef, _ Payload) []BindingHint {
	seen := map[string]struct{}{}
	var hints []BindingHint
	for _, r := range refs {
		for _, env := range r.EnvRefs {
			if _, ok := seen[env]; ok {
				continue
			}
			seen[env] = struct{}{}
			hints = append(hints, BindingHint{
				Kind:    "secret",
				Name:    env,
				Reason:  "referenced via process.env in compiled output",
				Sources: []string{r.CompiledPath},
			})
		}
	}
	return hints
}

// elideDeadRoutes intentionally ships every scanned route.
//
// This is a decision, not an unfinished stub. A "dead route" would be a
// compiled module no request can reach — but the dispatcher resolves routes
// dynamically from the manifest, and dynamic segments, rewrites and middleware
// rewrites all defeat naive static reachability analysis. A wrong elision 404s
// a live route: a correctness bug traded for a few KB of bundle. Bad trade.
//
// The count is retained in the signature because CompileStats reports it, and
// because a future, *provably* safe elision (e.g. a compiled file with no
// exported handler that is absent from every manifest table) could fill it in.
func elideDeadRoutes(refs []ModuleRef, _ RouteInfo) ([]ModuleRef, int) {
	return refs, 0
}
