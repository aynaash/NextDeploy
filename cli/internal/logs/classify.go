package logs

import (
	"fmt"
	"strings"
)

// Owner identifies which layer is responsible for an error.
type Owner string

const (
	OwnerApp        Owner = "APP"
	OwnerNextDeploy Owner = "NEXTDEPLOY"
	OwnerPlatform   Owner = "PLATFORM"
	OwnerUnknown    Owner = "UNKNOWN"
)

// ParseOwner parses a user-supplied owner filter string.
// An empty string means "no filter" and returns "" with nil error.
func ParseOwner(s string) (Owner, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "":
		return "", nil
	case "app":
		return OwnerApp, nil
	case "nextdeploy", "nd":
		return OwnerNextDeploy, nil
	case "platform", "cf", "cloudflare":
		return OwnerPlatform, nil
	case "unknown":
		return OwnerUnknown, nil
	default:
		return "", fmt.Errorf("invalid owner %q (expected: app, nextdeploy, platform, unknown)", s)
	}
}

// Diagnosis describes the classified fault owner, machine-readable code, and actionable hint.
type Diagnosis struct {
	Owner   Owner  `json:"owner"`
	Code    string `json:"code"`
	Summary string `json:"summary"`
	Hint    string `json:"hint"`
}

// Signal holds all available request/error signals used to classify a failure.
type Signal struct {
	Method     string
	Path       string
	URL        string
	Status     int
	Outcome    string
	Headers    map[string]string
	Messages   []string
	Exceptions []string
}

func (s Signal) header(name string) string {
	if len(s.Headers) == 0 {
		return ""
	}
	if v, ok := s.Headers[name]; ok {
		return v
	}
	want := strings.ToLower(name)
	for k, v := range s.Headers {
		if strings.ToLower(k) == want {
			return v
		}
	}
	return ""
}

// OwnerBadge returns a consistent ANSI-colored badge for terminal output.
func OwnerBadge(o Owner) string {
	switch o {
	case OwnerApp:
		return "\033[1;33m[APP]\033[0m"
	case OwnerNextDeploy:
		return "\033[1;31m[NEXTDEPLOY]\033[0m"
	case OwnerPlatform:
		return "\033[1;36m[PLATFORM]\033[0m"
	default:
		return "\033[90m[UNKNOWN]\033[0m"
	}
}

// DedupeMessages collapses Cloudflare's multi-fragment log bursts (e.g. request echo,
// bare "⨯", short stack, full stack) into distinct, non-redundant messages.
func DedupeMessages(msgs []string) []string {
	type item struct {
		raw  string
		norm string
	}
	var nonEcho []item
	var echoOnly []item

	for _, m := range msgs {
		trimmed := strings.TrimSpace(m)
		if trimmed == "" {
			continue
		}
		norm := normalizeForDedup(trimmed)
		if norm == "" {
			continue
		}
		if isRequestEchoMessage(norm) {
			echoOnly = append(echoOnly, item{raw: trimmed, norm: norm})
			continue
		}
		nonEcho = append(nonEcho, item{raw: trimmed, norm: norm})
	}

	candidates := nonEcho
	if len(candidates) == 0 {
		candidates = echoOnly
	}

	// Keep the longest/most informative version when one normalized message is a
	// prefix or suffix of another, preserving order of first appearance.
	kept := make([]item, 0, len(candidates))
	for _, c := range candidates {
		merged := false
		for i, k := range kept {
			if k.norm == c.norm {
				if len(c.raw) > len(k.raw) {
					kept[i] = c
				}
				merged = true
				break
			}
			if strings.Contains(k.norm, c.norm) {
				merged = true
				break
			}
			if strings.Contains(c.norm, k.norm) {
				kept[i] = c
				merged = true
				break
			}
		}
		if !merged {
			kept = append(kept, c)
		}
	}

	out := make([]string, len(kept))
	for i, k := range kept {
		out[i] = k.raw
	}
	return out
}

// BestDetail extracts a concise single-line preview from deduplicated messages.
func BestDetail(msgs []string) string {
	deduped := DedupeMessages(msgs)
	if len(deduped) == 0 {
		return ""
	}
	for _, m := range deduped {
		lines := strings.Split(m, "\n")
		for _, ln := range lines {
			clean := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(ln), "⨯"))
			if clean == "" {
				continue
			}
			return truncateOneLine(clean, 140)
		}
	}
	return truncateOneLine(deduped[0], 140)
}

func normalizeForDedup(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "⨯")
	return strings.TrimSpace(s)
}

