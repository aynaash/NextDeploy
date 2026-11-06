package daemon

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aynaash/nextdeploy/shared/config"
	"github.com/aynaash/nextdeploy/shared/nextcore"
)

func sidecarTestMeta(distDir string) *nextcore.NextCorePayload {
	return &nextcore.NextCorePayload{
		DistDir: distDir,
		GoSidecar: &config.GoSidecarConfig{
			Enabled: true,
		},
		RouteInfo: nextcore.RouteInfo{
			SSGRoutes: map[string]string{"/": "index.html"},
		},
	}
}

type synchronizedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *synchronizedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *synchronizedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func testHTTPPort(t *testing.T, server *httptest.Server) int {
	t.Helper()
	u, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil {
		t.Fatal(err)
	}
	return port
}

func TestSidecarServesIndexedPrerenderedRoute(t *testing.T) {
	release := t.TempDir()
	pageDir := filepath.Join(release, "dist-custom", "server", "pages")
	if err := os.MkdirAll(pageDir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pageDir, "index.html"), []byte("<html>prerendered</html>"), 0o600); err != nil {
		t.Fatal(err)
	}
	node := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unexpected Node request", http.StatusInternalServerError)
	}))
	defer node.Close()

	handler, err := NewSidecarHandler(release, "", filepath.Join(t.TempDir(), "cache"), testHTTPPort(t, node), sidecarTestMeta("dist-custom"))
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
	if recorder.Code != http.StatusOK || recorder.Body.String() != "<html>prerendered</html>" {
		t.Fatalf("response = %d %q", recorder.Code, recorder.Body.String())
	}
	if recorder.Header().Get("X-NextDeploy-Cache") != "HIT" {
		t.Fatalf("cache header = %q", recorder.Header().Get("X-NextDeploy-Cache"))
	}
	if recorder.Header().Get("X-NextDeploy-Handler") != "go-html" {
		t.Fatalf("handler header = %q, want go-html", recorder.Header().Get("X-NextDeploy-Handler"))
	}
	stats := handler.stats.snapshot()
	if stats.Requests != 1 || stats.HTMLHits != 1 || stats.NodeFallbacks != 0 {
		t.Fatalf("sidecar stats = %+v, want one HTML hit and no Node fallback", stats)
	}
}

func TestSidecarRSCRequiresIndexedPayload(t *testing.T) {
	var nodeRequests atomic.Int32
	node := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nodeRequests.Add(1)
		_, _ = w.Write([]byte("node-rsc"))
	}))
	defer node.Close()

	release := t.TempDir()
	pageDir := filepath.Join(release, ".next", "server", "pages")
	if err := os.MkdirAll(pageDir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pageDir, "index.html"), []byte("html"), 0o600); err != nil {
		t.Fatal(err)
	}
	handler, err := NewSidecarHandler(release, "", filepath.Join(t.TempDir(), "cache"), testHTTPPort(t, node), sidecarTestMeta(".next"))
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("RSC", "1")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Body.String() != "node-rsc" || nodeRequests.Load() != 1 {
		t.Fatalf("RSC response = %q, Node requests = %d", recorder.Body.String(), nodeRequests.Load())
	}
	if recorder.Header().Get("X-NextDeploy-Handler") != "node" ||
		recorder.Header().Get("X-NextDeploy-Fallback") != "not-prerendered" {
		t.Fatalf("handler/fallback headers = %q/%q",
			recorder.Header().Get("X-NextDeploy-Handler"),
			recorder.Header().Get("X-NextDeploy-Fallback"))
	}
	stats := handler.stats.snapshot()
	if stats.Requests != 1 || stats.NodeFallbacks != 1 || stats.RSCHits != 0 {
		t.Fatalf("sidecar stats = %+v, want one Node fallback and no RSC hit", stats)
	}
}

