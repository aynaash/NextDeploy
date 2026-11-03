package daemon

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"io"
	"math"
	"mime"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/aynaash/nextdeploy/shared/nextcore"
	xdraw "golang.org/x/image/draw"
)

const (
	maxImageBytes           = 20 << 20
	maxImagePixels          = 12_000_000
	maxImageOutputPixels    = 8_000_000
	maxImageOutputDimension = 4096
	imageMemoryBudget       = 256 << 20
	maxEncodedImageBytes    = 20 << 20
)

func (s *SidecarHandler) handleImageOptimization(w http.ResponseWriter, r *http.Request) {
	cfg := s.meta.ImageConfig
	if cfg != nil && (cfg.Unoptimized || (cfg.Loader != "" && cfg.Loader != "default")) {
		s.nodeProxy.ServeHTTP(w, r)
		return
	}
	if !sidecarCanServeJPEG(r.Header.Get("Accept"), cfg) {
		s.nodeProxy.ServeHTTP(w, r)
		return
	}

	query := r.URL.Query()
	if len(query["url"]) != 1 || len(query["w"]) != 1 || len(query["q"]) > 1 {
		http.Error(w, "invalid image optimization query", http.StatusBadRequest)
		return
	}
	width, err := strconv.Atoi(query.Get("w"))
	if err != nil || width <= 0 {
		http.Error(w, "invalid image width", http.StatusBadRequest)
		return
	}
	if _, allowed := s.deviceSizes[width]; !allowed {
		http.Error(w, "image width is not configured", http.StatusBadRequest)
		return
	}
	quality := 75
	if query.Get("q") != "" {
		quality, err = strconv.Atoi(query.Get("q"))
		if err != nil || quality < 1 || quality > 100 {
			http.Error(w, "invalid image quality", http.StatusBadRequest)
			return
		}
	}
	if !imageQualityAllowed(quality, cfg) {
		http.Error(w, "image quality is not configured", http.StatusBadRequest)
		return
	}

	sourceURL := query.Get("url")
	cacheKey := sha256.Sum256([]byte(sourceURL + "\x00" + strconv.Itoa(width) + "\x00" + strconv.Itoa(quality)))
	cachePath := filepath.Join(s.imageCacheDir, hex.EncodeToString(cacheKey[:])+".jpg")
	cacheTTL := 60
	if cfg != nil && cfg.MinimumCacheTTL > cacheTTL {
		cacheTTL = cfg.MinimumCacheTTL
	}
	if info, err := os.Stat(cachePath); err == nil && time.Since(info.ModTime()) < time.Duration(cacheTTL)*time.Second {
		if data, readErr := os.ReadFile(cachePath); readErr == nil {
			s.writeOptimizedImage(w, r, data, cacheTTL)
			return
		} else {
			s.logf("[gosidecar] read optimized image cache %q: %v", cachePath, readErr)
		}
	} else if err != nil && !os.IsNotExist(err) {
		s.logf("[gosidecar] inspect optimized image cache %q: %v", cachePath, err)
	}

	select {
	case s.imageSem <- struct{}{}:
		defer func() { <-s.imageSem }()
	case <-r.Context().Done():
		return
	default:
		http.Error(w, "image optimizer is busy", http.StatusServiceUnavailable)
		return
	}

	source, err := s.loadImageSource(r.Context(), sourceURL)
	if err != nil {
		if errors.Is(err, errImageSourceTooLarge) {
			s.nodeProxy.ServeHTTP(w, r)
			return
		}
		status := http.StatusBadGateway
		if strings.Contains(err.Error(), "not permitted") {
			status = http.StatusForbidden
		} else if strings.Contains(err.Error(), "invalid") {
			status = http.StatusBadRequest
		}
		http.Error(w, err.Error(), status)
		return
	}
	imageConfig, sourceFormat, err := image.DecodeConfig(bytes.NewReader(source))
	if err != nil {
		s.nodeProxy.ServeHTTP(w, r)
		return
	}
	if !imagePixelCountWithin(imageConfig.Width, imageConfig.Height, maxImagePixels) {
		s.nodeProxy.ServeHTTP(w, r)
		return
	}
	if !sidecarSupportsImageSource(source, sourceFormat) {
		s.nodeProxy.ServeHTTP(w, r)
		return
	}
	decoded, format, err := image.Decode(bytes.NewReader(source))
	if err != nil {
		s.nodeProxy.ServeHTTP(w, r)
		return
	}
	if format != "jpeg" && format != "png" {
		s.nodeProxy.ServeHTTP(w, r)
		return
	}
	if _, cmyk := decoded.(*image.CMYK); cmyk {
		s.nodeProxy.ServeHTTP(w, r)
		return
	}
	bounds := decoded.Bounds()
	targetWidth := min(width, bounds.Dx())
	targetHeight := max(1, int(math.Round(float64(bounds.Dy())*float64(targetWidth)/float64(bounds.Dx()))))
	if targetWidth > maxImageOutputDimension || targetHeight > maxImageOutputDimension ||
		!imagePixelCountWithin(targetWidth, targetHeight, maxImageOutputPixels) {
		s.nodeProxy.ServeHTTP(w, r)
		return
	}
	resized := image.NewRGBA(image.Rect(0, 0, targetWidth, targetHeight))
	draw.Draw(resized, resized.Bounds(), &image.Uniform{C: color.White}, image.Point{}, draw.Src)
	xdraw.CatmullRom.Scale(resized, resized.Bounds(), decoded, bounds, draw.Over, nil)
	encoded := &limitedImageBuffer{limit: maxEncodedImageBytes}
	if err := jpeg.Encode(encoded, resized, &jpeg.Options{Quality: quality}); err != nil {
		if errors.Is(err, errImageOutputTooLarge) {
			s.nodeProxy.ServeHTTP(w, r)
			return
		}
		http.Error(w, "failed to encode optimized image", http.StatusInternalServerError)
		return
	}
	output := encoded.Bytes()
	if err := writeAtomicFile(cachePath, output); err != nil {
		s.logf("[gosidecar] cache optimized image: %v", err)
	}
	s.writeOptimizedImage(w, r, output, cacheTTL)
}

