package serverless

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	cwTypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	lambdaTypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/aws/smithy-go"

	"github.com/aynaash/nextdeploy/internal/packaging"
	cfgTypes "github.com/aynaash/nextdeploy/shared/config"
	"github.com/aynaash/nextdeploy/shared/nextcore"
)

// secretsExtensionLatestParam is the AWS-published public SSM parameter that
// resolves to the current x86_64 Parameters & Secrets Lambda Extension layer
// ARN for the caller's region — correct owner account AND latest version in a
// single ssm:GetParameter call. This is AWS's recommended resolution method
// (see "Retrieving the latest Lambda extension ARN version" in the Systems
// Manager user guide) and the only one that works cross-account: the layer is
// AWS-owned, so lambda:ListLayerVersions never succeeds against it. We attach
// the extension only to x86_64 functions (we never set Architectures, so Lambda
// defaults to x86_64).
const secretsExtensionLatestParam = "/aws/service/aws-parameters-and-secrets-lambda-extension/x86/latest"

// secretsExtensionLayerFallback maps a region to the x86_64 extension layer ARN
// to use when the SSM public parameter is unavailable (e.g. ssm:GetParameter is
// denied). The owner account ID differs per region and the version drifts as AWS
// republishes, so these are a pinned snapshot from the AWS docs (x86_64 table,
// last verified 2026-07). Layer versions are immutable, so a pinned version keeps
// resolving even after AWS publishes a newer one — this only misses new versions,
// it never breaks. Prefer secretsExtensionLatestParam; this is the safety net.
var secretsExtensionLayerFallback = map[string]string{
	"us-east-2":      "arn:aws:lambda:us-east-2:590474943231:layer:AWS-Parameters-and-Secrets-Lambda-Extension:94",
	"us-east-1":      "arn:aws:lambda:us-east-1:177933569100:layer:AWS-Parameters-and-Secrets-Lambda-Extension:88",
	"us-west-1":      "arn:aws:lambda:us-west-1:997803712105:layer:AWS-Parameters-and-Secrets-Lambda-Extension:84",
	"us-west-2":      "arn:aws:lambda:us-west-2:345057560386:layer:AWS-Parameters-and-Secrets-Lambda-Extension:88",
	"af-south-1":     "arn:aws:lambda:af-south-1:317013901791:layer:AWS-Parameters-and-Secrets-Lambda-Extension:85",
	"ap-east-1":      "arn:aws:lambda:ap-east-1:768336418462:layer:AWS-Parameters-and-Secrets-Lambda-Extension:81",
	"ap-east-2":      "arn:aws:lambda:ap-east-2:890742577149:layer:AWS-Parameters-and-Secrets-Lambda-Extension:54",
	"ap-south-2":     "arn:aws:lambda:ap-south-2:070087711984:layer:AWS-Parameters-and-Secrets-Lambda-Extension:76",
	"ap-southeast-3": "arn:aws:lambda:ap-southeast-3:490737872127:layer:AWS-Parameters-and-Secrets-Lambda-Extension:79",
	"ap-southeast-4": "arn:aws:lambda:ap-southeast-4:090732460067:layer:AWS-Parameters-and-Secrets-Lambda-Extension:69",
	"ap-southeast-5": "arn:aws:lambda:ap-southeast-5:381492012281:layer:AWS-Parameters-and-Secrets-Lambda-Extension:68",
	"ap-southeast-6": "arn:aws:lambda:ap-southeast-6:995508174458:layer:AWS-Parameters-and-Secrets-Lambda-Extension:63",
	"ap-south-1":     "arn:aws:lambda:ap-south-1:176022468876:layer:AWS-Parameters-and-Secrets-Lambda-Extension:83",
	"ap-northeast-3": "arn:aws:lambda:ap-northeast-3:576959938190:layer:AWS-Parameters-and-Secrets-Lambda-Extension:79",
	"ap-northeast-2": "arn:aws:lambda:ap-northeast-2:738900069198:layer:AWS-Parameters-and-Secrets-Lambda-Extension:84",
	"ap-southeast-1": "arn:aws:lambda:ap-southeast-1:044395824272:layer:AWS-Parameters-and-Secrets-Lambda-Extension:86",
	"ap-southeast-2": "arn:aws:lambda:ap-southeast-2:665172237481:layer:AWS-Parameters-and-Secrets-Lambda-Extension:90",
	"ap-southeast-7": "arn:aws:lambda:ap-southeast-7:941377119484:layer:AWS-Parameters-and-Secrets-Lambda-Extension:69",
	"ap-northeast-1": "arn:aws:lambda:ap-northeast-1:133490724326:layer:AWS-Parameters-and-Secrets-Lambda-Extension:85",
	"ca-central-1":   "arn:aws:lambda:ca-central-1:200266452380:layer:AWS-Parameters-and-Secrets-Lambda-Extension:92",
	"ca-west-1":      "arn:aws:lambda:ca-west-1:243964427225:layer:AWS-Parameters-and-Secrets-Lambda-Extension:56",
	"cn-north-1":     "arn:aws-cn:lambda:cn-north-1:287114880934:layer:AWS-Parameters-and-Secrets-Lambda-Extension:86",
	"cn-northwest-1": "arn:aws-cn:lambda:cn-northwest-1:287310001119:layer:AWS-Parameters-and-Secrets-Lambda-Extension:81",
	"eu-central-1":   "arn:aws:lambda:eu-central-1:187925254637:layer:AWS-Parameters-and-Secrets-Lambda-Extension:86",
	"eu-west-1":      "arn:aws:lambda:eu-west-1:015030872274:layer:AWS-Parameters-and-Secrets-Lambda-Extension:90",
	"eu-west-2":      "arn:aws:lambda:eu-west-2:133256977650:layer:AWS-Parameters-and-Secrets-Lambda-Extension:84",
	"eu-south-1":     "arn:aws:lambda:eu-south-1:325218067255:layer:AWS-Parameters-and-Secrets-Lambda-Extension:79",
	"eu-west-3":      "arn:aws:lambda:eu-west-3:780235371811:layer:AWS-Parameters-and-Secrets-Lambda-Extension:83",
	"eu-south-2":     "arn:aws:lambda:eu-south-2:524103009944:layer:AWS-Parameters-and-Secrets-Lambda-Extension:75",
	"eusc-de-east-1": "arn:aws-eusc:lambda:eusc-de-east-1:041683371183:layer:AWS-Parameters-and-Secrets-Lambda-Extension:5",
	"eu-north-1":     "arn:aws:lambda:eu-north-1:427196147048:layer:AWS-Parameters-and-Secrets-Lambda-Extension:79",
	"il-central-1":   "arn:aws:lambda:il-central-1:148806536434:layer:AWS-Parameters-and-Secrets-Lambda-Extension:56",
	"eu-central-2":   "arn:aws:lambda:eu-central-2:772501565639:layer:AWS-Parameters-and-Secrets-Lambda-Extension:63",
	"mx-central-1":   "arn:aws:lambda:mx-central-1:241533131596:layer:AWS-Parameters-and-Secrets-Lambda-Extension:53",
	"me-south-1":     "arn:aws:lambda:me-south-1:832021897121:layer:AWS-Parameters-and-Secrets-Lambda-Extension:58",
	"me-central-1":   "arn:aws:lambda:me-central-1:858974508948:layer:AWS-Parameters-and-Secrets-Lambda-Extension:60",
	"sa-east-1":      "arn:aws:lambda:sa-east-1:933737806257:layer:AWS-Parameters-and-Secrets-Lambda-Extension:88",
	"us-gov-east-1":  "arn:aws-us-gov:lambda:us-gov-east-1:129776340158:layer:AWS-Parameters-and-Secrets-Lambda-Extension:79",
	"us-gov-west-1":  "arn:aws-us-gov:lambda:us-gov-west-1:127562683043:layer:AWS-Parameters-and-Secrets-Lambda-Extension:83",
}

