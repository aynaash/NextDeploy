// Package infrasniff inspects a Next.js project's source tree and package.json
// to infer which Cloudflare resources it needs (D1, R2, KV, Workers AI,
// Hyperdrive, Vectorize, Queues), which server-side secrets it references, and
// whether it has an auth layer worth protecting. It powers the `nextdeploy init`
// "use my existing app" path, prefilling nextdeploy.yml instead of asking the
// user to hand-write bindings.
//
// It is intentionally heuristic and read-only: signals are best-effort hints a
// human reviews, never silent infrastructure mutations.
package infrasniff

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Resource is a Cloudflare resource kind the sniffer can detect.
type Resource string

const (
	ResD1         Resource = "d1"
	ResR2         Resource = "r2"
	ResKV         Resource = "kv"
	ResAI         Resource = "ai"
	ResHyperdrive Resource = "hyperdrive"
	ResVectorize  Resource = "vectorize"
	ResQueue      Resource = "queue"
)

// Signal is one detected resource need with the evidence behind it.
type Signal struct {
	Resource Resource
	Reason   string // human-readable evidence, e.g. `dep "drizzle-orm/d1"` or `type R2Bucket`
}

// Result is the full sniff outcome.
type Result struct {
	Signals     []Signal // one per (resource, reason); dedup with Resources()
	Secrets     []string // server-side env var names referenced (NEXT_PUBLIC_* excluded), sorted
	Auth        bool     // an auth library was detected → suggest a protection.auth block
	FilesParsed int
	// Wrangler is the parsed wrangler config when the project ships one. Its
	// bindings are authoritative (real names + IDs) and should be preferred
	// over heuristic signals when prefilling nextdeploy.yml.
	Wrangler *WranglerConfig
}

