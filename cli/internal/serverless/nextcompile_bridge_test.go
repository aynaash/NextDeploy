package serverless

import (
	"testing"

	"github.com/aynaash/nextdeploy/shared/nextcore"
)

func TestToCompilePayload_MapsAllFields(t *testing.T) {
	meta := &nextcore.NextCorePayload{
		AppName:    "demo",
		DistDir:    ".next",
		OutputMode: nextcore.OutputModeStandalone,
		GitCommit:  "abc123",
		NextBuildMetadata: nextcore.NextBuildMetadata{
			BuildID:      "build-id-42",
			HasAppRouter: true,
		},
		RouteInfo: nextcore.RouteInfo{
			StaticRoutes:     []string{"/", "/about"},
			DynamicRoutes:    []string{"/blog/[slug]"},
			SSGRoutes:        map[string]string{"/about": "/about.html"},
			ISRRoutes:        map[string]string{"/news": "/news.html"},
			ISRDetail:        []nextcore.ISRRoute{{Path: "/news", Tags: []string{"t1"}, Revalidate: 60}},
			APIRoutes:        []string{"/api/x"},
			SSRRoutes:        []string{"/dashboard"},
			MiddlewareRoutes: []string{"/"},
			FallbackRoutes:   map[string]string{},
		},
		Middleware: &nextcore.MiddlewareConfig{
			Path:    "middleware.ts",
			Runtime: "edge",
			Matchers: []nextcore.MiddlewareRoute{
				{Pathname: "/api/:path*", Pattern: "^/api/.*"},
			},
		},
		ImageConfig: &nextcore.ImageConfig{
			RemotePatterns: []nextcore.ImageRemotePattern{{Protocol: "https", Hostname: "cdn.example.com"}},
			Domains:        []string{"images.example.com"},
			Formats:        []string{"image/avif"},
			Unoptimized:    false,
		},
	}

	got := toCompilePayload(meta, nil)

	if got.AppName != "demo" {
		t.Errorf("AppName: got %q", got.AppName)
	}
	if got.DistDir != ".next" {
		t.Errorf("DistDir: got %q", got.DistDir)
	}
	if got.OutputMode != "standalone" {
		t.Errorf("OutputMode: got %q", got.OutputMode)
	}
	if !got.HasAppRouter {
		t.Error("HasAppRouter lost")
	}
	if got.BuildID != "build-id-42" {
		t.Errorf("BuildID: got %q", got.BuildID)
	}
	if got.GitCommit != "abc123" {
		t.Errorf("GitCommit: got %q", got.GitCommit)
	}

	// Routes passed through (same shape).
	if len(got.Routes.StaticRoutes) != 2 {
		t.Errorf("StaticRoutes: got %v", got.Routes.StaticRoutes)
	}
	if got.Routes.SSGRoutes["/about"] != "/about.html" {
		t.Errorf("SSGRoutes lost entry")
	}
	if len(got.Routes.ISRDetail) != 1 || got.Routes.ISRDetail[0].Revalidate != 60 {
		t.Errorf("ISRDetail lost: %+v", got.Routes.ISRDetail)
	}

	// Middleware retained (lossy matcher conversion).
	if got.Middleware == nil {
		t.Fatal("Middleware nil")
	}
	if got.Middleware.Runtime != "edge" {
		t.Errorf("Middleware runtime: got %q", got.Middleware.Runtime)
	}
	if len(got.Middleware.Matchers) != 1 {
		t.Fatalf("expected 1 matcher, got %d", len(got.Middleware.Matchers))
	}
	if got.Middleware.Matchers[0].Pathname != "/api/:path*" {
		t.Errorf("matcher pathname: %+v", got.Middleware.Matchers[0])
	}

	// Image config is preserved into the manifest payload for the runtime SSRF guard.
	if got.ImageConfig == nil {
		t.Fatal("ImageConfig nil")
	}
	if len(got.ImageConfig.RemotePatterns) != 1 {
		t.Fatalf("expected 1 remote pattern, got %d", len(got.ImageConfig.RemotePatterns))
	}
	if got.ImageConfig.RemotePatterns[0].Hostname != "cdn.example.com" {
		t.Errorf("remote pattern hostname: got %q", got.ImageConfig.RemotePatterns[0].Hostname)
	}
}

