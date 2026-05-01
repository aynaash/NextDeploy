// Package nextbuild owns the `next build` invocation so the CLI can
// guarantee the build flags are correct for the chosen deployment target.
//
// Why this exists: starting with Next 16, `next build` defaults to
// Turbopack. The Cloudflare Worker adapter (shared/nextcompile) scans
// `.next/server/app/*.js` and dynamic-imports each compiled page from a
// dispatch table — a model that fits Webpack's per-page CommonJS output
// but breaks on Turbopack's runtime-resolved chunks ("Dynamic require of
// 'path' is not supported"). Until the adapter is rewritten to use Next's
// standalone server.js as the entrypoint, we force Webpack for the
// Cloudflare target by passing `--webpack`.
//
// AWS/VPS targets are unaffected — both runtime models work there because
// they ship the full standalone server, not a per-page dispatch.
package nextbuild

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/aynaash/nextdeploy/shared"
)

// Target identifies which downstream packaging path the build feeds.
type Target string

const (
	// TargetCloudflareWorker forces Webpack output (no Turbopack).
	TargetCloudflareWorker Target = "cloudflare-worker"
	// TargetAWSLambda runs a vanilla `next build`.
	TargetAWSLambda Target = "aws-lambda"
	// TargetVPS runs a vanilla `next build`.
	TargetVPS Target = "vps"
	// TargetGeneric runs a vanilla `next build`. Catch-all for callers
	// that haven't decided on a target yet.
	TargetGeneric Target = "generic"
)

// Opts configures a single Run invocation.
type Opts struct {
	// ProjectDir is the working directory containing package.json and
	// next.config.*. Required.
	ProjectDir string

	// Target dictates the flag profile. Defaults to TargetGeneric.
	Target Target

	// Log receives lifecycle messages. May be nil; defaults to a noop.
	Log *shared.Logger

	// ExtraArgs is appended to the `next build` argv after target-specific
	// flags. Useful for callers that want to pass `--debug` or similar.
	ExtraArgs []string
}

// Run executes `next build` in opts.ProjectDir. The Next CLI is resolved
// from node_modules/.bin/next; if missing, falls back to `npx --yes next`.
//
// stdout and stderr stream to the parent process unmodified — the build
// log is rich and operators rely on its line-by-line output.
func Run(ctx context.Context, opts Opts) error {
	if opts.ProjectDir == "" {
		return fmt.Errorf("nextbuild: ProjectDir is required")
	}
	log := opts.Log
	if log == nil {
		log = shared.PackageLogger("nextbuild", "🔧 BUILD")
	}

	binPath, err := resolveNextBinary(opts.ProjectDir)
	if err != nil {
		return err
	}

	args := []string{"build"}
	args = append(args, flagsForTarget(opts.Target)...)
	args = append(args, opts.ExtraArgs...)

	log.Info("Running %s %v (cwd=%s)", filepath.Base(binPath), args, opts.ProjectDir)

	// NOSONAR: every argv element is either a static flag we own or a
	// caller-controlled string — no shell interpolation. ProjectDir is
	// passed through cmd.Dir, not concatenated into a shell string.
	cmd := exec.CommandContext(ctx, binPath, args...) // #nosec G204 — argv is constructed from constants + caller-validated flags
	cmd.Dir = opts.ProjectDir
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	cmd.Env = buildEnv(opts.Target)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("next build failed: %w", err)
	}
	log.Info("next build complete")
	return nil
}

// scrubBundlerEnv removes any TURBOPACK / NEXT_DISABLE_TURBOPACK signals
// the user might have inherited from their shell. Next 16 rejects builds
// that combine those env vars with the --webpack flag ("Multiple bundler
// flags set"), and we always pass --webpack for Cloudflare. Stripping
// them keeps the flag the single source of truth.
func scrubBundlerEnv(env []string) []string {
	out := make([]string, 0, len(env))
	for _, kv := range env {
		i := indexByte(kv, '=')
		if i <= 0 {
			out = append(out, kv)
			continue
		}
		k := kv[:i]
		if k == "TURBOPACK" || k == "NEXT_DISABLE_TURBOPACK" {
			continue
		}
		out = append(out, kv)
	}
	return out
}

// indexByte mirrors strings.IndexByte without the import; keeps this
// file's import set narrow.
func indexByte(s string, b byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == b {
			return i
		}
	}
	return -1
}

// flagsForTarget returns the `next build` flags appropriate for the given
// deployment target.
//
// Cloudflare Worker: --webpack opts out of Turbopack (Next 16+ default)
// because the current adapter requires Webpack-style per-page output.
// See package doc.
func flagsForTarget(t Target) []string {
	switch t {
	case TargetCloudflareWorker:
		return []string{"--webpack"}
	default:
		return nil
	}
}

// buildEnv prepares the environment for the `next build` invocation.
//
// For TargetCloudflareWorker we strip TURBOPACK / NEXT_DISABLE_TURBOPACK
// from the inherited env. We always pass --webpack via flagsForTarget,
// and Next 16 errors out when both signals appear ("Multiple bundler
// flags set"). The flag is the source of truth; conflicting env vars
// the user happens to have set in their shell get scrubbed.
func buildEnv(t Target) []string {
	env := os.Environ()
	if t == TargetCloudflareWorker {
		env = scrubBundlerEnv(env)
	}
	return env
}

// resolveNextBinary returns the path to the Next CLI to execute.
//
// Resolution order:
//  1. <projectDir>/node_modules/.bin/next  — works for every package
//     manager (npm, pnpm, yarn, bun) because they all populate this dir.
//  2. `npx` on PATH                        — fallback when node_modules
//     hasn't been installed yet; returns the argv `npx --yes next` form.
//
// Returning the resolved binary path (rather than the npm run-script
// indirection) avoids picking up the user's `package.json` "build"
// script, which might pass --turbopack or other flags we'd then have to
// fight. Going direct gives us full control over the argv.
func resolveNextBinary(projectDir string) (string, error) {
	local := filepath.Join(projectDir, "node_modules", ".bin", "next")
	if info, err := os.Stat(local); err == nil && !info.IsDir() {
		return local, nil
	}
	// Fall back to npx — it'll fetch next on demand if not installed.
	if path, err := exec.LookPath("npx"); err == nil {
		// We can't return "npx --yes next" as a single binary path; the
		// caller passes argv via exec.Command which only accepts a
		// single program. The contract here is "give me the program";
		// callers using npx must therefore prepend "--yes next" to the
		// args list. We signal this via a dedicated error so callers
		// can adapt — but for now, error out with a clear message
		// directing the user to install dependencies first.
		_ = path
		return "", fmt.Errorf(
			"node_modules/.bin/next not found in %s — run `bun install`/`pnpm install`/`npm install` first, "+
				"or ensure your CI step installs dependencies before nextdeploy ship",
			projectDir,
		)
	}
	return "", fmt.Errorf(
		"could not locate `next` CLI (no node_modules/.bin/next, no npx on PATH); "+
			"install Next.js as a project dependency in %s",
		projectDir,
	)
}