func sidecarCanServeJPEG(accept string, cfg *nextcore.ImageConfig) bool {
	formats := []string{"image/webp"}
	if cfg != nil && len(cfg.Formats) > 0 {
		formats = cfg.Formats
	}
	jpegConfigured := false
	for _, format := range formats {
		format = strings.ToLower(strings.TrimSpace(format))
		if format == "image/jpeg" || format == "image/jpg" {
			jpegConfigured = true
			continue
		}
		if acceptsImageFormat(accept, format) {
			return false
		}
	}
	return jpegConfigured || cfg == nil || len(cfg.Formats) == 0
}

func acceptsImageFormat(accept, format string) bool {
	bestSpecificity := -1
	bestQuality := 0.0
	for _, item := range strings.Split(accept, ",") {
		mediaType, params, err := mime.ParseMediaType(strings.TrimSpace(item))
		if err != nil {
			continue
		}
		quality := 1.0
		if q, exists := params["q"]; exists {
			quality, err = strconv.ParseFloat(q, 64)
			if err != nil || quality < 0 || quality > 1 {
				continue
			}
		}
		specificity := -1
		switch mediaType {
		case format:
			specificity = 2
		case "image/*":
			specificity = 1
		case "*/*":
			specificity = 0
		}
		if specificity > bestSpecificity {
			bestSpecificity = specificity
			bestQuality = quality
		} else if specificity == bestSpecificity && quality > bestQuality {
			bestQuality = quality
		}
	}
	return bestSpecificity >= 0 && bestQuality > 0
}

func imageQualityAllowed(quality int, cfg *nextcore.ImageConfig) bool {
	if cfg == nil || len(cfg.Qualities) == 0 {
		return true
	}
	for _, allowed := range cfg.Qualities {
		if quality == allowed {
			return true
		}
	}
	return false
}

func memoryAwareImageWorkers(configured int) int {
	if configured < 1 {
		configured = 1
	}
	perWorker := int64(maxImagePixels*4 + maxImageOutputPixels*4 + 2*maxImageBytes)
	memoryWorkers := int(int64(imageMemoryBudget) / perWorker)
	if memoryWorkers < 1 {
		memoryWorkers = 1
	}
	return min(configured, memoryWorkers)
}

func imagePixelCountWithin(width, height, limit int) bool {
	return width > 0 && height > 0 && limit > 0 && width <= limit/height
}

