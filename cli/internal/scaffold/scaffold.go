// Package scaffold generates a Cloudflare-ready fullstack Next.js starter
// (App Router + Drizzle + better-auth + R2 + Workers AI + proxy.ts guard) from
// embedded templates. It powers `nextdeploy init` when the user opts to scaffold
// a new app rather than deploy their existing one.
//
// The DB layer is pluggable: "d1" (Cloudflare native SQLite) or "byo"
// (bring-your-own Postgres via Hyperdrive). A db-<variant> overlay is written
// on top of the shared base.
package scaffold

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

//go:embed all:templates
var tmplFS embed.FS

// DBVariant selects the database layer the template wires up.
type DBVariant string

const (
	DBD1  DBVariant = "d1"
	DBBYO DBVariant = "byo" // bring-your-own Postgres via Hyperdrive
)

// Options configures a scaffold run.
type Options struct {
	AppName   string
	DBVariant DBVariant
	Dir       string // target directory (created if missing)
}

// placeholder tokens replaced in every template file.
const appNameToken = "__APP_NAME__"

// Scaffold writes the base template plus the db-<variant> overlay into opts.Dir,
// substituting the app name. Existing files are never overwritten — their paths
// are returned in `skipped` so the caller can report them. Returns the written
// (created) paths and the skipped (pre-existing) paths, both sorted.
func Scaffold(opts Options) (written, skipped []string, err error) {
	if opts.AppName == "" {
		return nil, nil, fmt.Errorf("app name is required")
	}
	variant := opts.DBVariant
	if variant == "" {
		variant = DBD1
	}
	if variant != DBD1 && variant != DBBYO {
		return nil, nil, fmt.Errorf("unknown db variant %q (want d1 or byo)", variant)
	}
	if opts.Dir == "" {
		return nil, nil, fmt.Errorf("target dir is required")
	}

	// Overlays applied in order; later sources may add files but never clobber.
	roots := []string{"templates/base", "templates/db-" + string(variant)}
	for _, root := range roots {
		w, s, err := copyTree(root, opts.Dir, opts.AppName)
		if err != nil {
			return nil, nil, err
		}
		written = append(written, w...)
		skipped = append(skipped, s...)
	}
	sort.Strings(written)
	sort.Strings(skipped)
	return written, skipped, nil
}

func copyTree(root, destDir, appName string) (written, skipped []string, err error) {
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
		target := filepath.Join(destDir, rel)

		if _, statErr := os.Stat(target); statErr == nil {
			skipped = append(skipped, target)
			return nil
		}

		data, err := tmplFS.ReadFile(p)
		if err != nil {
			return fmt.Errorf("read template %s: %w", p, err)
		}
		out := strings.ReplaceAll(string(data), appNameToken, appName)

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

// TemplateFiles returns the relative paths shipped for a given variant (base +
// overlay), sorted. Used by tests and the init summary.
func TemplateFiles(variant DBVariant) ([]string, error) {
	if variant == "" {
		variant = DBD1
	}
	seen := map[string]bool{}
	for _, root := range []string{"templates/base", "templates/db-" + string(variant)} {
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
	out := make([]string, 0, len(seen))
	for k := range seen {
		out = append(out, k)
	}
	sort.Strings(out)
	return out, nil
}