func isRequestEchoMessage(s string) bool {
	for _, method := range []string{"GET ", "POST ", "PUT ", "PATCH ", "DELETE ", "HEAD ", "OPTIONS "} {
		if strings.HasPrefix(s, method) && (strings.Contains(s, "http://") || strings.Contains(s, "https://")) && !strings.Contains(s, "\n") {
			return true
		}
	}
	return false
}

func truncateOneLine(s string, max int) string {
	s = strings.ReplaceAll(s, "\r", "")
	s = strings.ReplaceAll(s, "\n", " | ")
	if len(s) > max {
		return s[:max] + "..."
	}
	return s
}

// Classify determines whether a failure belongs to APP, NEXTDEPLOY, PLATFORM, or UNKNOWN.
func Classify(sig Signal) Diagnosis {
	var allParts []string
	allParts = append(allParts, sig.Messages...)
	allParts = append(allParts, sig.Exceptions...)
	rawText := strings.Join(allParts, "\n")
	lowerText := strings.ToLower(rawText)
	lowerOutcome := strings.ToLower(strings.TrimSpace(sig.Outcome))

	// Rule group A: Cloudflare / Platform limits & edge blocks
	switch lowerOutcome {
	case "exceededcpu":
		return Diagnosis{
			Owner:   OwnerPlatform,
			Code:    "platform_cpu_limit",
			Summary: "Worker exceeded Cloudflare CPU time limit",
			Hint:    "Optimize hot server paths or move heavy computation off the request path.",
		}
	case "exceededmemory":
		return Diagnosis{
			Owner:   OwnerPlatform,
			Code:    "platform_memory_limit",
			Summary: "Worker exceeded Cloudflare memory limit",
			Hint:    "Reduce in-memory bundle/payload size or stream large responses instead of buffering.",
		}
	case "exceededresources":
		return Diagnosis{
			Owner:   OwnerPlatform,
			Code:    "platform_resource_limit",
			Summary: "Worker exceeded Cloudflare runtime resource limits",
			Hint:    "Inspect worker CPU/memory usage and subrequest count.",
		}
	case "scriptstartup":
		return Diagnosis{
			Owner:   OwnerPlatform,
			Code:    "platform_script_startup",
			Summary: "Worker failed Cloudflare script startup limits",
			Hint:    "Reduce top-level module initialization work and bundle size.",
		}
	case "canceled":
		return Diagnosis{
			Owner:   OwnerPlatform,
			Code:    "platform_canceled",
			Summary: "Request canceled before Worker completed",
			Hint:    "Client disconnected early or Cloudflare terminated an overdue request.",
		}
	case "responsestreamdisconnected":
		return Diagnosis{
			Owner:   OwnerPlatform,
			Code:    "platform_stream_disconnected",
			Summary: "Client disconnected during response stream",
			Hint:    "Usually benign client abort during streaming/SSR.",
		}
	}

	if sig.Status == 429 && !containsAny(lowerText, "rate_limit kv", "kvbinding") {
		return Diagnosis{
			Owner:   OwnerPlatform,
			Code:    "platform_rate_limited",
			Summary: "Request blocked by edge rate limiter (429)",
			Hint:    "Adjust rate_limit in nextdeploy.yml if this threshold is too low.",
		}
	}

	if (sig.Status == 502 || sig.Status == 503 || sig.Status == 504) &&
		containsAny(lowerText, "timeout", "timed out", "upstream", "gateway", "network connection lost") {
		return Diagnosis{
			Owner:   OwnerPlatform,
			Code:    "platform_upstream_timeout",
			Summary: fmt.Sprintf("Platform/upstream gateway failure (HTTP %d)", sig.Status),
			Hint:    "Check upstream API latency, external fetch timeouts, or Cloudflare transient errors.",
		}
	}

	// Rule group B: Explicit NextDeploy runtime / adapter errors
	if containsAny(lowerText, "[nextcompile cachehandler", "[nextcompile isr write-back failed]") {
		return Diagnosis{
			Owner:   OwnerNextDeploy,
			Code:    "nextdeploy_cache_r2",
			Summary: "NextDeploy R2 cache/ISR write failed",
			Hint:    "Verify ASSETS R2 binding permissions and bucket health.",
		}
	}

	if strings.Contains(lowerText, "[nextcompile ") {
		return Diagnosis{
			Owner:   OwnerNextDeploy,
			Code:    "nextdeploy_runtime_error",
			Summary: "NextDeploy Worker runtime handler failed",
			Hint:    "Run with --raw to inspect the underlying stack; report if reproducible.",
		}
	}

	if containsAny(lowerText, "rate_limit kvbinding", "rate_limit kv.get failed", "rate_limit kv.put failed") {
		return Diagnosis{
			Owner:   OwnerNextDeploy,
			Code:    "nextdeploy_guard_kv",
			Summary: "NextDeploy edge guard KV binding error",
			Hint:    "Check the KV namespace binding name in nextdeploy.yml and Cloudflare KV quotas.",
		}
	}

	if containsAny(lowerText, "__nextdeployvfs", "__nextdeployloadmodule", "__nextdeploycachehandler", "vfs_fs_shim", "http_bridge", "nextserver_driver", "cf_cache_handler") {
		return Diagnosis{
			Owner:   OwnerNextDeploy,
			Code:    "nextdeploy_adapter_internal",
			Summary: "NextDeploy Cloudflare adapter runtime error",
			Hint:    "Run with --raw to inspect the adapter stack and update/report NextDeploy CLI.",
		}
	}

	if strings.Contains(lowerText, "[unenv]") && strings.Contains(lowerText, "not implemented") {
		return Diagnosis{
			Owner:   OwnerNextDeploy,
			Code:    "nextdeploy_node_polyfill",
			Summary: "Unimplemented Node.js API polyfill in Worker runtime",
			Hint:    "Raise compatibility_date in nextdeploy.yml or avoid this Node API on Workers.",
		}
	}

	if strings.Contains(lowerText, "enoent") && containsAny(lowerText, ".next/", "build_id", "routes-manifest.json") {
		return Diagnosis{
			Owner:   OwnerNextDeploy,
			Code:    "nextdeploy_vfs_missing",
			Summary: "Missing .next build file in NextDeploy virtual filesystem",
			Hint:    "Rebuild and redeploy; if it persists, file a NextDeploy bug.",
		}
	}

	// Rule group C: Context.externalRequire during module evaluation
	if strings.Contains(lowerText, "context.externalrequire") {
		if containsAny(lowerText, "next/dist/", "__nextdeploy") {
			return Diagnosis{
				Owner:   OwnerNextDeploy,
				Code:    "nextdeploy_external_require",
				Summary: "NextDeploy bundle failed to resolve internal Next.js module",
				Hint:    "Rebuild with latest NextDeploy CLI or report bundle externalization bug.",
			}
		}
		return Diagnosis{
			Owner:   OwnerApp,
			Code:    "app_external_require",
			Summary: "Route failed to load a Node-only module on Workers (externalRequire)",
			Hint:    "Check server imports on this route for Node-only deps (fs, gray-matter, child_process, native addons), bundle data at build time, or set compatibility_date >= 2025-09-01 in nextdeploy.yml.",
		}
	}

	// Rule group D: Next.js 16 RSC segment prefetch failing in NextDeploy Worker
	hasSegmentPrefetchHeader := sig.header("next-router-segment-prefetch") != ""
	isRSCPrefetch := strings.Contains(sig.URL, "_rsc=") && sig.header("next-router-prefetch") == "1"
	hasExplicitAppException := hasAppExceptionPattern(rawText, lowerText)

	if (hasSegmentPrefetchHeader || isRSCPrefetch) &&
		(sig.Status >= 500 || strings.Contains(rawText, "NextNodeServer.renderToResponseWithComponentsImpl")) &&
		!hasExplicitAppException {
		return Diagnosis{
			Owner:   OwnerNextDeploy,
			Code:    "nextdeploy_rsc_segment_prefetch",
			Summary: "Next.js RSC segment prefetch failed in NextDeploy Worker runtime",
			Hint:    "Pre-rendered .segment.rsc files are not served from R2 for next-router-segment-prefetch requests (NextDeploy runtime issue; normal page loads still work).",
		}
	}

	// Rule group E: Application code errors
	if containsAny(lowerText, "cloudflare env not available", "getenv() must be called inside a request") {
		return Diagnosis{
			Owner:   OwnerApp,
			Code:    "app_getenv_outside_request",
			Summary: "App called getEnv() outside a request context",
			Hint:    "Call getEnv() inside a Server Component, Route Handler, Server Action, or proxy.ts request.",
		}
	}

	if strings.Contains(lowerText, "headers() returns a read-only headers") {
		return Diagnosis{
			Owner:   OwnerApp,
			Code:    "app_readonly_headers",
			Summary: "App attempted to mutate read-only headers()",
			Hint:    "Set response headers on the NextResponse / Response object instead of mutating headers().",
		}
	}

	if strings.Contains(lowerText, "unsupported content-type for server action") {
		return Diagnosis{
			Owner:   OwnerApp,
			Code:    "app_invalid_server_action",
			Summary: "Unsupported Content-Type sent to Server Action",
			Hint:    "Ensure Server Action forms/requests use multipart/form-data, text/plain, or application/x-www-form-urlencoded.",
		}
	}

	if strings.Contains(lowerText, "after() threw:") {
		return Diagnosis{
			Owner:   OwnerApp,
			Code:    "app_after_callback_error",
			Summary: "Background after() callback threw an error",
			Hint:    "Add try/catch error handling inside your next/server after() callback.",
		}
	}

	if strings.Contains(lowerText, "enoent") {
		return Diagnosis{
			Owner:   OwnerApp,
			Code:    "app_fs_read_missing",
			Summary: "App tried to read a local file that does not exist on Workers",
			Hint:    "Cloudflare Workers have no local project filesystem — inline/prebuild data into JSON or store it in R2.",
		}
	}

	if hasExplicitAppException {
		return Diagnosis{
			Owner:   OwnerApp,
			Code:    "app_runtime_exception",
			Summary: "Unhandled exception in application code",
			Hint:    "Inspect the failing route handler/component and add input/null guards or error handling.",
		}
	}

	if hasUserSourcePath(rawText) {
		return Diagnosis{
			Owner:   OwnerApp,
			Code:    "app_route_error",
			Summary: "Error originated in application route/component source",
			Hint:    "Inspect the referenced app/src/lib file and stack trace with --raw.",
		}
	}

	if sig.Status >= 400 && sig.Status < 500 {
		return Diagnosis{
			Owner:   OwnerApp,
			Code:    "app_http_4xx",
			Summary: fmt.Sprintf("Application returned HTTP %d", sig.Status),
			Hint:    "Check route matching, auth checks, or request validation in your app.",
		}
	}

	// Rule group F: Fallback
	if sig.Status >= 500 &&
		strings.Contains(rawText, "NextNodeServer.") &&
		strings.Contains(sig.URL, "_rsc=") &&
		hasEmptyCrossError(sig.Messages) {
		return Diagnosis{
			Owner:   OwnerNextDeploy,
			Code:    "nextdeploy_rsc_render_error",
			Summary: "Next.js RSC render failed inside NextDeploy Worker with no app error",
			Hint:    "Likely a missing R2 static/segment artifact or Next.js Worker shim gap; run with --raw to inspect.",
		}
	}

	return Diagnosis{
		Owner:   OwnerUnknown,
		Code:    "unknown_error",
		Summary: "Unclassified server error",
		Hint:    "Run with --raw to inspect full logs, or add try/catch logging in your route.",
	}
}