func sidecarSupportsImageSource(source []byte, format string) bool {
	switch format {
	case "jpeg":
		return !jpegHasUnsupportedMetadata(source)
	case "png":
		return !pngRequiresNode(source)
	default:
		return false
	}
}

func jpegHasUnsupportedMetadata(source []byte) bool {
	return bytes.Contains(source, []byte("Exif\x00\x00")) ||
		bytes.Contains(source, []byte("ICC_PROFILE\x00"))
}

func pngRequiresNode(source []byte) bool {
	if len(source) < 26 || !bytes.Equal(source[:8], []byte{137, 80, 78, 71, 13, 10, 26, 10}) {
		return true
	}
	if source[24] > 8 {
		return true
	}
	if source[25] == 4 || source[25] == 6 {
		return true
	}
	for _, chunkType := range [][]byte{
		[]byte("tRNS"),
		[]byte("acTL"),
		[]byte("iCCP"),
		[]byte("sRGB"),
		[]byte("gAMA"),
	} {
		if bytes.Contains(source, chunkType) {
			return true
		}
	}
	return false
}

var errImageOutputTooLarge = errors.New("encoded image exceeds output size limit")
var errImageSourceTooLarge = errors.New("image source exceeds input size limit")

type limitedImageBuffer struct {
	buffer bytes.Buffer
	limit  int
}

func (b *limitedImageBuffer) Write(p []byte) (int, error) {
	if b.buffer.Len()+len(p) > b.limit {
		return 0, errImageOutputTooLarge
	}
	return b.buffer.Write(p)
}

func (b *limitedImageBuffer) Bytes() []byte {
	return b.buffer.Bytes()
}

func (s *SidecarHandler) loadImageSource(ctx context.Context, source string) ([]byte, error) {
	if strings.HasPrefix(source, "/") && !strings.HasPrefix(source, "//") {
		decodedPath, err := url.PathUnescape(source)
		if err != nil {
			return nil, fmt.Errorf("invalid local image path")
		}
		for _, segment := range strings.Split(decodedPath, "/") {
			if segment == ".." {
				return nil, fmt.Errorf("invalid local image path")
			}
		}
		relative := strings.TrimPrefix(path.Clean(decodedPath), "/")
		if relative == "" || relative == "." || !filepath.IsLocal(filepath.FromSlash(relative)) {
			return nil, fmt.Errorf("invalid local image path")
		}
		root, err := os.OpenRoot(s.publicDir)
		if err != nil {
			return nil, fmt.Errorf("open public image directory: %w", err)
		}
		defer root.Close()
		file, err := root.Open(filepath.FromSlash(relative))
		if err != nil {
			return nil, fmt.Errorf("open local image: %w", err)
		}
		defer file.Close()
		info, err := file.Stat()
		if err != nil {
			return nil, fmt.Errorf("stat local image: %w", err)
		}
		if !info.Mode().IsRegular() {
			return nil, fmt.Errorf("invalid local image")
		}
		if info.Size() > maxImageBytes {
			return nil, errImageSourceTooLarge
		}
		data, err := io.ReadAll(io.LimitReader(file, maxImageBytes+1))
		if err != nil {
			return nil, fmt.Errorf("read local image: %w", err)
		}
		if len(data) > maxImageBytes {
			return nil, errImageSourceTooLarge
		}
		return data, nil
	}
	parsed, err := url.Parse(source)
	if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.User != nil || parsed.Host == "" {
		return nil, fmt.Errorf("invalid remote image URL")
	}
	if !s.remoteImageAllowed(parsed) {
		return nil, fmt.Errorf("remote image host is not permitted")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("create remote image request: %w", err)
	}
	resp, err := s.imageClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch remote image: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("remote image returned HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxImageBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read remote image: %w", err)
	}
	if len(data) > maxImageBytes {
		return nil, errImageSourceTooLarge
	}
	return data, nil
}

func dialPublicImageHost(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, fmt.Errorf("invalid remote image address: %w", err)
	}
	ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, fmt.Errorf("resolve remote image host: %w", err)
	}
	if len(ips) == 0 {
		return nil, fmt.Errorf("remote image host resolved to no addresses")
	}
	for _, resolved := range ips {
		ip, ok := netip.AddrFromSlice(resolved.IP)
		if !ok || !isPublicImageIP(ip.Unmap()) {
			return nil, fmt.Errorf("remote image host resolves to a non-public address")
		}
	}
	dialer := &net.Dialer{Timeout: 5 * time.Second, KeepAlive: 30 * time.Second}
	var lastErr error
	for _, resolved := range ips {
		conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(resolved.IP.String(), port))
		if err == nil {
			return conn, nil
		}
		lastErr = err
	}
	return nil, fmt.Errorf("connect to remote image host: %w", lastErr)
}

