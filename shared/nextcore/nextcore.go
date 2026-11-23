// Package nextcore introspects Next.js projects, build manifests, routes, and static assets.
package nextcore

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/aynaash/nextdeploy/shared"
	"github.com/aynaash/nextdeploy/shared/config"
	"github.com/aynaash/nextdeploy/shared/git"
)

// Standard paths generated and read by NextDeploy's build metadata pipeline.
const (
	BuildLockFileName = ".nextdeploy/build.lock"
	MetadataFileName  = ".nextdeploy/metadata.json"
	AssetsOutputDir   = ".nextdeploy/assets"
	PublicDir         = "public"
)

// NextCoreLogger is the package-level logger for nextcore operations.
var (
	NextCoreLogger = shared.PackageLogger("nextcore", "📦 NEXTCORE")
)

// GenerateMetadata runs `next build`, reads the emitted manifests, and writes
// .nextdeploy/metadata.json and .nextdeploy/build.lock.
func GenerateMetadata() (metadata NextCorePayload, err error) {
	return generateMetadata(false)
}

// RefreshMetadata regenerates .nextdeploy/metadata.json and build.lock from
// the existing Next.js build output without re-running `next build`. Falls
// back to a full build if the existing output has no BUILD_ID.
func RefreshMetadata() (metadata NextCorePayload, err error) {
	return generateMetadata(true)
}

func generateMetadata(skipBuild bool) (metadata NextCorePayload, err error) {
	cfg, err := config.Load()
	if err != nil {
		NextCoreLogger.Error("Failed to load configuration: %v", err)
		return NextCorePayload{}, err
	}

	cwd, err := os.Getwd()
	if err != nil {
		NextCoreLogger.Error("Error getting current working directory")
		return NextCorePayload{}, err
	}

	nextConfig, err := ParseNextConfigFile(filepath.Join(cwd, "next.config.mjs"))
	if err != nil {
		NextCoreLogger.Error("Failed to parse next config (non-fatal): %v", err)
	}
	features := DetectFeatures(nextConfig)

	packageManager, err := DetectPackageManager(cwd)
	if err != nil {
		NextCoreLogger.Error("Failed to detect package manager: %v", err)
		return NextCorePayload{}, err
	}

	buildMeta, err := resolveBuildMetadata(cwd, nextConfig, features.DistDir, packageManager.String(), skipBuild)
	if err != nil {
		NextCoreLogger.Error("Failed to collect build metadata: %v", err)
		return NextCorePayload{}, err
	}
	outputMode := detectOutputMode(cwd, nextConfig, features.DistDir)

	routeInfo, err := getRoutesFromManifests(buildMeta, features.DistDir)
	if err != nil {
		return NextCorePayload{}, err
	}

	imagesAssets, err := detectImageAssets(buildMeta, cwd, features.DistDir)
	if err != nil {
		NextCoreLogger.Error("Failed to detect image assets: %v", err)
		return NextCorePayload{}, err
	}

	middlewareConfig, proxyConfig, err := parseInterceptors(cwd)
	if err != nil {
		return NextCorePayload{}, err
	}

	staticAssets, err := ParseStaticAssets(cwd, features.DistDir)
	if err != nil {
		NextCoreLogger.Error("Failed to parse static assets: %v", err)
		return NextCorePayload{}, err
	}

	gitCommit, err := git.GetCommitHash()
	if err != nil {
		NextCoreLogger.Error("Failed to get git commit hash: %v", err)
		return NextCorePayload{}, err
	}
	NextCoreLogger.Debug("Git commit hash: %s", gitCommit)

	if err = copyStaticAssets(); err != nil {
		NextCoreLogger.Error("Failed to copy static assets: %v", err)
		return NextCorePayload{}, fmt.Errorf("failed to copy static assets: %w", err)
	}

	metadata = buildMetadataPayload(
		cfg, nextConfig, buildMeta, routeInfo, imagesAssets,
		middlewareConfig, proxyConfig, staticAssets, features,
		packageManager.String(), outputMode, gitCommit,
	)

	writeISRTagMap(cwd, metadata.RouteInfo.ISRDetail)

	if err = createBuildLock(&metadata, ConfigFingerprint(cfg)); err != nil {
		NextCoreLogger.Error("Failed to create build lock: %v", err)
		return NextCorePayload{}, fmt.Errorf("failed to create build lock: %w", err)
	}

	return metadata, nil
}

