package daemon

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/aynaash/nextdeploy/shared/nextcore"
)

const (
	sidecarHealthURI = "/_nextdeploy/sidecar-healthz"
)

type cachedEntry struct {
	htmlPath    string
	rscPath     string
	htmlBody    []byte
	rscBody     []byte
	revalidate  time.Duration
	htmlUpdated time.Time
	rscUpdated  time.Time
	htmlReval   atomic.Bool
	rscReval    atomic.Bool
	mu          sync.RWMutex
}

type SidecarHandler struct {
	releaseDir    string
	distDir       string
	publicDir     string
	sharedStatic  string
	imageCacheDir string
	basePath      string

	meta          *nextcore.NextCorePayload
	routes        map[string]*cachedEntry
	hasMiddleware bool

	imageSem       chan struct{}
	deviceSizes    map[int]struct{}
	nodeOrigin     *url.URL
	nodeTransport  *http.Transport
	nodeProxy      *httputil.ReverseProxy
	imageClient    *http.Client
	imageTransport *http.Transport
	logger         *log.Logger
	ctx            context.Context
	cancel         context.CancelFunc
	lifecycleMu    sync.Mutex
	closed         bool
	revalidations  sync.WaitGroup
}

func NewSidecarHandler(releaseDir, sharedStatic, imageCacheDir string, nodePort int, meta *nextcore.NextCorePayload) (*SidecarHandler, error) {
	if meta == nil {
		return nil, fmt.Errorf("sidecar metadata is required")
	}
	if nodePort < 1 || nodePort > 65535 {
		return nil, fmt.Errorf("invalid Node.js port %d", nodePort)
	}
	if !filepath.IsAbs(releaseDir) {
		return nil, fmt.Errorf("release directory must be absolute")
	}
	distDir := meta.DistDir
	if distDir == "" {
		distDir = ".next"
	}
	if !filepath.IsLocal(distDir) {
		return nil, fmt.Errorf("invalid Next.js dist directory %q", distDir)
	}
	resolvedDist := filepath.Join(releaseDir, distDir)
	if _, err := os.Stat(resolvedDist); err != nil {
		if !os.IsNotExist(err) || distDir == ".next" {
			return nil, fmt.Errorf("stat Next.js dist directory %s: %w", resolvedDist, err)
		}
		legacyDist := filepath.Join(releaseDir, ".next")
		if _, legacyErr := os.Stat(legacyDist); legacyErr != nil {
			return nil, fmt.Errorf("Next.js dist directory %s is unavailable: %w", resolvedDist, err)
		}
		resolvedDist = legacyDist
	}
	if sharedStatic == "" {
		sharedStatic = filepath.Join(resolvedDist, "static")
	}
	if err := os.MkdirAll(imageCacheDir, 0o750); err != nil {
		return nil, fmt.Errorf("create image cache directory: %w", err)
	}

	nodeOrigin := &url.URL{Scheme: "http", Host: fmt.Sprintf("127.0.0.1:%d", nodePort)}
	var h *SidecarHandler
	transport := &http.Transport{
		MaxIdleConns:        512,
		MaxIdleConnsPerHost: 512,
		IdleConnTimeout:     90 * time.Second,
	}
	proxy := httputil.NewSingleHostReverseProxy(nodeOrigin)
	proxy.Transport = transport
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		if h != nil {
			h.logf("[gosidecar] proxy to Node.js failed: %v", err)
		} else {
			log.Printf("[gosidecar] proxy to Node.js failed: %v", err)
		}
		http.Error(w, "upstream application unavailable", http.StatusBadGateway)
	}

	workers := 4
	if meta.GoSidecar != nil {
		workers = meta.GoSidecar.Workers()
	}
	workers = memoryAwareImageWorkers(workers)
	deviceSizes := make(map[int]struct{})
	if meta.ImageConfig != nil {
		sizes := append(append([]int(nil), meta.ImageConfig.DeviceSizes...), meta.ImageConfig.ImageSizes...)
		for _, size := range sizes {
			if size > 0 {
				deviceSizes[size] = struct{}{}
			}
		}
	}
	if len(deviceSizes) == 0 {
		for _, size := range []int{640, 750, 828, 1080, 1200, 1920, 2048, 3840} {
			deviceSizes[size] = struct{}{}
		}
	}
	handlerCtx, cancel := context.WithCancel(context.Background())
	h = &SidecarHandler{
		releaseDir:    releaseDir,
		distDir:       resolvedDist,
		publicDir:     filepath.Join(releaseDir, "public"),
		sharedStatic:  sharedStatic,
		imageCacheDir: imageCacheDir,
		basePath:      strings.TrimSuffix(meta.BasePath, "/"),
		meta:          meta,
		routes:        make(map[string]*cachedEntry),
		hasMiddleware: (meta.Middleware != nil && len(meta.Middleware.Matchers) > 0) || len(meta.RouteInfo.MiddlewareRoutes) > 0,
		imageSem:      make(chan struct{}, workers),
		deviceSizes:   deviceSizes,
		nodeOrigin:    nodeOrigin,
		nodeTransport: transport,
		nodeProxy:     proxy,
		logger:        log.Default(),
		ctx:           handlerCtx,
		cancel:        cancel,
	}
	h.imageTransport = &http.Transport{
		Proxy:                 nil,
		DialContext:           dialPublicImageHost,
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
		IdleConnTimeout:       30 * time.Second,
	}
	h.imageClient = &http.Client{
		Transport: h.imageTransport,
		Timeout:   15 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 || !h.remoteImageAllowed(req.URL) {
				return fmt.Errorf("image redirect is not allowed")
			}
			return nil
		},
	}
	if meta.GoSidecar != nil && meta.GoSidecar.ISRCacheEnabled() {
		h.indexPrerenderedRoutes()
	}
	return h, nil
}