// isMissingGetLayerVersion reports whether err is an IAM AccessDenied on
// lambda:GetLayerVersion — the one error code we want to react to with the
// opt-in allow_secrets_in_env fallback. Typed API check + substring match:
// defense in depth in case AWS reformats the message.
func isMissingGetLayerVersion(err error) bool {
	if err == nil {
		return false
	}
	var apiErr smithy.APIError
	if !errors.As(err, &apiErr) {
		return false
	}
	if apiErr.ErrorCode() != "AccessDeniedException" {
		return false
	}
	return strings.Contains(err.Error(), "lambda:GetLayerVersion")
}

// secretsExtensionLayerARN returns the full x86_64 extension layer ARN for the
// given region: the value resolved from SSM and cached on the provider, else the
// pinned per-region fallback. Returns "" when neither is available (an unknown
// region with SSM denied) — callers must treat "" as "attach no layer".
func (p *AWSProvider) secretsExtensionLayerARN(region string) string {
	if p.secretsLayerARN != "" {
		return p.secretsLayerARN
	}
	return secretsExtensionLayerFallback[region]
}

// resolveSecretsExtensionLayerARN reads the AWS-published public SSM parameter
// that points at the current x86_64 extension layer for the caller's region.
// Returns the full ARN, or "" when ssm:GetParameter is unavailable or the value
// is empty — the caller then falls back to secretsExtensionLayerFallback.
func (p *AWSProvider) resolveSecretsExtensionLayerARN(ctx context.Context) string {
	client := ssm.NewFromConfig(p.cfg)
	out, err := client.GetParameter(ctx, &ssm.GetParameterInput{
		Name: aws.String(secretsExtensionLatestParam),
	})
	if err != nil {
		p.log.Warn("Could not resolve Secrets Extension layer from SSM %s (falling back to pinned ARN): %v", secretsExtensionLatestParam, err)
		return ""
	}
	if out.Parameter == nil || out.Parameter.Value == nil {
		return ""
	}
	return *out.Parameter.Value
}

// layerNameAndVersion splits a full layer version ARN
// (arn:...:layer:NAME:VERSION) into the version-less layer ARN and the numeric
// version, as required by lambda:GetLayerVersion. ok is false when the ARN has
// no trailing numeric version.
func layerNameAndVersion(arn string) (nameARN string, version int64, ok bool) {
	i := strings.LastIndex(arn, ":")
	if i < 0 {
		return "", 0, false
	}
	v := mustAtoi64(arn[i+1:])
	if v == nil {
		return "", 0, false
	}
	return arn[:i], *v, true
}

// probeSecretsExtensionLayer calls GetLayerVersion as a lightweight IAM probe.
// Runs at Initialize time, before any S3 upload or Lambda mutation, so a
// misconfigured deploy fails before it leaves partial state behind.
//
//   - layer resolves + read succeeds → nil (happy path)
//   - AccessDenied on lambda:GetLayerVersion + AllowSecretsInEnv=false → error
//     (bail before mutating anything)
//   - AccessDenied + AllowSecretsInEnv=true → nil (warn now; the existing
//     retry path in ensureMainLambdaUpdatedToCurrentZip handles the fallback
//     layer drop — defense in depth).
//   - any other error → nil (probe is advisory; don't block deploys on
//     transient list/describe failures)
func (p *AWSProvider) probeSecretsExtensionLayer(ctx context.Context, appCfg *cfgTypes.NextDeployConfig) error {
	if appCfg == nil || appCfg.Serverless == nil {
		return nil
	}
	region := p.cfg.Region
	if region == "" {
		return nil
	}

	// Resolve the full layer ARN up front (correct owner account + latest
	// version) so every subsequent CreateFunction / UpdateFunctionConfiguration
	// uses the same value. Silent fallback to the pinned per-region ARN if SSM
	// is unavailable — non-fatal.
	if arn := p.resolveSecretsExtensionLayerARN(ctx); arn != "" {
		p.secretsLayerARN = arn
	}

	layerARN := p.secretsExtensionLayerARN(region)
	if layerARN == "" {
		// Unknown region with SSM denied — no ARN to probe or attach. Advisory
		// only; the CreateFunction/UpdateFunctionConfiguration paths treat an
		// empty ARN as "attach no layer".
		p.log.Warn("No Secrets Extension layer ARN known for region %s and SSM lookup failed; deploying without the extension.", region)
		return nil
	}

	nameARN, version, ok := layerNameAndVersion(layerARN)
	if !ok {
		p.log.Warn("Could not parse Secrets Extension layer ARN %q; skipping IAM probe.", layerARN)
		return nil
	}

	client := lambda.NewFromConfig(p.cfg)
	_, err := client.GetLayerVersion(ctx, &lambda.GetLayerVersionInput{
		LayerName:     aws.String(nameARN),
		VersionNumber: aws.Int64(version),
	})
	if err == nil {
		return nil
	}

	if isMissingGetLayerVersion(err) {
		if !appCfg.Serverless.AllowSecretsInEnv {
			p.log.Error("\n%s", missingLayerBlockedBanner)
			return errors.New(missingLayerBlockedErr)
		}
		p.log.Warn("\n%s", missingLayerFallbackBanner)
		return nil
	}

	p.log.Warn("Secrets Extension layer probe returned a non-IAM error (continuing anyway): %v", err)
	return nil
}

