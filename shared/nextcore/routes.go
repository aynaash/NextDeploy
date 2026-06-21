package nextcore

import (
	"os"
	"path/filepath"
	"strings"
)

func getRoutesFromManifests(buildMeta *NextBuildMetadata, distDir string) (*RouteInfo, error) {
	if distDir == "" {
		distDir = ".next"
	}
	info := &RouteInfo{
		SSGRoutes:      make(map[string]string),
		ISRRoutes:      make(map[string]string),
		FallbackRoutes: make(map[string]string),
	}
	if routesManifest, ok := buildMeta.RoutesManifest.(map[string]any); ok {
		if staticRoutes, ok := routesManifest["staticRoutes"].([]any); ok {
			for _, route := range staticRoutes {
				if routeMap, ok := route.(map[string]any); ok {
					if page, ok := routeMap["page"].(string); ok {
						info.StaticRoutes = append(info.StaticRoutes, page)
					}
				}
			}
		}
		if dynamicRoutes, ok := routesManifest["dynamicRoutes"].([]any); ok {
			for _, route := range dynamicRoutes {
				if routeMap, ok := route.(map[string]any); ok {
					if page, ok := routeMap["page"].(string); ok {
						info.DynamicRoutes = append(info.DynamicRoutes, page)
					}
				}
			}
		}
	}
	if prerenderManifest, ok := buildMeta.PrerenderManifest.(map[string]any); ok {
		if routes, ok := prerenderManifest["routes"].(map[string]any); ok {
			for route, details := range routes {
				detailMap, ok := details.(map[string]any)
				if !ok {
					continue
				}

				// initialRevalidateSeconds shape varies across Next versions:
				//   number > 0 → ISR with that revalidate window
				//   number 0   → SSG (no revalidation)
				//   false      → SSG (Next 16 App Router default for pure static pages)
				//   nil        → SSG (older Next versions)
				// The previous code only handled the float64 case, so Next 16
				// App Router routes with `false` were silently dropped. The
				// runtime then had nothing to serve from R2 and fell through to
				// the compiled-module dispatch path, which can't invoke an
				// App Router page module.
				revalSecs, isNumber := detailMap["initialRevalidateSeconds"].(float64)

				// Both branches (SSG and ISR) store the R2 key for the
				// prerendered HTML — the runtime's serveSSGFromR2 needs
				// the same shape regardless of bucket. Earlier code
				// stored a local .rsc path in ISRRoutes, which made the
				// runtime serve nothing for ISR routes (the value
				// wasn't an R2 key). The ISRDetail slice still carries
				// the revalidation metadata for tag-based invalidation.
				r2Key := resolveSSGR2Key(distDir, route)
				if r2Key == "" {
					// No prerendered HTML on disk — skip; runtime falls
					// through to the live dispatcher.
					continue
				}

				switch {
				case isNumber && revalSecs > 0:
					info.ISRRoutes[route] = r2Key

					isrRoute := ISRRoute{
						Path:       route,
						Revalidate: int(revalSecs),
						Tags:       []string{route},
					}
					if routeTagsIfc, hasTags := detailMap["tags"].([]any); hasTags {
						for _, t := range routeTagsIfc {
							if tagStr, isStr := t.(string); isStr {
								isrRoute.Tags = append(isrRoute.Tags, tagStr)
							}
						}
					}
					info.ISRDetail = append(info.ISRDetail, isrRoute)

				default:
					// Pure SSG (initialRevalidateSeconds is false / 0 / nil).
					info.SSGRoutes[route] = r2Key
				}
			}
		}
		if dynamicRoutes, ok := prerenderManifest["dynamicRoutes"].(map[string]any); ok {
			for route, details := range dynamicRoutes {
				if detailMap, ok := details.(map[string]any); ok {
					if fallback, ok := detailMap["fallback"].(string); ok && fallback != "" {
						info.FallbackRoutes[route] = fallback
					}
				}
			}
		}
	}

	if buildManifest, ok := buildMeta.BuildManifest.(map[string]any); ok {
		if middleware, ok := buildManifest["middleware"].(map[string]any); ok {
			for route := range middleware {
				info.MiddlewareRoutes = append(info.MiddlewareRoutes, route)
			}
		}
	}

	return info, nil
}

// resolveSSGR2Key returns the R2 object key the runtime should look up
// to serve the prerendered HTML for `route`. Returns "" when no
// prerendered HTML actually exists on disk for this route — callers
// treat that as "no SSG to ship for this route".
//
// The R2 key convention is set by the serverless packager's
// addPrerenderedAsset: route-stripped-of-leading-slash + ".html",
// with the special case "/" → "index.html". This function must stay in
// lock-step with the packager — they're two halves of the same wire
// format, and a divergence shows up as 404s from R2 with no obvious
// log line.
//
// To check existence, we probe both App Router (.next/server/app) and
// Pages Router (.next/server/pages); whichever has the file determines
// nothing about the R2 key (which is route-derived) — we only need
// confirmation that the file exists somewhere so the packager will
// upload it.
func resolveSSGR2Key(distDir, route string) string {
	rel := route
	if rel == "/" || rel == "" {
		rel = "/index"
	}
	probes := []string{
		filepath.Join(distDir, "server", "app", rel+".html"),
		filepath.Join(distDir, "server", "pages", rel+".html"),
	}
	exists := false
	for _, p := range probes {
		if _, err := os.Stat(p); err == nil {
			exists = true
			break
		}
	}
	if !exists {
		return ""
	}

	r2Key := route
	if r2Key == "/" || r2Key == "" {
		return "index.html"
	}
	r2Key = strings.TrimPrefix(r2Key, "/")
	return r2Key + ".html"
}
