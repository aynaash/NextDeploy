package serverless

import (
	"github.com/aynaash/nextdeploy/shared/config"
	"github.com/aynaash/nextdeploy/shared/nextcompile"
	"github.com/aynaash/nextdeploy/shared/nextcore"
	"github.com/aynaash/nextdeploy/shared/protection"
)

// buildProtectionRuntime maps the YAML cloudflare.protection block onto the
// decoupled protection.Config and normalizes it. Returns (nil, nil) when no
// protection is declared (or it's disabled) — the compiler then emits a `null`
// protection.json and the guard is a no-op.
func buildProtectionRuntime(cfg *config.NextDeployConfig) (*protection.Runtime, error) {
	if cfg == nil || cfg.Serverless == nil || cfg.Serverless.Cloudflare == nil || cfg.Serverless.Cloudflare.Protection == nil {
		return nil, nil
	}
	p := cfg.Serverless.Cloudflare.Protection
	c := &protection.Config{
		Enabled:     p.Enabled,
		PublicPaths: p.PublicPaths,
		Allow:       p.Allow,
		Deny:        p.Deny,

		TrustForwardedFor: p.TrustForwardedFor,
	}
	if p.Auth != nil {
		c.Auth = &protection.Auth{
			SecretEnv:      p.Auth.SecretEnv,
			CookieName:     p.Auth.CookieName,
			ProtectedPaths: p.Auth.ProtectedPaths,
			LoginPath:      p.Auth.LoginPath,
		}
	}
	if p.RateLimit != nil {
		c.RateLimit = &protection.RateLimit{
			KVBinding:         p.RateLimit.KVBinding,
			RequestsPerMinute: p.RateLimit.RequestsPerMinute,
			Paths:             p.RateLimit.Paths,
		}
	}
	return protection.BuildRuntime(c)
}

// toCompilePayload translates nextcore.NextCorePayload (+ nextdeploy config)
// into the shape nextcompile.Compile expects. The duplication is deliberate:
// nextcompile lives under shared/ and must not import from cli/ or take a
// hard dependency on the full nextcore shape, so we carry a minimal mirror
// of the types and translate at the adapter boundary.
//
// Fields populated today:
//   - AppName, DistDir, OutputMode, HasAppRouter, BuildID, GitCommit
//   - Routes (1:1 field mapping — nextcore and nextcompile RouteInfo match)
//   - Middleware, including each matcher's has/missing conditions
//   - The next.config projection: BasePath, AssetPrefix, ImageConfig, I18n.
//     These are parsed once at build time by nextcore.ParseNextConfigFile and
//     carried on NextCorePayload — we never re-parse next.config here, because
//     the bridge runs on the deploy side and may be fed a payload serialized
//     during an earlier build on another machine.
func toCompilePayload(meta *nextcore.NextCorePayload, _ *config.NextDeployConfig) nextcompile.Payload {
	if meta == nil {
		return nextcompile.Payload{}
	}

	p := nextcompile.Payload{
		AppName:      meta.AppName,
		DistDir:      meta.DistDir,
		OutputMode:   string(meta.OutputMode),
		HasAppRouter: meta.NextBuildMetadata.HasAppRouter,
		BuildID:      meta.NextBuildMetadata.BuildID,
		GitCommit:    meta.GitCommit,
		Routes:       convertRoutes(meta.RouteInfo),
		BasePath:     meta.BasePath,
		AssetPrefix:  meta.AssetPrefix,
		ImageConfig:  convertImageConfig(meta.ImageConfig),
		I18n:         convertI18nConfig(meta.I18n),
		PublicFiles:  publicFileKeys(meta.StaticAssets),
	}

	if meta.Middleware != nil {
		p.Middleware = &nextcompile.MiddlewareConfig{
			Path:     meta.Middleware.Path,
			Runtime:  meta.Middleware.Runtime,
			Matchers: convertMiddlewareMatchers(meta.Middleware.Matchers),
		}
	}

	return p
}