func buildMetadataPayload(
	cfg *config.NextDeployConfig,
	nextConfig *NextConfig,
	buildMeta *NextBuildMetadata,
	routeInfo *RouteInfo,
	imagesAssets *ImageAssets,
	middlewareConfig, proxyConfig *MiddlewareConfig,
	staticAssets *StaticAssets,
	features *DetectedFeatures,
	packageManager string,
	outputMode OutputMode,
	gitCommit string,
) NextCorePayload {
	metadata := NextCorePayload{
		AppName:           cfg.App.Name,
		NextBuildMetadata: *buildMeta,
		Config: config.SafeConfig{
			AppName:     cfg.App.Name,
			Domain:      cfg.App.Domain.Name,
			Port:        cfg.App.Port,
			Environment: cfg.App.Environment,
			TargetType:  cfg.ResolveTargetType(""),
		},
		CDNEnabled:       cfg.App.CDNEnabled,
		Domain:           cfg.App.Domain.Name,
		RouteInfo:        *routeInfo,
		DetectedFeatures: features,
		DistDir:          features.DistDir,
		ExportDir:        features.ExportDir,
		Middleware:       middlewareConfig,
		Proxy:            proxyConfig,
		StaticAssets:     staticAssets,
		GitCommit:        gitCommit,
		GitDirty:         git.IsDirty(),
		GeneratedAt:      time.Now().Format(time.RFC3339),
		PackageManager:   packageManager,
		OutputMode:       outputMode,
		ImageAssets:      *imagesAssets,
		Resources:        cfg.App.Resources,
		GoSidecar:        cfg.ResolveGoSidecar(),
	}
	applyNextConfigProjection(&metadata, nextConfig)
	return metadata
}

func applyNextConfigProjection(metadata *NextCorePayload, nextConfig *NextConfig) {
	if nextConfig == nil {
		return
	}
	metadata.BasePath = nextConfig.BasePath
	metadata.AssetPrefix = nextConfig.AssetPrefix
	metadata.I18n = nextConfig.I18n
	metadata.ImageConfig = nextConfig.Images
}

func parseInterceptors(cwd string) (middlewareCfg, proxyCfg *MiddlewareConfig, err error) {
	middlewareCfg, err = ParseMiddleware(cwd)
	if err != nil {
		NextCoreLogger.Error("Failed to parse middleware configuration: %v", err)
		return nil, nil, err
	}
	proxyCfg, err = ParseProxy(cwd)
	if err != nil {
		NextCoreLogger.Error("Failed to parse proxy configuration: %v", err)
		return nil, nil, err
	}
	return middlewareCfg, proxyCfg, nil
}

func writeISRTagMap(cwd string, isrDetail []ISRRoute) {
	if len(isrDetail) == 0 {
		return
	}
	tagMap := BuildTagMap(isrDetail)
	if tagMapData, err := json.MarshalIndent(tagMap, "", "  "); err == nil {
		assetsDir := filepath.Join(cwd, AssetsOutputDir)
		_ = os.MkdirAll(assetsDir, 0o750)
		_ = os.WriteFile(filepath.Join(assetsDir, "isr-tag-map.json"), tagMapData, 0o600)
	}
}