func TestToCompilePayload_NilSafe(t *testing.T) {
	// Converter must never panic on nil inputs — DeployCompute gates on
	// static-export mode but defensive programming covers every call path.
	got := toCompilePayload(nil, nil)
	if got.AppName != "" {
		t.Errorf("expected zero payload, got %+v", got)
	}
	if got.Middleware != nil {
		t.Errorf("expected nil middleware, got %+v", got.Middleware)
	}
}

func TestToCompilePayload_NoMiddleware(t *testing.T) {
	meta := &nextcore.NextCorePayload{
		AppName: "nomw",
		DistDir: ".next",
	}
	got := toCompilePayload(meta, nil)
	if got.Middleware != nil {
		t.Errorf("expected nil middleware when nextcore has none; got %+v", got.Middleware)
	}
}

func TestToCompilePayload_ForwardsNextConfig(t *testing.T) {
	meta := &nextcore.NextCorePayload{
		AppName:     "app",
		BasePath:    "/docs",
		AssetPrefix: "https://cdn.example.com",
		I18n: &nextcore.I18nConfig{
			Locales:         []string{"en", "fr"},
			DefaultLocale:   "en",
			LocaleDetection: true,
		},
		ImageConfig: &nextcore.ImageConfig{
			Domains: []string{"img.example.com"},
			RemotePatterns: []nextcore.ImageRemotePattern{
				{Protocol: "https", Hostname: "cdn.example.com"},
			},
		},
	}

	p := toCompilePayload(meta, nil)

	if p.BasePath != "/docs" {
		t.Errorf("BasePath = %q, want /docs", p.BasePath)
	}
	if p.AssetPrefix != "https://cdn.example.com" {
		t.Errorf("AssetPrefix = %q, want https://cdn.example.com", p.AssetPrefix)
	}
	if p.I18n == nil || p.I18n.DefaultLocale != "en" || len(p.I18n.Locales) != 2 {
		t.Fatalf("I18n not forwarded: %+v", p.I18n)
	}
	if !p.I18n.LocaleDetection {
		t.Error("LocaleDetection not forwarded")
	}
	if p.ImageConfig == nil || len(p.ImageConfig.RemotePatterns) != 1 {
		t.Fatalf("ImageConfig not forwarded: %+v", p.ImageConfig)
	}
	if p.ImageConfig.RemotePatterns[0].Hostname != "cdn.example.com" {
		t.Errorf("remote pattern hostname = %q", p.ImageConfig.RemotePatterns[0].Hostname)
	}
}

// A payload built from an app with no next.config (ParseNextConfigFile is
// non-fatal and yields nil) must produce zero values, not a panic.
func TestToCompilePayload_NilNextConfigProjectionIsSafe(t *testing.T) {
	p := toCompilePayload(&nextcore.NextCorePayload{AppName: "app"}, nil)
	if p.BasePath != "" || p.AssetPrefix != "" || p.I18n != nil || p.ImageConfig != nil {
		t.Errorf("expected zero next.config projection, got %+v", p)
	}
}

func TestConvertMiddlewareMatchers_CarriesConditions(t *testing.T) {
	in := []nextcore.MiddlewareRoute{{
		Pathname: "/dashboard/:path*",
		Pattern:  "^/dashboard/.*",
		Has:      []nextcore.MiddlewareCondition{{Type: "header", Key: "x-env", Value: "prod"}},
		Missing:  []nextcore.MiddlewareCondition{{Type: "cookie", Key: "session"}},
	}}

	out := convertMiddlewareMatchers(in)

	if len(out) != 1 {
		t.Fatalf("want 1 matcher, got %d", len(out))
	}
	if len(out[0].Has) != 1 || out[0].Has[0].Value != "prod" {
		t.Fatalf("has not forwarded: %+v", out[0].Has)
	}
	if len(out[0].Missing) != 1 || out[0].Missing[0].Key != "session" {
		t.Fatalf("missing not forwarded: %+v", out[0].Missing)
	}
}

func TestConvertMiddlewareMatchers_NoConditionsStayNil(t *testing.T) {
	out := convertMiddlewareMatchers([]nextcore.MiddlewareRoute{{Pathname: "/a"}})
	if out[0].Has != nil || out[0].Missing != nil {
		t.Errorf("expected nil condition slices, got %+v", out[0])
	}
}
