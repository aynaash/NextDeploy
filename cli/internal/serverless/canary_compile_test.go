//go:build canary

// Canary compile harness. Not part of the normal test suite — it's invoked by
// the Next.js ecosystem canary matrix (.github/workflows/nextjs-canary-matrix.yml)
// with `go test -tags canary`. It drives the real BuildWorkerBundle compile path
// (13-phase nextcompile + esbuild) against a freshly-scaffolded fixture so any
// drift in Next.js's standalone output, action manifest, or RSC surface fails
// here before it reaches users.
//
// It deliberately does NOT deploy: BuildWorkerBundle produces the local
// worker.mjs and uploads nothing, so no Cloudflare credentials are required.
//
// The fixture directory is passed via CANARY_FIXTURE_DIR because `go test` runs
// with its working directory set to the package source dir, not the fixture.
package serverless

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/aynaash/nextdeploy/shared"
	"github.com/aynaash/nextdeploy/shared/config"
	"github.com/aynaash/nextdeploy/shared/nextcore"
)

func TestCanaryCompile(t *testing.T) {
	fixture := os.Getenv("CANARY_FIXTURE_DIR")
	if fixture == "" {
		t.Fatal("CANARY_FIXTURE_DIR not set; this harness is driven by the canary workflow")
	}
	if err := os.Chdir(fixture); err != nil {
		t.Fatalf("chdir to fixture %q: %v", fixture, err)
	}

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("load nextdeploy.yml: %v", err)
	}
	meta, err := nextcore.LoadMetadata()
	if err != nil {
		t.Fatalf("load .nextdeploy/metadata.json (did `nextdeploy build` run?): %v", err)
	}

	standaloneDir := filepath.Join(".next", "standalone")
	if _, err := os.Stat(standaloneDir); err != nil {
		t.Fatalf("standalone build output missing at %s: %v", standaloneDir, err)
	}

	log := shared.PackageLogger("canary", "🐤 CANARY")
	out, err := BuildWorkerBundle(context.Background(), standaloneDir, &meta, cfg, log)
	if err != nil {
		t.Fatalf("BuildWorkerBundle failed (likely compiler drift against the current Next.js release): %v", err)
	}

	if _, err := os.Stat(out); err != nil {
		t.Fatalf("worker bundle reported at %s but not found: %v", out, err)
	}
	t.Logf("canary compile OK: %s", out)
}