func mustAtoi64(s string) *int64 {
	var n int64
	for _, c := range s {
		if c < '0' || c > '9' {
			return nil
		}
		n = n*10 + int64(c-'0')
	}
	return &n
}

// missingLayerBlockedBanner is shown when the IAM principal lacks
// `lambda:GetLayerVersion` AND `serverless.allow_secrets_in_env` is NOT set.
// We fail loudly rather than silently leaking secrets into Lambda env vars.
const missingLayerBlockedBanner = `════════════════════ DEPLOYMENT BLOCKED ════════════════════
AccessDenied on lambda:GetLayerVersion for the AWS Secrets Extension layer.
The layer is AWS-owned, so this is a cross-account read — both your identity
policy AND the layer's resource policy must allow it. Depending on your setup:

  • Standard IAM user/role: add this to your identity policy —
      {
        "Effect": "Allow",
        "Action": "lambda:GetLayerVersion",
        "Resource": "arn:aws:lambda:*:*:layer:AWS-Parameters-and-Secrets-Lambda-Extension:*"
      }

  • Already an admin or the account root (root has every identity permission):
    the block is NOT your identity policy. Check, in order —
      - an AWS Organizations SCP denying cross-account lambda:GetLayerVersion;
      - a permissions boundary on the role;
      - a wrong region/ARN (owner account differs per region — NextDeploy
        resolves it from the public SSM parameter, but a denied ssm:GetParameter
        falls back to a pinned list that may not cover a brand-new region).

Insecure escape hatch (NOT RECOMMENDED): set ` + "`serverless.allow_secrets_in_env: true`" + `
in nextdeploy.yml. This injects every secret directly into Lambda environment
variables, where they are visible in the AWS console, CloudTrail, and persist
in every published Lambda version forever (you cannot redact a published version).
════════════════════════════════════════════════════════════`

// missingLayerFallbackBanner is shown when the user has opted into the
// insecure fallback and we're about to drop the Secrets Extension layer.
const missingLayerFallbackBanner = `════════════════════ INSECURE FALLBACK ════════════════════
` + "`serverless.allow_secrets_in_env`" + ` is enabled.
Injecting secrets directly into Lambda env vars.
These will be visible in the AWS console and persist in every Lambda version.
Add lambda:GetLayerVersion to IAM and unset this flag as soon as possible.
════════════════════════════════════════════════════════════`

// missingLayerBlockedErr is the error returned to the caller when a deploy is
// refused for lack of lambda:GetLayerVersion. Wrapped with %w at the call site
// so errors.Is/As keep working.
const missingLayerBlockedErr = "missing lambda:GetLayerVersion IAM permission; refusing to leak secrets into Lambda env (set serverless.allow_secrets_in_env to override)"

// ImageOptConfig is the JSON contract consumed by the imgopt Lambda binary.
// AllowedDomains is the flattened list of every Next.js image source — both
// the legacy `images.domains[]` and the hostnames extracted from
// `images.remotePatterns[]`. The nextcore feature detector merges both into
// HasExternalImages, so the imgopt lambda only needs the flat list.
type ImageOptConfig struct {
	AllowedDomains []string `json:"allowed_domains"`
	DeviceSizes    []int    `json:"device_sizes"`
	ImageSizes     []int    `json:"image_sizes"`
	Formats        []string `json:"formats"`
}

func (p *AWSProvider) extractImageConfig(meta *nextcore.NextCorePayload) string {
	cfg := ImageOptConfig{
		AllowedDomains: []string{},
		DeviceSizes:    []int{640, 750, 828, 1080, 1200, 1920, 2048, 3840},
		ImageSizes:     []int{16, 32, 48, 64, 96, 128, 256, 384},
		Formats:        []string{"image/webp"},
	}

	if meta.DetectedFeatures != nil && len(meta.DetectedFeatures.HasExternalImages) > 0 {
		cfg.AllowedDomains = meta.DetectedFeatures.HasExternalImages
	}

	jsonBytes, _ := json.Marshal(cfg)
	return string(jsonBytes)
}

func (p *AWSProvider) getLambdaFunctionName(appCfg *cfgTypes.NextDeployConfig) string {
	return strings.ToLower(fmt.Sprintf("%s-%s", appCfg.App.Name, appCfg.App.Environment))
}