func resolveBuildMetadata(cwd string, nextConfig *NextConfig, distDir, packageManager string, skipBuild bool) (*NextBuildMetadata, error) {
	if skipBuild {
		if buildMeta, err := ReadBuildMetadata(cwd, distDir); err == nil {
			return buildMeta, nil
		} else {
			NextCoreLogger.Warn("Existing build output incomplete (%v) — running Next.js build.", err)
		}
	}
	buildCmd, err := buildCommand(packageManager)
	if err != nil {
		NextCoreLogger.Error("Failed to get build command: %v", err)
		return nil, err
	}
	nextVersion, _ := GetNextJsVersion(filepath.Join(cwd, "package.json"))
	buildCmd = MaybeInjectWebpackFlag(buildCmd, cwd, nextConfig, nextVersion, NextCoreLogger)
	return CollectBuildMetadata(buildCmd)
}

// LoadMetadata reads and unmarshals the generated .nextdeploy/metadata.json file.
func LoadMetadata() (NextCorePayload, error) {
	data, err := os.ReadFile(MetadataFileName)
	if err != nil {
		return NextCorePayload{}, fmt.Errorf("failed to read metadata file: %w (did you run 'nextdeploy build'?)", err)
	}

	var metadata NextCorePayload
	if err = json.Unmarshal(data, &metadata); err != nil {
		return NextCorePayload{}, fmt.Errorf("failed to unmarshal metadata: %w", err)
	}

	return metadata, nil
}

func copyStaticAssets() error {
	srcDir := "public"
	dstDir := filepath.Join(".nextdeploy", "assets")

	// Create destination directory
	if err := os.MkdirAll(dstDir, 0o750); err != nil {
		NextCoreLogger.Error("Failed to create destination directory: %v", err)
		return err
	}

	return filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			NextCoreLogger.Error("Error walking path %s: %v", path, err)
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Create relative path
		relPath, err := filepath.Rel(srcDir, path)
		if err != nil {
			NextCoreLogger.Error("Failed to get relative path for %s: %v", path, err)
			return err
		}

		// Create destination path
		dstPath := filepath.Join(dstDir, relPath)

		// Create destination directory structure
		if err = os.MkdirAll(filepath.Dir(dstPath), 0o750); err != nil {
			NextCoreLogger.Error("Failed to create directory for %s: %v", dstPath, err)
			return err
		}

		// Copy file
		return copyFile(path, dstPath)
	})
}

