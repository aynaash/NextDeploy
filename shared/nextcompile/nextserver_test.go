package nextcompile

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveEngine_AutoDetectsTurbopack(t *testing.T) {
	dir := t.TempDir()
	// Webpack layout → EngineLegacy under EngineAuto
	writeFile(t, filepath.Join(dir, ".next", "server", "chunks", "471.js"), `module.exports = {}`)
	if got := ResolveEngine(EngineAuto, dir, ".next"); got != EngineLegacy {
		t.Fatalf("webpack tree under auto: got %q, want %q", got, EngineLegacy)
	}
	if got := ResolveEngine(EngineNextServer, dir, ".next"); got != EngineNextServer {
		t.Fatalf("explicit nextserver: got %q, want %q", got, EngineNextServer)
	}

	// Add Turbopack marker → EngineNextServer under EngineAuto
	writeFile(t, filepath.Join(dir, ".next", "server", "chunks", "ssr", "[turbopack]_runtime.js"), `module.exports = {}`)
	if got := ResolveEngine(EngineAuto, dir, ".next"); got != EngineNextServer {
		t.Fatalf("turbopack tree under auto: got %q, want %q", got, EngineNextServer)
	}
	if got := ResolveEngine(EngineLegacy, dir, ".next"); got != EngineLegacy {
		t.Fatalf("explicit legacy override: got %q, want %q", got, EngineLegacy)
	}
}

func TestCompile_NextServerEngineEmitsVFSAndChunksForTurbopack(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "package.json"), `{"name":"turbo-app","dependencies":{"next":"16.0.10","react":"19.2.0"}}`)
	writeFile(t, filepath.Join(dir, ".next", "BUILD_ID"), `turbo-build-123`)
	writeFile(t, filepath.Join(dir, ".next", "routes-manifest.json"), `{"version":3,"basePath":""}`)
	writeFile(t, filepath.Join(dir, ".next", "required-server-files.json"), `{"config":{"trailingSlash":false,"images":{"unoptimized":true}}}`)
	// .nft.json trace files must be skipped from VFS
	writeFile(t, filepath.Join(dir, ".next", "server", "app", "page.js.nft.json"), `{"files":["huge"]}`)
	writeFile(t, filepath.Join(dir, ".next", "server", "app", "page.js"), `module.exports = { default: () => "hi" }`)
	writeFile(t, filepath.Join(dir, ".next", "server", "chunks", "ssr", "[turbopack]_runtime.js"), `module.exports = { loadChunk: (p) => require(p) }`)
	writeFile(t, filepath.Join(dir, ".next", "server", "chunks", "ssr", "chunk_abc.js"), `module.exports = { ok: true }`)

	outDir := filepath.Join(dir, ".nextdeploy-cf")
	bundle, err := Compile(context.Background(), CompileOpts{
		StandaloneDir: dir,
		OutDir:        outDir,
		Target:        TargetCloudflareWorker,
		Engine:        EngineAuto,
		Payload: Payload{
			AppName:      "turbo-app",
			DistDir:      ".next",
			HasAppRouter: true,
			BuildID:      "turbo-build-123",
		},
	})
	if err != nil {
		t.Fatalf("Compile(nextserver): %v", err)
	}
	if bundle.ResolvedEngine != EngineNextServer {
		t.Fatalf("ResolvedEngine = %q, want %q", bundle.ResolvedEngine, EngineNextServer)
	}
	if bundle.VFSPath == "" || bundle.ChunksPath == "" {
		t.Fatalf("expected VFSPath and ChunksPath to be populated: %+v", bundle)
	}

	vfsBytes, err := os.ReadFile(bundle.VFSPath) // #nosec G304
	if err != nil {
		t.Fatalf("read vfs.mjs: %v", err)
	}
	vfsSrc := string(vfsBytes)
	if !strings.Contains(vfsSrc, "turbo-build-123") || !strings.Contains(vfsSrc, `"trailingSlash":false`) {
		t.Errorf("vfs.mjs missing expected manifests/config:\n%s", vfsSrc)
	}
	if strings.Contains(vfsSrc, ".nft.json") {
		t.Errorf("vfs.mjs should skip .nft.json trace files")
	}

	chunksBytes, err := os.ReadFile(bundle.ChunksPath) // #nosec G304
	if err != nil {
		t.Fatalf("read chunks.mjs: %v", err)
	}
	chunksSrc := string(chunksBytes)
	for _, want := range []string{
		`".next/server/chunks/ssr/[turbopack]_runtime.js": () => require("../../.next/server/chunks/ssr/[turbopack]_runtime.js")`,
		`".next/server/chunks/ssr/chunk_abc.js": () => require("../../.next/server/chunks/ssr/chunk_abc.js")`,
		`".next/server/app/page.js": () => require("../../.next/server/app/page.js")`,
	} {
		if !strings.Contains(chunksSrc, want) {
			t.Errorf("chunks.mjs missing %q\ngot:\n%s", want, chunksSrc)
		}
	}

	entryBytes, err := os.ReadFile(bundle.EntryPath) // #nosec G304
	if err != nil {
		t.Fatalf("read worker_entry.mjs: %v", err)
	}
	if !strings.Contains(string(entryBytes), "next/dist/server/next-server.js") {
		t.Errorf("worker_entry.mjs should target next/dist/server/next-server.js")
	}
}