func (p *AWSProvider) DeployCompute(ctx context.Context, pkg *packaging.PackageResult, appCfg *cfgTypes.NextDeployConfig, meta *nextcore.NextCorePayload) error {
	if meta.OutputMode == nextcore.OutputModeExport {
		p.log.Info("Export Mode detected. Skipping Lambda deployment (static only).")
		// Update CloudFront for static only
		bucketName := p.getS3BucketName(appCfg)
		// Determine domain
		domain := meta.Domain
		if domain == "" {
			domain = appCfg.App.Domain.Name
		}

		_, err := p.ensureCloudFrontDistributionExists(ctx, appCfg.Serverless, bucketName, "", "", domain)
		if err != nil {
			p.log.Warn("Failed to ensure CloudFront Distribution for static site: %v", err)
		}
		return nil
	}

	p.log.Info("Deploying Compute Layer to AWS Lambda for app: %s...", appCfg.App.Name)

	if appCfg.Serverless == nil || appCfg.Serverless.IAMRole == "" || strings.Contains(appCfg.Serverless.IAMRole, "role-name") {
		p.log.Info("Ensuring managed 'nextdeploy-serverless-role' policies are up to date...")
		_, roleErr := p.ensureExecutionRoleExists(ctx)
		if roleErr != nil {
			p.log.Warn("Failed to update IAM execution role, permissions may be stale: %v", roleErr)
		}
	}

	client := lambda.NewFromConfig(p.cfg)
	functionName := p.getLambdaFunctionName(appCfg)
	p.verboseLog("  Lambda function name: %s", functionName)

	// Use pre-built zip from packager. We never read it into memory: it's
	// streamed to S3 below and Lambda is pointed at the S3 object via
	// FunctionCode.S3Key. This keeps a 250MB-class zip off the heap and also
	// sidesteps the 50MB ceiling on direct ZipFile uploads.
	zipPath := pkg.LambdaZipPath
	p.verboseLog("  Lambda zip size: %s (%s)", formatBytes(pkg.LambdaZipSize), zipPath)

	// 3a. Stream the zip to S3. This is the deploy artifact (CreateFunction /
	// UpdateFunctionCode reference it) AND the rollback-history entry, so a
	// failure here is fatal — unlike the old best-effort history save.
	bucketForHistory := p.getS3BucketName(appCfg)
	if err := p.ensureBucketExists(ctx, s3.NewFromConfig(p.cfg), bucketForHistory, appCfg.Serverless.Region); err != nil {
		return fmt.Errorf("failed to ensure deploy bucket exists: %w", err)
	}
	s3ZipKey, err := p.saveLambdaZipToS3(ctx, bucketForHistory, functionName, zipPath, meta)
	if err != nil {
		return fmt.Errorf("failed to stage lambda zip in S3: %w", err)
	}
	p.log.Info("Deployment zip staged: s3://%s/%s", bucketForHistory, s3ZipKey)
	// 4. Ensure Log Group and Alarms exist for observability
	if err := p.ensureLogGroupExists(ctx, functionName); err != nil {
		p.log.Warn("Failed to ensure Log Group: %v", err)
	}
	if err := p.ensureLambdaErrorAlarm(ctx, functionName); err != nil {
		p.log.Warn("Failed to ensure CloudWatch Alarm: %v", err)
	}

	// 5. Ensure Lambda function exists (provision if missing).
	functionJustCreated, err := p.ensureLambdaFunctionExists(ctx, client, functionName, appCfg.App.Name, appCfg.Serverless, bucketForHistory, s3ZipKey)
	if err != nil {
		return err
	}

	// 5. Ensure Lambda Function URL exists
	functionUrl, err := p.ensureLambdaFunctionURLExists(ctx, client, functionName)
	if err != nil {
		p.log.Warn("Failed to ensure Lambda Function URL (distribution might fail): %v", err)
	}

	// 6. Provision auxiliary infrastructure that CloudFront depends on (or is
	// independent of) BEFORE creating the distribution.
	//
	// Ordering rules:
	//   1. SQS queue (cached for reuse — see C1 in REVIEW.md).
	//   2. Image-optimization Lambda + URL (CloudFront needs its origin URL).
	//   3. CloudFront distribution → backfill discovered ID into config.
	//   4. ISR revalidation Lambda LAST, so it sees a real DISTRIBUTION_ID
	//      (see C2 in REVIEW.md — first-deploy bug where reval booted with "").
	p.log.Info("Ensuring Auxiliary Lambdas exist (ISR: %v, ImgOpt: %v)...", appCfg.Serverless.IsrRevalidation, appCfg.Serverless.ImageOptimization)

	// 6a. SQS queue (single source of truth for queueUrl/queueArn)
	var (
		revalQueueUrl string
		revalQueueArn string
	)
	if appCfg.Serverless != nil && appCfg.Serverless.IsrRevalidation {
		qUrl, qArn, qErr := p.ensureRevalidationQueueExists(ctx, appCfg.App.Name)
		if qErr != nil {
			p.log.Warn("Failed to ensure Revalidation SQS Queue: %v", qErr)
		} else {
			revalQueueUrl = qUrl
			revalQueueArn = qArn
		}
	}

	// 6b. Image-optimization Lambda (CloudFront origin)
	var imgOptUrl string
	if appCfg.Serverless != nil && appCfg.Serverless.ImageOptimization {
		imgOptName := functionName + "-imgopt"
		p.verboseLog("  Image Optimization Lambda name: %s", imgOptName)
		zipBytes, err := getEmbeddedLambdaZip("imgopt")
		if err != nil {
			p.log.Warn("Failed to load embedded imgopt lambda (is it built?): %v", err)
		} else {
			// Extract image config from metadata
			imageConfigJSON := p.extractImageConfig(meta)

			envVars := map[string]string{
				"DISTRIBUTION_ID":   appCfg.Serverless.CloudFrontId,
				"SOURCE_BUCKET":     p.getS3BucketName(appCfg),
				"IMAGE_CONFIG_JSON": imageConfigJSON,
			}
			_, err = p.ensureAuxiliaryLambdaExists(ctx, client, imgOptName, zipBytes, envVars)
			if err != nil {
				p.log.Warn("Failed to ensure Image Optimization Lambda: %v", err)
			} else {
				imgOptUrl, err = p.ensureLambdaFunctionURLExists(ctx, client, imgOptName)
				if err != nil {
					p.log.Warn("Failed to ensure Function URL for Image Optimization Lambda: %v", err)
				}
			}
		}
	}

	// 6c. CloudFront distribution
	bucketName := p.getS3BucketName(appCfg)
	domain := meta.Domain
	if domain == "" {
		domain = appCfg.App.Domain.Name
	}

	p.log.Info("Ensuring CloudFront distribution exists for Lambda origin (Domain: %s)...", domain)
	distributionId, err := p.ensureCloudFrontDistributionExists(ctx, appCfg.Serverless, bucketName, functionUrl, imgOptUrl, domain)
	if err != nil {
		p.log.Warn("Failed to ensure CloudFront Distribution: %v", err)
	} else {
		p.verboseLog("  CloudFront distribution ID: %s", distributionId)

		// Backfill so downstream consumers (reval lambda env, resource map)
		// see the discovered ID even on first deploy. See C2 in REVIEW.md.
		if appCfg.Serverless != nil {
			appCfg.Serverless.CloudFrontId = distributionId
		}

		// 7. Update S3 Bucket Policy for OAC
		if err := p.updateS3BucketPolicyForOAC(ctx, bucketName, distributionId); err != nil {
			p.log.Warn("Failed to update S3 Bucket Policy for OAC: %v", err)
		}

		// Get distribution domain name to show the user
		cfClient := cloudfront.NewFromConfig(p.cfg)
		dist, _ := cfClient.GetDistribution(ctx, &cloudfront.GetDistributionInput{Id: aws.String(distributionId)})
		if dist != nil && dist.Distribution != nil {
			p.log.Info("🚀 Application is accessible at: https://%s", *dist.Distribution.DomainName)
		}

		// Trigger invalidation for the newly managed distribution
		if err := p.invalidateCloudFront(ctx, distributionId); err != nil {
			p.log.Warn("Cache invalidation failed (non-fatal): %v", err)
		}
	}

	// 6d. ISR revalidation Lambda — created AFTER CloudFront so DISTRIBUTION_ID
	// is real, and reuses the cached queueUrl/queueArn from 6a.
	if appCfg.Serverless != nil && appCfg.Serverless.IsrRevalidation && revalQueueUrl != "" {
		revalName := functionName + "-reval"
		p.verboseLog("  ISR Revalidation Lambda name: %s", revalName)
		zipBytes, zipErr := getEmbeddedLambdaZip("revalidator")
		if zipErr != nil {
			p.log.Warn("Failed to load embedded revalidator lambda (is it built?): %v", zipErr)
		} else {
			envVars := map[string]string{
				"DISTRIBUTION_ID": appCfg.Serverless.CloudFrontId,
				"CACHE_BUCKET":    p.getS3BucketName(appCfg),
				"QUEUE_URL":       revalQueueUrl,
			}
			if _, auxErr := p.ensureAuxiliaryLambdaExists(ctx, client, revalName, zipBytes, envVars); auxErr != nil {
				p.log.Warn("Failed to ensure Revalidation Lambda: %v", auxErr)
			} else if trigErr := p.ensureLambdaSQSTrigger(ctx, client, revalName, revalQueueArn); trigErr != nil {
				p.log.Warn("Failed to ensure SQS mapping for Revalidation Lambda: %v", trigErr)
			}
		}
	}

	if !functionJustCreated {
		p.log.Info("Updating Lambda function code...")
		_, err := client.UpdateFunctionCode(ctx, &lambda.UpdateFunctionCodeInput{
			FunctionName: aws.String(functionName),
			S3Bucket:     aws.String(bucketForHistory),
			S3Key:        aws.String(s3ZipKey),
		})
		if err != nil {
			return fmt.Errorf("failed to update Lambda code: %w", err)
		}
		p.log.Info("Lambda code updated successfully. Waiting for update to stabilize...")

		if err := p.waitForLambdaStable(ctx, client, functionName); err != nil {
			p.log.Warn("Timed out waiting for Lambda stability: %v", err)
		}
	}

	secretName := p.secretName(appCfg.App.Name)

	handler := "bridge.handler"
	if appCfg.Serverless != nil && appCfg.Serverless.Handler != "" {
		handler = appCfg.Serverless.Handler
	}

	p.log.Info("Updating Lambda configuration (Handler: %s)...", handler)
	secretsExtensionLayer := p.secretsExtensionLayerARN(p.cfg.Region)

	// Reuse the queue URL cached in 6a — avoids a second AWS API call and
	// ensures we never silently drop ND_REVALIDATION_QUEUE on transient errors.
	envVars := map[string]string{
		"ND_SECRET_NAME":  secretName,
		"NODE_ENV":        "production",
		"ND_CACHE_BUCKET": fmt.Sprintf("nextdeploy-%s-%s-assets-%s", appCfg.App.Name, appCfg.App.Environment, p.accountID),
	}

	if revalQueueUrl != "" {
		envVars["ND_REVALIDATION_QUEUE"] = revalQueueUrl
	}

	maxRetries := 5
	var layersToApply []string
	if secretsExtensionLayer != "" {
		layersToApply = []string{secretsExtensionLayer}
	}
	layerFallbackApplied := false
	for i := 0; i < maxRetries; i++ {
		// If we're in fallback mode, we need to manually fetch and merge secrets into the environment
		if layerFallbackApplied {
			p.log.Info("Fallback mode active: Fetching and merging secrets into Lambda environment...")
			cloudSecrets, err := p.GetSecrets(ctx, appCfg.App.Name)
			if err != nil {
				p.log.Warn("Failed to fetch cloud secrets for environment injection: %v", err)
			} else {
				maps.Copy(envVars, cloudSecrets)
				p.log.Success("Injected %d secrets directly into Lambda environment.", len(cloudSecrets))
			}
		}

		_, err := client.UpdateFunctionConfiguration(ctx, &lambda.UpdateFunctionConfigurationInput{
			FunctionName: aws.String(functionName),
			Handler:      aws.String(handler),
			Environment: &lambdaTypes.Environment{
				Variables: envVars,
			},
			Layers: layersToApply,
		})
		if err == nil {
			break
		}

		// Insecure fallback: only triggered when the IAM principal lacks
		// lambda:GetLayerVersion AND the user has explicitly opted in via
		// `serverless.allow_secrets_in_env`. Without opt-in we fail loudly so
		// secrets are never silently leaked into Lambda env vars / console /
		// CloudTrail / version history. Usually the S5 precheck in Initialize
		// has already bailed; this branch is defense in depth.
		if isMissingGetLayerVersion(err) && len(layersToApply) > 0 {
			if !appCfg.Serverless.AllowSecretsInEnv {
				p.log.Error("\n%s", missingLayerBlockedBanner)
				return errors.New(missingLayerBlockedErr)
			}
			if !layerFallbackApplied {
				p.log.Warn("\n%s", missingLayerFallbackBanner)
				layersToApply = nil // remove the layer and retry immediately
				// Signal fallback to bridge.js so it skips the extension fetch
				// and does not FATAL on "no secrets returned".
				envVars["ND_SECRETS_MODE"] = "env"
				layerFallbackApplied = true
				i-- // safe: only happens once, guarded by flag
				continue
			}
		}

		var conflict *lambdaTypes.ResourceConflictException
		if errors.As(err, &conflict) && i < maxRetries-1 {
			p.log.Warn("Lambda is busy, retrying configuration update (%d/%d)...", i+1, maxRetries)
			time.Sleep(2 * time.Second)
			continue
		}
		p.log.Error("Failed to update Lambda configuration: %v", err)
		return fmt.Errorf("failed to update Lambda configuration: %w", err)
	}

	p.log.Info("Waiting for Lambda configuration update to stabilize...")
	if err := p.waitForLambdaStable(ctx, client, functionName); err != nil {
		p.log.Warn("Timed out waiting for Lambda stability after config update: %v", err)
	}

	// Publish a numbered version for rollback support
	pubOutput, err := client.PublishVersion(ctx, &lambda.PublishVersionInput{
		FunctionName: aws.String(functionName),
		Description:  aws.String(fmt.Sprintf("nextdeploy deploy %s", time.Now().Format(time.RFC3339))),
	})
	if err != nil {
		p.log.Warn("Failed to publish Lambda version (rollback may not work): %v", err)
	} else {
		p.log.Info("Published Lambda version %s for rollback support.", *pubOutput.Version)
	}

	return nil
}

