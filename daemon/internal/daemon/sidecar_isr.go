package daemon

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const maxRouteBody = 32 << 20

func (s *SidecarHandler) servePrerendered(w http.ResponseWriter, r *http.Request, route string, entry *cachedEntry, isRSC bool) bool {
	entry.mu.RLock()
	body := entry.htmlBody
	contentType := "text/html; charset=utf-8"
	if isRSC {
		body = entry.rscBody
		contentType = "text/x-component"
	}
	body = append([]byte(nil), body...)
	lastUpdated, ttl := entry.htmlUpdated, entry.revalidate
	if isRSC {
		lastUpdated = entry.rscUpdated
	}
	entry.mu.RUnlock()
	if len(body) == 0 {
		return false
	}
	stale := ttl > 0 && time.Since(lastUpdated) >= ttl
	if isRSC && stale {
		return false
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "private, no-cache")
	w.Header().Add("Vary", "RSC")
	etag := `"` + fmt.Sprintf("%x", sha256.Sum256(body)) + `"`
	w.Header().Set("ETag", etag)
	if stale {
		w.Header().Set("X-NextDeploy-Cache", "STALE")
		s.triggerBackgroundRevalidation(route, entry, r, false)
	} else {
		w.Header().Set("X-NextDeploy-Cache", "HIT")
	}
	if matchesETag(r.Header.Get("If-None-Match"), etag) {
		w.WriteHeader(http.StatusNotModified)
		return true
	}
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	w.WriteHeader(http.StatusOK)
	if r.Method == http.MethodHead {
		return true
	}
	if _, err := w.Write(body); err != nil {
		s.logf("[gosidecar] write prerendered route %q: %v", route, err)
	}
	return true
}

func matchesETag(header, etag string) bool {
	for _, candidate := range strings.Split(header, ",") {
		if candidate = strings.TrimSpace(candidate); candidate == "*" || candidate == etag {
			return true
		}
	}
	return false
}

func (s *SidecarHandler) triggerBackgroundRevalidation(route string, entry *cachedEntry, request *http.Request, isRSC bool) {
	requestURI := request.URL.RequestURI()
	headers := request.Header.Clone()
	if requestURI == "" {
		requestURI = "/"
	}
	reval := &entry.htmlReval
	if isRSC {
		reval = &entry.rscReval
	}
	if !reval.CompareAndSwap(false, true) {
		return
	}
	started := s.startRevalidation(func() {
		defer reval.Store(false)
		parent := s.ctx
		if parent == nil {
			parent = context.Background()
		}
		ctx, cancel := context.WithTimeout(parent, 30*time.Second)
		defer cancel()
		for attempt := 0; attempt < 10; attempt++ {
			body, state, err := s.requestNodeForRevalidation(ctx, requestURI, headers)
			if err != nil {
				s.logf("[gosidecar] revalidate %q: %v", route, err)
				return
			}
			switch strings.ToUpper(state) {
			case "MISS", "HIT", "REVALIDATED":
				entry.mu.Lock()
				if isRSC {
					entry.rscBody = body
					entry.rscUpdated = time.Now()
				} else {
					entry.htmlBody = body
					entry.htmlUpdated = time.Now()
				}
				entry.mu.Unlock()
				return
			case "STALE":
				select {
				case <-ctx.Done():
					return
				case <-time.After(300 * time.Millisecond):
				}
			default:
				s.logf("[gosidecar] Node.js did not confirm revalidation for %q (x-nextjs-cache=%q)", route, state)
				return
			}
		}
		s.logf("[gosidecar] revalidation did not complete for %q before retry limit", route)
	})
	if !started {
		reval.Store(false)
	}
}

func (s *SidecarHandler) requestNodeForRevalidation(ctx context.Context, requestURI string, headers http.Header) ([]byte, string, error) {
	parsed, err := url.ParseRequestURI(requestURI)
	if err != nil {
		return nil, "", fmt.Errorf("parse revalidation request URI: %w", err)
	}
	target := *s.nodeOrigin
	target.Path, target.RawPath, target.RawQuery = parsed.Path, parsed.RawPath, parsed.RawQuery
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
	if err != nil {
		return nil, "", fmt.Errorf("create revalidation request: %w", err)
	}
	req.Header = headers.Clone()
	req.Header.Del("If-None-Match")
	req.Header.Del("If-Modified-Since")
	resp, err := s.nodeTransport.RoundTrip(req)
	if err != nil {
		return nil, "", fmt.Errorf("request Node.js: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", fmt.Errorf("Node.js returned HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxRouteBody+1))
	if err != nil {
		return nil, "", fmt.Errorf("read Node.js response: %w", err)
	}
	if len(body) > maxRouteBody {
		return nil, "", fmt.Errorf("Node.js response exceeded %d bytes", maxRouteBody)
	}
	return body, resp.Header.Get("X-Nextjs-Cache"), nil
}
