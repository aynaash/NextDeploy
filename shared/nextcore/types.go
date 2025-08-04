package nextcore

import (
	"nextdeploy/shared/config"
)

type NextCorePayload struct {
	AppName           string                   `json:"app_name"`
	NextVersion       string                   `json:"next_version"`
	NextBuildMetadata NextBuildMetadata        `json:"nextbuildmetadata"`
	StaticRoutes      []string                 `json:"static_routes"`
	Dynamic           []string                 `json:"dymanic_routes"`
	BuildCommand      string                   `json:"build_command"`
	StartCommand      string                   `json:"start_command"`
	HasImageAssets    bool                     `json:"has_image_assets"`
	CDNEnabled        bool                     `json:"cdn_enabled"`
	Domain            string                   `json:"domain"`
	Middleware        *MiddlewareConfig        `json:"middleware"`
	StaticAssets      *StaticAssets            `json:"static_assets"`
	GitCommit         string                   `json:"git_commit,omitempty"`
	GitDirty          bool                     `json:"git_dirty,omitempty"`
	GeneratedAt       string                   `json:"generated_at,omitempty"`
	BuildLockFile     string                   `json:"build_lock_file,omitempty"`
	MetadataFilePath  string                   `json:"metadata_file_path,omitempty"`
	AssetsOutputDir   string                   `json:"assets_output_dir,omitempty"`
	Config            *config.NextDeployConfig `json:"config,omitempty"`
	ImageAssets       ImageAssets              `json:"image_assets"`    // Detected image assets
	RouteInfo         RouteInfo                `json:"route_info"`      // Information about routes
	Output            string                   `json:"standalone"`      // "standalone", "export", etc.
	NextBuild         NextBuild                `json:"next_build"`      // Full Next.js build structure
	WorkingDir        string                   `json:"working_dir"`     // Working directory for the build
	RootDir           string                   `json:"root_dir"`        // Root directory of the Next.js project
	PackageManager    string                   `json:"package_manager"` // "npm", "yarn", "pnpm", etc.https://shadcn-nextjs-dashboard.vercel.app/dashboard
	Entrypoint        string                   `json:"entrypoint"`      // Entrypoint for the application

}

// DeployMetadata contains deployment metadata
type DeployMetadata struct {
	GeneratedAt string            `json:"generated_at"`
	Routes      RoutesManifest    `json:"routes"`
	BuildInfo   BuildManifest     `json:"build_info"`
	Middleware  []string          `json:"middleware"`
	EnvVars     map[string]string `json:"env_vars"`
}

// BuildLock contains git state information
type BuildLock struct {
	GitCommit   string `json:"git_commit"`
	GitDirty    bool   `json:"git_dirty"`
	GeneratedAt string `json:"generated_at"`
	Metadata    string `json:"metadata_file"`
}

// RoutesManifest represents the Next.js routes manifest
type RoutesManifest struct {
	Version       int            `json:"version"`
	Pages         []string       `json:"pages"`
	DynamicRoutes []DynamicRoute `json:"dynamicRoutes"`
}

type DynamicRoute struct {
	Page      string            `json:"page"`
	Regex     string            `json:"regex"`
	RouteKeys map[string]string `json:"routeKeys"`
}

// BuildManifest represents the Next.js build manifest
type BuildManifest struct {
	Pages map[string][]string `json:"pages"`
}

type StaticAsset struct {
	Path         string `json:"path"`          // Relative path from project root
	AbsolutePath string `json:"absolute_path"` // Absolute filesystem path
	PublicPath   string `json:"public_path"`   // URL path where asset is served
	Type         string `json:"type"`          // "image", "font", "stylesheet", "script", "document", "other"
	Extension    string `json:"extension"`     // File extension
	Size         int64  `json:"size"`          // File size in bytes
}

