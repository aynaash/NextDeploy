package serverless

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"strings"
	"sync"

	"github.com/aynaash/nextdeploy/cli/internal/serverless/cfstate"
	"github.com/aynaash/nextdeploy/shared/config"
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
func (p *CloudflareProvider) ProvisionResources(ctx context.Context, cfg *config.NextDeployConfig) (retErr error) {
	if cfg.Serverless == nil || cfg.Serverless.Cloudflare == nil || cfg.Serverless.Cloudflare.Resources == nil {
		return nil
	}
	res := cfg.Serverless.Cloudflare.Resources
	if p.remoteStateEnabled() {
		bucket := p.getBucketName(cfg)
		if err := p.ensureR2BucketExists(ctx, bucket); err != nil {
			return fmt.Errorf("ensure R2 bucket for remote state: %w", err)
		}
		if err := p.SyncResourceState(ctx, cfg); err != nil {
			return err
		}
	}
	if p.provisioned == nil {
		p.provisioned = newResourceMap()
	}
	// Persist whatever we manage to provision even if a later resource fails —
	// each loop below returns on first error, and without this a partial
	// provision would leave already-created (paid) resources unrecorded and
	// invisible to orphan detection / `nextdeploy destroy`.
	defer func() {
		if err := p.persistResourceState(ctx, cfg); err != nil {
			retErr = errors.Join(retErr, fmt.Errorf("persist resource state: %w", err))
		}
	}()

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

	return nil
}

// cfStatePath returns the env-scoped path for the encrypted resource manifest.
// Production uses the legacy path (backward compatible with existing deploys);
// non-production environments get an env-suffixed path so a preview teardown
// can never read — or delete — production's provisioned resources.
func (p *CloudflareProvider) cfStatePath() string {
	if p.isProductionEnv() {
		return ".nextdeploy/cf-state.json.enc"
	}
	return fmt.Sprintf(".nextdeploy/cf-state.%s.json.enc", p.environment)
}

// refResolverWithFallback tries the in-memory provisioned map first, then falls
// back to the encrypted cf-state manifest. This lets `ship --no-provision`
// (the CI path: apply in one process, ship in another) resolve ref: bindings
// from resources provisioned by a prior `apply` without re-provisioning.
func (p *CloudflareProvider) refResolverWithFallback() refResolver {
	return func(kind, name string) (string, bool) {
		// Fast path: in-memory map from a ProvisionResources call in this process.
		if p.provisioned != nil {
			if id, ok := p.provisioned.get(kind, name); ok {
				return id, true
			}
		}
		// Fallback: the persisted state manifest from a prior apply/ship.
		if p.apiToken == "" {
			return "", false
		}
		key, err := p.cfStateEncryptionKey()
		if err != nil {
			return "", false
		}
		manifest, err := p.loadResourceManifest(p.cfStatePath(), key)
		if err != nil {
			p.log.Warn("ref resolution: could not read state manifest (%v)", err)
			return "", false
		}
		rec, ok := manifest.Get(kind, name)
		if ok {
			p.log.Info("Resolved ref %s:%s from state manifest (id=%s)", kind, name, rec.ID)
			return rec.ID, true
		}
		// Fallback: if not found in state manifest (e.g. cache was invalidated on token rotation),
		// query Cloudflare API directly.
		liveID, liveErr := p.findResourceIDLive(context.Background(), kind, name)
		if liveErr == nil && liveID != "" {
			p.log.Info("Resolved ref %s:%s live from Cloudflare (id=%s)", kind, name, liveID)
			return liveID, true
		}
		return "", false
	}
}

// findResourceIDLive queries the Cloudflare API directly to find the UUID / ID
// of an existing resource by name when it is not in the local state manifest.
func (p *CloudflareProvider) findResourceIDLive(ctx context.Context, kind, name string) (string, error) {
	switch kind {
	case "d1":
		return p.findD1ID(ctx, name)
	case "kv":
		return p.findKVNamespaceID(ctx, name)
	case "hyperdrive":
		return p.findHyperdriveID(ctx, name)
	case "queue":
		return p.findQueueID(ctx, name)
	case "vectorize", "ai_gateway":
		return name, nil
	default:
		return "", fmt.Errorf("unknown resource kind: %s", kind)
	}
}

// persistResourceState writes provisioned IDs locally and syncs them to R2
// when shared state encryption is configured.
func (p *CloudflareProvider) persistResourceState(ctx context.Context, cfg *config.NextDeployConfig) error {
	if p.provisioned == nil || p.apiToken == "" {
		return nil
	}
	key, err := p.cfStateEncryptionKey()
	if err != nil {
		return fmt.Errorf("derive encryption key: %w", err)
	}

	statePath := p.cfStatePath()
	prior, err := p.loadResourceManifest(statePath, key)
	if err != nil {
		return fmt.Errorf("read prior manifest: %w", err)
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

	now := stateRecordTimestamp()
	// Merge into the prior manifest rather than rebuilding from scratch. The old
	// cfstate.New() dropped orphans (recorded earlier, no longer declared) right
	// after warning about them — destroying the only record that made cleanup
	// possible. Keeping them means they stay visible every run until `destroy`.
	manifest := prior
	for k, id := range snap {
		kind, name, found := strings.Cut(k, ":")
		if !found {
			continue
		}
		manifest.Set(kind, name, id, now)
	}
	if err := cfstate.Save(statePath, manifest, key); err != nil {
		return fmt.Errorf("save local manifest: %w", err)
	}
	p.log.Info("Recorded %d resource(s) to encrypted state manifest (%s)", len(snap), statePath)
	if p.remoteStateEnabled() {
		if err := p.SyncResourceState(ctx, cfg); err != nil {
			return err
		}
	}
	return nil
}