func (s *SidecarHandler) Close() {
	s.lifecycleMu.Lock()
	if !s.closed {
		s.closed = true
		if s.cancel != nil {
			s.cancel()
		}
	}
	s.lifecycleMu.Unlock()
	s.revalidations.Wait()
	if s.nodeTransport != nil {
		s.nodeTransport.CloseIdleConnections()
	}
	if s.imageTransport != nil {
		s.imageTransport.CloseIdleConnections()
	}
}

func (s *SidecarHandler) startRevalidation(run func()) bool {
	s.lifecycleMu.Lock()
	if s.closed {
		s.lifecycleMu.Unlock()
		return false
	}
	s.revalidations.Add(1)
	s.lifecycleMu.Unlock()
	go func() {
		defer s.revalidations.Done()
		run()
	}()
	return true
}

func (s *SidecarHandler) indexPrerenderedRoutes() {
	isrTTLs := make(map[string]int, len(s.meta.RouteInfo.ISRDetail))
	for _, detail := range s.meta.RouteInfo.ISRDetail {
		isrTTLs[detail.Path] = detail.Revalidate
	}
	for route := range s.meta.RouteInfo.SSGRoutes {
		if entry := s.loadRouteFiles(route, 0); entry != nil {
			s.routes[route] = entry
		}
	}
	for route := range s.meta.RouteInfo.ISRRoutes {
		if ttl := isrTTLs[route]; ttl > 0 {
			if entry := s.loadRouteFiles(route, time.Duration(ttl)*time.Second); entry != nil {
				s.routes[route] = entry
			}
		}
	}
}

func (s *SidecarHandler) loadRouteFiles(route string, revalidate time.Duration) *cachedEntry {
	relative, err := routeFileName(route)
	if err != nil {
		s.logf("[gosidecar] ignoring unsafe prerender route %q: %v", route, err)
		return nil
	}
	for _, router := range []string{"app", "pages"} {
		root := filepath.Join(s.distDir, "server", router)
		htmlPath, html, htmlModTime, err := readWithinRoot(root, relative+".html")
		if err != nil {
			continue
		}
		entry := &cachedEntry{
			htmlPath:    htmlPath,
			revalidate:  revalidate,
			htmlUpdated: htmlModTime,
			htmlBody:    html,
		}
		if rscPath, rsc, rscModTime, rscErr := readWithinRoot(root, relative+".rsc"); rscErr == nil {
			entry.rscPath = rscPath
			entry.rscBody = rsc
			entry.rscUpdated = rscModTime
		}
		if generatedAt, parseErr := time.Parse(time.RFC3339, s.meta.GeneratedAt); parseErr == nil {
			if generatedAt.Before(entry.htmlUpdated) {
				entry.htmlUpdated = generatedAt
			}
			if !entry.rscUpdated.IsZero() && generatedAt.Before(entry.rscUpdated) {
				entry.rscUpdated = generatedAt
			}
		}
		return entry
	}
	return nil
}

