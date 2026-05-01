// Package buildflow is the single entry point for "prepare a Next.js
// project for deployment". Both `nextdeploy build` and `nextdeploy ship`
// call Run; ship continues on to deploy, build exits.
//
// Why this exists: until this consolidation, build and ship had divergent
// pre-deploy contracts.
//
//   - `nextdeploy build` validated output mode + features and produced a
//     VPS tarball, but never invoked `next build` itself.
//   - `nextdeploy ship` only ran `next build` for the Cloudflare path
//     (forcing --webpack), and skipped the build-side validations entirely.
//
// The result was inconsistent pre-conditions across the three deployment
// targets (VPS, AWS Lambda, Cloudflare Worker) and two places where
// "should we rebuild?" / "is this output valid?" logic lived. Run owns
// both questions for every target.
package buildflow

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/aynaash/nextdeploy/internal/packaging"
	"github.com/aynaash/nextdeploy/shared"
	"github.com/aynaash/nextdeploy/shared/config"
	"github.com/aynaash/nextdeploy/shared/nextbuild"
	"github.com/aynaash/nextdeploy/shared/nextcore"
	"github.com/aynaash/nextdeploy/shared/utils"
)

// Opts configures a single Run invocation.
type Opts struct {
	// ProjectDir is the working directory containing package.json and
	// next.config.*. Defaults to "." when empty.
	ProjectDir string

	// Cfg drives target-aware behavior. Required.
	Cfg *config.NextDeployConfig

	// Force bypasses the incremental skip (git-commit unchanged → no
	// rebuild). Wired to `nextdeploy build --force`. Ship always passes
	// false because shipping with an unverified-stale build is a footgun.
	Force bool

	// Log receives lifecycle messages. Required.
	Log *shared.Logger
}

// Result is the artifact set produced by a Run.
type Result struct {
	Payload nextcore.NextCorePayload

	// EffectiveTarget is the target this build was prepared for —
	// "serverless" or "vps". Resolved from Cfg + payload.
	EffectiveTarget string

	// StandaloneDir is the path to .next/standalone. Populated when
	// OutputMode is standalone (required for serverless, common for VPS).
	StandaloneDir string

	// ReleaseDir / TarballPath are populated only on the VPS path —
	// the artifact `nextdeploy ship` uploads to the remote daemon.
	ReleaseDir  string
	TarballPath string

	// Skipped is true when the incremental check matched and no rebuild
	// was attempted. `nextdeploy build` treats this as "exit success";
	// ship treats it as "use the existing artifacts".
	Skipped bool
}