// Resources returns the unique detected resources, sorted.
func (r *Result) Resources() []Resource {
	seen := map[Resource]bool{}
	var out []Resource
	for _, s := range r.Signals {
		if !seen[s.Resource] {
			seen[s.Resource] = true
			out = append(out, s.Resource)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// depSignals maps a package.json dependency (exact name or import-path prefix)
// to the resource it implies. Checked against dependency *names*.
var depSignals = []struct {
	dep      string
	resource Resource
}{
	{"@libsql/client", ResD1},
	{"@neondatabase/serverless", ResHyperdrive},
	{"postgres", ResHyperdrive},
	{"pg", ResHyperdrive},
	{"mysql2", ResHyperdrive},
	{"@cloudflare/ai", ResAI},
	{"openai", ResAI},
	{"ai", ResAI},
	{"@vectorize/client", ResVectorize},
}

// authDeps imply an auth layer (→ protection.auth + AUTH_SECRET).
var authDeps = []string{"better-auth", "next-auth", "@auth/core", "@clerk/nextjs", "lucia"}

// sourceSignals are regexes run over source files. The first capture is unused;
// presence is the signal. Ordered for deterministic reason strings.
var sourceSignals = []struct {
	re       *regexp.Regexp
	resource Resource
	reason   string
}{
	{regexp.MustCompile(`\bD1Database\b`), ResD1, "type D1Database"},
	{regexp.MustCompile(`drizzle-orm/d1`), ResD1, `import "drizzle-orm/d1"`},
	{regexp.MustCompile(`\bR2Bucket\b`), ResR2, "type R2Bucket"},
	{regexp.MustCompile(`\bKVNamespace\b`), ResKV, "type KVNamespace"},
	{regexp.MustCompile(`\bVectorizeIndex\b`), ResVectorize, "type VectorizeIndex"},
	{regexp.MustCompile(`\bHyperdrive\b`), ResHyperdrive, "type Hyperdrive"},
	{regexp.MustCompile(`drizzle-orm/postgres-js`), ResHyperdrive, `import "drizzle-orm/postgres-js"`},
	{regexp.MustCompile(`postgres://|postgresql://`), ResHyperdrive, "postgres connection string"},
	{regexp.MustCompile(`@cf/[a-z0-9-]+/`), ResAI, "Workers AI model id (@cf/...)"},
	{regexp.MustCompile(`env\.AI\b`), ResAI, "env.AI usage"},
	{regexp.MustCompile(`\bQueue<|env\.QUEUE\b`), ResQueue, "Queue binding usage"},
}

// envRef matches process.env.X and env.X identifiers (uppercase snake names).
var envRef = regexp.MustCompile(`(?:process\.env|env)\.([A-Z][A-Z0-9_]{1,})`)

// bindingNames are env.X identifiers that are Worker bindings, not secrets, so
// they're excluded from the secrets list.
var bindingNames = map[string]bool{
	"DB": true, "ASSETS": true, "AI": true, "KV": true, "CACHE": true,
	"QUEUE": true, "VECTORIZE": true, "HYPERDRIVE": true, "RATE_LIMIT": true,
}

var sourceExts = map[string]bool{
	".ts": true, ".tsx": true, ".js": true, ".jsx": true, ".mjs": true, ".cjs": true,
}

var skipDirs = map[string]bool{
	"node_modules": true, ".next": true, ".git": true, "dist": true,
	"build": true, "out": true, ".vercel": true, ".turbo": true,
}

// Sniff scans projectDir and returns the inferred infrastructure. Missing
// files / unreadable entries are skipped, not fatal — partial signal is useful.
func Sniff(projectDir string) (*Result, error) {
	res := &Result{}
	signalSet := map[string]bool{} // dedup "resource|reason"
	secretSet := map[string]bool{}

	addSignal := func(r Resource, reason string) {
		key := string(r) + "|" + reason
		if signalSet[key] {
			return
		}
		signalSet[key] = true
		res.Signals = append(res.Signals, Signal{Resource: r, Reason: reason})
	}

	// 1. package.json dependencies.
	if deps, err := readDeps(projectDir); err == nil {
		for _, ds := range depSignals {
			if deps[ds.dep] {
				addSignal(ds.resource, `dep "`+ds.dep+`"`)
			}
		}
		for _, ad := range authDeps {
			if deps[ad] {
				res.Auth = true
				secretSet["AUTH_SECRET"] = true
			}
		}
	}

	// 2. wrangler config — authoritative declared bindings.
	if w, err := readWrangler(projectDir); err == nil && w != nil {
		res.Wrangler = w
		for _, b := range w.D1 {
			addSignal(ResD1, "wrangler d1_databases["+b.Name+"]")
		}
		for _, b := range w.KV {
			addSignal(ResKV, "wrangler kv_namespaces["+b.Name+"]")
		}
		for _, b := range w.R2 {
			addSignal(ResR2, "wrangler r2_buckets["+b.Name+"]")
		}
		for _, b := range w.Hyperdrive {
			addSignal(ResHyperdrive, "wrangler hyperdrive["+b.Name+"]")
		}
		for _, b := range w.Vectorize {
			addSignal(ResVectorize, "wrangler vectorize["+b.Name+"]")
		}
		for _, b := range w.Queues {
			addSignal(ResQueue, "wrangler queues["+b.Name+"]")
		}
		if w.AI != "" {
			addSignal(ResAI, "wrangler ai["+w.AI+"]")
		}
	}

	// 3. Source files.
	_ = filepath.WalkDir(projectDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable
		}
		if d.IsDir() {
			if skipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if !sourceExts[strings.ToLower(filepath.Ext(path))] {
			return nil
		}
		data, err := os.ReadFile(path) // #nosec G304 — scanning the user's own project
		if err != nil {
			return nil
		}
		res.FilesParsed++
		src := string(data)

		for _, ss := range sourceSignals {
			if ss.re.MatchString(src) {
				addSignal(ss.resource, ss.reason)
			}
		}
		for _, m := range envRef.FindAllStringSubmatch(src, -1) {
			name := m[1]
			if strings.HasPrefix(name, "NEXT_PUBLIC_") || bindingNames[name] {
				continue
			}
			secretSet[name] = true
		}
		return nil
	})

	res.Secrets = sortedKeys(secretSet)
	return res, nil
}

func readDeps(projectDir string) (map[string]bool, error) {
	data, err := os.ReadFile(filepath.Join(projectDir, "package.json")) // #nosec G304
	if err != nil {
		return nil, err
	}
	var pkg struct {
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return nil, err
	}
	out := map[string]bool{}
	for name := range pkg.Dependencies {
		out[name] = true
	}
	for name := range pkg.DevDependencies {
		out[name] = true
	}
	return out, nil
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
