package nextcore

import (
	"encoding/json"
	"strconv"
	"errors"
	"fmt"
	"github.com/robertkrimen/otto"
	"io"
	"maps"
	"nextdeploy/shared"
	"nextdeploy/shared/config"

	"nextdeploy/shared/git"
	"regexp"

	"time"

	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	BuildLockFileName = ".nextdeploy/build.lock"
	MetadataFileName  = ".nextdeploy/metadata.json"
	AssetsOutputDir   = ".nextdeploy/assets"
	PublicDir         = "public"
)

var (
	NextCoreLogger = shared.PackageLogger("nextcore", "📦 NEXTCORE")
)

// TODO: add temporal workflow context for metadata ingeestion and usage pipelines
func GenerateMetadata() error {
	// This function will generate metadata for the Next.js application
	// and return a NextCorePayload with the necessary fields filled.
	NextCoreLogger.Info("Generating metadata for Next.js application...")
	//Get the app name
	cfg, err := config.Load()
	if err != nil {
		NextCoreLogger.Error("Failed to load configuration: %v", err)
		return err
	}

	AppName := cfg.App.Name

	// Get the nextjs version
	NextJsVersion, err := GetNextJsVersion("package.json")
	if err != nil {
		NextCoreLogger.Error("Failed to get Next.js version: %v", err)
		return err
	}
	NextCoreLogger.Info("Next.js version: %s", NextJsVersion)
	// get the build meta data
	NextCoreLogger.Info("Collecting build metadata...")
	buildMeta, err := CollectBuildMetadata()
	if err != nil {
		NextCoreLogger.Error("Failed to collect build metadata: %v", err)
		return err
	}
	NextCoreLogger.Debug("The build metadata looks like this:%v", buildMeta)
	// add config data to the metadata also
	config, err := config.Load()
	if err != nil {
		NextCoreLogger.Error("Failed to load configuration: %v", err)
		return err
	}
	// static_routes := []string{}
	routeInfo, err := getRoutesFromManifests(buildMeta)
	if err != nil {
		return err
	}
	cwd, err := os.Getwd()
	packageManager, err := DetectPackageManager(cwd)
	buildCommand, err := buildCommand(packageManager.String())

	startCommand, err := startCommand(packageManager.String())
	if err != nil {
		NextCoreLogger.Error("Failed to get start command: %v", err)
		return err
	}
	imagesAssets, err := detectImageAssets(buildMeta, cwd)
	var HasImageAssets bool
	if err != nil {
		NextCoreLogger.Error("Failed to detect image assets: %v", err)
		return err
	}
	if imagesAssets == nil {
		NextCoreLogger.Info("No image assets found in the Next.js build")
	} else {
		HasImageAssets = true
		NextCoreLogger.Debug("Image assets detected: %v", imagesAssets)
	}

	nextconfig, err := ParseNextConfig(cwd)
	if err != nil {
		NextCoreLogger.Error("failed to get parse nextconfig ")
		return err
	}

	domainName := config.App.Domain
	middlewareConfig, err := ParseMiddleware(cwd)

	if err != nil {
		NextCoreLogger.Error("Failed to parse middleware configuration: %v", err)
		return err
	}

	StaticAssets, err := ParseStaticAssets(cwd)
	if err != nil {
		NextCoreLogger.Error("Failed to parse static assets: %v", err)
		return err
	}

	gitCommt, err := git.GetCommitHash()
	if err != nil {
		NextCoreLogger.Error("Failed to get git commit hash: %v", err)
		return err
	} else {
		NextCoreLogger.Debug("Git commit hash: %s", gitCommt)
	}
	gitDiry := git.IsDirty()

	PayloadPath, err := filepath.Abs(filepath.Join(cwd, MetadataFileName))
	buildLockPath, err := filepath.Abs(filepath.Join(cwd, BuildLockFileName))
	AssetsOutputDir, err := filepath.Abs(filepath.Join(cwd, AssetsOutputDir))
	// 4. Copy static assets
	if err := copyStaticAssets(); err != nil {
		NextCoreLogger.Error("Failed to copy static assets: %v", err)
		return fmt.Errorf("failed to copy static assets: %w", err)
	}

	// 4. Track git state
	metadata := NextCorePayload{
		AppName:           AppName,
		NextVersion:       NextJsVersion,
		NextBuildMetadata: *buildMeta,
		Config:            config,
		BuildCommand:      buildCommand.String(),
		StartCommand:      startCommand,
		HasImageAssets:    HasImageAssets,
		NextConfig:        nextconfig,
		CDNEnabled:        false,
		Domain:            domainName,
		RouteInfo:         *routeInfo,
		Middleware:        middlewareConfig,
		StaticAssets:      StaticAssets,
		GitCommit:         gitCommt,
		GitDirty:          gitDiry,
		GeneratedAt:       time.Now().Format(time.RFC3339),
		MetadataFilePath:  PayloadPath,
		BuildLockFile:     buildLockPath,
		AssetsOutputDir:   AssetsOutputDir,
	}

	if err := createBuildLock(&metadata); err != nil {
		NextCoreLogger.Error("Failed to create build lock: %v", err)
		return fmt.Errorf("failed to create build lock: %w", err)
	}

	return nil
}