// copyFile copies a file from src to dst.
func copyFile(src, dst string) (err error) {
	// #nosec G304
	source, err := os.Open(src)
	if err != nil {
		NextCoreLogger.Error("Failed to open source file %s: %v", src, err)
		return err
	}
	defer func() {
		if closeErr := source.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()

	// #nosec G304
	destination, err := os.Create(dst)
	if err != nil {
		NextCoreLogger.Error("Failed to create destination file %s: %v", dst, err)
		return err
	}
	defer func() {
		if closeErr := destination.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()

	_, err = io.Copy(destination, source)
	return err
}

// createBuildLock writes the metadata payload and a build.lock using the git
// state already captured on the payload, plus a fingerprint of the config
// inputs (configHash) so ValidateBuildState can detect an uncommitted config
// edit that the git commit alone would miss.
func createBuildLock(metadata *NextCorePayload, configHash string) error {
	payloadData, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		NextCoreLogger.Error("Failed to marshal metadata: %v", err)
		return err
	}
	if err = os.WriteFile(MetadataFileName, payloadData, 0o600); err != nil {
		NextCoreLogger.Error("Failed to write metadata json: %v", err)
		return err
	}

	lockData, err := json.MarshalIndent(BuildLock{
		GitCommit:   metadata.GitCommit,
		GitDirty:    metadata.GitDirty,
		GeneratedAt: metadata.GeneratedAt,
		Metadata:    MetadataFileName,
		ConfigHash:  configHash,
	}, "", "  ")
	if err != nil {
		NextCoreLogger.Error("Failed to marshal build lock: %v", err)
		return err
	}
	return os.WriteFile(BuildLockFileName, lockData, 0o600)
}

// ConfigFingerprint hashes ONLY the config fields that change the built
// artifact — the ones copied into metadata.json and read by the daemon. An
// unrelated edit (a comment, an SSH setting) must not force a needless rebuild,
// but any of these must.
//
// Every field added here means "changing this triggers a rebuild", so add
// deliberately. Returns "" for a nil config, which callers treat as unknown.
func ConfigFingerprint(cfg *config.NextDeployConfig) string {
	if cfg == nil {
		return ""
	}
	sidecar := cfg.ResolveGoSidecar()
	sidecarEnabled := sidecar != nil && sidecar.Enabled
	isrEnabled := sidecar != nil && sidecar.ISRCacheEnabled()
	imageEnabled := sidecar != nil && sidecar.ImageOptEnabled()
	imageWorkers := 0
	if sidecarEnabled {
		imageWorkers = sidecar.Workers()
	}
	h := sha256.New()
	// NUL separators so "ab"+"c" and "a"+"bc" can't collide; sha256.Hash.Write never errors.
	_, _ = fmt.Fprintf(h, "v2\x00name=%s\x00domain=%s\x00env=%s\x00port=%d\x00target=%s\x00cdn=%t\x00gosidecar=%t\x00isr=%t\x00image=%t\x00imageworkers=%d\x00",
		cfg.App.Name,
		cfg.App.Domain.Name,
		cfg.App.Environment,
		cfg.App.Port,
		cfg.ResolveTargetType(""),
		cfg.App.CDNEnabled,
		sidecarEnabled,
		isrEnabled,
		imageEnabled,
		imageWorkers,
	)
	return hex.EncodeToString(h.Sum(nil))
}

// ValidateBuildState reports whether the previous build can be reused. A nil
// error means "safe to skip". cfg may be nil, which forces a rebuild — the
// caller couldn't prove the config is unchanged, and skipping on an unproven
// key is the bug this guards against.
func ValidateBuildState(cfg *config.NextDeployConfig) error {
	lockPath := filepath.Join(".nextdeploy", "build.lock")
	// #nosec G304
	data, err := os.ReadFile(lockPath)
	if err != nil {
		NextCoreLogger.Error("Failed to read build lock file: %v", err)
		return fmt.Errorf("failed to read build lock: %w", err)
	}

	var lock BuildLock
	if err = json.Unmarshal(data, &lock); err != nil {
		NextCoreLogger.Error("Failed to parse build lock file: %v", err)
		return fmt.Errorf("failed to parse build lock: %w", err)
	}

	currentCommit, err := git.GetCommitHash()
	if err != nil {
		NextCoreLogger.Error("Failed to get current git commit: %v", err)
		return fmt.Errorf("failed to get current git commit: %w", err)
	}
	if currentCommit != lock.GitCommit {
		NextCoreLogger.Error("Git commit mismatch: expected %s, got %s", lock.GitCommit, currentCommit)
		return fmt.Errorf("git commit mismatch: expected %s, got %s", lock.GitCommit, currentCommit)
	}

	if git.IsDirty() && !lock.GitDirty {
		return errors.New("working directory is dirty but build lock expects clean state")
	}

	// The git checks above cover the CODE inputs. The config inputs need their
	// own key: editing app.name in nextdeploy.yml without committing leaves the
	// commit and the dirty flag unchanged, so the build was skipped and the
	// previous tarball — carrying the OLD app name in its metadata.json — was
	// shipped and rejected by the daemon.
	current := ConfigFingerprint(cfg)
	if current == "" || lock.ConfigHash == "" || lock.ConfigHash != current {
		return errors.New("config changed since last build (app, target, cdn, or GoSidecar settings) — rebuilding")
	}

	return nil
}

var assetExtensions = map[string]string{
	// Images
	".jpg":  "image",
	".jpeg": "image",
	".png":  "image",
	".gif":  "image",
	".webp": "image",
	".avif": "image",
	".svg":  "image",
	".ico":  "image",

	// Fonts
	".woff":  "font",
	".woff2": "font",
	".ttf":   "font",
	".otf":   "font",
	".eot":   "font",

	// Stylesheets
	".css": "stylesheet",

	// Scripts
	".js":  "script",
	".mjs": "script",
	".cjs": "script",
	".jsx": "script",
	".ts":  "script",
	".tsx": "script",

	// Documents
	".html": "document",
	".htm":  "document",
	".pdf":  "document",
	".txt":  "document",
	".md":   "document",
	".json": "document",
	".xml":  "document",
}

// ParseStaticAssets scans the project for static assets.
func ParseStaticAssets(projectDir, distDir string) (*StaticAssets, error) {
	assets := &StaticAssets{}

	if distDir == "" {
		distDir = ".next"
	}

	// 1. Scan public directory
	publicDir := filepath.Join(projectDir, "public")
	if _, err := os.Stat(publicDir); err == nil {
		NextCoreLogger.Debug("Scanning public directory: %s", publicDir)
		publicAssets, err := scanDirectory(publicDir, projectDir, "/")
		if err != nil {
			NextCoreLogger.Error("Failed to scan public directory: %v", err)
			return nil, fmt.Errorf("failed to scan public directory: %w", err)
		}
		assets.PublicDir = publicAssets
	}

	// 2. Scan static directory (legacy)
	staticDir := filepath.Join(projectDir, "static")
	if _, err := os.Stat(staticDir); err == nil {
		NextCoreLogger.Debug("Scanning static directory: %s", staticDir)
		staticAssets, err := scanDirectory(staticDir, projectDir, "/static")
		if err != nil {
			NextCoreLogger.Error("Failed to scan static directory: %v", err)
			return nil, fmt.Errorf("failed to scan static directory: %w", err)
		}
		assets.StaticFolder = staticAssets
	}

	// 3. Scan distDir/static directory
	nextStaticDir := filepath.Join(projectDir, distDir, "static")
	if _, err := os.Stat(nextStaticDir); err == nil {
		NextCoreLogger.Debug("Scanning %s/static directory: %s", distDir, nextStaticDir)
		nextStaticAssets, err := scanDirectory(nextStaticDir, projectDir, "/_next/static")
		if err != nil {
			return nil, fmt.Errorf("failed to scan .next/static directory: %w", err)
		}
		assets.NextStatic = nextStaticAssets
	}

	// 4. Scan for other common static assets in root
	rootAssets, err := scanRootAssets(projectDir)
	if err != nil {
		NextCoreLogger.Error("Failed to scan root assets: %v", err)
		return nil, fmt.Errorf("failed to scan root assets: %w", err)
	}
	assets.OtherAssets = rootAssets

	return assets, nil
}

// scanDirectory recursively scans a directory for static assets.
func scanDirectory(dirPath, projectDir, publicPathPrefix string) ([]StaticAsset, error) {
	var assets []StaticAsset

	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			NextCoreLogger.Error("Error accessing path %s: %v", path, err)
			return err
		}

		if !info.IsDir() {
			relPath, err := filepath.Rel(dirPath, path)
			if err != nil {
				NextCoreLogger.Error("Failed to get relative path for %s: %v", path, err)
				return err
			}

			ext := strings.ToLower(filepath.Ext(path))
			assetType := "other"
			if t, ok := assetExtensions[ext]; ok {
				assetType = t
			}

			relProjectPath, err := filepath.Rel(projectDir, path)
			if err != nil {
				return err
			}

			publicPath := filepath.Join(publicPathPrefix, relPath)
			// Convert path separators to forward slashes for URLs
			publicPath = filepath.ToSlash(publicPath)

			assets = append(assets, StaticAsset{
				Path:         relProjectPath,
				AbsolutePath: path,
				PublicPath:   publicPath,
				Type:         assetType,
				Extension:    ext,
				Size:         info.Size(),
			})
		}
		return nil
	})

	return assets, err
}

