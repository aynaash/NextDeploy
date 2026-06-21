package serverless

import (
	"context"
	"fmt"
	"maps"
	"strings"
	"sync"
	"time"

	"github.com/aynaash/nextdeploy/cli/internal/serverless/cfstate"
	"github.com/aynaash/nextdeploy/shared/config"
	"github.com/aynaash/nextdeploy/shared/secrets"
)

// resourceMap holds the IDs of resources provisioned by ensure* methods so
// later code (binding metadata, plan diffs) can resolve `ref: <name>` to a
// real CF UUID. Keys are namespaced as "<kind>:<name>" (e.g.
// "hyperdrive:pesastream-db") so different resource types can share a name.
type resourceMap struct {
	mu sync.RWMutex
	m  map[string]string
}

func newResourceMap() *resourceMap {
	return &resourceMap{m: map[string]string{}}
}

func (r *resourceMap) set(kind, name, id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.m[kind+":"+name] = id
}

func (r *resourceMap) get(kind, name string) (string, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	id, ok := r.m[kind+":"+name]
	return id, ok
}

// snapshot returns a copy of the "kind:name" → id map for persistence.
func (r *resourceMap) snapshot() map[string]string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make(map[string]string, len(r.m))
	maps.Copy(out, r.m)
	return out
}

// ProvisionResources walks cfg.Serverless.Cloudflare.Resources and calls the
// matching ensure* helper for each declared resource. Stops on first error.
// Results are stashed on p.provisioned for binding ref resolution at upload
// time. Safe to call before DeployCompute.
func (p *CloudflareProvider) ProvisionResources(ctx context.Context, cfg *config.NextDeployConfig) error {
	if cfg.Serverless == nil || cfg.Serverless.Cloudflare == nil || cfg.Serverless.Cloudflare.Resources == nil {
		return nil
	}
	res := cfg.Serverless.Cloudflare.Resources
	if p.provisioned == nil {
		p.provisioned = newResourceMap()
	}

	for _, d := range res.D1 {
		id, err := p.ensureD1Database(ctx, d)
		if err != nil {
			return fmt.Errorf("d1 %q: %w", d.Name, err)
		}
		p.provisioned.set("d1", d.Name, id)
	}

	for _, k := range res.KV {
		id, err := p.ensureKVNamespace(ctx, k.Name)
		if err != nil {
			return fmt.Errorf("kv %q: %w", k.Name, err)
		}
		p.provisioned.set("kv", k.Name, id)
	}

	for _, h := range res.Hyperdrive {
		id, err := p.ensureHyperdrive(ctx, h)
		if err != nil {
			return fmt.Errorf("hyperdrive %q: %w", h.Name, err)
		}
		p.provisioned.set("hyperdrive", h.Name, id)
	}

	for _, q := range res.Queues {
		id, err := p.ensureQueue(ctx, q.Name)
		if err != nil {
			return fmt.Errorf("queue %q: %w", q.Name, err)
		}
		p.provisioned.set("queue", q.Name, id)
	}

	for _, v := range res.Vectorize {
		id, err := p.ensureVectorizeIndex(ctx, v)
		if err != nil {
			return fmt.Errorf("vectorize %q: %w", v.Name, err)
		}
		p.provisioned.set("vectorize", v.Name, id)
	}

	for _, g := range res.AIGateway {
		slug, err := p.ensureAIGateway(ctx, g)
		if err != nil {
			return fmt.Errorf("ai_gateway %q: %w", g.Slug, err)
		}
		p.provisioned.set("ai_gateway", g.Slug, slug)
	}

	for _, d := range res.DNS {
		if err := p.ensureDNSRecord(ctx, d); err != nil {
			return fmt.Errorf("dns %s/%s: %w", d.Type, d.Name, err)
		}
	}

	if res.ZoneSettings != nil {
		if err := p.ensureZoneSettings(ctx, *res.ZoneSettings); err != nil {
			return fmt.Errorf("zone_settings: %w", err)
		}
	}

	// Persist the provisioned resource IDs to an encrypted state manifest and
	// surface any orphans (recorded earlier, no longer declared). Non-fatal:
	// the deploy must not fail just because we couldn't write the manifest.
	p.persistResourceState()

	return nil
}

// cfStatePath is where the encrypted resource manifest lives, relative to the
// project root.
const cfStatePath = ".nextdeploy/cf-state.json.enc"

// persistResourceState writes the provisioned resource IDs to the encrypted
// manifest and logs any orphaned resources. The encryption key is derived from
// the account's API token, so the manifest is only readable with the same
// credential. Every failure is logged and swallowed — this is bookkeeping, not
// a deploy gate.
func (p *CloudflareProvider) persistResourceState() {
	if p.provisioned == nil || p.apiToken == "" {
		return
	}
	key, err := secrets.DeriveKey(p.apiToken)
	if err != nil {
		p.log.Warn("state manifest: derive key failed (skipping): %v", err)
		return
	}

	prior, err := cfstate.Load(cfStatePath, key)
	if err != nil {
		// Don't mask a corrupt/foreign state file, but don't block the deploy.
		p.log.Warn("state manifest: could not read prior state (%v) — rewriting fresh", err)
		prior = cfstate.New()
	}

	snap := p.provisioned.snapshot()
	declared := make(map[string]bool, len(snap))
	for k := range snap {
		declared[k] = true
	}
	for _, orphan := range prior.Orphans(declared) {
		p.log.Warn("orphaned resource: %s %q (id=%s) was provisioned earlier but is no longer in nextdeploy.yml — clean up manually if unused",
			orphan.Kind, orphan.Name, orphan.ID)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	manifest := cfstate.New()
	for k, id := range snap {
		kind, name, found := strings.Cut(k, ":")
		if !found {
			continue
		}
		manifest.Set(kind, name, id, now)
	}
	if err := cfstate.Save(cfStatePath, manifest, key); err != nil {
		p.log.Warn("state manifest: save failed (non-fatal): %v", err)
		return
	}
	p.log.Info("Recorded %d resource(s) to encrypted state manifest (%s)", len(snap), cfStatePath)
}