func copyStaticAssets() error {
	srcDir := "public"
	dstDir := filepath.Join(".nextdeploy", "assets")

	// Create destination directory
	if err := os.MkdirAll(dstDir, 0755); err != nil {
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
		if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
			NextCoreLogger.Error("Failed to create directory for %s: %v", dstPath, err)
			return err
		}

		// Copy file
		return copyFile(path, dstPath)
	})
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	source, err := os.Open(src)
	if err != nil {
		NextCoreLogger.Error("Failed to open source file %s: %v", src, err)
		return err
	}
	defer source.Close()

	destination, err := os.Create(dst)
	if err != nil {
		NextCoreLogger.Error("Failed to create destination file %s: %v", dst, err)
		return err
	}
	defer destination.Close()

	_, err = io.Copy(destination, source)
	return err
}

// createBuildLock creates the build.lock file with git state
func createBuildLock(metadata *NextCorePayload) error {
	commitHash, err := git.GetCommitHash()
	if err != nil {
		NextCoreLogger.Error("Failed to get git commit hash: %v", err)
		return fmt.Errorf("failed to get git commit hash: %w", err)
	}

	dirty := git.IsDirty()
	// Write metadata to json file
	fileName := ".nextdeploy/metadata.json"
	marshalledData, err := json.MarshalIndent(metadata, "", "  ")
	os.WriteFile(fileName, marshalledData, 0644)

	buildLock := BuildLock{
		GitCommit:   commitHash,
		GitDirty:    dirty,
		GeneratedAt: metadata.GeneratedAt,
		Metadata:    fileName,
	}

	data, err := json.MarshalIndent(buildLock, "", "  ")
	if err != nil {
		NextCoreLogger.Error("Failed to marshal build lock: %v", err)
		return err
	}

	return os.WriteFile(filepath.Join(".nextdeploy", "build.lock"), data, 0644)
}

// getPublicEnvVars collects NEXT_PUBLIC_* environment variables
func getPublicEnvVars() map[string]string {
	vars := make(map[string]string)
	for _, env := range os.Environ() {
		if strings.HasPrefix(env, "NEXT_PUBLIC_") {
			parts := strings.SplitN(env, "=", 2)
			if len(parts) == 2 {
				vars[parts[0]] = parts[1]
			}
		}
	}
	return vars
}