// Rollback reverts the Lambda function to a previous deployed zip using the S3
// deployment history. Target selection:
//   - opts.ToCommit: prefix-match against history GitCommit (errors if not found
//     within retention).
//   - opts.Steps (default 1): walk N entries back from the current (last) one.
//
// This is instant — no HTTP download required.
func (p *AWSProvider) Rollback(ctx context.Context, appCfg *cfgTypes.NextDeployConfig, opts RollbackOptions) error {
	client := lambda.NewFromConfig(p.cfg)
	functionName := p.getLambdaFunctionName(appCfg)
	bucketName := p.getS3BucketName(appCfg)

	p.log.Info("Rolling back Lambda function %s...", functionName)

	// Fetch deployment history from S3 manifest
	history, err := p.getLambdaDeploymentHistory(ctx, bucketName, functionName)
	if err != nil {
		return fmt.Errorf("rollback failed: %w", err)
	}

	target, err := selectRollbackTarget(history, opts)
	if err != nil {
		return err
	}

	if target.GitDirty {
		p.log.Warn("Target deployment %s was built from a dirty working tree", shortCommit(target.GitCommit))
	}

	label := target.S3Key
	if target.GitCommit != "" {
		label = fmt.Sprintf("%s (commit %s)", target.S3Key, shortCommit(target.GitCommit))
	}
	p.log.Info("Rolling back to: %s", label)

	// Update Lambda code directly from S3 — no download needed
	_, err = client.UpdateFunctionCode(ctx, &lambda.UpdateFunctionCodeInput{
		FunctionName: aws.String(functionName),
		S3Bucket:     aws.String(bucketName),
		S3Key:        aws.String(target.S3Key),
	})
	if err != nil {
		return fmt.Errorf("failed to restore Lambda code from S3: %w", err)
	}

	p.log.Info("Lambda code restored. Waiting for stabilization...")
	if err := p.waitForLambdaStable(ctx, client, functionName); err != nil {
		p.log.Warn("Timed out waiting for Lambda stability after rollback: %v", err)
	}

	// Publish the rollback as a new version, tagged with the target commit so
	// the Lambda console shows what is actually running.
	pubDesc := fmt.Sprintf("nextdeploy rollback at %s", time.Now().Format(time.RFC3339))
	if target.GitCommit != "" {
		pubDesc = fmt.Sprintf("nextdeploy rollback to %s at %s", shortCommit(target.GitCommit), time.Now().Format(time.RFC3339))
	}
	_, _ = client.PublishVersion(ctx, &lambda.PublishVersionInput{
		FunctionName: aws.String(functionName),
		Description:  aws.String(pubDesc),
	})

	// Invalidate CloudFront cache
	if err := p.InvalidateCache(ctx, appCfg); err != nil {
		p.log.Warn("Cache invalidation after rollback failed (non-fatal): %v", err)
	}

	p.log.Info("✅ Rollback complete! Lambda is running the previous deployment.")
	return nil
}

