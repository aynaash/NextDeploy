package serverless

import (
	"context"

	"github.com/aynaash/nextdeploy/cli/internal/serverless/aws"
	"github.com/aynaash/nextdeploy/cli/internal/serverless/cloudflare"
	"github.com/aynaash/nextdeploy/cli/internal/serverless/cloudflare/adapter"
	"github.com/aynaash/nextdeploy/cli/internal/serverless/core"
	"github.com/aynaash/nextdeploy/shared"
	"github.com/aynaash/nextdeploy/shared/config"
	"github.com/aynaash/nextdeploy/shared/nextcore"
)

// Core provider contract & shared report types.
type (
	Provider              = core.Provider
	RollbackOptions       = core.RollbackOptions
	ServerlessResourceMap = core.ServerlessResourceMap
)

// AWS provider types.
type (
	AWSProvider     = aws.AWSProvider
	DeploymentEntry = aws.DeploymentEntry
	ImageOptConfig  = aws.ImageOptConfig
)

// Cloudflare provider & plan/tail types.
type (
	CloudflareProvider = cloudflare.CloudflareProvider
	PlanAction         = cloudflare.PlanAction
	PlanItem           = cloudflare.PlanItem
	PlanResult         = cloudflare.PlanResult
	UnmanagedResource  = cloudflare.UnmanagedResource
	CFTailMessage      = cloudflare.CFTailMessage
	CFTailLogEntry     = cloudflare.CFTailLogEntry
	CFTailException    = cloudflare.CFTailException
	CFTailEventDetail  = cloudflare.CFTailEventDetail
	CFTailRequest      = cloudflare.CFTailRequest
	CFTailResponse     = cloudflare.CFTailResponse
)

// PlanAction constants.
const (
	PlanCreate         = cloudflare.PlanCreate
	PlanUpdate         = cloudflare.PlanUpdate
	PlanNoOp           = cloudflare.PlanNoOp
	PlanImmutableDrift = cloudflare.PlanImmutableDrift
)

// NewAWSProvider creates a new AWS serverless provider.
func NewAWSProvider(verbose bool) *AWSProvider {
	return aws.NewAWSProvider(verbose)
}

// NewCloudflareProvider creates a new Cloudflare serverless provider.
func NewCloudflareProvider() *CloudflareProvider {
	return cloudflare.NewCloudflareProvider()
}

// CloudflareWorkerName computes the Worker script name for an app and environment.
func CloudflareWorkerName(appName, environment string) string {
	return cloudflare.CloudflareWorkerName(appName, environment)
}

// CloudflareAssetBucketName computes the R2 asset bucket name for an app and environment.
func CloudflareAssetBucketName(appName, environment string) string {
	return cloudflare.CloudflareAssetBucketName(appName, environment)
}

// BuildWorkerBundle adapts a Next.js standalone build into a single Cloudflare Worker module bundle.
func BuildWorkerBundle(
	ctx context.Context,
	standaloneDir string,
	meta *nextcore.NextCorePayload,
	cfg *config.NextDeployConfig,
	log *shared.Logger,
) (string, error) {
	return adapter.BuildWorkerBundle(ctx, standaloneDir, meta, cfg, log)
}