// scanRootAssets scans for common static files in project root.
func scanRootAssets(projectDir string) ([]StaticAsset, error) {
	var assets []StaticAsset

	rootFiles := []string{
		"favicon.ico",
		"robots.txt",
		"sitemap.xml",
		"manifest.json",
	}

	for _, file := range rootFiles {
		path := filepath.Join(projectDir, file)
		if _, err := os.Stat(path); err == nil {
			info, err := os.Stat(path)
			if err != nil {
				NextCoreLogger.Debug("Failed to get file info for %s: %v", path, err)
				continue
			}

			ext := strings.ToLower(filepath.Ext(path))
			assetType := "other"
			if t, ok := assetExtensions[ext]; ok {
				assetType = t
			}

			assets = append(assets, StaticAsset{
				Path:         file,
				AbsolutePath: path,
				PublicPath:   "/" + file,
				Type:         assetType,
				Extension:    ext,
				Size:         info.Size(),
			})
		}
	}

	return assets, nil
}

// ParseMiddleware parses Next.js middleware.{ts,js} configuration (at root or src/).
// Falls back to proxy.{ts,js} (at root or src/) only when no middleware file exists
// so callers reading payload.Middleware directly remain backward compatible.
func ParseMiddleware(projectDir string) (*MiddlewareConfig, error) {
	return parseInterceptorFiles([]string{
		filepath.Join(projectDir, "middleware.ts"),
		filepath.Join(projectDir, "middleware.js"),
		filepath.Join(projectDir, "src", "middleware.ts"),
		filepath.Join(projectDir, "src", "middleware.js"),
		filepath.Join(projectDir, "proxy.ts"),
		filepath.Join(projectDir, "proxy.js"),
		filepath.Join(projectDir, "src", "proxy.ts"),
		filepath.Join(projectDir, "src", "proxy.js"),
	})
}

