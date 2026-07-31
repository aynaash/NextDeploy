package nextcompile

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestBuildManifest_MinimalPayload(t *testing.T) {
	p := Payload{
		AppName:      "demo",
		BasePath:     "/app",
		OutputMode:   "standalone",
		HasAppRouter: true,
		Routes: RouteInfo{
			StaticRoutes:  []string{"/", "/about"},
			SSRRoutes:     []string{"/dashboard"},
			APIRoutes:     []string{"/api/users"},
			DynamicRoutes: []string{"/blog/[slug]"},
			SSGRoutes:     map[string]string{"/blog": "/blog.html"},
			ISRRoutes:     map[string]string{"/news": "/news.html"},
			ISRDetail: []ISRRoute{
				{Path: "/news", Tags: []string{"news", "home"}, Revalidate: 60},
			},
		},
	}
	nv := NextVersion{Raw: "14.2.3"}
	rv := ReactVersion{Raw: "18.3.1"}
	now := time.Date(2026, 4, 18, 12, 0, 0, 0, time.UTC)

	m := BuildManifest(p, nv, rv, nil, now)

	if m.SchemaVersion != manifestSchemaVersion {
		t.Errorf("schema version: got %s", m.SchemaVersion)
	}
	if m.AppName != "demo" {
		t.Errorf("app name lost")
	}
	if m.GeneratedAt != "2026-04-18T12:00:00Z" {
		t.Errorf("generated at: %s", m.GeneratedAt)
	}
	if m.ISR.Intervals["/news"] != 60 {
		t.Errorf("interval missing")
	}
	if got := m.ISR.Tags["news"]; len(got) != 1 || got[0] != "/news" {
		t.Errorf("tag index: got %v", got)
	}
	if got := m.Routes.Static; len(got) != 2 || got[0] != "/" {
		t.Errorf("static routes: %v", got)
	}
}

func TestBuildManifest_Deterministic(t *testing.T) {
	// Two identical payloads with keys inserted in different orders should
	// produce byte-identical JSON — guards the reproducible-build claim.
	base := Payload{
		AppName: "dup",
		Routes: RouteInfo{
			StaticRoutes: []string{"/b", "/a", "/c"}, // unsorted input
			SSGRoutes:    map[string]string{"/z": "z.html", "/a": "a.html"},
		},
	}
	nv := NextVersion{Raw: "14.0.0"}
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	a := BuildManifest(base, nv, ReactVersion{}, nil, now)
	b := BuildManifest(base, nv, ReactVersion{}, nil, now)

	ab, _ := json.Marshal(a)
	bb, _ := json.Marshal(b)
	if !bytes.Equal(ab, bb) {
		t.Fatalf("not deterministic:\n%s\n%s", ab, bb)
	}

	// Also verify static routes are sorted in output.
	decoded := Manifest{}
	_ = json.Unmarshal(ab, &decoded)
	if decoded.Routes.Static[0] != "/a" || decoded.Routes.Static[2] != "/c" {
		t.Errorf("static not sorted: %v", decoded.Routes.Static)
	}
}

func TestEmitManifest_WritesValidJSON(t *testing.T) {
	dir := t.TempDir()
	m := BuildManifest(
		Payload{AppName: "t", Routes: RouteInfo{StaticRoutes: []string{"/"}}},
		NextVersion{Raw: "14.2.0"},
		ReactVersion{Raw: "18.3.1"},
		nil,
		time.Now(),
	)

	path, err := EmitManifest(m, dir)
	if err != nil {
		t.Fatalf("EmitManifest: %v", err)
	}
	if exp := filepath.Join(dir, "_nextdeploy", "manifest.json"); path != exp {
		t.Errorf("path: got %s, want %s", path, exp)
	}

	buf, err := os.ReadFile(path) // #nosec G304
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var back Manifest
	if err := json.Unmarshal(buf, &back); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, buf)
	}
	if back.AppName != "t" {
		t.Errorf("round trip lost appName")
	}
	if back.SchemaVersion != manifestSchemaVersion {
		t.Errorf("schema version lost in round trip")
	}
	// Trailing newline for clean diffs.
	if len(buf) == 0 || buf[len(buf)-1] != '\n' {
		t.Errorf("missing trailing newline")
	}
}