// StaticAssets contains all static assets grouped by type
type StaticAssets struct {
	PublicDir    []StaticAsset `json:"public_dir"`    // Assets from public directory
	StaticFolder []StaticAsset `json:"static_folder"` // Assets from static folder (legacy)
	NextStatic   []StaticAsset `json:"next_static"`   // Assets from .next/static
	OtherAssets  []StaticAsset `json:"other_assets"`  // Other detected static assets
}
type MiddlewareConfig struct {
	Path         string            `json:"path"`                    // Path to middleware file
	Matchers     []MiddlewareRoute `json:"matchers"`                // Route matchers
	Runtime      string            `json:"runtime,omitempty"`       // "edge" or "nodejs"
	Regions      []string          `json:"regions,omitempty"`       // Deployment regions (for Edge)
	UnstableFlag string            `json:"unstable_flag,omitempty"` // Any unstable flags used
}

// MiddlewareRoute represents a single route matcher
type MiddlewareRoute struct {
	Pathname string                `json:"pathname,omitempty"` // Exact path match
	Pattern  string                `json:"pattern,omitempty"`  // Regex pattern
	Has      []MiddlewareCondition `json:"has,omitempty"`      // Conditions
	Missing  []MiddlewareCondition `json:"missing,omitempty"`  // Negative conditions
	Type     string                `json:"type,omitempty"`     // "header", "cookie", etc.
	Key      string                `json:"key,omitempty"`      // Condition key
	Value    string                `json:"value,omitempty"`    // Condition value
}

// MiddlewareCondition represents a condition for route matching
type MiddlewareCondition struct {
	Type  string `json:"type"`            // "header", "cookie", "query", "host"
	Key   string `json:"key,omitempty"`   // Condition key
	Value string `json:"value,omitempty"` // Condition value
}

// NextConfig represents the complete Next.js configuration
type NextConfig struct {
	// Standard Configuration
	BasePath        string                 `json:"basePath,omitempty"`
	Output          string                 `json:"output,omitempty"` // "standalone", "export", etc.
	Images          *ImageConfig           `json:"images,omitempty"`
	ReactStrictMode bool                   `json:"reactStrictMode,omitempty"`
	PoweredByHeader bool                   `json:"poweredByHeader,omitempty"`
	TrailingSlash   bool                   `json:"trailingSlash,omitempty"`
	PageExtensions  []string               `json:"pageExtensions,omitempty"`
	AssetPrefix     string                 `json:"assetPrefix,omitempty"`
	DistDir         string                 `json:"distDir,omitempty"`
	CleanDistDir    bool                   `json:"cleanDistDir,omitempty"`
	GenerateBuildId interface{}            `json:"generateBuildId,omitempty"` // func or string
	OnDemandEntries map[string]interface{} `json:"onDemandEntries,omitempty"`
	CompileOptions  map[string]interface{} `json:"compileOptions,omitempty"`

	// Routing
	Headers                    []interface{} `json:"headers,omitempty"`
	Redirects                  []interface{} `json:"redirects,omitempty"`
	Rewrites                   []interface{} `json:"rewrites,omitempty"`
	SkipMiddlewareUrlNormalize bool          `json:"skipMiddlewareUrlNormalize,omitempty"`
	SkipTrailingSlashRedirect  bool          `json:"skipTrailingSlashRedirect,omitempty"`

	// Runtime
	Env                 map[string]string      `json:"env,omitempty"`
	PublicRuntimeConfig map[string]interface{} `json:"publicRuntimeConfig,omitempty"`
	ServerRuntimeConfig map[string]interface{} `json:"serverRuntimeConfig,omitempty"`

	// Compiler
	Compiler *CompilerConfig `json:"compiler,omitempty"`

	// Webpack
	Webpack  interface{} `json:"webpack,omitempty"`
	Webpack5 bool        `json:"webpack5,omitempty"`

	// Experimental Features
	Experimental *ExperimentalConfig `json:"experimental,omitempty"`

	// Edge
	EdgeRegions []string `json:"edgeRegions,omitempty"`
	EdgeRuntime string   `json:"edgeRuntime,omitempty"` // "experimental-edge"

	// i18n
	I18n *I18nConfig `json:"i18n,omitempty"`

	// Analytics
	AnalyticsId string `json:"analyticsId,omitempty"`

	// MDX
	MdxRs bool `json:"mdxRs,omitempty"`
}