func convertImageConfig(in *nextcore.ImageConfig) *nextcompile.ImageConfig {
	if in == nil {
		return nil
	}
	out := &nextcompile.ImageConfig{
		RemotePatterns: convertImageRemotePatterns(in.RemotePatterns),
		Domains:        append([]string(nil), in.Domains...),
		Formats:        append([]string(nil), in.Formats...),
		Unoptimized:    in.Unoptimized,
	}
	return out
}

// convertI18nConfig projects nextcore's i18n block onto the compiler mirror.
// nextcore.I18nConfig also carries per-domain locale routing (Domains), which
// the compiler has no consumer for — the explicit field-by-field copy is the
// boundary contract, and it stops compiling if either side's shape drifts.
func convertI18nConfig(in *nextcore.I18nConfig) *nextcompile.I18nConfig {
	if in == nil {
		return nil
	}
	return &nextcompile.I18nConfig{
		Locales:         append([]string(nil), in.Locales...),
		DefaultLocale:   in.DefaultLocale,
		LocaleDetection: in.LocaleDetection,
	}
}

func convertImageRemotePatterns(in []nextcore.ImageRemotePattern) []nextcompile.ImageRemotePattern {
	if len(in) == 0 {
		return nil
	}
	out := make([]nextcompile.ImageRemotePattern, len(in))
	for i, r := range in {
		out[i] = nextcompile.ImageRemotePattern{
			Protocol: r.Protocol,
			Hostname: r.Hostname,
			Port:     r.Port,
			Pathname: r.Pathname,
		}
	}
	return out
}

func convertRoutes(in nextcore.RouteInfo) nextcompile.RouteInfo {
	return nextcompile.RouteInfo{
		StaticRoutes:     in.StaticRoutes,
		DynamicRoutes:    in.DynamicRoutes,
		SSGRoutes:        in.SSGRoutes,
		SSRRoutes:        in.SSRRoutes,
		ISRRoutes:        in.ISRRoutes,
		ISRDetail:        convertISRDetail(in.ISRDetail),
		APIRoutes:        in.APIRoutes,
		FallbackRoutes:   in.FallbackRoutes,
		MiddlewareRoutes: in.MiddlewareRoutes,
	}
}

func convertISRDetail(in []nextcore.ISRRoute) []nextcompile.ISRRoute {
	if len(in) == 0 {
		return nil
	}
	out := make([]nextcompile.ISRRoute, len(in))
	for i, r := range in {
		out[i] = nextcompile.ISRRoute{
			Path:       r.Path,
			Tags:       r.Tags,
			Revalidate: r.Revalidate,
		}
	}
	return out
}

func convertMiddlewareMatchers(in []nextcore.MiddlewareRoute) []nextcompile.MiddlewareMatcher {
	if len(in) == 0 {
		return nil
	}
	out := make([]nextcompile.MiddlewareMatcher, len(in))
	for i, m := range in {
		out[i] = nextcompile.MiddlewareMatcher{
			Pathname: m.Pathname,
			Pattern:  m.Pattern,
			Has:      convertMiddlewareConditions(m.Has),
			Missing:  convertMiddlewareConditions(m.Missing),
		}
	}
	return out
}

func convertMiddlewareConditions(in []nextcore.MiddlewareCondition) []nextcompile.MiddlewareCondition {
	if len(in) == 0 {
		return nil
	}
	out := make([]nextcompile.MiddlewareCondition, len(in))
	for i, c := range in {
		out[i] = nextcompile.MiddlewareCondition{Type: c.Type, Key: c.Key, Value: c.Value}
	}
	return out
}

func publicFileKeys(sa *nextcore.StaticAssets) []string {
	if sa == nil || len(sa.PublicDir) == 0 {
		return nil
	}
	out := make([]string, 0, len(sa.PublicDir))
	for _, a := range sa.PublicDir {
		out = append(out, a.Path)
	}
	return out
}