func isPublicImageIP(ip netip.Addr) bool {
	if !ip.IsValid() || !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsUnspecified() || ip.IsMulticast() {
		return false
	}
	reserved := []string{
		"100.64.0.0/10",
		"192.0.0.0/24",
		"192.0.2.0/24",
		"192.88.99.0/24",
		"198.18.0.0/15",
		"198.51.100.0/24",
		"203.0.113.0/24",
		"240.0.0.0/4",
		"2001::/23",
		"2001:db8::/32",
		"2002::/16",
		"3fff::/20",
	}
	if ip.Is4() {
		for _, rawPrefix := range reserved[:8] {
			prefix := netip.MustParsePrefix(rawPrefix)
			if prefix.Contains(ip) {
				return false
			}
		}
		return true
	}
	if !netip.MustParsePrefix("2000::/3").Contains(ip) {
		return false
	}
	for _, rawPrefix := range reserved[8:] {
		prefix := netip.MustParsePrefix(rawPrefix)
		if prefix.Contains(ip) {
			return false
		}
	}
	return true
}

func (s *SidecarHandler) remoteImageAllowed(target *url.URL) bool {
	cfg := s.meta.ImageConfig
	if cfg == nil {
		return false
	}
	for _, pattern := range cfg.RemotePatterns {
		if pattern.Hostname == "" || !hostnameMatches(pattern.Hostname, target.Hostname()) {
			continue
		}
		if pattern.Protocol != "" && strings.TrimSuffix(pattern.Protocol, ":") != target.Scheme {
			continue
		}
		if pattern.Port != "" && pattern.Port != target.Port() {
			continue
		}
		if pattern.Pathname != "" && !wildcardMatch(pattern.Pathname, target.Path) {
			continue
		}
		if pattern.SearchConfigured && target.RawQuery != strings.TrimPrefix(pattern.Search, "?") {
			continue
		}
		return true
	}
	for _, domain := range cfg.Domains {
		if exactHostnameMatches(domain, target.Hostname()) {
			return true
		}
	}
	return false
}

func hostnameMatches(pattern, host string) bool {
	pattern = strings.ToLower(strings.TrimSuffix(pattern, "."))
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	if strings.HasPrefix(pattern, "**.") {
		suffix := strings.TrimPrefix(pattern, "**.")
		return host == suffix || strings.HasSuffix(host, "."+suffix)
	}
	if strings.HasPrefix(pattern, "*.") {
		suffix := strings.TrimPrefix(pattern, "*")
		if !strings.HasSuffix(host, suffix) {
			return false
		}
		subdomain := strings.TrimSuffix(host, suffix)
		return subdomain != "" && !strings.Contains(subdomain, ".")
	}
	return pattern == host
}

func exactHostnameMatches(pattern, host string) bool {
	return strings.EqualFold(strings.TrimSuffix(pattern, "."), strings.TrimSuffix(host, "."))
}

func wildcardMatch(pattern, value string) bool {
	expr := regexp.QuoteMeta(pattern)
	expr = strings.ReplaceAll(expr, `\*\*`, `.*`)
	expr = strings.ReplaceAll(expr, `\*`, `[^/]*`)
	re, err := regexp.Compile("^" + expr + "$")
	return err == nil && re.MatchString(value)
}

func writeAtomicFile(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".image-cache-*")
	if err != nil {
		return fmt.Errorf("create image cache temp file: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(0o640); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("chmod image cache: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write image cache: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("sync image cache: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close image cache: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("commit image cache: %w", err)
	}
	return nil
}

func (s *SidecarHandler) writeOptimizedImage(w http.ResponseWriter, r *http.Request, data []byte, ttl int) {
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", fmt.Sprintf("public, max-age=%d", ttl))
	w.Header().Add("Vary", "Accept")
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.WriteHeader(http.StatusOK)
	if r.Method == http.MethodHead {
		return
	}
	if _, err := w.Write(data); err != nil {
		s.logf("[gosidecar] write optimized image: %v", err)
	}
}