func TestBuildManifest_NextConfigProjection(t *testing.T) {
	p := Payload{
		AppName:     "app",
		BasePath:    "/docs",
		AssetPrefix: "https://cdn.x",
		I18n:        &I18nConfig{Locales: []string{"fr", "en"}, DefaultLocale: "en"},
		ImageConfig: &ImageConfig{Domains: []string{"img.x"}},
	}
	m := BuildManifest(p, NextVersion{Raw: "15.0.0"}, ReactVersion{Raw: "19.0.0"}, nil, time.Unix(0, 0))

	if m.BasePath != "/docs" {
		t.Errorf("basePath = %q, want /docs", m.BasePath)
	}
	if m.AssetPrefix != "https://cdn.x" {
		t.Errorf("assetPrefix = %q, want https://cdn.x", m.AssetPrefix)
	}
	if m.I18n == nil || m.I18n.DefaultLocale != "en" {
		t.Fatalf("i18n not carried: %+v", m.I18n)
	}
	// Locales are sorted for byte-identical output.
	if m.I18n.Locales[0] != "en" {
		t.Errorf("locales not sorted: %v", m.I18n.Locales)
	}
	if m.Images == nil || len(m.Images.Domains) != 1 {
		t.Fatalf("images not carried: %+v", m.Images)
	}
}

func TestBuildManifest_MiddlewareConditions(t *testing.T) {
	p := Payload{
		AppName: "app",
		Middleware: &MiddlewareConfig{
			Path: "middleware.ts",
			Matchers: []MiddlewareMatcher{{
				Pathname: "/app/:path*",
				Pattern:  "^/app/.*",
				Has:      []MiddlewareCondition{{Type: "cookie", Key: "session"}},
				Missing:  []MiddlewareCondition{{Type: "header", Key: "x-skip"}},
			}},
		},
	}
	m := BuildManifest(p, NextVersion{Raw: "15"}, ReactVersion{Raw: "19"}, nil, time.Unix(0, 0))
	if m.Middleware == nil || len(m.Middleware.Matchers) != 1 {
		t.Fatalf("middleware not carried: %+v", m.Middleware)
	}
	got := m.Middleware.Matchers[0]
	if len(got.Has) != 1 || got.Has[0].Key != "session" || got.Has[0].Type != "cookie" {
		t.Fatalf("has not carried: %+v", got.Has)
	}
	if len(got.Missing) != 1 || got.Missing[0].Type != "header" {
		t.Fatalf("missing not carried: %+v", got.Missing)
	}
	// The conditions must survive to the emitted JSON the runtime reads.
	b, err := json.Marshal(m.Middleware)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(b, []byte(`"has":[{"type":"cookie","key":"session"}]`)) {
		t.Errorf("has conditions missing from manifest JSON: %s", b)
	}
}

func TestBuildManifest_MiddlewareWithoutConditionsOmitsKeys(t *testing.T) {
	p := Payload{
		AppName: "app",
		Middleware: &MiddlewareConfig{
			Path:     "middleware.ts",
			Matchers: []MiddlewareMatcher{{Pathname: "/app/:path*"}},
		},
	}
	m := BuildManifest(p, NextVersion{Raw: "15"}, ReactVersion{Raw: "19"}, nil, time.Unix(0, 0))
	b, err := json.Marshal(m.Middleware)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(b, []byte(`"has"`)) || bytes.Contains(b, []byte(`"missing"`)) {
		t.Errorf("empty conditions must be omitted, got: %s", b)
	}
}