// Run executes the unified build flow:
//
//  1. Incremental skip (unless Force): if git commit is unchanged, return
//     early with a fresh metadata payload.
//  2. Generate metadata (nextcore.GenerateMetadata) — reads next.config
//     and the routes/prerender manifests.
//  3. Validate output mode + features against the resolved target.
//  4. Decide whether `next build` needs to run, and with which flags
//     (Cloudflare requires --webpack; AWS / VPS take the user's default).
//  5. For VPS: copy public/ + static/ + metadata.json into the release
//     directory and create app.tar.gz.
//  6. Audit the standalone tree — informational warnings for size.
func Run(ctx context.Context, opts Opts) (*Result, error) {
	if opts.Cfg == nil {
		return nil, fmt.Errorf("buildflow: Cfg is required")
	}
	if opts.Log == nil {
		return nil, fmt.Errorf("buildflow: Log is required")
	}
	project := opts.ProjectDir
	if project == "" {
		project = "."
	}

	// ── 1. Incremental skip ────────────────────────────────────────────
	if !opts.Force {
		if err := nextcore.ValidateBuildState(); err == nil {
			opts.Log.Info("Git commit unchanged — skipping build (incremental state matched).")
			payload, mErr := nextcore.GenerateMetadata()
			if mErr != nil {
				return nil, fmt.Errorf("regenerate metadata after incremental skip: %w", mErr)
			}
			return &Result{
				Payload:         payload,
				EffectiveTarget: opts.Cfg.ResolveTargetType(payload.Config.TargetType),
				StandaloneDir:   filepath.Join(payload.DistDir, "standalone"),
				Skipped:         true,
			}, nil
		}
	}

	// ── 2. Metadata ────────────────────────────────────────────────────
	payload, err := nextcore.GenerateMetadata()
	if err != nil {
		return nil, fmt.Errorf("generate metadata: %w", err)
	}
	target := opts.Cfg.ResolveTargetType(payload.Config.TargetType)
	opts.Log.Info("Build target: %s", target)

	// ── 3. Validate ────────────────────────────────────────────────────
	if err := validateForTarget(target, &payload, opts.Cfg, opts.Log); err != nil {
		return nil, err
	}

	// ── 4. next build (if needed) ──────────────────────────────────────
	rebuilt, err := ensureNextBuild(ctx, target, opts.Cfg, opts.Log)
	if err != nil {
		return nil, err
	}
	if rebuilt {
		// Manifests changed underneath us — refresh.
		payload, err = nextcore.GenerateMetadata()
		if err != nil {
			return nil, fmt.Errorf("regenerate metadata after build: %w", err)
		}
	}

	standaloneDir := filepath.Join(payload.DistDir, "standalone")
	result := &Result{
		Payload:         payload,
		EffectiveTarget: target,
		StandaloneDir:   standaloneDir,
	}

	// ── 5. VPS artifact ────────────────────────────────────────────────
	if target == "vps" {
		releaseDir, tarballPath, err := buildVPSArtifact(payload, opts.Log)
		if err != nil {
			return nil, err
		}
		result.ReleaseDir = releaseDir
		result.TarballPath = tarballPath
	}

	// ── 6. Audit ───────────────────────────────────────────────────────
	if payload.OutputMode == nextcore.OutputModeStandalone {
		if report, err := packaging.AuditStandaloneSize(standaloneDir); err == nil {
			opts.Log.Info("Bundle audit: %.2fMB total (node_modules: %.2fMB)", report.TotalMB, report.NodeModulesMB)
			if len(report.TopOffenders) > 0 {
				top := report.TopOffenders[0]
				opts.Log.Info("  Top offender: %s (%.2fMB)", top.Package, top.SizeMB)
			}
			// AWS Lambda's 250MB limit is the strictest of the three
			// targets; flag at 200MB / fail at 250MB regardless of
			// target so a CF-bound build can still flag a future AWS
			// switch as risky.
			if report.TotalMB > 250 {
				opts.Log.Warn("Bundle %.2fMB exceeds AWS Lambda's 250MB limit.", report.TotalMB)
			} else if report.TotalMB > 200 {
				opts.Log.Warn("Bundle %.2fMB approaching AWS Lambda's 250MB limit.", report.TotalMB)
			}
		}
	}

	return result, nil
}

// validateForTarget enforces output-mode / feature constraints that fail
// early in the build, not deep inside a packager. Errors are fatal;
// warnings let the deploy continue.
func validateForTarget(target string, payload *nextcore.NextCorePayload, cfg *config.NextDeployConfig, log *shared.Logger) error {
	if target == "serverless" && payload.OutputMode != nextcore.OutputModeStandalone {
		return fmt.Errorf("serverless deployments require 'output: \"standalone\"' in next.config — set it and rebuild")
	}
	if target == "vps" && payload.OutputMode == nextcore.OutputModeStandalone {
		log.Warn("Targeting 'vps' with 'output: standalone' — works, but the default mode is often preferred for VPS.")
	}
	if payload.DetectedFeatures != nil && payload.DetectedFeatures.HasServerActions && payload.OutputMode == nextcore.OutputModeExport {
		return fmt.Errorf("Server Actions detected with OutputMode=export — change Next.js config to a runtime-enabled mode")
	}

	if target == "serverless" && payload.DetectedFeatures != nil {
		if len(payload.RouteInfo.ISRRoutes) > 0 && !cfg.App.CDNEnabled {
			log.Warn("ISR routes detected but CDN is not enabled — revalidation may not work correctly.")
		}
		if payload.DetectedFeatures.HasStripe && os.Getenv("STRIPE_SECRET_KEY") == "" {
			log.Warn("Stripe detected but STRIPE_SECRET_KEY is not set in environment.")
		}
	}
	return nil
}

