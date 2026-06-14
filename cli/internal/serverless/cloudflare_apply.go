package serverless

import (
	"context"
	"fmt"

	"github.com/aynaash/nextdeploy/shared/config"
)

// Apply reconciles the declared IaC layer: it computes a read-only plan (the
// diff), then provisions everything that is missing (create-if-absent),
// returning the plan so the caller can report what was created / updated /
// already in place.
//
// This is the "recreate resources" path: a D1 database, KV namespace, queue,
// Hyperdrive config, etc. that was deleted out-of-band shows up as a Create in
// the plan and is recreated by ProvisionResources. Re-running is safe — already
// existing resources are no-ops, and D1 migrations are idempotent (tracked in
// the _nextdeploy_migrations table), so a recreated database re-applies its
// schema exactly once.
//
// Apply stops on the first immutable drift (e.g. Vectorize dimension change),
// surfacing it instead of silently destroying data.
func (p *CloudflareProvider) Apply(ctx context.Context, cfg *config.NextDeployConfig) (*PlanResult, error) {
	plan, err := p.Plan(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("plan: %w", err)
	}
	if plan.HasDrift() {
		return plan, fmt.Errorf("immutable drift detected — resolve it before applying (see plan)")
	}
	if err := p.ProvisionResources(ctx, cfg); err != nil {
		return plan, fmt.Errorf("provision: %w", err)
	}
	return plan, nil
}