type CompilerConfig struct {
	Emotion               interface{} `json:"emotion,omitempty"`
	ReactRemoveProperties interface{} `json:"reactRemoveProperties,omitempty"`
	RemoveConsole         interface{} `json:"removeConsole,omitempty"`
	StyledComponents      interface{} `json:"styledComponents,omitempty"`
	Relay                 interface{} `json:"relay,omitempty"`
}

type ExperimentalConfig struct {
	// App Router
	AppDir                       bool   `json:"appDir,omitempty"`
	CaseSensitiveRoutes          bool   `json:"caseSensitiveRoutes,omitempty"`
	UseDeploymentId              bool   `json:"useDeploymentId,omitempty"`
	UseDeploymentIdServerActions bool   `json:"useDeploymentIdServerActions,omitempty"`
	DeploymentId                 string `json:"deploymentId,omitempty"`

	// Server Components
	ServerComponents           bool `json:"serverComponents,omitempty"`
	ServerActions              bool `json:"serverActions,omitempty"`
	ServerActionsBodySizeLimit int  `json:"serverActionsBodySizeLimit,omitempty"`

	// Optimizations
	OptimizeCss                   bool    `json:"optimizeCss,omitempty"`
	OptimisticClientCache         bool    `json:"optimisticClientCache,omitempty"`
	ClientRouterFilter            bool    `json:"clientRouterFilter,omitempty"`
	ClientRouterFilterRedirects   bool    `json:"clientRouterFilterRedirects,omitempty"`
	ClientRouterFilterAllowedRate float64 `json:"clientRouterFilterAllowedRate,omitempty"`

	// Build System
	ExternalDir                       string        `json:"externalDir,omitempty"`
	ExternalMiddlewareRewritesResolve bool          `json:"externalMiddlewareRewritesResolve,omitempty"`
	FallbackNodePolyfills             bool          `json:"fallbackNodePolyfills,omitempty"`
	ForceSwcTransforms                bool          `json:"forceSwcTransforms,omitempty"`
	FullySpecified                    bool          `json:"fullySpecified,omitempty"`
	SwcFileReading                    bool          `json:"swcFileReading,omitempty"`
	SwcMinify                         bool          `json:"swcMinify,omitempty"`
	SwcPlugins                        []interface{} `json:"swcPlugins,omitempty"`
	SwcTraceProfiling                 bool          `json:"swcTraceProfiling,omitempty"`

	// Turbopack
	Turbo      map[string]interface{} `json:"turbo,omitempty"`
	Turbotrace map[string]interface{} `json:"turbotrace,omitempty"`

	// Other
	ScrollRestoration       bool     `json:"scrollRestoration,omitempty"`
	NewNextLinkBehavior     bool     `json:"newNextLinkBehavior,omitempty"`
	ManualClientBasePath    bool     `json:"manualClientBasePath,omitempty"`
	LegacyBrowsers          bool     `json:"legacyBrowsers,omitempty"`
	DisableOptimizedLoading bool     `json:"disableOptimizedLoading,omitempty"`
	GzipSize                bool     `json:"gzipSize,omitempty"`
	SharedPool              bool     `json:"sharedPool,omitempty"`
	WebVitalsAttribution    []string `json:"webVitalsAttribution,omitempty"`
	InstrumentationHook     string   `json:"instrumentationHook,omitempty"`
}

type ImageConfig struct {
	Domains               []string             `json:"domains,omitempty"`
	Formats               []string             `json:"formats,omitempty"`
	DeviceSizes           []int                `json:"deviceSizes,omitempty"`
	ImageSizes            []int                `json:"imageSizes,omitempty"`
	Path                  string               `json:"path,omitempty"`
	Loader                string               `json:"loader,omitempty"`
	LoaderFile            string               `json:"loaderFile,omitempty"`
	MinimumCacheTTL       int                  `json:"minimumCacheTTL,omitempty"`
	Unoptimized           bool                 `json:"unoptimized,omitempty"`
	ContentSecurityPolicy string               `json:"contentSecurityPolicy,omitempty"`
	RemotePatterns        []ImageRemotePattern `json:"remotePatterns,omitempty"`
}