func TestSidecarReportsNodeFallbackReasons(t *testing.T) {
	release := t.TempDir()
	pageDir := filepath.Join(release, ".next", "server", "pages")
	if err := os.MkdirAll(pageDir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pageDir, "index.html"), []byte("prerendered"), 0o600); err != nil {
		t.Fatal(err)
	}
	node := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, r.URL.RequestURI())
	}))
	defer node.Close()
	meta := sidecarTestMeta(".next")
	meta.Middleware = &nextcore.MiddlewareConfig{
		Matchers: []nextcore.MiddlewareRoute{{Pathname: "/admin/:path*"}},
	}
	handler, err := NewSidecarHandler(release, "", filepath.Join(t.TempDir(), "cache"), testHTTPPort(t, node), meta)
	if err != nil {
		t.Fatal(err)
	}
	defer handler.Close()

	tests := []struct {
		path   string
		reason string
	}{
		{path: "/posts/1", reason: "not-prerendered"},
		{path: "/?preview=1", reason: "query-string"},
		{path: "/admin/users", reason: "middleware"},
	}
	for _, test := range tests {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, test.path, nil))
		if got := recorder.Header().Get("X-NextDeploy-Handler"); got != "node" {
			t.Errorf("%s handler = %q, want node", test.path, got)
		}
		if got := recorder.Header().Get("X-NextDeploy-Fallback"); got != test.reason {
			t.Errorf("%s fallback = %q, want %q", test.path, got, test.reason)
		}
	}
	if stats := handler.stats.snapshot(); stats.Requests != 3 || stats.NodeFallbacks != 3 {
		t.Errorf("sidecar stats = %+v, want 3 requests and 3 Node fallbacks", stats)
	}
}

func TestSidecarCountsRevalidationFailures(t *testing.T) {
	var logs synchronizedBuffer
	handler := &SidecarHandler{logger: log.New(&logs, "", 0)}
	handler.recordRevalidationFailure("failed to refresh route %q", "/news")
	if stats := handler.stats.snapshot(); stats.RevalidationFailures != 1 {
		t.Fatalf("revalidation failures = %d, want 1", stats.RevalidationFailures)
	}
	if !strings.Contains(logs.String(), `failed to refresh route "/news"`) {
		t.Fatalf("revalidation failure was not logged: %q", logs.String())
	}
}

func TestPrerenderReadsCannotEscapeRootThroughSymlinks(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.html")
	if err := os.WriteFile(outside, []byte("outside"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "page.html")); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := readWithinRoot(root, "page.html"); err == nil {
		t.Fatal("prerender reader followed a symlink outside its root")
	}
}

func TestMiddlewareMatcherOnlyBypassesMatchingPaths(t *testing.T) {
	matches, known := middlewarePathMatches("/admin/:path*", "", "/admin/users/1")
	if !known || !matches {
		t.Fatalf("admin wildcard match = %v, known = %v", matches, known)
	}
	matches, known = middlewarePathMatches("/admin/:path*", "", "/public")
	if !known || matches {
		t.Fatalf("public route match = %v, known = %v", matches, known)
	}
	matches, known = middlewarePathMatches("", "[", "/anything")
	if known || !matches {
		t.Fatalf("invalid pattern should conservatively bypass: match=%v known=%v", matches, known)
	}
}

func TestNextNegativeLookaheadMiddlewareMatcherKeepsExcludedPathsEligible(t *testing.T) {
	matcher := `/((?!api|_next/static|_next/image|favicon.ico|icon|.*\.[\w]+$).*)`
	for _, path := range []string{"/", "/dashboard"} {
		matches, known := middlewarePathMatches(matcher, "", path)
		if !known || !matches {
			t.Errorf("middleware path %q = %v, known = %v; want matching", path, matches, known)
		}
	}
	for _, path := range []string{"/_next/image", "/_next/static/app.js", "/robots.txt", "/api/auth"} {
		matches, known := middlewarePathMatches(matcher, "", path)
		if !known || matches {
			t.Errorf("excluded path %q = %v, known = %v; want not matching", path, matches, known)
		}
	}
}

