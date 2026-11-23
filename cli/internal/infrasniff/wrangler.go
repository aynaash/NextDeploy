package infrasniff

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

// WranglerConfig is the subset of a wrangler config the sniffer understands.
// Unlike the heuristic source scan, these bindings are authoritative: the user
// already declared exact names and resource IDs, so init can prefill them
// verbatim instead of guessing.
type WranglerConfig struct {
	Source             string                  // file the config was read from
	Name               string                  // worker name
	CompatibilityDate  string                  // compatibility date
	CompatibilityFlags []string                // compatibility flags
	D1                 []WranglerBinding       // binding + database_name + database_id
	KV                 []WranglerBinding       // binding + id
	R2                 []WranglerBinding       // binding + bucket_name
	Hyperdrive         []WranglerBinding       // binding + id
	Vectorize          []WranglerBinding       // binding + index_name
	Queues             []WranglerBinding       // producer binding + queue
	QueueConsumers     []WranglerQueueConsumer // consumer queue settings
	AI                 string                  // AI binding name (empty when absent)
	DurableObjects     []WranglerDOBinding     // durable object bindings
	Migrations         []WranglerMigration     // durable object SQLite migrations
	Crons              []string                // cron triggers
	CustomDomains      []string                // custom domains
	Vars               []string                // plain-text var names (not secrets)
}

// WranglerBinding is one declared binding. Fields are populated per kind; unused
// ones stay empty (e.g. R2 has Resource=bucket_name, ID empty).
type WranglerBinding struct {
	Name          string // JS binding variable (e.g. "DB")
	Resource      string // database_name / bucket_name / index_name / queue
	ID            string // database_id / namespace id / hyperdrive id
	MigrationsDir string // D1 migrations_dir when declared in wrangler config
}

type WranglerQueueConsumer struct {
	Queue           string
	MaxBatchSize    int
	MaxBatchTimeout int
	MaxRetries      int
	DeadLetterQueue string
}

type WranglerDOBinding struct {
	Name      string
	ClassName string
	Script    string
}

type WranglerMigration struct {
	Tag              string
	NewSQLiteClasses []string
	NewClasses       []string
	DeletedClasses   []string
}

// wranglerFiles are checked in order; the first present wins. jsonc/json are the
// modern default; toml is the legacy format.
var wranglerFiles = []string{"wrangler.jsonc", "wrangler.json", "wrangler.toml"}

type wBinding struct {
	Binding       string `json:"binding" toml:"binding"`
	Name          string `json:"name" toml:"name"`
	DatabaseName  string `json:"database_name" toml:"database_name"`
	DatabaseID    string `json:"database_id" toml:"database_id"`
	MigrationsDir string `json:"migrations_dir" toml:"migrations_dir"`
	BucketName    string `json:"bucket_name" toml:"bucket_name"`
	ID            string `json:"id" toml:"id"`
	IndexName     string `json:"index_name" toml:"index_name"`
	Queue         string `json:"queue" toml:"queue"`
}

func (w *wBinding) getBindingName() string {
	if w.Binding != "" {
		return w.Binding
	}
	return w.Name
}

type wConsumer struct {
	Queue           string `json:"queue" toml:"queue"`
	MaxBatchSize    int    `json:"max_batch_size" toml:"max_batch_size"`
	MaxBatchTimeout int    `json:"max_batch_timeout" toml:"max_batch_timeout"`
	MaxRetries      int    `json:"max_retries" toml:"max_retries"`
	DeadLetterQueue string `json:"dead_letter_queue" toml:"dead_letter_queue"`
}

type wQueues struct {
	Producers []wBinding  `json:"producers" toml:"producers"`
	Consumers []wConsumer `json:"consumers" toml:"consumers"`
}

type wDOBinding struct {
	Name       string `json:"name" toml:"name"`
	Binding    string `json:"binding" toml:"binding"`
	ClassName  string `json:"class_name" toml:"class_name"`
	ScriptName string `json:"script_name" toml:"script_name"`
}