// ParseProxy parses Next 16+ proxy.{ts,js} configuration independently (at root or src/).
func ParseProxy(projectDir string) (*MiddlewareConfig, error) {
	return parseInterceptorFiles([]string{
		filepath.Join(projectDir, "proxy.ts"),
		filepath.Join(projectDir, "proxy.js"),
		filepath.Join(projectDir, "src", "proxy.ts"),
		filepath.Join(projectDir, "src", "proxy.js"),
	})
}

func parseInterceptorFiles(candidates []string) (*MiddlewareConfig, error) {
	var found string
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			found = p
			break
		}
	}
	if found == "" {
		return nil, nil
	}

	content, err := os.ReadFile(found) // #nosec G304
	if err != nil {
		NextCoreLogger.Error("Failed to read interceptor file %s: %v", found, err)
		return nil, fmt.Errorf("failed to read %s: %w", filepath.Base(found), err)
	}

	matchers, err := parseMiddlewareMatchers(string(content))
	if err != nil {
		return nil, fmt.Errorf("failed to parse matchers in %s: %w", filepath.Base(found), err)
	}

	cfg := &MiddlewareConfig{
		Path:     found,
		Matchers: matchers,
		Runtime:  "nodejs",
	}
	if strings.Contains(string(content), "runtime: 'edge'") || strings.Contains(string(content), `runtime: "edge"`) {
		cfg.Runtime = "edge"
	}
	if regions := parseEdgeRegions(string(content)); len(regions) > 0 {
		cfg.Regions = regions
	}
	if flag := parseUnstableFlag(string(content)); flag != "" {
		cfg.UnstableFlag = flag
	}
	return cfg, nil
}

// parseMiddlewareMatchers extracts route matchers from middleware file.
func parseMiddlewareMatchers(content string) ([]MiddlewareRoute, error) {
	if matchers, ok := parseConfigObjectMatchers(content); ok {
		return matchers, nil
	}
	if matchers, ok := parseInlineMatcher(content); ok {
		return matchers, nil
	}
	return parseSimplePathMatchers(content), nil
}

