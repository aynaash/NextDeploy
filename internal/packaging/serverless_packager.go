package packaging

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/aynaash/nextdeploy/shared"
	"github.com/aynaash/nextdeploy/shared/nextcore"
)

type PackageResult struct {
	// StandaloneTarPath is the deployable artifact: a gzipped tar of the raw
	// Next.js standalone directory, with no target-specific shims baked in.
	// Cloudflare extracts it before running its adapter. Empty if tar creation
	// failed (a warning is logged but packaging does not fail — providers that
	// need it will surface a clear error themselves).
	StandaloneTarPath string
	StandaloneTarSize int64
	StaticAssets      []StaticAsset
	SizeWarning       string
}

// StaticAsset is one file destined for the CDN bucket (R2), keyed by the path
// it is served under.
type StaticAsset struct {
	LocalPath    string
	Key          string
	CacheControl string
	ContentType  string
}

type Packager struct {
	projectRoot   string
	buildDir      string
	standaloneDir string
	publicDir     string
	payload       *nextcore.NextCorePayload
	tmpDir        string
}

func NewPackager(projectRoot string, payload *nextcore.NextCorePayload) (*Packager, error) {
	buildDir := filepath.Join(projectRoot, payload.DistDir)
	standaloneDir := filepath.Join(buildDir, "standalone")

	if _, err := os.Stat(standaloneDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("standalone directory not found at %s", standaloneDir)
	}

	tmpDir, err := os.MkdirTemp("", "nextdeploy-pkg-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp dir: %w", err)
	}

	return &Packager{
		projectRoot:   projectRoot,
		buildDir:      buildDir,
		standaloneDir: standaloneDir,
		publicDir:     filepath.Join(projectRoot, "public"),
		payload:       payload,
		tmpDir:        tmpDir,
	}, nil
}

func (p *Packager) Cleanup() { os.RemoveAll(p.tmpDir) }

func (p *Packager) Package() (*PackageResult, error) {
	result := &PackageResult{}

	assets, err := p.collectStaticAssets()
	if err != nil {
		return nil, err
	}
	result.StaticAssets = assets

	tarPath := filepath.Join(p.tmpDir, "app.tar.gz")
	if err := shared.CreateTarGz(p.standaloneDir, tarPath); err != nil {
		// Non-fatal: leave the fields zero-valued so Cloudflare's DeployCompute
		// falls back to reading standalone from disk and can decide how loudly
		// to complain.
		fmt.Fprintf(os.Stderr, "warning: could not tar standalone dir for portable artifact: %v\n", err)
	} else if info, err := os.Stat(tarPath); err == nil {
		result.StandaloneTarPath = tarPath
		result.StandaloneTarSize = info.Size()
	}

	return result, nil
}

