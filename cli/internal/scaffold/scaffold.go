// Package scaffold generates a Cloudflare-ready fullstack Next.js starter
// (App Router + Drizzle + better-auth + R2 + Workers AI + proxy.ts guard) from
// embedded templates. It powers `nextdeploy init` when the user opts to scaffold
// a new app rather than deploy their existing one.
//
// The DB layer is pluggable: "d1" (Cloudflare native SQLite), "byo"
// (bring-your-own Postgres via Hyperdrive), or "none" (no database).
// A db-<variant> overlay is written on top of the shared base.
package scaffold

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/aynaash/nextdeploy/cli/internal/cicd"
)

//go:embed all:templates
var tmplFS embed.FS

// DBVariant selects the database layer the template wires up.
type DBVariant string

const (
	DBD1   DBVariant = "d1"
	DBBYO  DBVariant = "byo"  // bring-your-own Postgres via Hyperdrive
	DBNone DBVariant = "none" // no database
)

// Platform selects the deployment target platform.
type Platform string

const (
	PlatformCloudflare Platform = "cloudflare"
	PlatformVPS        Platform = "vps"
	PlatformAWS        Platform = "aws"
)

// Options configures a scaffold run.
type Options struct {
	AppName         string
	Platform        Platform
	DBVariant       DBVariant
	EnableGoSidecar bool
	Dir             string // target directory (created if missing)
	// InfraOnly skips starter app pages/package.json/tsconfig/README and writes
	// only deployment infra (nextdeploy.yml, middleware.ts/proxy.ts, lib/env.ts,
	// migrations, CI). Used when scaffolding Cloudflare infra into an existing
	// Next.js app so root app/ pages never shadow an existing src/app/ directory.
	InfraOnly bool
	// NextVersion is the detected Next.js version in package.json (e.g. "^15.1.0",
	// "^16.0.0"). Next 16+ uses proxy.ts; Next 15 and earlier (including the
	// default starter) use middleware.ts.
	NextVersion string
}

// placeholder tokens replaced in every template file.
const appNameToken = "__APP_NAME__"

// Scaffold writes the base template plus platform overlays into opts.Dir,
// substituting the app name and platform details. Existing files are never
// overwritten — their paths are returned in `skipped` so the caller can report
// them. Returns the written (created) paths and the skipped (pre-existing) paths,
// both sorted.
func Scaffold(opts Options) (written, skipped []string, err error) {
	if opts.AppName == "" {
		return nil, nil, fmt.Errorf("app name is required")
	}
	if opts.Dir == "" {
		return nil, nil, fmt.Errorf("target dir is required")
	}

	platform := opts.Platform
	if platform == "" {
		platform = PlatformCloudflare
	}

	var roots []string
	platformName := "Cloudflare Workers"
	platformSlug := "cloudflare"

	switch platform {
	case PlatformVPS:
		platformName = "VPS (Self-Hosted)"
		platformSlug = "vps"
		roots = []string{"templates/base", "templates/platform-vps"}
	case PlatformAWS:
		platformName = "AWS Serverless"
		platformSlug = "aws"
		roots = []string{"templates/base", "templates/platform-aws"}
	case PlatformCloudflare:
		variant := opts.DBVariant
		if variant == "" {
			variant = DBD1
		}
		if variant != DBD1 && variant != DBBYO && variant != DBNone {
			return nil, nil, fmt.Errorf("unknown db variant %q (want d1, byo, or none)", variant)
		}
		if opts.InfraOnly {
			roots = []string{"templates/platform-cloudflare", "templates/db-" + string(variant)}
		} else {
			roots = []string{"templates/base", "templates/platform-cloudflare", "templates/db-" + string(variant)}
		}
	default:
		return nil, nil, fmt.Errorf("unknown platform %q (want cloudflare, vps, or aws)", platform)
	}

	for _, root := range roots {
		w, s, err := copyTree(root, opts.Dir, opts.AppName, platformName, platformSlug, opts.InfraOnly, opts.NextVersion)
		if err != nil {
			return nil, nil, err
		}
		written = append(written, w...)
		skipped = append(skipped, s...)
	}

	// For VPS, adjust sidecar setting if explicitly disabled.
	if platform == PlatformVPS && !opts.EnableGoSidecar {
		cfgPath := filepath.Join(opts.Dir, "nextdeploy.yml")
		if data, err := os.ReadFile(cfgPath); err == nil {
			modified := strings.Replace(string(data), "enabled: true # High-performance Go sidecar", "enabled: false # High-performance Go sidecar", 1)
			_ = os.WriteFile(cfgPath, []byte(modified), 0o640)
		}
	}

	// The deploy workflow is rendered, not templated.
	wfPath := filepath.Join(opts.Dir, filepath.FromSlash(cicd.WorkflowPath))
	if _, statErr := os.Stat(wfPath); statErr == nil {
		skipped = append(skipped, wfPath)
	} else {
		if err := os.MkdirAll(filepath.Dir(wfPath), 0o750); err != nil {
			return nil, nil, err
		}
		if err := os.WriteFile(wfPath, []byte(cicd.RenderDeployWorkflow()), 0o640); err != nil {
			return nil, nil, fmt.Errorf("write %s: %w", wfPath, err)
		}
		written = append(written, wfPath)
	}

	sort.Strings(written)
	sort.Strings(skipped)
	return written, skipped, nil
}