// ensureNextBuild decides whether `next build` needs to run for the
// resolved target and runs it if so. Returns (rebuilt, err): rebuilt is
// true when the build actually executed (so the caller knows to refresh
// any cached metadata).
//
// Decision matrix:
//
//	target=serverless, provider=cloudflare:
//	    no standalone           → run with --webpack
//	    standalone is Turbopack → re-run with --webpack (overwrite)
//	    standalone is Webpack   → no-op
//	target=serverless (AWS) / target=vps:
//	    no standalone → run vanilla `next build`
//	    standalone exists → no-op (trust the user)
//
// The Cloudflare branch runs the explicit Webpack path because the
// adapter scans .next/server/app/*.js and dynamic-imports each compiled
// page from a dispatch table — Turbopack's runtime-resolved chunks crash
// once esbuild bundles them. AWS and VPS ship the full standalone
// server.js so either bundler works there.
func ensureNextBuild(ctx context.Context, target string, cfg *config.NextDeployConfig, log *shared.Logger) (bool, error) {
	standalone := ".next/standalone"
	info, err := os.Stat(standalone)
	hasBuild := err == nil && info.IsDir()

	if target == "serverless" && cfg.Serverless != nil && cfg.Serverless.Provider == "cloudflare" {
		if hasBuild && !nextbuild.IsTurbopackOutput(".") {
			log.Info("Existing Webpack standalone detected — skipping rebuild.")
			return false, nil
		}
		if !hasBuild {
			log.Info("No standalone build found — running `next build --webpack`.")
		} else {
			log.Warn("Turbopack standalone detected — Cloudflare adapter requires Webpack output.")
			log.Info("Re-running `next build --webpack` (this overwrites .next/).")
		}
		if err := nextbuild.Run(ctx, nextbuild.Opts{
			ProjectDir: ".",
			Target:     nextbuild.TargetCloudflareWorker,
			Log:        log,
		}); err != nil {
			return false, err
		}
		return true, nil
	}

	if hasBuild {
		log.Info("Existing standalone build detected — skipping rebuild.")
		return false, nil
	}
	log.Info("No standalone build found — running `next build`.")
	if err := nextbuild.Run(ctx, nextbuild.Opts{
		ProjectDir: ".",
		Target:     nextbuildTargetFor(target),
		Log:        log,
	}); err != nil {
		return false, err
	}
	return true, nil
}

func nextbuildTargetFor(target string) nextbuild.Target {
	switch target {
	case "vps":
		return nextbuild.TargetVPS
	case "serverless":
		return nextbuild.TargetAWSLambda
	default:
		return nextbuild.TargetGeneric
	}
}

// buildVPSArtifact stages public/ + static/ + metadata.json into the
// release directory and tars it into app.tar.gz. Mirrors what the old
// `nextdeploy build` did for the VPS path.
func buildVPSArtifact(payload nextcore.NextCorePayload, log *shared.Logger) (releaseDir, tarballPath string, err error) {
	rd := ""
	switch payload.OutputMode {
	case nextcore.OutputModeStandalone:
		rd = filepath.Join(payload.DistDir, "standalone")
		log.Info("Copying public/ → %s/public/", rd)
		if err := utils.CopyDir("public", filepath.Join(rd, "public")); err != nil {
			return "", "", fmt.Errorf("copy public/: %w", err)
		}
		log.Info("Copying %s/static/ → %s/%s/static/", payload.DistDir, rd, payload.DistDir)
		if err := utils.CopyDir(filepath.Join(payload.DistDir, "static"), filepath.Join(rd, payload.DistDir, "static")); err != nil {
			return "", "", fmt.Errorf("copy %s/static/: %w", payload.DistDir, err)
		}
		if err := utils.CopyFile(".nextdeploy/metadata.json", filepath.Join(rd, "metadata.json")); err != nil {
			return "", "", fmt.Errorf("copy metadata.json: %w", err)
		}
	case nextcore.OutputModeExport:
		rd = payload.ExportDir
		if err := utils.CopyFile(".nextdeploy/metadata.json", filepath.Join(rd, "metadata.json")); err != nil {
			return "", "", fmt.Errorf("copy metadata.json: %w", err)
		}
	default:
		rd = "."
		if err := utils.CopyFile(".nextdeploy/metadata.json", "metadata.json"); err != nil {
			return "", "", fmt.Errorf("copy metadata.json: %w", err)
		}
	}
	log.Info("Release directory: %s", rd)

	tarball := "app.tar.gz"
	log.Info("Creating tarball: %s", tarball)
	if err := utils.CreateTarball(rd, tarball, "vps", &payload, log); err != nil {
		return "", "", fmt.Errorf("create tarball: %w", err)
	}
	return rd, tarball, nil
}