func (p *Packager) collectStaticAssets() ([]StaticAsset, error) {
	var assets []StaticAsset

	// 1. public/
	if _, err := os.Stat(p.publicDir); err == nil {
		_ = filepath.Walk(p.publicDir, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return err
			}
			rel, _ := filepath.Rel(p.publicDir, path)
			assets = append(assets, StaticAsset{
				LocalPath:    path,
				Key:          rel,
				CacheControl: cacheControlForPublicFile(rel),
				ContentType:  mimeForExt(filepath.Ext(path)),
			})
			return nil
		})
	}

	// 2. .next/static/
	staticDir := filepath.Join(p.buildDir, "static")
	if _, err := os.Stat(staticDir); err == nil {
		_ = filepath.Walk(staticDir, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return err
			}
			rel, _ := filepath.Rel(p.buildDir, path)
			assets = append(assets, StaticAsset{
				LocalPath:    path,
				Key:          filepath.Join("_next", rel),
				CacheControl: "public, max-age=31536000, immutable",
				ContentType:  mimeForExt(filepath.Ext(path)),
			})
			return nil
		})
	}

	// 3. Prerendered routes.
	//
	// Three separate sources can declare a prerendered HTML for a route:
	//   - StaticRoutes: routes-manifest.json's staticRoutes (mostly Pages
	//     Router; in App Router this is often just /_app, /_error).
	//   - SSGRoutes:    prerender-manifest.json entries with no revalidate
	//     window (Next 16 App Router default for fully-static pages).
	//   - ISRRoutes:    prerender-manifest.json entries with a revalidate
	//     window.
	// Iterating only the first two missed App Router landing pages — the
	// manifest then claimed an SSG entry the runtime tried to fetch from
	// R2 but the packager never uploaded, producing a silent fall-through
	// to the compiled-module dispatch path (which can't invoke an App
	// Router page). Dedupe by route so addPrerenderedAsset isn't asked to
	// stat + queue the same path twice.
	seen := make(map[string]struct{})
	queue := func(route string) {
		if _, dup := seen[route]; dup {
			return
		}
		seen[route] = struct{}{}
		p.addPrerenderedAsset(&assets, route)
	}
	for _, route := range p.payload.RouteInfo.StaticRoutes {
		queue(route)
	}
	for route := range p.payload.RouteInfo.SSGRoutes {
		queue(route)
	}
	for route := range p.payload.RouteInfo.ISRRoutes {
		queue(route)
	}

	// 4. ISR Tag Map (if enabled/built)
	tagMapPath := filepath.Join(p.projectRoot, ".nextdeploy", "assets", "isr-tag-map.json")
	if _, err := os.Stat(tagMapPath); err == nil {
		assets = append(assets, StaticAsset{
			LocalPath:    tagMapPath,
			Key:          "isr-tag-map.json",
			CacheControl: "public, max-age=0, must-revalidate",
			ContentType:  "application/json",
		})
	}

	return assets, nil
}

func (p *Packager) addPrerenderedAsset(assets *[]StaticAsset, routePath string) {
	// Root route "/" maps to index.html — both for the R2 key and for
	// locating the on-disk file. filepath.Join(prefix, "/") collapses to
	// just `prefix`, which would make us look for `.next/server/app.html`
	// (doesn't exist) instead of `.next/server/app/index.html`. We
	// normalize routePath to "/index" for the FS probe but keep the R2
	// key at "index.html" — the runtime's lookup pulls it by R2 key.
	keyBase := strings.TrimPrefix(routePath, "/")
	if keyBase == "" {
		keyBase = "index"
	}
	fsRoutePath := routePath
	if fsRoutePath == "/" || fsRoutePath == "" {
		fsRoutePath = "/index"
	}

	// Standalone output structure: .next/standalone/.next/server/
	standaloneNext := filepath.Join(p.standaloneDir, ".next")

	prefixes := []string{
		filepath.Join(standaloneNext, "server", "app"),
		filepath.Join(standaloneNext, "server", "pages"),
	}

	pkgLog := shared.PackageLogger("packaging", "📦 PKG")
	htmlAdded := false
	for _, prefix := range prefixes {
		serverPath := filepath.Join(prefix, fsRoutePath)

		htmlPath := serverPath + ".html"
		if _, err := os.Stat(htmlPath); err == nil {
			*assets = append(*assets, StaticAsset{
				LocalPath:    htmlPath,
				Key:          keyBase + ".html",
				CacheControl: "public, max-age=0, must-revalidate",
				ContentType:  "text/html; charset=utf-8",
			})
			htmlAdded = true
			pkgLog.Debug("Prerendered HTML queued: %s → %s", routePath, keyBase+".html")
		}

		rscPath := serverPath + ".rsc"
		if _, err := os.Stat(rscPath); err == nil {
			*assets = append(*assets, StaticAsset{
				LocalPath:    rscPath,
				Key:          keyBase + ".rsc",
				CacheControl: "public, max-age=0, must-revalidate",
				ContentType:  "text/x-component",
			})
		}
	}
	if !htmlAdded {
		pkgLog.Warn("Prerendered HTML missing for route %s — looked under %s and %s. The runtime will fall through to the live dispatcher.",
			routePath, prefixes[0], prefixes[1])
	}
}