func (p *AWSProvider) ensureLambdaFunctionExists(ctx context.Context, client *lambda.Client, name, appName string, sCfg *cfgTypes.ServerlessConfig, s3Bucket, s3Key string) (justCreated bool, err error) {
	_, err = client.GetFunction(ctx, &lambda.GetFunctionInput{
		FunctionName: aws.String(name),
	})
	if err == nil {
		return false, nil // Already exists
	}

	var notFound *lambdaTypes.ResourceNotFoundException
	if !errors.As(err, &notFound) {
		return false, fmt.Errorf("failed to check Lambda function status: %w", err)
	}

	// Function does not exist — create it
	handler := "bridge.handler"
	if sCfg.Handler != "" {
		handler = sCfg.Handler
	}

	runtime := lambdaTypes.RuntimeNodejs20x
	if sCfg.Runtime != "" {
		runtime = lambdaTypes.Runtime(sCfg.Runtime)
	}

	memory := int32(1024)
	if sCfg.MemorySize != 0 {
		memory = sCfg.MemorySize
	}

	timeout := int32(30)
	if sCfg.Timeout != 0 {
		timeout = sCfg.Timeout
	}

	// Determine IAM Role (Manual vs Auto-Provisioned)
	var roleArn string
	if sCfg.IAMRole != "" && !strings.Contains(sCfg.IAMRole, "role-name") {
		roleArn = sCfg.IAMRole
		if strings.Contains(roleArn, "ACCOUNT_ID") && p.accountID != "" {
			roleArn = strings.ReplaceAll(roleArn, "ACCOUNT_ID", p.accountID)
			p.log.Info("Automatically replaced ACCOUNT_ID placeholder in IAM Role ARN.")
		}
	} else {
		p.log.Info("No valid IAM Role provided, attempting to use/create managed 'nextdeploy-serverless-role'...")
		roleArn, err = p.ensureExecutionRoleExists(ctx)
		if err != nil {
			return false, fmt.Errorf("failed to ensure IAM execution role exists: %w", err)
		}
	}

	p.log.Info("Lambda function %s does not exist, creating with role %s (Handler: %s, Runtime: %s)...", name, roleArn, handler, runtime)

	// Managed Layer for Secrets Extension (Node.js 20 compatible). Empty when
	// the ARN could not be resolved (unknown region + SSM denied) — attach no
	// layer in that case rather than a malformed one.
	secretsExtensionLayer := p.secretsExtensionLayerARN(p.cfg.Region)
	var createLayers []string
	if secretsExtensionLayer != "" {
		createLayers = []string{secretsExtensionLayer}
	}

	createInput := &lambda.CreateFunctionInput{
		Code: &lambdaTypes.FunctionCode{
			S3Bucket: aws.String(s3Bucket),
			S3Key:    aws.String(s3Key),
		},
		FunctionName: aws.String(name),
		Role:         aws.String(roleArn),
		Handler:      aws.String(handler),
		Runtime:      runtime,
		Environment: &lambdaTypes.Environment{
			Variables: map[string]string{
				"NODE_ENV":       "production",
				"ND_SECRET_NAME": p.secretName(appName),
			},
		},
		Timeout:    aws.Int32(timeout),
		MemorySize: aws.Int32(memory),
		Layers:     createLayers,
	}

	maxRetries := 10
	retryDelay := 5 * time.Second
	layerFallbackApplied := false
	for i := 0; i < maxRetries; i++ {
		_, createErr := client.CreateFunction(ctx, createInput)
		if createErr == nil {
			p.log.Info("Lambda function %s created successfully.", name)
			return true, nil
		}

		// See UpdateFunctionConfiguration block above for the security rationale.
		if isMissingGetLayerVersion(createErr) && len(createInput.Layers) > 0 {
			if !sCfg.AllowSecretsInEnv {
				p.log.Error("\n%s", missingLayerBlockedBanner)
				return false, fmt.Errorf("%s. underlying: %w", missingLayerBlockedErr, createErr)
			}
			if !layerFallbackApplied {
				p.log.Warn("\n%s", missingLayerFallbackBanner)
				createInput.Layers = nil // remove the layer and retry immediately
				// Mirror the UpdateFunctionConfiguration fallback: tell bridge.js
				// to skip the extension fetch and avoid the FATAL-on-empty guard.
				if createInput.Environment != nil && createInput.Environment.Variables != nil {
					createInput.Environment.Variables["ND_SECRETS_MODE"] = "env"
				}
				layerFallbackApplied = true
				i-- // safe: only happens once, guarded by flag
				continue
			}
		}

		var invalidParam *lambdaTypes.InvalidParameterValueException
		if errors.As(createErr, &invalidParam) && strings.Contains(createErr.Error(), "role") && i < maxRetries-1 {
			p.log.Warn("IAM role not yet propagated, retrying CreateFunction (%d/%d) in %s...", i+1, maxRetries, retryDelay)
			time.Sleep(retryDelay)
			continue
		}

		return false, fmt.Errorf("failed to create Lambda function: %w", createErr)
	}

	return false, fmt.Errorf("failed to create Lambda function after %d retries: IAM role did not propagate in time", maxRetries)
}

