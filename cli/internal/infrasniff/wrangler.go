package infrasniff

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// WranglerConfig is the subset of a wrangler config the sniffer understands.
// Unlike the heuristic source scan, these bindings are authoritative: the user
// already declared exact names and resource IDs, so init can prefill them
// verbatim instead of guessing.
type WranglerConfig struct {
	Source     string            // file the config was read from
	Name       string            // worker name
	D1         []WranglerBinding // binding + database_name + database_id
	KV         []WranglerBinding // binding + id
	R2         []WranglerBinding // binding + bucket_name
	Hyperdrive []WranglerBinding // binding + id
	Vectorize  []WranglerBinding // binding + index_name
	Queues     []WranglerBinding // producer binding + queue
	AI         string            // AI binding name (empty when absent)
	Vars       []string          // plain-text var names (not secrets)
}

// WranglerBinding is one declared binding. Fields are populated per kind; unused
// ones stay empty (e.g. R2 has Resource=bucket_name, ID empty).
type WranglerBinding struct {
	Name     string // JS binding variable (e.g. "DB")
	Resource string // database_name / bucket_name / index_name / queue
	ID       string // database_id / namespace id / hyperdrive id
}

// wranglerFiles are checked in order; the first present wins. jsonc/json are the
// modern default; toml is the legacy format.
var wranglerFiles = []string{"wrangler.jsonc", "wrangler.json", "wrangler.toml"}

type wBinding struct {
	Binding      string `json:"binding" toml:"binding"`
	DatabaseName string `json:"database_name" toml:"database_name"`
	DatabaseID   string `json:"database_id" toml:"database_id"`
	BucketName   string `json:"bucket_name" toml:"bucket_name"`
	ID           string `json:"id" toml:"id"`
	IndexName    string `json:"index_name" toml:"index_name"`
	Queue        string `json:"queue" toml:"queue"`
}

type wranglerRaw struct {
	Name         string                 `json:"name" toml:"name"`
	D1Databases  []wBinding             `json:"d1_databases" toml:"d1_databases"`
	KVNamespaces []wBinding             `json:"kv_namespaces" toml:"kv_namespaces"`
	R2Buckets    []wBinding             `json:"r2_buckets" toml:"r2_buckets"`
	Hyperdrive   []wBinding             `json:"hyperdrive" toml:"hyperdrive"`
	Vectorize    []wBinding             `json:"vectorize" toml:"vectorize"`
	Queues       *wQueues               `json:"queues" toml:"queues"`
	AI           *wAI                   `json:"ai" toml:"ai"`
	Vars         map[string]interface{} `json:"vars" toml:"vars"`
}

type wQueues struct {
	Producers []wBinding `json:"producers" toml:"producers"`
}

type wAI struct {
	Binding string `json:"binding" toml:"binding"`
}

// readWrangler finds and parses the first wrangler config in projectDir.
// Returns (nil, nil) when none is present.
func readWrangler(projectDir string) (*WranglerConfig, error) {
	for _, name := range wranglerFiles {
		path := filepath.Join(projectDir, name)
		data, err := os.ReadFile(path) // #nosec G304 — user's own project
		if err != nil {
			continue
		}
		raw, err := parseWranglerBytes(name, data)
		if err != nil {
			return nil, err
		}
		return wranglerFromRaw(name, raw), nil
	}
	return nil, nil
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
	cfg := &WranglerConfig{Source: source, Name: raw.Name}
	for _, d := range raw.D1Databases {
		cfg.D1 = append(cfg.D1, WranglerBinding{Name: d.Binding, Resource: d.DatabaseName, ID: d.DatabaseID})
	}
	for _, k := range raw.KVNamespaces {
		cfg.KV = append(cfg.KV, WranglerBinding{Name: k.Binding, ID: k.ID})
	}
	for _, r := range raw.R2Buckets {
		cfg.R2 = append(cfg.R2, WranglerBinding{Name: r.Binding, Resource: r.BucketName})
	}
	for _, h := range raw.Hyperdrive {
		cfg.Hyperdrive = append(cfg.Hyperdrive, WranglerBinding{Name: h.Binding, ID: h.ID})
	}
	for _, v := range raw.Vectorize {
		cfg.Vectorize = append(cfg.Vectorize, WranglerBinding{Name: v.Binding, Resource: v.IndexName})
	}
	if raw.Queues != nil {
		for _, q := range raw.Queues.Producers {
			cfg.Queues = append(cfg.Queues, WranglerBinding{Name: q.Binding, Resource: q.Queue})
		}
	}
	if raw.AI != nil && raw.AI.Binding != "" {
		cfg.AI = raw.AI.Binding
	}
	for name := range raw.Vars {
		cfg.Vars = append(cfg.Vars, name)
	}
	return cfg
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
			for i+1 < len(in) && !(in[i] == '*' && in[i+1] == '/') {
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