func parseConfigObjectMatchers(content string) ([]MiddlewareRoute, bool) {
	configObjRegex := regexp.MustCompile(`(?:export\s+const\s+)?config\s*=\s*{([^}]*)}`)
	configMatches := configObjRegex.FindStringSubmatch(content)
	if len(configMatches) <= 1 {
		return nil, false
	}

	cleaned := normalizeJSObjectToJSON(configMatches[1])
	var cfg struct {
		Matcher json.RawMessage `json:"matcher"`
	}
	if err := json.Unmarshal([]byte(cleaned), &cfg); err != nil {
		return nil, false
	}

	var paths []string
	if err := json.Unmarshal(cfg.Matcher, &paths); err == nil {
		matchers := make([]MiddlewareRoute, 0, len(paths))
		for _, path := range paths {
			matchers = append(matchers, MiddlewareRoute{Pathname: path})
		}
		return matchers, true
	}

	var routes []MiddlewareRoute
	if err := json.Unmarshal(cfg.Matcher, &routes); err == nil {
		return routes, true
	}
	return nil, false
}

func normalizeJSObjectToJSON(body string) string {
	cleaned := strings.ReplaceAll(body, "'", `"`)
	keyRegex := regexp.MustCompile(`(\s*)([a-zA-Z0-9_]+):`)
	cleaned = keyRegex.ReplaceAllString(cleaned, `$1"$2":`)
	trailingCommaRegex := regexp.MustCompile(`,(\s*[}\]])`)
	cleaned = trailingCommaRegex.ReplaceAllString(cleaned, `$1`)
	cleaned = strings.ReplaceAll(cleaned, "\n", "")
	cleaned = fmt.Sprintf("{%s}", cleaned)
	return trailingCommaRegex.ReplaceAllString(cleaned, `$1`)
}

func parseInlineMatcher(content string) ([]MiddlewareRoute, bool) {
	matcherRegex := regexp.MustCompile(`matcher:\s*(\[[^\]]+\]|{[^}]+}|"[^"]+"|'[^']+')`)
	matcherMatches := matcherRegex.FindStringSubmatch(content)
	if len(matcherMatches) <= 1 {
		return nil, false
	}

	cleaned := strings.ReplaceAll(matcherMatches[1], "'", `"`)
	cleaned = strings.ReplaceAll(cleaned, "\n", "")

	switch {
	case strings.HasPrefix(cleaned, "["):
		var paths []string
		if err := json.Unmarshal([]byte(cleaned), &paths); err == nil {
			matchers := make([]MiddlewareRoute, 0, len(paths))
			for _, path := range paths {
				matchers = append(matchers, MiddlewareRoute{Pathname: path})
			}
			return matchers, true
		}
	case strings.HasPrefix(cleaned, `"`):
		return []MiddlewareRoute{{Pathname: strings.Trim(cleaned, `"`)}}, true
	case strings.HasPrefix(cleaned, "{"):
		var matcher struct {
			Pathname string `json:"pathname"`
			Pattern  string `json:"pattern"`
		}
		if err := json.Unmarshal([]byte(cleaned), &matcher); err == nil {
			return []MiddlewareRoute{{
				Pathname: matcher.Pathname,
				Pattern:  matcher.Pattern,
			}}, true
		}
	}
	return nil, false
}

func parseSimplePathMatchers(content string) []MiddlewareRoute {
	var matchers []MiddlewareRoute
	pathRegex := regexp.MustCompile(`path:\s*['"]([^'"]+)['"]`)
	for _, match := range pathRegex.FindAllStringSubmatch(content, -1) {
		if len(match) > 1 {
			matchers = append(matchers, MiddlewareRoute{Pathname: match[1]})
		}
	}
	return matchers
}