func containsAny(haystack string, needles ...string) bool {
	for _, n := range needles {
		if strings.Contains(haystack, n) {
			return true
		}
	}
	return false
}

func hasAppExceptionPattern(rawText, lowerText string) bool {
	for _, tok := range []string{
		"TypeError",
		"ReferenceError",
		"SyntaxError",
		"RangeError",
		"URIError",
		"ZodError",
		"PrismaClient",
		"PostgresError",
		"D1_ERROR",
	} {
		if strings.Contains(rawText, tok) {
			return true
		}
	}
	return containsAny(lowerText,
		"cannot read properties of",
		"is not a function",
		"is not defined",
		"fetch failed",
		"econnrefused",
		"hydration",
	) || hasNonEmptyErrorColon(rawText)
}

func hasNonEmptyErrorColon(rawText string) bool {
	for _, line := range strings.Split(rawText, "\n") {
		trimmed := strings.TrimSpace(line)
		if idx := strings.Index(trimmed, "Error:"); idx >= 0 {
			after := strings.TrimSpace(trimmed[idx+len("Error:"):])
			if after != "" {
				return true
			}
		}
	}
	return false
}

func hasUserSourcePath(rawText string) bool {
	for _, line := range strings.Split(rawText, "\n") {
		if strings.Contains(line, "node_modules/") || strings.Contains(line, "next/dist/") {
			continue
		}
		for _, seg := range []string{"app/", "pages/", "src/", "lib/", "components/"} {
			if strings.Contains(line, seg) {
				return true
			}
		}
	}
	return false
}

func hasEmptyCrossError(msgs []string) bool {
	for _, m := range msgs {
		t := strings.TrimSpace(m)
		if strings.HasPrefix(t, "⨯") {
			rest := strings.TrimSpace(strings.TrimPrefix(t, "⨯"))
			if rest == "" || strings.HasPrefix(rest, "at ") {
				return true
			}
		}
	}
	return false
}
