// Package cfstate is the persistent, encrypted record of the Cloudflare
// resources NextDeploy has provisioned (D1, KV, Hyperdrive, Queues, Vectorize,
// …) keyed by kind+name → real CF ID.
//
// Why it exists: name-based `ensure*` already prevents duplicate creation, but a
// manifest lets the engine (a) skip live `list` round-trips on re-runs and
// (b) detect orphans — resources it recorded but the user no longer declares in
// nextdeploy.yml. The file is AES-256-GCM encrypted (the resource graph of an
// account is sensitive metadata) and written 0600.
package cfstate

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/aynaash/nextdeploy/shared/secrets"
)

// Record is one provisioned Cloudflare resource.
type Record struct {
	Kind      string `json:"kind"`       // d1, kv, hyperdrive, queue, vectorize, ai_gateway
	Name      string `json:"name"`       // declared name
	ID        string `json:"id"`         // CF resource UUID / identifier
	UpdatedAt string `json:"updated_at"` // RFC3339, stamped by the caller
}

// Manifest maps "kind:name" → Record.
type Manifest struct {
	Version   int               `json:"version"`
	Resources map[string]Record `json:"resources"`
}

// New returns an empty manifest.
func New() *Manifest {
	return &Manifest{Version: 1, Resources: map[string]Record{}}
}

// Key is the canonical map key for a (kind, name) pair. Exported so callers can
// build the "declared" set passed to Orphans.
func Key(kind, name string) string { return kind + ":" + name }

// Set records (or replaces) a resource.
func (m *Manifest) Set(kind, name, id, updatedAt string) {
	if m.Resources == nil {
		m.Resources = map[string]Record{}
	}
	m.Resources[Key(kind, name)] = Record{Kind: kind, Name: name, ID: id, UpdatedAt: updatedAt}
}

// Get returns the recorded resource, if any.
func (m *Manifest) Get(kind, name string) (Record, bool) {
	r, ok := m.Resources[Key(kind, name)]
	return r, ok
}

// List returns all records sorted by kind:name (deterministic).
func (m *Manifest) List() []Record {
	out := make([]Record, 0, len(m.Resources))
	for _, r := range m.Resources {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return Key(out[i].Kind, out[i].Name) < Key(out[j].Kind, out[j].Name) })
	return out
}

// Orphans returns recorded resources whose key is NOT in declared (the set of
// "kind:name" the user currently declares). These were provisioned earlier but
// dropped from nextdeploy.yml — candidates for manual cleanup.
func (m *Manifest) Orphans(declared map[string]bool) []Record {
	var out []Record
	for k, r := range m.Resources {
		if !declared[k] {
			out = append(out, r)
		}
	}
	sort.Slice(out, func(i, j int) bool { return Key(out[i].Kind, out[i].Name) < Key(out[j].Kind, out[j].Name) })
	return out
}

// Save writes the manifest AES-256-GCM encrypted (key must be 32 bytes) to path,
// mode 0600. Parent dirs are created.
func Save(path string, m *Manifest, encKey []byte) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	ciphertext, err := secrets.Encrypt(data, encKey)
	if err != nil {
		return fmt.Errorf("encrypt manifest: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	return os.WriteFile(path, ciphertext, 0o600)
}

// Load reads + decrypts the manifest. A missing file yields a fresh (empty)
// manifest, not an error — first run is normal. A decrypt failure (wrong key or
// corruption) IS an error so a split-brain isn't silently masked.
func Load(path string, encKey []byte) (*Manifest, error) {
	data, err := os.ReadFile(path) // #nosec G304 -- caller-controlled state path
	if os.IsNotExist(err) {
		return New(), nil
	}
	if err != nil {
		return nil, err
	}
	// secrets.Encrypt base64-encodes its output, but secrets.Decrypt expects raw
	// ciphertext — decode here so the pair round-trips correctly.
	rawCipher, err := base64.StdEncoding.DecodeString(string(data))
	if err != nil {
		return nil, fmt.Errorf("decode manifest (corrupt state): %w", err)
	}
	plain, err := secrets.Decrypt(rawCipher, encKey)
	if err != nil {
		return nil, fmt.Errorf("decrypt manifest (wrong key or corrupt state): %w", err)
	}
	var m Manifest
	if err := json.Unmarshal(plain, &m); err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}
	if m.Resources == nil {
		m.Resources = map[string]Record{}
	}
	return &m, nil
}