type ImageRemotePattern struct {
	Protocol string `json:"protocol,omitempty"`
	Hostname string `json:"hostname,omitempty"`
	Port     string `json:"port,omitempty"`
	Pathname string `json:"pathname,omitempty"`
}

type I18nConfig struct {
	Locales         []string `json:"locales"`
	DefaultLocale   string   `json:"defaultLocale"`
	Domains         []Domain `json:"domains,omitempty"`
	LocaleDetection bool     `json:"localeDetection,omitempty"`
}

type Domain struct {
	Domain        string   `json:"domain"`
	Locales       []string `json:"locales,omitempty"`
	DefaultLocale string   `json:"defaultLocale,omitempty"`
}
type ImageAsset struct {
	Path           string `json:"path"`             // Relative path to the image
	AbsolutePath   string `json:"absolute_path"`    // Absolute filesystem path
	PublicPath     string `json:"public_path"`      // URL path where the image is served
	Format         string `json:"format"`           // Image format (jpg, png, webp, etc.)
	IsOptimized    bool   `json:"is_optimized"`     // Whether the image is optimized by Next.js
	IsStaticImport bool   `json:"is_static_import"` // Whether the image is statically imported
	Width          int    `json:"width,omitempty"`  // Original width (if available)
	Height         int    `json:"height,omitempty"` // Original height (if available)
}

// ImageAssets contains all detected image assets
type ImageAssets struct {
	PublicImages    []ImageAsset `json:"public_images"`    // Images from public directory
	OptimizedImages []ImageAsset `json:"optimized_images"` // Images processed by Next.js Image Optimization
	StaticImports   []ImageAsset `json:"static_imports"`   // Images statically imported in components
}

type RouteInfo struct {
	StaticRoutes     []string          `json:"static_routes"`
	DynamicRoutes    []string          `json:"dynamic_routes"`
	SSGRoutes        map[string]string `json:"ssg_routes"`        // path to HTML file
	SSRRoutes        []string          `json:"ssr_routes"`        // API routes
	ISRRoutes        map[string]string `json:"isr_routes"`        // path to revalidate info
	APIRoutes        []string          `json:"api_routes"`        // Next.js API routes
	FallbackRoutes   map[string]string `json:"fallback_routes"`   // dynamic routes with fallback
	MiddlewareRoutes []string          `json:"middleware_routes"` // routes with middleware
}
type NextBuildMetadata struct {
	BuildID               string      `json:"buildId"`
	BuildManifest         interface{} `json:"buildManifest"`
	AppBuildManifest      interface{} `json:"appBuildManifest"`
	PrerenderManifest     interface{} `json:"prerenderManifest"`
	RoutesManifest        interface{} `json:"routesManifest"`
	ImagesManifest        interface{} `json:"imagesManifest"`
	AppPathRoutesManifest interface{} `json:"appPathRoutesManifest"`
	ReactLoadableManifest interface{} `json:"reactLoadableManifest"`
	Diagnostics           []string    `json:"diagnostics"`
}

// NextBuild represents the core structure of a Next.js build output
type NextBuild struct {
	RootFiles      RootFiles     `json:"root_files"`
	Cache          Cache         `json:"cache"`
	Server         Server        `json:"server"`
	Static         Static        `json:"static"`
	HasAppRouter   bool          `json:"has_app_router"`   // True if using App Router
	HasPagesRouter bool          `json:"has_pages_router"` // True if using Pages Router
	BuildMetadata  BuildMetadata `json:"build_metadata"`
}