func copyTree(root, destDir, appName, platformName, platformSlug string, infraOnly bool, nextVersion string) (written, skipped []string, err error) {
	useProxy := usesProxyConvention(nextVersion)
	useSrcDir := infraOnly && hasSrcAppLayout(destDir)

	walkErr := fs.WalkDir(tmplFS, root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		slashRel := filepath.ToSlash(rel)
		if infraOnly && (strings.HasPrefix(slashRel, "app/") || slashRel == "README.md") {
			return nil
		}

		targetRel := rel
		isInterceptor := slashRel == "middleware.ts"
		if isInterceptor {
			// If the project already has any Next.js middleware/proxy file, keep it
			// rather than introducing a conflicting sibling.
			if existing := findExistingInterceptor(destDir); existing != "" {
				skipped = append(skipped, existing)
				return nil
			}
			name := "middleware.ts"
			if useProxy {
				name = "proxy.ts"
			}
			if useSrcDir {
				targetRel = filepath.Join("src", name)
			} else {
				targetRel = name
			}
		}

		target := filepath.Join(destDir, targetRel)

		if _, statErr := os.Stat(target); statErr == nil {
			skipped = append(skipped, target)
			return nil
		}

		data, err := tmplFS.ReadFile(p)
		if err != nil {
			return fmt.Errorf("read template %s: %w", p, err)
		}
		out := strings.ReplaceAll(string(data), appNameToken, appName)
		out = strings.ReplaceAll(out, "__PLATFORM_NAME__", platformName)
		out = strings.ReplaceAll(out, "__PLATFORM_SLUG__", platformSlug)
		if isInterceptor && useProxy {
			out = renderAsProxyTS(out)
		}

		if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
			return err
		}
		if err := os.WriteFile(target, []byte(out), 0o640); err != nil {
			return fmt.Errorf("write %s: %w", target, err)
		}
		written = append(written, target)
		return nil
	})
	return written, skipped, walkErr
}

// usesProxyConvention reports whether the target Next.js version uses proxy.ts
// (Next.js 16+) instead of middleware.ts (Next.js 15 and earlier).
func usesProxyConvention(nextVersion string) bool {
	v := strings.TrimSpace(strings.ToLower(nextVersion))
	if v == "" {
		return false // default scaffold ships Next.js 15
	}
	if v == "latest" || v == "canary" || strings.HasPrefix(v, "canary") {
		return true
	}
	// Extract the first integer major version from strings like "^16.0.0", ">=16", "v16.1.0".
	major := 0
	foundDigit := false
	for i := 0; i < len(v); i++ {
		c := v[i]
		if c >= '0' && c <= '9' {
			major = major*10 + int(c-'0')
			foundDigit = true
		} else if foundDigit {
			break
		}
	}
	return foundDigit && major >= 16
}

// hasSrcAppLayout reports whether dir uses a src/app or src/pages layout
// without a root app/ or pages/ directory.
func hasSrcAppLayout(dir string) bool {
	for _, rootDir := range []string{"app", "pages"} {
		if info, err := os.Stat(filepath.Join(dir, rootDir)); err == nil && info.IsDir() {
			return false
		}
	}
	for _, srcDir := range []string{filepath.Join("src", "app"), filepath.Join("src", "pages")} {
		if info, err := os.Stat(filepath.Join(dir, srcDir)); err == nil && info.IsDir() {
			return true
		}
	}
	return false
}

func findExistingInterceptor(dir string) string {
	candidates := []string{
		"middleware.ts", "middleware.js",
		filepath.Join("src", "middleware.ts"), filepath.Join("src", "middleware.js"),
		"proxy.ts", "proxy.js",
		filepath.Join("src", "proxy.ts"), filepath.Join("src", "proxy.js"),
	}
	for _, rel := range candidates {
		p := filepath.Join(dir, rel)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

func renderAsProxyTS(middlewareSrc string) string {
	out := strings.Replace(middlewareSrc, "// middleware.ts — Next.js middleware", "// proxy.ts — Next.js 16+ proxy", 1)
	out = strings.Replace(out, "export function middleware(", "export default function proxy(", 1)
	return out
}

// TemplateFiles returns the relative paths shipped for a given variant (base +
// overlay), sorted. Used by tests and the init summary.
func TemplateFiles(variant DBVariant) ([]string, error) {
	if variant == "" {
		variant = DBD1
	}
	seen := map[string]bool{}
	for _, root := range []string{"templates/base", "templates/platform-cloudflare", "templates/db-" + string(variant)} {
		err := fs.WalkDir(tmplFS, root, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			rel, _ := filepath.Rel(root, p)
			seen[rel] = true
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	// Rendered, not embedded — but Scaffold does write it, so the init
	// summary must list it.
	seen[filepath.FromSlash(cicd.WorkflowPath)] = true

	out := make([]string, 0, len(seen))
	for k := range seen {
		out = append(out, k)
	}
	sort.Strings(out)
	return out, nil
}