// ValidateBuildState checks if the current git state matches the build lock
func ValidateBuildState() error {
	lockPath := filepath.Join(".nextdeploy", "build.lock")
	data, err := os.ReadFile(lockPath)
	if err != nil {
		NextCoreLogger.Error("Failed to read build lock file: %v", err)
		return fmt.Errorf("failed to read build lock: %w", err)
	}

	var lock BuildLock
	if err := json.Unmarshal(data, &lock); err != nil {
		NextCoreLogger.Error("Failed to parse build lock file: %v", err)
		return fmt.Errorf("failed to parse build lock: %w", err)
	}

	currentCommit, err := git.GetCommitHash()
	if err != nil {
		NextCoreLogger.Error("Failed to get current git commit: %v", err)
		return fmt.Errorf("failed to get current git commit: %w", err)
	}
	//TODO: use this data to avoid unnecessary builds
	if currentCommit != lock.GitCommit {
		NextCoreLogger.Error("Git commit mismatch: expected %s, got %s", lock.GitCommit, currentCommit)
		return fmt.Errorf("git commit mismatch: expected %s, got %s", lock.GitCommit, currentCommit)
	}

	if git.IsDirty() && !lock.GitDirty {
		return errors.New("working directory is dirty but build lock expects clean state")
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

// ParseStaticAssets scans the project for static assets
func ParseStaticAssets(projectDir string) (*StaticAssets, error) {
	assets := &StaticAssets{}

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

	// 3. Scan .next/static directory
	nextStaticDir := filepath.Join(projectDir, ".next", "static")
	if _, err := os.Stat(nextStaticDir); err == nil {
		NextCoreLogger.Debug("Scanning .next/static directory: %s", nextStaticDir)
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

// scanDirectory recursively scans a directory for static assets
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

// scanRootAssets scans for common static files in project root
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

// ParseMiddleware parses Next.js middleware configuration
func ParseMiddleware(projectDir string) (*MiddlewareConfig, error) {
	config := &MiddlewareConfig{
		Path:     filepath.Join(projectDir, "middleware.js"),
		Matchers: []MiddlewareRoute{},
		Runtime:  "nodejs", // Default runtime
	}

	// Check for middleware.ts first, then middleware.js
	middlewarePaths := []string{
		filepath.Join(projectDir, "middleware.ts"),
		filepath.Join(projectDir, "middleware.js"),
	}

	var middlewareFile string
	for _, path := range middlewarePaths {
		if _, err := os.Stat(path); err == nil {
			middlewareFile = path
			config.Path = path
			break
		}
	}

	if middlewareFile == "" {
		return nil, nil // No middleware file found
	}

	// Read middleware file content
	content, err := os.ReadFile(middlewareFile)
	if err != nil {
		NextCoreLogger.Error("Failed to read middleware file: %v", err)
		return nil, fmt.Errorf("failed to read middleware file: %w", err)
	}

	// Parse middleware matchers
	matchers, err := parseMiddlewareMatchers(string(content))
	if err != nil {
		return nil, fmt.Errorf("failed to parse middleware matchers: %w", err)
	}
	config.Matchers = matchers

	// Check for Edge runtime
	if strings.Contains(string(content), "runtime: 'edge'") {
		config.Runtime = "edge"
	}

	// Check for regions configuration
	if regions := parseEdgeRegions(string(content)); len(regions) > 0 {
		config.Regions = regions
	}

	// Check for unstable flags
	if flag := parseUnstableFlag(string(content)); flag != "" {
		config.UnstableFlag = flag
	}

	return config, nil
}

// parseMiddlewareMatchers extracts route matchers from middleware file
func parseMiddlewareMatchers(content string) ([]MiddlewareRoute, error) {
	var matchers []MiddlewareRoute

	// First try to parse config object style
	configObjRegex := regexp.MustCompile(`config\s*=\s*{([^}]*)}`)
	configMatches := configObjRegex.FindStringSubmatch(content)
	if len(configMatches) > 1 {
		// Try to parse as JSON (with some cleaning)
		cleaned := strings.ReplaceAll(configMatches[1], "'", `"`)
		cleaned = strings.ReplaceAll(cleaned, "\n", "")
		cleaned = fmt.Sprintf("{%s}", cleaned)

		var config struct {
			Matcher []struct {
				Pathname string `json:"pathname"`
				Pattern  string `json:"pattern"`
				Has      []struct {
					Type  string `json:"type"`
					Key   string `json:"key"`
					Value string `json:"value"`
				} `json:"has"`
				Missing []struct {
					Type  string `json:"type"`
					Key   string `json:"key"`
					Value string `json:"value"`
				} `json:"missing"`
			} `json:"matcher"`
		}

		if err := json.Unmarshal([]byte(cleaned), &config); err == nil {
			for _, m := range config.Matcher {
				route := MiddlewareRoute{
					Pathname: m.Pathname,
					Pattern:  m.Pattern,
				}

				for _, h := range m.Has {
					route.Has = append(route.Has, MiddlewareCondition{
						Type:  h.Type,
						Key:   h.Key,
						Value: h.Value,
					})
				}

				for _, miss := range m.Missing {
					route.Missing = append(route.Missing, MiddlewareCondition{
						Type:  miss.Type,
						Key:   miss.Key,
						Value: miss.Value,
					})
				}

				matchers = append(matchers, route)
			}
			return matchers, nil
		}
	}

	// Fallback to parsing individual matchers
	matcherRegex := regexp.MustCompile(`matcher:\s*(\[[^\]]+\]|{[^}]+})`)
	matcherMatches := matcherRegex.FindStringSubmatch(content)
	if len(matcherMatches) > 1 {
		cleaned := strings.ReplaceAll(matcherMatches[1], "'", `"`)
		cleaned = strings.ReplaceAll(cleaned, "\n", "")

		// Handle array of paths
		if strings.HasPrefix(cleaned, "[") {
			var paths []string
			if err := json.Unmarshal([]byte(cleaned), &paths); err == nil {
				for _, path := range paths {
					matchers = append(matchers, MiddlewareRoute{
						Pathname: path,
					})
				}
				return matchers, nil
			}
		}

		// Handle object matcher
		if strings.HasPrefix(cleaned, "{") {
			var matcher struct {
				Pathname string `json:"pathname"`
				Pattern  string `json:"pattern"`
			}
			if err := json.Unmarshal([]byte(cleaned), &matcher); err == nil {
				matchers = append(matchers, MiddlewareRoute{
					Pathname: matcher.Pathname,
					Pattern:  matcher.Pattern,
				})
				return matchers, nil
			}
		}
	}

	// Fallback to parsing simple path strings
	pathRegex := regexp.MustCompile(`path:\s*['"]([^'"]+)['"]`)
	pathMatches := pathRegex.FindAllStringSubmatch(content, -1)
	for _, match := range pathMatches {
		if len(match) > 1 {
			matchers = append(matchers, MiddlewareRoute{
				Pathname: match[1],
			})
		}
	}

	return matchers, nil
}

// parseEdgeRegions extracts regions from Edge runtime configuration
func parseEdgeRegions(content string) []string {
	regionsRegex := regexp.MustCompile(`regions:\s*(\[[^\]]+\])`)
	matches := regionsRegex.FindStringSubmatch(content)
	if len(matches) > 1 {
		cleaned := strings.ReplaceAll(matches[1], "'", `"`)
		var regions []string
		if err := json.Unmarshal([]byte(cleaned), &regions); err == nil {
			return regions
		}
	}
	return nil
}

// parseUnstableFlag extracts unstable configuration flags
func parseUnstableFlag(content string) string {
	flagRegex := regexp.MustCompile(`unstable_(\w+):\s*true`)
	matches := flagRegex.FindStringSubmatch(content)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}
func ParseNextConfig(projectDir string) (*NextConfig, error) {
	configPaths := []string{
		filepath.Join(projectDir, "next.config.ts"),
		filepath.Join(projectDir, "next.config.js"),
		filepath.Join(projectDir, "next.config.mjs"),
		filepath.Join(projectDir, "next.config.cjs"),
	}

	var configFile string
	for _, path := range configPaths {
		if _, err := os.Stat(path); err == nil {
			NextCoreLogger.Error("Found Next.js config file: %s", path)
			configFile = path
			break
		}
	}

	if configFile == "" {
		return &NextConfig{}, nil // Return empty config if no file found
	}

	content, err := os.ReadFile(configFile)
	if err != nil {
		NextCoreLogger.Error("Failed to read config file: %v", err)
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Extract the configuration object from the file
	configObj, err := extractConfigObject(string(content), filepath.Ext(configFile))
	if err != nil {
		NextCoreLogger.Error("Failed to extract config object: %v", err)
		return nil, fmt.Errorf("failed to extract config: %w", err)
	}

	// Parse the configuration into our struct
	nextConfig, err := parseConfigObject(configObj)
	if err != nil {
		NextCoreLogger.Error("Failed to parse config object: %v", err)
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	return nextConfig, nil
}
func extractNextConfig(jsContent string) (map[string]interface{}, error) {
	// Step 1: Extract the nextConfig object from JS code
	configObj, err := extractJSObject(jsContent, "nextConfig")
	if err != nil {
		return nil, err
	}

	// Step 2: Convert to JSON (handling JS-specific syntax)
	jsonStr := jsToJSON(configObj)

	// Step 3: Parse into Go map
	config := make(map[string]interface{})
	err = json.Unmarshal([]byte(jsonStr), &config)
	if err != nil {
		return nil, err
	}

	// Step 4: Handle special cases (like functions)
	handleSpecialCases(config, jsContent)

	return config, nil
}
func extractJSObject(jsContent, objectName string) (string, error) {
	// Simple regex approach - for production use a proper JS parser
	pattern := regexp.MustCompile(`const\s+` + objectName + `\s*=\s*({[\s\S]*?})\s*;`)
	matches := pattern.FindStringSubmatch(jsContent)
	if len(matches) < 2 {
		return "", fmt.Errorf("could not find %s object", objectName)
	}
	return matches[1], nil
}

func jsToJSON(js string) string {
	// Convert JavaScript object to JSON format
	// This is a simplified version - you'd need more robust handling

	// Remove trailing commas
	re := regexp.MustCompile(`,\s*([}\]])`)
	js = re.ReplaceAllString(js, "$1")

	// Convert single quotes to double quotes
	js = strings.ReplaceAll(js, `'`, `"`)

	// Remove JS comments
	js = regexp.MustCompile(`/\*.*?\*/`).ReplaceAllString(js, "")
	js = regexp.MustCompile(`//.*`).ReplaceAllString(js, "")

	return js
}
func handleSpecialCases(config map[string]interface{}, jsContent string) {
	// Handle webpack function
	if webpackConfig, exists := config["webpack"]; exists {
		if webpackStr, ok := webpackConfig.(string); ok {
			// Extract function body
			if strings.Contains(webpackStr, "function") || strings.Contains(webpackStr, "=>") {
				config["webpack"] = map[string]interface{}{
					"__type__": "function",
					"body":     extractFunctionBody(webpackStr),
				}
			}
		}
	}

	// Add any other special case handling here
}
func extractFunctionBody(funcStr string) string {
	// Extract the body of a function
	// This is simplified - would need better parsing for production
	start := strings.Index(funcStr, "{")
	end := strings.LastIndex(funcStr, "}")
	if start == -1 || end == -1 {
		return funcStr
	}
	return strings.TrimSpace(funcStr[start+1 : end])
}

// extractConfigObject extracts the configuration object from the config file
func extractConfigObject(content string, ext string) (map[string]interface{}, error) {
	// For TypeScript files, we need to transpile first
	// TODO: write logic to extrac config data in key value pattern
	NextCoreLogger.Debug("Extracting config object from content: %s", content)
	config := make(map[string]interface{})
	// Use Otto to evaluate the JS content and extract the config object
	if ext == "ts" {
		config, err := extractConfig(content, ext)
		if err != nil {
			NextCoreLogger.Error("Failed to extract config from TypeScript: %v", err)
			return nil, fmt.Errorf("failed to extract config: %w", err)
		}
		NextCoreLogger.Debug("Extracted config from TypeScript: %v", config)
	}
	// If it's a JavaScript file, we can directly evaluate it
	if ext == "js" || ext == "mjs" || ext == "cjs" {
		config, err := extractConfig(content, ext)
		if err != nil {
			NextCoreLogger.Error("Failed to extract config from JavaScript: %v", err)
			return nil, fmt.Errorf("failed to extract config: %w", err)
		}
		NextCoreLogger.Debug("Extracted config from JavaScript: %v", config)
	}

	// Create a new JavaScript VM
	return config, nil
}
func extractConfig(content string, ext string) (map[string]interface{}, error) {
	config := make(map[string]interface{})

	// handle ts files
	if ext == ".ts" {
		content = transpileTypeScriptConfig(content)
	} else {
		content = strings.TrimSpace(content)
	}

	// try js evaluation firstt
	if err := extractWithOtto(content, &config); err != nil {
		NextCoreLogger.Error("Failed to extract config with Otto: %v", err)
		return config, fmt.Errorf("failed to extract config: %w", err)
	}
	return config, nil
}

func extractWithOtto(content string, config *map[string]interface{}) error {
	vm := otto.New()
	if _, err := vm.Run(content); err != nil {
		NextCoreLogger.Error("Failed to run JS content in Otto: %v", err)
		return fmt.Errorf("failed to run JS content: %w", err)
	}
	exportPatterns := []string{
		"module.exports",
		"exports",
		"(typeof exports === 'object' && typeof module === 'object') ? module.exports : exports.default || exports",
		"(function() { try { return config || settings || cfg || configuration; } catch(e) { return undefined; } })()",
	}
	for _, pattern := range exportPatterns {
		if value, err := vm.Run(pattern); err == nil && !value.IsUndefined() {
			if exported, err := value.Export(); err == nil {
				if exportedMap, ok := exported.(map[string]interface{}); ok {
					for k, v := range exportedMap {
						maps.Copy(*config, map[string]interface{}{
							k: v,
						},
						)
						return nil
					}
				}
			}
		}

		return fmt.Errorf("no valid export found in JS content")
	}
	return fmt.Errorf("failed to extract config object from JS content")
}

// transpileTypeScriptConfig does a simple TS-to-JS conversion for config files
func transpileTypeScriptConfig(content string) string {
	// Remove TypeScript type annotations
	re := regexp.MustCompile(`:\s*\w+\s*([,;}])`)
	content = re.ReplaceAllString(content, "$1")

	// Remove interface/type declarations
	re = regexp.MustCompile(`(?m)^\s*(export\s+)?(interface|type)\s+\w+\s*({[^}]*}|=.*)?\s*$`)
	content = re.ReplaceAllString(content, "")

	// Convert export default to module.exports
	re = regexp.MustCompile(`export\s+default`)
	content = re.ReplaceAllString(content, "module.exports =")

	return strings.TrimSpace(content)
}
func extractWithRegex(content string, config map[string]interface{}) {
	// Match key: value pairs
	re := regexp.MustCompile(`(?m)^\s*(?:export\s+|const\s+|let\s+|var\s+)?(\w+)\s*[:=]\s*([^;\n]+);?$`)
	matches := re.FindAllStringSubmatch(content, -1)

	for _, match := range matches {
		if len(match) >= 3 {
			key := strings.TrimSpace(match[1])
			value := strings.TrimSpace(match[2])

			// Remove surrounding quotes
			value = strings.Trim(value, `'"`)

			// Type detection
			switch {
			case value == "true":
				config[key] = true
			case value == "false":
				config[key] = false
			case strings.HasPrefix(value, `'`) || strings.HasPrefix(value, `"`):
				config[key] = strings.Trim(value, `'"`)
			default:
				if num, err := strconv.ParseFloat(value, 64); err == nil {
					if strings.Contains(value, ".") {
						config[key] = num
					} else {
						config[key] = int(num)
					}
				} else {
					config[key] = value
				}
			}
		}
	}
}

func printConfig(config map[string]interface{}) {
	for k, v := range config {
		fmt.Printf("  %s: %v (%T)\n", k, v, v)
	}
}

// parseConfigObject converts the raw config map to our structured NextConfig
func parseConfigObject(config map[string]interface{}) (*NextConfig, error) {
	result := &NextConfig{
		Compiler:            make(map[string]interface{}),
		Experimental:        make(map[string]interface{}),
		Env:                 make(map[string]string),
		PublicRuntimeConfig: make(map[string]interface{}),
		ServerRuntimeConfig: make(map[string]interface{}),
	}

	// Helper function to get string value
	getString := func(key string) string {
		if val, ok := config[key].(string); ok {
			return val
		}
		return ""
	}

	// Helper function to get bool value
	getBool := func(key string) bool {
		if val, ok := config[key].(bool); ok {
			return val
		}
		return false
	}

	// Parse basic configuration
	result.BasePath = getString("basePath")
	result.Output = getString("output")
	result.ReactStrictMode = getBool("reactStrictMode")
	result.PoweredByHeader = getBool("poweredByHeader")
	result.TrailingSlash = getBool("trailingSlash")

	// Parse images configuration
	if images, ok := config["images"].(map[string]interface{}); ok {
		result.Images = parseImageConfig(images)
	}

	// Parse compiler configuration
	if compiler, ok := config["compiler"].(map[string]interface{}); ok {
		result.Compiler = compiler
	}

	// Parse experimental features
	if experimental, ok := config["experimental"].(map[string]interface{}); ok {
		result.Experimental = experimental
	}

	// Parse webpack configuration
	if webpack, ok := config["webpack"]; ok {
		result.Webpack = webpack
	}

	// Parse headers, redirects, rewrites
	if headers, ok := config["headers"].([]interface{}); ok {
		result.Headers = headers
	}
	if redirects, ok := config["redirects"].([]interface{}); ok {
		result.Redirects = redirects
	}
	if rewrites, ok := config["rewrites"].([]interface{}); ok {
		result.Rewrites = rewrites
	}

	// Parse environment variables
	if env, ok := config["env"].(map[string]interface{}); ok {
		for k, v := range env {
			if s, ok := v.(string); ok {
				result.Env[k] = s
			}
		}
	}

	// Parse runtime configs
	if publicRuntimeConfig, ok := config["publicRuntimeConfig"].(map[string]interface{}); ok {
		result.PublicRuntimeConfig = publicRuntimeConfig
	}
	if serverRuntimeConfig, ok := config["serverRuntimeConfig"].(map[string]interface{}); ok {
		result.ServerRuntimeConfig = serverRuntimeConfig
	}

	return result, nil
}

// parseImageConfig parses the images configuration
func parseImageConfig(images map[string]interface{}) ImageConfig {
	result := ImageConfig{
		Loader: "default",
	}

	if domains, ok := images["domains"].([]interface{}); ok {
		for _, d := range domains {
			if s, ok := d.(string); ok {
				result.Domains = append(result.Domains, s)
			}
		}
	}

	if formats, ok := images["formats"].([]interface{}); ok {
		for _, f := range formats {
			if s, ok := f.(string); ok {
				result.Formats = append(result.Formats, s)
			}
		}
	}

	if deviceSizes, ok := images["deviceSizes"].([]interface{}); ok {
		for _, s := range deviceSizes {
			if n, ok := s.(float64); ok {
				result.DeviceSizes = append(result.DeviceSizes, int(n))
			}
		}
	}

	if imageSizes, ok := images["imageSizes"].([]interface{}); ok {
		for _, s := range imageSizes {
			if n, ok := s.(float64); ok {
				result.ImageSizes = append(result.ImageSizes, int(n))
			}
		}
	}

	if loader, ok := images["loader"].(string); ok {
		result.Loader = loader
	}

	if path, ok := images["path"].(string); ok {
		result.Path = path
	}

	if ttl, ok := images["minimumCacheTTL"].(float64); ok {
		result.MinimumCacheTTL = int(ttl)
	}

	if unoptimized, ok := images["unoptimized"].(bool); ok {
		result.Unoptimized = unoptimized
	}

	return result
}

// detectImageAssets finds all image assets in the Next.js build
func detectImageAssets(buildMeta *NextBuildMetadata, projectDir string) (*ImageAssets, error) {
	assets := &ImageAssets{}
	var err error

	// 1. Find images in public directory
	publicDir := filepath.Join(projectDir, PublicDir)
	assets.PublicImages, err = findPublicImages(publicDir, projectDir)
	if err != nil {
		NextCoreLogger.Error("Failed to find public images: %v", err)
		return nil, err
	}

	// 2. Find optimized images from Next.js image manifest
	if buildMeta.ImagesManifest != nil {
		if imagesManifest, ok := buildMeta.ImagesManifest.(map[string]interface{}); ok {
			assets.OptimizedImages = parseImagesManifest(imagesManifest, projectDir)
		}
	}

	// 3. Find static image imports from build manifest
	if buildMeta.BuildManifest != nil {
		if buildManifest, ok := buildMeta.BuildManifest.(map[string]interface{}); ok {
			assets.StaticImports = parseStaticImageImports(buildManifest, projectDir)
		}
	}

	return assets, nil
}

// findPublicImages scans the public directory for image assets
func findPublicImages(publicDir, projectDir string) ([]ImageAsset, error) {
	var images []ImageAsset

	// Supported image extensions
	imageExts := map[string]bool{
		".jpg":  true,
		".jpeg": true,
		".png":  true,
		".webp": true,
		".gif":  true,
		".avif": true,
		".svg":  true,
	}

	err := filepath.Walk(publicDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() {
			ext := strings.ToLower(filepath.Ext(path))
			if imageExts[ext] {
				relPath, err := filepath.Rel(publicDir, path)
				if err != nil {
					return err
				}

				images = append(images, ImageAsset{
					Path:         relPath,
					AbsolutePath: path,
					PublicPath:   filepath.Join("/", relPath),
					Format:       strings.TrimPrefix(ext, "."),
					IsOptimized:  false,
				})
			}
		}
		return nil
	})

	return images, err
}

// parseImagesManifest extracts info from Next.js images-manifest.json
func parseImagesManifest(manifest map[string]interface{}, projectDir string) []ImageAsset {
	var images []ImageAsset

	if imagesMap, ok := manifest["images"].(map[string]interface{}); ok {
		for _, img := range imagesMap {
			if imgMap, ok := img.(map[string]interface{}); ok {
				path, _ := imgMap["path"].(string)
				format, _ := imgMap["format"].(string)

				asset := ImageAsset{
					Path:         path,
					AbsolutePath: filepath.Join(projectDir, ".next", "server", path),
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
		}
	}

	return images
}

// parseStaticImageImports finds statically imported images from build manifest
func parseStaticImageImports(buildManifest map[string]interface{}, projectDir string) []ImageAsset {
	var images []ImageAsset

	if files, ok := buildManifest["staticImageImports"].(map[string]interface{}); ok {
		for path, data := range files {
			if dataMap, ok := data.(map[string]interface{}); ok {
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
		}
	}

	return images
}
func startCommand(PackageManager string) (string, error) {
	if PackageManager == "" {
		return "", fmt.Errorf("no package manager provided")
	}
	switch PackageManager {
	case "npm":
		return "npm start", nil
	case "yarn":
		return "yarn start", nil
	case "pnpm":
		return "pnpm start", nil
	default:
		return "npm start", fmt.Errorf("unsupported package manager: %s", PackageManager)

	}
}
func buildCommand(PackageManager string) (PackageManager, error) {

	if PackageManager == "" {
		PackageManager = "npm" // default to npm if not specified
	}

	switch PackageManager {
	case "npm":
		return "npm run build", nil
	case "yarn":
		return "yarn build", nil
	case "pnpm":
		return "pnpm run build", nil
	default:
		return "npm run build", fmt.Errorf("unsupported package manager: %s", PackageManager)
	}

}

func getRoutesFromManifests(buildMeta *NextBuildMetadata) (*RouteInfo, error) {
	info := &RouteInfo{
		SSGRoutes:      make(map[string]string),
		ISRRoutes:      make(map[string]string),
		FallbackRoutes: make(map[string]string),
	}
	// process routes from route-manifest.json
	// Process routes from routes-manifest.json
	if routesManifest, ok := buildMeta.RoutesManifest.(map[string]interface{}); ok {
		if staticRoutes, ok := routesManifest["staticRoutes"].([]interface{}); ok {
			for _, route := range staticRoutes {
				if routeMap, ok := route.(map[string]interface{}); ok {
					if page, ok := routeMap["page"].(string); ok {
						info.StaticRoutes = append(info.StaticRoutes, page)
					}
				}
			}
		}
		if dynamicRoutes, ok := routesManifest["dynamicRoutes"].([]interface{}); ok {
			for _, route := range dynamicRoutes {
				if routeMap, ok := route.(map[string]interface{}); ok {
					if page, ok := routeMap["page"].(string); ok {
						info.DynamicRoutes = append(info.DynamicRoutes, page)
					}
				}
			}
		}
	}
	// Process prerender-manifest.json
	if prerenderManifest, ok := buildMeta.PrerenderManifest.(map[string]interface{}); ok {
		if routes, ok := prerenderManifest["routes"].(map[string]interface{}); ok {
			for route, details := range routes {
				if detailMap, ok := details.(map[string]interface{}); ok {
					if initialRevalidate, ok := detailMap["initialRevalidateSeconds"].(float64); ok {
						if initialRevalidate > 0 {
							info.ISRRoutes[route] = filepath.Join(".next", "server", detailMap["dataRoute"].(string))
						} else {
							info.SSGRoutes[route] = filepath.Join(".next", "server", "pages", route+".html")
						}
					}
				}
			}
		}
		if dynamicRoutes, ok := prerenderManifest["dynamicRoutes"].(map[string]interface{}); ok {
			for route, details := range dynamicRoutes {
				if detailMap, ok := details.(map[string]interface{}); ok {
					if fallback, ok := detailMap["fallback"].(string); ok && fallback != "" {
						info.FallbackRoutes[route] = fallback
					}
				}
			}
		}
	}

	// Process middleware routes from build-manifest.json
	if buildManifest, ok := buildMeta.BuildManifest.(map[string]interface{}); ok {
		if middleware, ok := buildManifest["middleware"].(map[string]interface{}); ok {
			for route := range middleware {
				info.MiddlewareRoutes = append(info.MiddlewareRoutes, route)
			}
		}
	}

	return info, nil
}
func CollectBuildMetadata() (*NextBuildMetadata, error) {
	projectDir, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	NextCoreLogger.Debug("Build the next to generate build metadata")
	PackageManager, err := DetectPackageManager(projectDir)
	if err != nil {
		PackageManager = "npm"
	}
	buildCommand, err := buildCommand(string(PackageManager))
	if err != nil {
		PackageManager = "npm"
	}
	if err := os.MkdirAll(".nextdeploy", 0755); err != nil {
		return nil, fmt.Errorf("failed to create .nextdeploy directory: %w", err)
	}
	cmd := exec.Command("sh", "-c", buildCommand.String())
	cmd.Dir = projectDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stdout

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("build failed:%w", err)
	}

	nextDir := filepath.Join(projectDir, ".next")
	buildID, err := os.ReadFile(filepath.Join(nextDir, "BUILD_ID"))
	if err != nil {
		return nil, fmt.Errorf("faileed to read BUILD_ID:%s", err)
	}
	readJSON := func(filename string) (interface{}, error) {
		data, err := os.ReadFile(filepath.Join(nextDir, filename))
		if err != nil {
			return nil, err
		}
		var result interface{}
		if err := json.Unmarshal(data, &result); err != nil {
			return nil, err
		}
		return result, nil
	}

	// Collect all manifests
	buildManifest, _ := readJSON("build-manifest.json")
	appBuildManifest, _ := readJSON("app-build-manifest.json")
	prerenderManifest, _ := readJSON("prerender-manifest.json")
	routesManifest, _ := readJSON("routes-manifest.json")
	imagesManifest, _ := readJSON("images-manifest.json")
	appPathRoutesManifest, _ := readJSON("app-path-routes-manifest.json")
	reactLoadableManifest, _ := readJSON("react-loadable-manifest.json")
	var diagnostics []string
	diagnosticsDir := filepath.Join(nextDir, "diagnostics")
	if files, err := os.ReadDir(diagnosticsDir); err != nil {
		for _, file := range files {
			diagnostics = append(diagnostics, file.Name())
		}
	}

	return &NextBuildMetadata{
		BuildID:               string(buildID),
		BuildManifest:         buildManifest,
		AppBuildManifest:      appBuildManifest,
		PrerenderManifest:     prerenderManifest,
		RoutesManifest:        routesManifest,
		ImagesManifest:        imagesManifest,
		AppPathRoutesManifest: appPathRoutesManifest,
		ReactLoadableManifest: reactLoadableManifest,
		Diagnostics:           diagnostics,
	}, nil

}