// parseUnstableFlag extracts unstable configuration flags.
func parseUnstableFlag(content string) string {
	flagRegex := regexp.MustCompile(`unstable_(\w+):\s*true`)
	matches := flagRegex.FindStringSubmatch(content)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

func detectImageAssets(buildMeta *NextBuildMetadata, projectDir, distDir string) (*ImageAssets, error) {
	assets := &ImageAssets{}
	var err error

	publicDir := filepath.Join(projectDir, PublicDir)
	assets.PublicImages, err = findPublicImages(publicDir, projectDir)
	if err != nil {
		NextCoreLogger.Error("Failed to find public images: %v", err)
		return nil, err
	}

	if buildMeta.ImagesManifest != nil {
		if imagesManifest, ok := buildMeta.ImagesManifest.(map[string]any); ok {
			manifestImages := parseImagesManifest(imagesManifest, projectDir, distDir)
			assets.OptimizedImages = append(assets.OptimizedImages, manifestImages...)
		}
	}

	if buildMeta.BuildManifest != nil {
		if buildManifest, ok := buildMeta.BuildManifest.(map[string]any); ok {
			assets.StaticImports = parseStaticImageImports(buildManifest, projectDir)
		}
	}

	return assets, nil
}

func findPublicImages(publicDir, projectDir string) ([]ImageAsset, error) {
	_ = projectDir // retained for signature parity; paths are public-relative
	var images []ImageAsset

	err := filepath.Walk(publicDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if assetExtensions[ext] != "image" {
			return nil
		}
		relPath, err := filepath.Rel(publicDir, path)
		if err != nil {
			return err
		}
		images = append(images, ImageAsset{
			Path:         relPath,
			AbsolutePath: path,
			PublicPath:   "/" + filepath.ToSlash(relPath),
			Format:       strings.TrimPrefix(ext, "."),
			IsOptimized:  false,
		})
		return nil
	})

	return images, err
}

func parseImagesManifest(manifest map[string]any, projectDir, distDir string) []ImageAsset {
	if distDir == "" {
		distDir = ".next"
	}
	var images []ImageAsset

	imagesMap, ok := manifest["images"].(map[string]any)
	if !ok {
		return images
	}

	for _, img := range imagesMap {
		imgMap, ok := img.(map[string]any)
		if !ok {
			continue
		}
		path, _ := imgMap["path"].(string)
		format, _ := imgMap["format"].(string)

		asset := ImageAsset{
			Path:         path,
			AbsolutePath: filepath.Join(projectDir, distDir, "server", path),
			PublicPath:   "/_next/image?url=" + path + "&w=3840&q=75", // Example URL
			Format:       format,
			IsOptimized:  true,
		}

		if width, ok := imgMap["width"].(float64); ok {
			asset.Width = int(width)
		}
		if height, ok := imgMap["height"].(float64); ok {
			asset.Height = int(height)
		}

		images = append(images, asset)
	}

	return images
}

func parseStaticImageImports(buildManifest map[string]any, projectDir string) []ImageAsset {
	var images []ImageAsset

	files, ok := buildManifest["staticImageImports"].(map[string]any)
	if !ok {
		return images
	}

	for path, data := range files {
		dataMap, ok := data.(map[string]any)
		if !ok {
			continue
		}
		format := strings.TrimPrefix(filepath.Ext(path), ".")

		asset := ImageAsset{
			Path:           path,
			AbsolutePath:   filepath.Join(projectDir, path),
			PublicPath:     path, // This will be hashed in the actual build
			Format:         format,
			IsOptimized:    true,
			IsStaticImport: true,
		}

		if width, ok := dataMap["width"].(float64); ok {
			asset.Width = int(width)
		}
		if height, ok := dataMap["height"].(float64); ok {
			asset.Height = int(height)
		}

		images = append(images, asset)
	}

	return images
}

func buildCommand(packageManager string) (string, error) {
	if packageManager == "" {
		packageManager = "npm" // default to npm if not specified
	}

	switch packageManager {
	case "npm":
		return "npm run build", nil
	case "yarn":
		return "yarn build", nil
	case "pnpm":
		return "pnpm run build", nil
	case "bun":
		return "bun run build", nil
	default:
		return "npm run build", fmt.Errorf("unsupported package manager: %s", packageManager)
	}
}