// RootFiles represents files in the root .next directory
type RootFiles struct {
	BuildManifest         string `json:"build_manifest"`          // build-manifest.json
	AppBuildManifest      string `json:"app_build_manifest"`      // app-build-manifest.json
	ReactLoadableManifest string `json:"react_loadable_manifest"` // react-loadable-manifest.json
	PackageJSON           string `json:"package_json"`            // package.json
	LastBuildTimestamp    string `json:"last_build_timestamp"`    // last
	TraceFile             string `json:"trace_file,omitempty"`    // trace (optional)
}

// Cache represents the .next/cache directory structure
type Cache struct {
	Images  []ImageCacheEntry `json:"images"`  // cache/images/
	Webpack WebpackCache      `json:"webpack"` // cache/webpack/
	SWC     []string          `json:"swc"`     // cache/swc/ (list of plugin paths)
}

type ImageCacheEntry struct {
	Hash      string `json:"hash"`       // Unique hash for the image
	Format    string `json:"format"`     // "webp", "png", etc.
	Width     int    `json:"width"`      // Original width
	Height    int    `json:"height"`     // Original height
	CachePath string `json:"cache_path"` // Path in cache directory
}

type WebpackCache struct {
	ClientDevelopment     []string `json:"client_development"`
	ClientProduction      []string `json:"client_production"`
	ServerDevelopment     []string `json:"server_development"`
	ServerProduction      []string `json:"server_production"`
	EdgeServerDevelopment []string `json:"edge_server_development"`
	EdgeServerProduction  []string `json:"edge_server_production"`
}

// Server represents the .next/server directory
type Server struct {
	Manifests    ServerManifests `json:"manifests"`
	AppRoutes    []AppRoute      `json:"app_routes"`
	VendorChunks []string        `json:"vendor_chunks"` // server/vendor-chunks/
	Middleware   Middleware      `json:"middleware"`
}

type ServerManifests struct {
	AppPaths        string `json:"app_paths"`        // app-paths-manifest.json
	Middleware      string `json:"middleware"`       // middleware-manifest.json
	Pages           string `json:"pages"`            // pages-manifest.json
	Font            string `json:"font"`             // next-font-manifest.json
	ServerReference string `json:"server_reference"` // server-reference-manifest.json
}

type AppRoute struct {
	RoutePath       string `json:"route_path"`       // e.g., "app/dashboard/page"
	PageJS          string `json:"page_js"`          // server-side component
	ClientReference string `json:"client_reference"` // client-reference-manifest.js
}

type Middleware struct {
	Path     string   `json:"path"`     // Path to middleware.js
	Matchers []string `json:"matchers"` // Routes middleware applies to
}

// Static represents the .next/static directory
type Static struct {
	Chunks  Chunks             `json:"chunks"`
	CSS     []CSSFile          `json:"css"`
	Media   []MediaFile        `json:"media"`
	Webpack []WebpackHotUpdate `json:"webpack"`
}

type Chunks struct {
	App       []string `json:"app"`       // app router chunks
	Pages     []string `json:"pages"`     // pages router chunks (if exists)
	Polyfills string   `json:"polyfills"` // polyfills.js
	Webpack   string   `json:"webpack"`   // webpack.js
	Main      string   `json:"main"`      // main-app.js
}

type CSSFile struct {
	Path     string `json:"path"`      // e.g., "css/app/layout.css"
	IsGlobal bool   `json:"is_global"` // Whether it's a global CSS file
}

type MediaFile struct {
	Name string `json:"name"` // Hashed filename
	Type string `json:"type"` // "font", "image", etc.
	Ext  string `json:"ext"`  // File extension (woff2, ttf, etc.)
}

type WebpackHotUpdate struct {
	Name string `json:"name"` // Hashed filename
	Type string `json:"type"` // "hot-update"
}

// BuildMetadata contains build information
type BuildMetadata struct {
	NextVersion   string `json:"next_version"`
	BuildTarget   string `json:"build_target"` // "server", "static", etc.
	BuildID       string `json:"build_id"`
	HasTypeScript bool   `json:"has_typescript"`
	HasESLint     bool   `json:"has_eslint"`
	OutputMode    string `json:"output_mode"` // "standalone", "export", etc.
}