func (d *wDOBinding) getName() string {
	if d.Name != "" {
		return d.Name
	}
	return d.Binding
}

type wDurableObjects struct {
	Bindings []wDOBinding `json:"bindings" toml:"bindings"`
}

type wMigration struct {
	Tag              string   `json:"tag" toml:"tag"`
	NewSQLiteClasses []string `json:"new_sqlite_classes" toml:"new_sqlite_classes"`
	NewClasses       []string `json:"new_classes" toml:"new_classes"`
	DeletedClasses   []string `json:"deleted_classes" toml:"deleted_classes"`
}

type wTriggers struct {
	Crons []string `json:"crons" toml:"crons"`
}

type wranglerRaw struct {
	Name               string                  `json:"name" toml:"name"`
	CompatibilityDate  string                  `json:"compatibility_date" toml:"compatibility_date"`
	CompatibilityFlags []string                `json:"compatibility_flags" toml:"compatibility_flags"`
	D1Databases        []wBinding              `json:"d1_databases" toml:"d1_databases"`
	KVNamespaces       []wBinding              `json:"kv_namespaces" toml:"kv_namespaces"`
	R2Buckets          []wBinding              `json:"r2_buckets" toml:"r2_buckets"`
	Hyperdrive         []wBinding              `json:"hyperdrive" toml:"hyperdrive"`
	Vectorize          []wBinding              `json:"vectorize" toml:"vectorize"`
	Queues             *wQueues                `json:"queues" toml:"queues"`
	AI                 any                     `json:"ai" toml:"ai"`
	DurableObjects     *wDurableObjects        `json:"durable_objects" toml:"durable_objects"`
	Migrations         []wMigration            `json:"migrations" toml:"migrations"`
	Triggers           *wTriggers              `json:"triggers" toml:"triggers"`
	Routes             any                     `json:"routes" toml:"routes"`
	Vars               map[string]any          `json:"vars" toml:"vars"`
	Env                map[string]*wranglerRaw `json:"env" toml:"env"`
}