func routeFileName(route string) (string, error) {
	route = strings.TrimPrefix(route, "/")
	if route == "" {
		route = "index"
	}
	relative := filepath.FromSlash(route)
	if !filepath.IsLocal(relative) {
		return "", fmt.Errorf("route escapes its root")
	}
	return relative, nil
}

func readWithinRoot(root, relative string) (string, []byte, time.Time, error) {
	if !filepath.IsLocal(relative) {
		return "", nil, time.Time{}, fmt.Errorf("path is not local")
	}
	rootHandle, err := os.OpenRoot(root)
	if err != nil {
		return "", nil, time.Time{}, err
	}
	defer rootHandle.Close()
	file, err := rootHandle.Open(relative)
	if err != nil {
		return "", nil, time.Time{}, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return "", nil, time.Time{}, err
	}
	if !info.Mode().IsRegular() {
		return "", nil, time.Time{}, fmt.Errorf("prerender path is not a regular file")
	}
	body, err := io.ReadAll(io.LimitReader(file, maxRouteBody+1))
	if err != nil {
		return "", nil, time.Time{}, err
	}
	if len(body) > maxRouteBody {
		return "", nil, time.Time{}, fmt.Errorf("prerender file exceeded %d bytes", maxRouteBody)
	}
	return filepath.Join(root, relative), body, info.ModTime(), nil
}

func (s *SidecarHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	parent := s.ctx
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithCancel(r.Context())
	stop := context.AfterFunc(parent, cancel)
	defer func() {
		stop()
		cancel()
	}()
	r = r.WithContext(ctx)

	w.Header().Set("X-NextDeploy-Sidecar", "go")
	if r.URL.Path == sidecarHealthURI {
		w.WriteHeader(http.StatusOK)
		return
	}
	reqPath := r.URL.Path
	if s.basePath != "" && (reqPath == s.basePath || strings.HasPrefix(reqPath, s.basePath+"/")) {
		reqPath = strings.TrimPrefix(reqPath, s.basePath)
		if reqPath == "" {
			reqPath = "/"
		}
	}
	middlewareMatch := s.middlewareMayMatch(r, reqPath)
	eligible := (r.Method == http.MethodGet || r.Method == http.MethodHead) &&
		(r.Body == nil || r.ContentLength == 0) && !hasBypassCookie(r) &&
		r.Header.Get("Next-Action") == ""

	if eligible && !middlewareMatch && reqPath == "/_next/image" &&
		s.meta.GoSidecar != nil && s.meta.GoSidecar.ImageOptEnabled() {
		s.handleImageOptimization(w, r)
		return
	}
	if eligible && !middlewareMatch {
		cleanRoute := reqPath
		if cleanRoute != "/" {
			cleanRoute = strings.TrimSuffix(cleanRoute, "/")
		}
		isRSC := r.Header.Get("RSC") == "1" || strings.HasSuffix(cleanRoute, ".rsc")
		if strings.HasSuffix(cleanRoute, ".rsc") {
			cleanRoute = strings.TrimSuffix(cleanRoute, ".rsc")
			if cleanRoute == "" {
				cleanRoute = "/"
			}
		}
		if entry, ok := s.routes[cleanRoute]; ok && len(r.URL.RawQuery) == 0 {
			if s.servePrerendered(w, r, cleanRoute, entry, isRSC) {
				return
			}
		}
	}
	w.Header().Set("X-NextDeploy-Cache", "MISS-SSR")
	s.nodeProxy.ServeHTTP(w, r)
}

func (s *SidecarHandler) logf(format string, args ...any) {
	logger := s.logger
	if logger == nil {
		logger = log.Default()
	}
	logger.Printf(format, args...)
}

func hasBypassCookie(r *http.Request) bool {
	for _, name := range []string{"__prerender_bypass", "__next_preview_data"} {
		if cookie, err := r.Cookie(name); err == nil && cookie.Value != "" {
			return true
		}
	}
	return false
}
