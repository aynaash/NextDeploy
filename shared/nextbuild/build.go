package nextbuild

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/aynaash/nextdeploy/shared"
)

type Target string

const (
	TargetCloudflareWorker Target = "cloudflare-worker"
	TargetVPS              Target = "vps"
	TargetGeneric          Target = "generic"
)

type Opts struct {
	ProjectDir string

	Target Target

	Log *shared.Logger

	ExtraArgs []string
}

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

	// #nosec G204 -- binPath is not user input: resolveNextBinary only ever
	// returns <projectDir>/node_modules/.bin/next after stat'ing it, and errors
	// otherwise. args are built here from a fixed verb plus flagsForTarget's
	// closed set; ExtraArgs comes from the caller's own flags, which is the
	// same trust level as the command line that invoked us. No shell is
	// involved — exec takes argv directly.
	cmd := exec.CommandContext(ctx, binPath, args...)
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

func indexByte(s string, b byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == b {
			return i
		}
	}
	return -1
}

func flagsForTarget(t Target) []string {
	switch t {
	case TargetCloudflareWorker:
		return []string{"--webpack"}
	default:
		return nil
	}
}

func buildEnv(t Target) []string {
	env := os.Environ()
	if t == TargetCloudflareWorker {
		env = scrubBundlerEnv(env)
	}
	return env
}

func resolveNextBinary(projectDir string) (string, error) {
	local := filepath.Join(projectDir, "node_modules", ".bin", "next")
	if info, err := os.Stat(local); err == nil && !info.IsDir() {
		return local, nil
	}
	if path, err := exec.LookPath("npx"); err == nil {
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