// findWranglerFile checks projectDir, then walks up to parent and grandparent
// directories (supporting monorepos like apps/web).
func findWranglerFile(projectDir string) (string, string) {
	dir := projectDir
	for i := 0; i < 3; i++ {
		for _, name := range wranglerFiles {
			p := filepath.Join(dir, name)
			if info, err := os.Stat(p); err == nil && !info.IsDir() {
				return p, name
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir || parent == "" {
			break
		}
		dir = parent
	}
	return "", ""
}

// readWrangler finds and parses the first wrangler config in projectDir or its parents.
// Returns (nil, nil) when none is present.
func readWrangler(projectDir string) (*WranglerConfig, error) {
	path, name := findWranglerFile(projectDir)
	if path == "" {
		return nil, nil
	}
	data, err := os.ReadFile(path) // #nosec G304 — user's own project
	if err != nil {
		return nil, err
	}
	raw, err := parseWranglerBytes(name, data)
	if err != nil {
		return nil, err
	}
	rel, relErr := filepath.Rel(projectDir, path)
	source := name
	if relErr == nil && rel != "" {
		source = rel
	}
	return wranglerFromRaw(source, raw), nil
}

func parseWranglerBytes(name string, data []byte) (*wranglerRaw, error) {
	var raw wranglerRaw
	if filepath.Ext(name) == ".toml" {
		if err := toml.Unmarshal(data, &raw); err != nil {
			return nil, err
		}
		return &raw, nil
	}
	// .json / .jsonc — strip comments + trailing commas before decoding.
	if err := json.Unmarshal(stripJSONC(data), &raw); err != nil {
		return nil, err
	}
	return &raw, nil
}

func wranglerFromRaw(source string, raw *wranglerRaw) *WranglerConfig {
	cfg := &WranglerConfig{
		Source:             source,
		Name:               raw.Name,
		CompatibilityDate:  raw.CompatibilityDate,
		CompatibilityFlags: raw.CompatibilityFlags,
	}
	for _, d := range raw.D1Databases {
		cfg.D1 = append(cfg.D1, WranglerBinding{
			Name:          d.getBindingName(),
			Resource:      d.DatabaseName,
			ID:            d.DatabaseID,
			MigrationsDir: d.MigrationsDir,
		})
	}
	for _, k := range raw.KVNamespaces {
		cfg.KV = append(cfg.KV, WranglerBinding{Name: k.getBindingName(), ID: k.ID})
	}
	for _, r := range raw.R2Buckets {
		cfg.R2 = append(cfg.R2, WranglerBinding{Name: r.getBindingName(), Resource: r.BucketName})
	}
	for _, h := range raw.Hyperdrive {
		cfg.Hyperdrive = append(cfg.Hyperdrive, WranglerBinding{Name: h.getBindingName(), ID: h.ID})
	}
	for _, v := range raw.Vectorize {
		cfg.Vectorize = append(cfg.Vectorize, WranglerBinding{Name: v.getBindingName(), Resource: v.IndexName})
	}

	if raw.Queues != nil {
		for _, q := range raw.Queues.Producers {
			cfg.Queues = append(cfg.Queues, WranglerBinding{Name: q.getBindingName(), Resource: q.Queue})
		}
		for _, c := range raw.Queues.Consumers {
			cfg.QueueConsumers = append(cfg.QueueConsumers, WranglerQueueConsumer{
				Queue:           c.Queue,
				MaxBatchSize:    c.MaxBatchSize,
				MaxBatchTimeout: c.MaxBatchTimeout,
				MaxRetries:      c.MaxRetries,
				DeadLetterQueue: c.DeadLetterQueue,
			})
		}
	}

	cfg.AI = parseAI(raw.AI)

	if raw.DurableObjects != nil {
		for _, b := range raw.DurableObjects.Bindings {
			name := b.getName()
			if name != "" && b.ClassName != "" {
				cfg.DurableObjects = append(cfg.DurableObjects, WranglerDOBinding{
					Name:      name,
					ClassName: b.ClassName,
					Script:    b.ScriptName,
				})
			}
		}
	}

	for _, m := range raw.Migrations {
		cfg.Migrations = append(cfg.Migrations, WranglerMigration{
			Tag:              m.Tag,
			NewSQLiteClasses: m.NewSQLiteClasses,
			NewClasses:       m.NewClasses,
			DeletedClasses:   m.DeletedClasses,
		})
	}

	if raw.Triggers != nil {
		cfg.Crons = append(cfg.Crons, raw.Triggers.Crons...)
	}

	cfg.CustomDomains = parseRoutes(raw.Routes)

	for name := range raw.Vars {
		cfg.Vars = append(cfg.Vars, name)
	}

	if raw.Env != nil {
		mergeEnv(cfg, raw.Env)
	}

	return cfg
}

func parseAI(raw any) string {
	if s, ok := raw.(string); ok {
		return s
	}
	if m, ok := raw.(map[string]any); ok {
		if b, ok := m["binding"].(string); ok {
			return b
		}
	}
	return ""
}

func parseRoutes(raw any) []string {
	arr, ok := raw.([]any)
	if !ok {
		return nil
	}
	var domains []string
	for _, item := range arr {
		switch v := item.(type) {
		case string:
			d := strings.Split(v, "/")[0]
			d = strings.TrimPrefix(d, "*.")
			if d != "" && strings.Contains(d, ".") {
				domains = append(domains, d)
			}
		case map[string]any:
			pattern, _ := v["pattern"].(string)
			if pattern != "" {
				isCustom, _ := v["custom_domain"].(bool)
				if isCustom {
					domains = append(domains, pattern)
				} else {
					d := strings.Split(pattern, "/")[0]
					d = strings.TrimPrefix(d, "*.")
					if d != "" && strings.Contains(d, ".") {
						domains = append(domains, d)
					}
				}
			}
		}
	}
	return domains
}

func mergeEnv(cfg *WranglerConfig, envRaw map[string]*wranglerRaw) {
	var prodEnv *wranglerRaw
	if p, ok := envRaw["production"]; ok && p != nil {
		prodEnv = p
	} else if p, ok := envRaw["prod"]; ok && p != nil {
		prodEnv = p
	}
	if prodEnv == nil {
		return
	}

	if len(cfg.CustomDomains) == 0 {
		cfg.CustomDomains = parseRoutes(prodEnv.Routes)
	}
	if len(cfg.Crons) == 0 && prodEnv.Triggers != nil {
		cfg.Crons = append(cfg.Crons, prodEnv.Triggers.Crons...)
	}
	if len(cfg.DurableObjects) == 0 && prodEnv.DurableObjects != nil {
		for _, b := range prodEnv.DurableObjects.Bindings {
			name := b.getName()
			if name != "" && b.ClassName != "" {
				cfg.DurableObjects = append(cfg.DurableObjects, WranglerDOBinding{
					Name:      name,
					ClassName: b.ClassName,
					Script:    b.ScriptName,
				})
			}
		}
	}
	if len(cfg.Migrations) == 0 && len(prodEnv.Migrations) > 0 {
		for _, m := range prodEnv.Migrations {
			cfg.Migrations = append(cfg.Migrations, WranglerMigration{
				Tag:              m.Tag,
				NewSQLiteClasses: m.NewSQLiteClasses,
				NewClasses:       m.NewClasses,
				DeletedClasses:   m.DeletedClasses,
			})
		}
	}
}

// stripJSONC removes // line comments, /* */ block comments, and trailing
// commas, producing parseable JSON. String contents (and escapes) are
// preserved so a "//" inside a string literal is left intact.
func stripJSONC(in []byte) []byte {
	out := make([]byte, 0, len(in))
	inString := false
	escaped := false
	for i := 0; i < len(in); i++ {
		c := in[i]
		if inString {
			out = append(out, c)
			if escaped {
				escaped = false
			} else if c == '\\' {
				escaped = true
			} else if c == '"' {
				inString = false
			}
			continue
		}
		switch {
		case c == '"':
			inString = true
			out = append(out, c)
		case c == '/' && i+1 < len(in) && in[i+1] == '/':
			for i < len(in) && in[i] != '\n' {
				i++
			}
			if i < len(in) {
				out = append(out, '\n')
			}
		case c == '/' && i+1 < len(in) && in[i+1] == '*':
			i += 2
			for i+1 < len(in) && (in[i] != '*' || in[i+1] != '/') {
				i++
			}
			i++ // land on '/', loop's i++ moves past
		default:
			out = append(out, c)
		}
	}
	return removeTrailingCommas(out)
}

// removeTrailingCommas drops a comma that is immediately followed (ignoring
// whitespace) by a closing } or ]. Respects string literals.
func removeTrailingCommas(in []byte) []byte {
	out := make([]byte, 0, len(in))
	inString := false
	escaped := false
	for i := range in {
		c := in[i]
		if inString {
			out = append(out, c)
			if escaped {
				escaped = false
			} else if c == '\\' {
				escaped = true
			} else if c == '"' {
				inString = false
			}
			continue
		}
		if c == '"' {
			inString = true
			out = append(out, c)
			continue
		}
		if c == ',' {
			j := i + 1
			for j < len(in) && (in[j] == ' ' || in[j] == '\t' || in[j] == '\n' || in[j] == '\r') {
				j++
			}
			if j < len(in) && (in[j] == '}' || in[j] == ']') {
				continue // drop the comma
			}
		}
		out = append(out, c)
	}
	return out
}