func (p *AWSProvider) waitForLambdaStable(ctx context.Context, client *lambda.Client, functionName string) error {
	maxRetries := 20
	for range maxRetries {
		output, err := client.GetFunction(ctx, &lambda.GetFunctionInput{
			FunctionName: aws.String(functionName),
		})
		if err != nil {
			return err
		}

		status := output.Configuration.LastUpdateStatus
		p.verboseLog("  Lambda update status: %s", status)

		if status == lambdaTypes.LastUpdateStatusSuccessful {
			return nil
		}
		if status == lambdaTypes.LastUpdateStatusFailed {
			return fmt.Errorf("lambda update failed: %s", *output.Configuration.LastUpdateStatusReason)
		}

		time.Sleep(2 * time.Second)
	}
	return fmt.Errorf("timed out waiting for lambda stability")
}

func (p *AWSProvider) ensureLambdaFunctionURLExists(ctx context.Context, client *lambda.Client, functionName string) (string, error) {
	p.log.Info("Ensuring Lambda Function URL exists for %s...", functionName)

	var functionUrl string
	// 1. Check if it already exists
	getOutput, err := client.GetFunctionUrlConfig(ctx, &lambda.GetFunctionUrlConfigInput{
		FunctionName: aws.String(functionName),
	})

	if err == nil {
		functionUrl = *getOutput.FunctionUrl
		p.log.Info("Lambda Function URL found: %s", functionUrl)

		if getOutput.AuthType != lambdaTypes.FunctionUrlAuthTypeAwsIam {
			p.log.Info("Updating Function URL AuthType to AWS_IAM for CloudFront OAC...")
			_, err = client.UpdateFunctionUrlConfig(ctx, &lambda.UpdateFunctionUrlConfigInput{
				FunctionName: aws.String(functionName),
				AuthType:     lambdaTypes.FunctionUrlAuthTypeAwsIam,
			})
			if err != nil {
				return "", fmt.Errorf("failed to update Function URL AuthType to AWS_IAM: %w", err)
			}
		}
	} else {
		var notFound *lambdaTypes.ResourceNotFoundException
		if !errors.As(err, &notFound) {
			return "", fmt.Errorf("failed to check for Function URL: %w", err)
		}

		// 2. Create it
		p.log.Info("Creating new Function URL for %s...", functionName)
		createOutput, err := client.CreateFunctionUrlConfig(ctx, &lambda.CreateFunctionUrlConfigInput{
			FunctionName: aws.String(functionName),
			AuthType:     lambdaTypes.FunctionUrlAuthTypeAwsIam,
		})
		if err != nil {
			return "", fmt.Errorf("failed to create Function URL: %w", err)
		}
		functionUrl = *createOutput.FunctionUrl
	}

	// 3. Purge existing public permissions to avoid collisions and stale states
	p.log.Info("Hardening Function URL permissions (purging old statements)...")
	policyOutput, err := client.GetPolicy(ctx, &lambda.GetPolicyInput{
		FunctionName: aws.String(functionName),
	})
	if err == nil && policyOutput.Policy != nil {
		sidsToRemove := []string{"AllowPublicFunctionUrl", "AllowCloudFrontOACAccess", "AllowCloudFrontOACAccessInvoke"}
		for i := range 20 {
			sidsToRemove = append(sidsToRemove, fmt.Sprintf("AllowPublicFunctionUrl-%d", i))
		}

		for _, sid := range sidsToRemove {
			if strings.Contains(*policyOutput.Policy, sid) {
				_, _ = client.RemovePermission(ctx, &lambda.RemovePermissionInput{
					FunctionName: aws.String(functionName),
					StatementId:  aws.String(sid),
				})
			}
		}
	}

	// 4. Add fresh permission for CloudFront OAC access
	p.log.Info("Applying fresh CloudFront OAC access permissions (InvokeFunctionUrl and InvokeFunction)...")

	stsClient := sts.NewFromConfig(p.cfg)
	callerId, err := stsClient.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return "", fmt.Errorf("failed to get caller identity: %w", err)
	}
	accountId := *callerId.Account

	// Define permissions we need to add. AWS recently requires both lambda:InvokeFunctionUrl and lambda:InvokeFunction
	// for newer Function URLs behind CloudFront OAC.
	permissions := []struct {
		StatementId string
		Action      string
	}{
		{
			StatementId: "AllowCloudFrontOACAccess",
			Action:      "lambda:InvokeFunctionUrl",
		},
		{
			StatementId: "AllowCloudFrontOACAccessInvoke",
			Action:      "lambda:InvokeFunction",
		},
	}

	for _, perm := range permissions {
		maxRetries := 5
		for i := range maxRetries {
			input := &lambda.AddPermissionInput{
				FunctionName:  aws.String(functionName),
				StatementId:   aws.String(perm.StatementId),
				Action:        aws.String(perm.Action),
				Principal:     aws.String("cloudfront.amazonaws.com"),
				SourceAccount: aws.String(accountId),
				SourceArn:     aws.String(fmt.Sprintf("arn:aws:cloudfront::%s:distribution/*", accountId)),
			}

			if perm.Action == "lambda:InvokeFunctionUrl" {
				input.FunctionUrlAuthType = lambdaTypes.FunctionUrlAuthTypeAwsIam
			}

			_, err = client.AddPermission(ctx, input)
			if err == nil {
				p.log.Info("CloudFront OAC access permission '%s' applied successfully.", perm.Action)
				break
			}
			if strings.Contains(err.Error(), "already exists") {
				p.log.Info("CloudFront OAC access permission '%s' already exists.", perm.Action)
				break
			}

			var conflict *lambdaTypes.ResourceConflictException
			if (errors.As(err, &conflict) || strings.Contains(err.Error(), "InProgress")) && i < maxRetries-1 {
				p.log.Warn("Lambda is busy, retrying permission application '%s' (%d/%d)...", perm.Action, i+1, maxRetries)
				time.Sleep(2 * time.Second)
				continue
			}
			p.log.Warn("Failed to add CloudFront permission '%s': %v", perm.Action, err)
			break
		}
	}

	return functionUrl, nil
}