func TestParsedNextMiddlewareMatcherUsesRouteSpecificExclusions(t *testing.T) {
	projectDir := t.TempDir()
	source := `export const config = {
  matcher: ["/((?!_next/static|_next/image|favicon.ico|icon|.*\\.[\\w]+$).*)"],
};`
	if err := os.WriteFile(filepath.Join(projectDir, "middleware.ts"), []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	middleware, err := nextcore.ParseMiddleware(projectDir)
	if err != nil {
		t.Fatal(err)
	}
	if middleware == nil || len(middleware.Matchers) != 1 {
		t.Fatalf("parsed middleware = %#v", middleware)
	}
	handler := &SidecarHandler{
		meta:          &nextcore.NextCorePayload{Middleware: middleware},
		hasMiddleware: true,
	}
	for path, want := range map[string]bool{
		"/":                    true,
		"/dashboard":           true,
		"/_next/image":         false,
		"/_next/static/app.js": false,
		"/robots.txt":          false,
		"/api/auth":            true,
	} {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		if got := handler.middlewareMayMatch(request, path); got != want {
			t.Errorf("middleware match for %q = %v, want %v", path, got, want)
		}
	}
}

func TestSidecarImageOptimizationResizesLocalAsset(t *testing.T) {
	release := t.TempDir()
	if err := os.MkdirAll(filepath.Join(release, ".next"), 0o750); err != nil {
		t.Fatal(err)
	}
	publicDir := filepath.Join(release, "public")
	if err := os.MkdirAll(publicDir, 0o750); err != nil {
		t.Fatal(err)
	}
	source := image.NewRGBA(image.Rect(0, 0, 40, 20))
	for y := 0; y < 20; y++ {
		for x := 0; x < 40; x++ {
			source.Set(x, y, color.RGBA{R: 200, A: 255})
		}
	}
	file, err := os.Create(filepath.Join(publicDir, "sample.jpg"))
	if err != nil {
		t.Fatal(err)
	}
	if err := jpeg.Encode(file, source, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	node := httptest.NewServer(http.NotFoundHandler())
	defer node.Close()
	meta := sidecarTestMeta(".next")
	imageOptimization := true
	meta.GoSidecar.ImageOptimization = &imageOptimization
	meta.ImageConfig = &nextcore.ImageConfig{DeviceSizes: []int{20}}
	handler, err := NewSidecarHandler(release, "", filepath.Join(t.TempDir(), "cache"), testHTTPPort(t, node), meta)
	if err != nil {
		t.Fatal(err)
	}
	if handler.imageTransport.DialContext == nil {
		t.Fatal("remote image client is missing the public-IP dialer")
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/_next/image?url=%2Fsample.jpg&w=20&q=80", nil))
	if recorder.Code != http.StatusOK || recorder.Header().Get("Content-Type") != "image/jpeg" {
		t.Fatalf("image response = %d, type %q, body %d bytes", recorder.Code, recorder.Header().Get("Content-Type"), recorder.Body.Len())
	}
	decoded, err := jpeg.Decode(bytes.NewReader(recorder.Body.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Bounds().Dx() != 20 || decoded.Bounds().Dy() != 10 {
		t.Fatalf("optimized dimensions = %v", decoded.Bounds())
	}
	if recorder.Header().Get("X-NextDeploy-Handler") != "go-image" {
		t.Fatalf("image handler header = %q, want go-image", recorder.Header().Get("X-NextDeploy-Handler"))
	}
	if stats := handler.stats.snapshot(); stats.ImageHits != 1 {
		t.Fatalf("image hits = %d, want 1", stats.ImageHits)
	}
}

func TestSidecarRejectsUnlistedRemoteImageHosts(t *testing.T) {
	meta := sidecarTestMeta(".next")
	meta.ImageConfig = &nextcore.ImageConfig{
		RemotePatterns: []nextcore.ImageRemotePattern{{
			Protocol: "https", Hostname: "images.example.com", Pathname: "/assets/**",
			Search: "v=1", SearchConfigured: true,
		}},
	}
	release := t.TempDir()
	if err := os.MkdirAll(filepath.Join(release, ".next"), 0o750); err != nil {
		t.Fatal(err)
	}
	node := httptest.NewServer(http.NotFoundHandler())
	defer node.Close()
	handler, err := NewSidecarHandler(release, "", filepath.Join(t.TempDir(), "cache"), testHTTPPort(t, node), meta)
	if err != nil {
		t.Fatal(err)
	}
	defer handler.Close()
	allowed, _ := url.Parse("https://images.example.com/assets/a.png?v=1")
	denied, _ := url.Parse("https://other.example.com/assets/a.png")
	deniedSearch, _ := url.Parse("https://images.example.com/assets/a.png?v=2")
	if !handler.remoteImageAllowed(allowed) {
		t.Fatal("configured remote image host was rejected")
	}
	if handler.remoteImageAllowed(denied) {
		t.Fatal("unconfigured remote image host was allowed")
	}
	if handler.remoteImageAllowed(deniedSearch) {
		t.Fatal("remote pattern search mismatch was allowed")
	}
	initial, _ := url.Parse("https://images.example.com/assets/a.png?v=1")
	redirected, _ := http.NewRequest(http.MethodGet, "https://images.example.com/private/a.png?v=1", nil)
	if err := handler.imageClient.CheckRedirect(redirected, []*http.Request{{URL: initial}}); err == nil {
		t.Fatal("redirect outside the configured path was allowed")
	}
	if hostnameMatches("*.example.com", "a.b.example.com") {
		t.Fatal("single-level hostname wildcard matched multiple subdomains")
	}
	if !hostnameMatches("*.example.com", "a.example.com") {
		t.Fatal("single-level hostname wildcard rejected one subdomain")
	}
	if !hostnameMatches("**.example.com", "a.b.example.com") ||
		!hostnameMatches("**.example.com", "example.com") {
		t.Fatal("multi-level hostname wildcard did not match expected hosts")
	}
	domainsHandler := &SidecarHandler{meta: &nextcore.NextCorePayload{
		ImageConfig: &nextcore.ImageConfig{Domains: []string{"*.example.com"}},
	}}
	domainURL, _ := url.Parse("https://a.example.com/image.jpg")
	if domainsHandler.remoteImageAllowed(domainURL) {
		t.Fatal("domains entry unexpectedly accepted a wildcard")
	}
	portHandler := &SidecarHandler{meta: &nextcore.NextCorePayload{
		ImageConfig: &nextcore.ImageConfig{RemotePatterns: []nextcore.ImageRemotePattern{{
			Protocol: "https", Hostname: "images.example.com", Pathname: "/assets/**",
		}}},
	}}
	arbitraryPort, _ := url.Parse("https://images.example.com:8443/assets/a.png")
	if !portHandler.remoteImageAllowed(arbitraryPort) {
		t.Fatal("pattern with omitted port did not match Next.js's any-port semantics")
	}
	portHandler.meta.ImageConfig.RemotePatterns[0].Port = "443"
	if portHandler.remoteImageAllowed(arbitraryPort) {
		t.Fatal("explicit remote pattern port accepted a different port")
	}
	if _, err := dialPublicImageHost(context.Background(), "tcp", "127.0.0.1:80"); err == nil {
		t.Fatal("private remote image address was allowed")
	}
	for _, address := range []string{"100.64.0.1", "192.0.2.1", "198.18.0.1", "2001:db8::1", "fc00::1"} {
		ip, err := netip.ParseAddr(address)
		if err != nil {
			t.Fatal(err)
		}
		if isPublicImageIP(ip) {
			t.Errorf("reserved IP %s was allowed", ip)
		}
	}
	for _, address := range []string{"8.8.8.8", "2606:4700:4700::1111"} {
		ip, err := netip.ParseAddr(address)
		if err != nil {
			t.Fatal(err)
		}
		if !isPublicImageIP(ip) {
			t.Errorf("public IP %s was rejected", ip)
		}
	}
}

func TestSidecarJPEGFallbackRespectsConfiguredFormats(t *testing.T) {
	tests := []struct {
		name     string
		accept   string
		config   *nextcore.ImageConfig
		wantJPEG bool
	}{
		{name: "default webp negotiation", accept: "image/avif,image/webp,image/*,*/*", config: &nextcore.ImageConfig{}, wantJPEG: false},
		{name: "jpeg only accepted", accept: "image/jpeg", config: &nextcore.ImageConfig{}, wantJPEG: true},
		{name: "configured webp only", accept: "image/jpeg", config: &nextcore.ImageConfig{Formats: []string{"image/webp"}}, wantJPEG: false},
		{name: "configured jpeg and webp jpeg accepted", accept: "image/jpeg", config: &nextcore.ImageConfig{Formats: []string{"image/jpeg", "image/webp"}}, wantJPEG: true},
		{name: "configured jpeg and webp webp accepted", accept: "image/webp,image/jpeg", config: &nextcore.ImageConfig{Formats: []string{"image/jpeg", "image/webp"}}, wantJPEG: false},
		{name: "webp explicitly rejected", accept: "image/webp;q=0,image/jpeg", config: &nextcore.ImageConfig{}, wantJPEG: true},
		{name: "long zero quality is rejected", accept: "image/webp;q=0.000000,image/jpeg", config: &nextcore.ImageConfig{}, wantJPEG: true},
		{name: "explicit zero beats wildcard", accept: "image/webp;q=0,*/*;q=1", config: &nextcore.ImageConfig{}, wantJPEG: true},
		{name: "positive quality is accepted", accept: "image/webp;q=0.0001,image/jpeg", config: &nextcore.ImageConfig{}, wantJPEG: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := sidecarCanServeJPEG(test.accept, test.config); got != test.wantJPEG {
				t.Fatalf("sidecarCanServeJPEG(%q) = %v, want %v", test.accept, got, test.wantJPEG)
			}
		})
	}
}

func TestSidecarImageLimitsAndQualities(t *testing.T) {
	if got := memoryAwareImageWorkers(64); got != 2 {
		t.Fatalf("memory-aware worker count = %d, want 2 for the configured memory budget", got)
	}
	for _, test := range []struct {
		width, height, limit int
		want                 bool
	}{
		{width: 4096, height: 2048, limit: maxImagePixels, want: true},
		{width: maxImagePixels + 1, height: 1, limit: maxImagePixels, want: false},
		{width: 1, height: 0, limit: maxImagePixels, want: false},
		{width: 1, height: -1, limit: maxImagePixels, want: false},
	} {
		if got := imagePixelCountWithin(test.width, test.height, test.limit); got != test.want {
			t.Errorf("imagePixelCountWithin(%d, %d, %d) = %v, want %v", test.width, test.height, test.limit, got, test.want)
		}
	}
	cfg := &nextcore.ImageConfig{Qualities: []int{50, 75}}
	if !imageQualityAllowed(75, cfg) || imageQualityAllowed(80, cfg) {
		t.Fatal("configured image qualities were not enforced")
	}
	if !imageQualityAllowed(80, &nextcore.ImageConfig{}) {
		t.Fatal("empty configured qualities should preserve Next.js defaults")
	}
}

func TestSidecarImageCacheHitDoesNotUseOptimizationSemaphore(t *testing.T) {
	release := t.TempDir()
	if err := os.MkdirAll(filepath.Join(release, ".next"), 0o750); err != nil {
		t.Fatal(err)
	}
	node := httptest.NewServer(http.NotFoundHandler())
	defer node.Close()
	meta := sidecarTestMeta(".next")
	imageOptimization := true
	meta.GoSidecar.ImageOptimization = &imageOptimization
	meta.ImageConfig = &nextcore.ImageConfig{DeviceSizes: []int{20}}
	cacheDir := t.TempDir()
	handler, err := NewSidecarHandler(release, "", cacheDir, testHTTPPort(t, node), meta)
	if err != nil {
		t.Fatal(err)
	}
	defer handler.Close()
	sourceURL := "/sample.jpg"
	key := sha256.Sum256([]byte(sourceURL + "\x00" + "20" + "\x00" + "75"))
	cached := []byte("cached-jpeg")
	if err := os.WriteFile(filepath.Join(cacheDir, fmt.Sprintf("%x.jpg", key)), cached, 0o600); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < cap(handler.imageSem); i++ {
		handler.imageSem <- struct{}{}
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/_next/image?url=%2Fsample.jpg&w=20", nil))
	if recorder.Code != http.StatusOK || !bytes.Equal(recorder.Body.Bytes(), cached) {
		t.Fatalf("cached response = %d %q", recorder.Code, recorder.Body.Bytes())
	}
	if len(handler.imageSem) != cap(handler.imageSem) {
		t.Fatal("cache hit acquired an image processing slot")
	}
	if recorder.Header().Get("Vary") != "Accept" {
		t.Fatalf("Vary = %q, want Accept", recorder.Header().Get("Vary"))
	}
	head := httptest.NewRecorder()
	handler.ServeHTTP(head, httptest.NewRequest(http.MethodHead, "/_next/image?url=%2Fsample.jpg&w=20", nil))
	if head.Code != http.StatusOK || head.Body.Len() != 0 || head.Header().Get("Content-Length") != strconv.Itoa(len(cached)) {
		t.Fatalf("HEAD response = %d, bytes=%d, length=%q", head.Code, head.Body.Len(), head.Header().Get("Content-Length"))
	}
}

func TestSidecarOnlyOptimizesSourcesWithSupportedSemantics(t *testing.T) {
	jpegSource := image.NewRGBA(image.Rect(0, 0, 2, 2))
	var encodedJPEG bytes.Buffer
	if err := jpeg.Encode(&encodedJPEG, jpegSource, nil); err != nil {
		t.Fatal(err)
	}
	if !sidecarSupportsImageSource(encodedJPEG.Bytes(), "jpeg") {
		t.Fatal("plain JPEG should be supported")
	}
	withEXIF := append(append([]byte(nil), encodedJPEG.Bytes()[:2]...), []byte{0xff, 0xe1, 0x00, 0x0c, 'E', 'x', 'i', 'f', 0x00, 0x00, 0x00, 0x00}...)
	withEXIF = append(withEXIF, encodedJPEG.Bytes()[2:]...)
	for _, metadata := range [][]byte{[]byte("Exif\x00\x00"), []byte("ICC_PROFILE\x00")} {
		withMetadata := append(append([]byte(nil), encodedJPEG.Bytes()...), metadata...)
		if sidecarSupportsImageSource(withMetadata, "jpeg") {
			t.Errorf("JPEG with metadata %q should be delegated to Node", metadata)
		}
	}
	if sidecarSupportsImageSource(withEXIF, "jpeg") {
		t.Fatal("JPEG with EXIF metadata should be delegated to Node")
	}
	transparent := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	var encodedPNG bytes.Buffer
	if err := png.Encode(&encodedPNG, transparent); err != nil {
		t.Fatal(err)
	}
	if sidecarSupportsImageSource(encodedPNG.Bytes(), "png") {
		t.Fatal("potentially transparent PNG should be delegated to Node")
	}
	opaquePNG := image.NewGray(image.Rect(0, 0, 2, 2))
	encodedPNG.Reset()
	if err := png.Encode(&encodedPNG, opaquePNG); err != nil {
		t.Fatal(err)
	}
	if sidecarSupportsImageSource(append(encodedPNG.Bytes(), []byte("acTL")...), "png") {
		t.Fatal("animated PNG should be delegated to Node")
	}
	if sidecarSupportsImageSource([]byte("gif data"), "gif") {
		t.Fatal("potentially animated formats should be delegated to Node")
	}
}

func TestSidecarManagerKeepsReleaseListenerPortsAcrossRestart(t *testing.T) {
	appsRoot := t.TempDir()
	releases := []string{t.TempDir(), t.TempDir()}
	for _, release := range releases {
		if err := os.MkdirAll(filepath.Join(release, ".next"), 0o750); err != nil {
			t.Fatal(err)
		}
	}
	nodeListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	nodePort := nodeListener.Addr().(*net.TCPAddr).Port
	_ = nodeListener.Close()

	manager := newSidecarManager(appsRoot)
	var logs synchronizedBuffer
	manager.SetLogger(log.New(&logs, "", 0))
	first, err := manager.Upsert("demo-app", releases[0], "", nodePort, sidecarTestMeta(".next"))
	if err != nil {
		t.Fatal(err)
	}
	firstAgain, err := manager.Upsert("demo-app", releases[0], "", nodePort, sidecarTestMeta(".next"))
	if err != nil {
		t.Fatal(err)
	}
	if first != firstAgain {
		t.Fatalf("same release changed port from %d to %d", first, firstAgain)
	}
	second, err := manager.Upsert("demo-app", releases[1], "", nodePort, sidecarTestMeta(".next"))
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatalf("different releases share listener port %d", first)
	}
	if !strings.Contains(logs.String(), "hot-swapped route table for demo-app") {
		t.Fatalf("sidecar manager did not use configured logger: %q", logs.String())
	}
	if err := manager.StopRelease("demo-app", releases[0]); err != nil {
		t.Fatal(err)
	}
	restored, err := manager.Upsert("demo-app", releases[0], "", nodePort, sidecarTestMeta(".next"))
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Shutdown()
	if restored != first {
		t.Fatalf("restored port = %d, want persisted port %d", restored, first)
	}
	if err := manager.StopRelease("demo-app", releases[0]); err != nil {
		t.Fatal(err)
	}
	response, err := http.Get("http://127.0.0.1:" + strconv.Itoa(restored) + sidecarHealthURI)
	if err == nil {
		_ = response.Body.Close()
		t.Fatal("retired release listener remained active")
	}
	response, err = http.Get("http://127.0.0.1:" + strconv.Itoa(second) + sidecarHealthURI)
	if err != nil {
		t.Fatalf("other release listener was interrupted: %v", err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("other release health status = %d", response.StatusCode)
	}
}

func TestISRRevalidationPublishesOnlyConfirmedNodeResponse(t *testing.T) {
	var requests atomic.Int32
	node := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if requests.Add(1) == 1 {
			w.Header().Set("X-Nextjs-Cache", "STALE")
			_, _ = w.Write([]byte("stale"))
			return
		}
		w.Header().Set("X-Nextjs-Cache", "REVALIDATED")
		_, _ = w.Write([]byte("fresh"))
	}))
	defer node.Close()
	port := testHTTPPort(t, node)
	transport := &http.Transport{}
	defer transport.CloseIdleConnections()
	handler := &SidecarHandler{
		nodeOrigin:    &url.URL{Scheme: "http", Host: "127.0.0.1:" + strconv.Itoa(port)},
		nodeTransport: transport,
	}
	entry := &cachedEntry{htmlBody: []byte("old"), htmlUpdated: time.Now().Add(-time.Hour), revalidate: time.Second}
	request := httptest.NewRequest(http.MethodGet, "/page", nil)
	handler.triggerBackgroundRevalidation("/page", entry, request, false)

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		entry.mu.RLock()
		body := string(entry.htmlBody)
		entry.mu.RUnlock()
		if body == "fresh" {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("revalidated body = %q after %d requests", entry.htmlBody, requests.Load())
}

func TestISRTracksHTMLAndRSCFreshnessIndependently(t *testing.T) {
	node := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Nextjs-Cache", "REVALIDATED")
		_, _ = w.Write([]byte("new-html"))
	}))
	defer node.Close()
	transport := &http.Transport{}
	defer transport.CloseIdleConnections()
	handler := &SidecarHandler{
		nodeOrigin:    &url.URL{Scheme: "http", Host: strings.TrimPrefix(node.URL, "http://")},
		nodeTransport: transport,
	}
	old := time.Now().Add(-time.Hour)
	fresh := time.Now()
	entry := &cachedEntry{
		htmlBody:    []byte("old-html"),
		rscBody:     []byte("fresh-rsc"),
		htmlUpdated: old,
		rscUpdated:  fresh,
		revalidate:  time.Second,
	}

	rscRecorder := httptest.NewRecorder()
	if !handler.servePrerendered(rscRecorder, httptest.NewRequest(http.MethodGet, "/page", nil), "/page", entry, true) {
		t.Fatal("fresh RSC representation was not served")
	}
	if rscRecorder.Body.String() != "fresh-rsc" || rscRecorder.Header().Get("X-NextDeploy-Cache") != "HIT" {
		t.Fatalf("RSC response = %q, cache = %q", rscRecorder.Body.String(), rscRecorder.Header().Get("X-NextDeploy-Cache"))
	}

	etag := `"` + fmt.Sprintf("%x", sha256.Sum256(entry.htmlBody)) + `"`
	htmlRequest := httptest.NewRequest(http.MethodGet, "/page", nil)
	htmlRequest.Header.Set("If-None-Match", etag)
	htmlRecorder := httptest.NewRecorder()
	if !handler.servePrerendered(htmlRecorder, htmlRequest, "/page", entry, false) {
		t.Fatal("stale HTML representation was not served conditionally")
	}
	if htmlRecorder.Code != http.StatusNotModified || htmlRecorder.Header().Get("X-NextDeploy-Cache") != "STALE" {
		t.Fatalf("stale conditional response = %d, cache = %q", htmlRecorder.Code, htmlRecorder.Header().Get("X-NextDeploy-Cache"))
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		entry.mu.RLock()
		htmlBody, htmlUpdated, rscBody, rscUpdated := string(entry.htmlBody), entry.htmlUpdated, string(entry.rscBody), entry.rscUpdated
		entry.mu.RUnlock()
		if htmlBody == "new-html" {
			if !htmlUpdated.After(old) || rscBody != "fresh-rsc" || !rscUpdated.Equal(fresh) {
				t.Fatalf("representation state after refresh: html=%q @ %v, RSC=%q @ %v", htmlBody, htmlUpdated, rscBody, rscUpdated)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("stale HTML was not refreshed")
}

func TestPruneSharedStaticRetainsReferencedFilesAndLeavesSymlinks(t *testing.T) {
	appDir := t.TempDir()
	for id, asset := range map[string]string{"1-old": "old.js", "2-new": "new.js"} {
		release := filepath.Join(appDir, "releases", id)
		static := filepath.Join(release, ".next", "static")
		if err := os.MkdirAll(static, 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Join(release, ".nextdeploy"), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(release, ".nextdeploy", "metadata.json"), []byte(`{"dist_dir":".next"}`), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(static, asset), []byte(asset), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	shared := filepath.Join(appDir, "shared_static")
	if err := os.MkdirAll(shared, 0o750); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"old.js", "new.js", "orphan.js"} {
		if err := os.WriteFile(filepath.Join(shared, name), []byte(name), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink("orphan.js", filepath.Join(shared, "keep-link")); err != nil {
		t.Fatal(err)
	}
	if err := pruneSharedStaticAt(appDir, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(shared, "new.js")); err != nil {
		t.Fatal("retained release asset was removed")
	}
	for _, name := range []string{"old.js", "orphan.js"} {
		if _, err := os.Stat(filepath.Join(shared, name)); !os.IsNotExist(err) {
			t.Fatalf("unreferenced asset %q still exists or stat failed: %v", name, err)
		}
	}
	if _, err := os.Lstat(filepath.Join(shared, "keep-link")); err != nil {
		t.Fatalf("symlink was removed: %v", err)
	}
}