func (p *AWSProvider) ensureAuxiliaryLambdaExists(ctx context.Context, client *lambda.Client, name string, zipContents []byte, envVars map[string]string) (bool, error) {
	_, err := client.GetFunction(ctx, &lambda.GetFunctionInput{
		FunctionName: aws.String(name),
	})
	if err == nil {
		// Update code directly since it exists
		p.verboseLog("Updating Auxiliary Lambda %s code...", name)
		_, err := client.UpdateFunctionCode(ctx, &lambda.UpdateFunctionCodeInput{
			FunctionName: aws.String(name),
			ZipFile:      zipContents,
		})
		if err != nil {
			return false, fmt.Errorf("failed to update Auxiliary Lambda code: %w", err)
		}

		err = p.waitForLambdaStable(ctx, client, name)

		// Update env vars
		_, err = client.UpdateFunctionConfiguration(ctx, &lambda.UpdateFunctionConfigurationInput{
			FunctionName: aws.String(name),
			Environment: &lambdaTypes.Environment{
				Variables: envVars,
			},
		})
		if err != nil {
			p.log.Warn("Failed to update config for auxiliary lambda %s: %v", name, err)
		}

		return false, nil
	}

	var notFound *lambdaTypes.ResourceNotFoundException
	if !errors.As(err, &notFound) {
		return false, fmt.Errorf("failed to check Auxiliary Lambda status: %w", err)
	}

	roleArn, err := p.ensureExecutionRoleExists(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to ensure IAM execution role for lambda exists: %w", err)
	}

	p.log.Info("Creating Auxiliary Lambda function %s...", name)

	createInput := &lambda.CreateFunctionInput{
		Code:         &lambdaTypes.FunctionCode{ZipFile: zipContents},
		FunctionName: aws.String(name),
		Role:         aws.String(roleArn),
		Handler:      aws.String("bootstrap"),
		Runtime:      lambdaTypes.RuntimeProvidedal2023,
		Environment: &lambdaTypes.Environment{
			Variables: envVars,
		},
		Timeout:    aws.Int32(30),
		MemorySize: aws.Int32(512),
	}

	maxRetries := 10
	retryDelay := 5 * time.Second
	for i := range maxRetries {
		_, err := client.CreateFunction(ctx, createInput)
		if err == nil {
			p.log.Info("Auxiliary Lambda function %s created successfully.", name)
			return true, nil
		}

		var invalidParam *lambdaTypes.InvalidParameterValueException
		if errors.As(err, &invalidParam) && strings.Contains(err.Error(), "role") && i < maxRetries-1 {
			p.log.Warn("IAM role not yet propagated, retrying CreateFunction (%d/%d)...", i+1, maxRetries)
			time.Sleep(retryDelay)
			continue
		}

		return false, fmt.Errorf("failed to create Auxiliary Lambda %s: %w", name, err)
	}

	return false, fmt.Errorf("failed to create Auxiliary Lambda %s after retries", name)
}

func (p *AWSProvider) ensureLogGroupExists(ctx context.Context, functionName string) error {
	p.log.Info("Ensuring CloudWatch Log Group exists with 30-day retention for %s...", functionName)
	logGroupName := "/aws/lambda/" + functionName
	client := cloudwatchlogs.NewFromConfig(p.cfg)

	_, err := client.CreateLogGroup(ctx, &cloudwatchlogs.CreateLogGroupInput{
		LogGroupName: aws.String(logGroupName),
	})
	if err != nil {
		if !strings.Contains(err.Error(), "AlreadyExists") {
			return fmt.Errorf("failed to create log group: %w", err)
		}
	}

	_, err = client.PutRetentionPolicy(ctx, &cloudwatchlogs.PutRetentionPolicyInput{
		LogGroupName:    aws.String(logGroupName),
		RetentionInDays: aws.Int32(30),
	})
	if err != nil {
		return fmt.Errorf("failed to set log retention policy: %w", err)
	}

	return nil
}

func (p *AWSProvider) ensureLambdaErrorAlarm(ctx context.Context, functionName string) error {
	p.log.Info("Ensuring CloudWatch Alarm (Error Rate > 1%%) exists for %s...", functionName)
	client := cloudwatch.NewFromConfig(p.cfg)

	alarmName := fmt.Sprintf("%s-error-rate", functionName)
	_, err := client.PutMetricAlarm(ctx, &cloudwatch.PutMetricAlarmInput{
		AlarmName:          aws.String(alarmName),
		AlarmDescription:   aws.String("Alarm if Lambda error rate exceeds 1% over 5 minutes"),
		MetricName:         aws.String("Errors"),
		Namespace:          aws.String("AWS/Lambda"),
		Statistic:          "Sum",
		Period:             aws.Int32(300), // 5 minutes
		EvaluationPeriods:  aws.Int32(1),
		Threshold:          aws.Float64(1.0),
		ComparisonOperator: "GreaterThanOrEqualToThreshold",
		Dimensions: []cwTypes.Dimension{
			{
				Name:  aws.String("FunctionName"),
				Value: aws.String(functionName),
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to create cloudwatch alarm: %w", err)
	}
	return nil
}
