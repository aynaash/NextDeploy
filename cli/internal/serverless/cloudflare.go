package serverless

import (
	"bytes"
	"context"
	"crypto/md5" // #nosec G501 — R2 ETag is content MD5; not used for security
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/aynaash/nextdeploy/internal/packaging"
	"github.com/aynaash/nextdeploy/shared"
	"github.com/aynaash/nextdeploy/shared/config"
	"github.com/aynaash/nextdeploy/shared/credstore"
	"github.com/aynaash/nextdeploy/shared/nextcore"
	"github.com/aynaash/nextdeploy/shared/sensitive"

	awsv2 "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/cloudflare/cloudflare-go/v6"
	"github.com/cloudflare/cloudflare-go/v6/cache"
	"github.com/cloudflare/cloudflare-go/v6/option"
	"github.com/cloudflare/cloudflare-go/v6/r2"
	"github.com/cloudflare/cloudflare-go/v6/workers"
	"github.com/cloudflare/cloudflare-go/v6/zones"
)

// CloudflareProvider implements Provider for Cloudflare Workers + R2.
//
// IMPORTANT — Next.js compatibility status:
//
// Cloudflare Workers do not run vanilla Node.js, so a Next.js standalone
// build cannot be uploaded as-is. Production deployments require the
// build to be adapted into a Worker-compatible bundle (see the cloudflare
// adapter step in the packager). Until that lands, DeployCompute will log
// a loud warning when given a non-static-export build.
//
// SDK usage:
//   - Management plane (workers, secrets, routes, R2 buckets, zone, cache):
//     github.com/cloudflare/cloudflare-go/v6
//   - R2 object plane (PUT/GET/DELETE objects): the SDK does not cover this;
//     we use the AWS S3 SDK pointed at R2's S3-compatible endpoint
//     (https://<account>.r2.cloudflarestorage.com).
//
// Credentials (resolved in this order — first non-empty wins):
//  1. Environment variables (CI-friendly, ephemeral)
//  2. Encrypted credstore at ~/.nextdeploy/credstore (per-machine, mode 0600)
//  3. Plaintext nextdeploy.yml (LEGACY — emits a loud WARN; prefer 1 or 2)
//
// Field map:
//   - CF API token:    CLOUDFLARE_API_TOKEN     | credstore[cloudflare].api_token         | cloudprovider.access_key
//   - CF account ID:   CLOUDFLARE_ACCOUNT_ID    | credstore[cloudflare].account_id        | cloudprovider.account_id
//   - R2 access key:   R2_ACCESS_KEY_ID         | credstore[cloudflare].r2_access_key_id  | (no yaml fallback)
//   - R2 secret key:   R2_SECRET_ACCESS_KEY     | credstore[cloudflare].r2_secret_key     | (no yaml fallback)
//
// Every resolved value is registered with the sensitive scrubber so it never
// leaks into log lines or error messages.
type CloudflareProvider struct {
	log         *shared.Logger
	cf          *cloudflare.Client
	r2s3        *s3.Client // S3-compat client for R2 objects (lazy: built on first DeployStatic)
	accountID   string
	apiToken    string // raw token value, kept so we can derive R2 creds from it
	apiTokenID  string // populated by verifyToken; the access-key-id half of derived R2 creds
	r2AccessKey string
	r2SecretKey string
	// r2ParentKeyID, when set, lets DeployStatic mint short-lived temp R2
	// credentials via /accounts/:id/r2/temp-access-credentials instead of
	// using a long-lived R2_SECRET_ACCESS_KEY. The parent key authorizes
	// the scope; the temp creds expire in ~1 hour.
	r2ParentKeyID string
	environment   string       // populated in Initialize
	provisioned   *resourceMap // standalone resource name → CF UUID, populated by ProvisionResources

	// pendingSecrets holds secrets staged via UpdateSecrets() that have not
	// yet been folded into a Worker upload. DeployCompute reads them, emits
	// them as secret_text bindings in the script metadata, and clears them
	// on success. This avoids the per-secret PUT loop (which is rate-
	// limited to ~13 req/s by CF, error code 10013) — instead the entire
	// secret set lands atomically in one Workers.Scripts.Update call.
	//
	// Standalone rotation paths (SetSecret/UnsetSecret) still use the
	// per-secret API and bypass this stash.
	pendingSecrets map[string]string
}

// cloudflareCreds is the resolved bag returned by loadCloudflareCreds.
type cloudflareCreds struct {
	apiToken      string
	accountID     string
	r2AccessKey   string
	r2SecretKey   string
	r2ParentKeyID string // permanent R2 access key id used to mint short-lived temp creds
}

// loadCloudflareCreds resolves credentials in the documented precedence order
// (env → credstore → yaml). Yaml usage emits a single WARN per call so leaks
// via committed config get noticed.
func loadCloudflareCreds(cfg *config.NextDeployConfig, log *shared.Logger) cloudflareCreds {
	c := cloudflareCreds{
		apiToken:      os.Getenv("CLOUDFLARE_API_TOKEN"),
		accountID:     os.Getenv("CLOUDFLARE_ACCOUNT_ID"),
		r2AccessKey:   os.Getenv("R2_ACCESS_KEY_ID"),
		r2SecretKey:   os.Getenv("R2_SECRET_ACCESS_KEY"),
		r2ParentKeyID: os.Getenv("R2_PARENT_ACCESS_KEY_ID"),
	}

	if c.apiToken == "" || c.accountID == "" || c.r2AccessKey == "" || c.r2SecretKey == "" || c.r2ParentKeyID == "" {
		stored, err := credstore.Load("cloudflare")
		if err == nil {
			if c.apiToken == "" {
				c.apiToken = stored["api_token"]
			}
			if c.accountID == "" {
				c.accountID = stored["account_id"]
			}
			if c.r2AccessKey == "" {
				c.r2AccessKey = stored["r2_access_key_id"]
			}
			if c.r2SecretKey == "" {
				c.r2SecretKey = stored["r2_secret_key"]
			}
			if c.r2ParentKeyID == "" {
				c.r2ParentKeyID = stored["r2_parent_access_key_id"]
			}
		}
	}

	if cfg.CloudProvider != nil {
		usedYaml := false
		if c.apiToken == "" && cfg.CloudProvider.AccessKey != "" {
			c.apiToken = cfg.CloudProvider.AccessKey
			usedYaml = true
		}
		if c.accountID == "" && cfg.CloudProvider.AccountID != "" {
			c.accountID = cfg.CloudProvider.AccountID
		}
		if usedYaml {
			log.Warn("⚠️  Cloudflare API token loaded from nextdeploy.yml — committing this file leaks creds.")
			log.Warn("⚠️  Recommended: 'nextdeploy creds set --provider cloudflare' (encrypted, mode 0600).")
		}
	}
	return c
}

func NewCloudflareProvider() *CloudflareProvider {
	return &CloudflareProvider{
		log: shared.PackageLogger("cloudflare", "☁️  CF::"),
	}
}

func (p *CloudflareProvider) workerName(appName string) string {
	env := p.environment
	if env == "" {
		env = "production"
	}
	return fmt.Sprintf("%s-%s", appName, env)
}

func (p *CloudflareProvider) bucketNameFromApp(appName string) string {
	env := p.environment
	if env == "" {
		env = "production"
	}
	return fmt.Sprintf("nextdeploy-%s-%s-assets", appName, env)
}

// Initialize wires up the Cloudflare SDK client and verifies the API token.
//
// The deploy is designed around a single-token UX: the user sets
// CLOUDFLARE_API_TOKEN and everything else (account ID, optionally R2
// credentials) is derived. If they want explicit overrides, env or
// credstore wins.
func (p *CloudflareProvider) Initialize(ctx context.Context, cfg *config.NextDeployConfig) error {
	p.log.Info("Initializing Cloudflare deployment session...")

	p.environment = cfg.App.Environment

	creds := loadCloudflareCreds(cfg, p.log)
	if creds.apiToken == "" {
		return fmt.Errorf("cloudflare API token not found (set CLOUDFLARE_API_TOKEN env, run 'nextdeploy creds set --provider cloudflare', or set cloudprovider.access_key in nextdeploy.yml)")
	}

	sensitive.Register(creds.apiToken, creds.r2AccessKey, creds.r2SecretKey, creds.r2ParentKeyID)
	p.apiToken = creds.apiToken
	p.r2AccessKey = creds.r2AccessKey
	p.r2SecretKey = creds.r2SecretKey
	p.r2ParentKeyID = creds.r2ParentKeyID

	// 15-minute per-request timeout: Worker uploads can be multi-megabyte
	// multipart bundles, and observed end-to-end latency on residential
	// connections + CF API processing has hit 5+ minutes for ~3MB
	// scripts. The SDK's WithRequestTimeout is per-retry, so the actual
	// wall-clock budget for a failing upload is timeout × MaxRetries.
	// Every other CF API call (zone list, DNS edit, secret list) returns
	// in well under a second, so the longer ceiling doesn't slow happy
	// paths. We also disable retries for the upload path because
	// re-uploading a 3MB bundle on a transient 5xx adds latency without
	// helping recover — the operator will rerun `nextdeploy ship`
	// faster than the SDK's retry-with-backoff.
	p.cf = cloudflare.NewClient(
		option.WithAPIToken(creds.apiToken),
		option.WithRequestTimeout(15*time.Minute),
		option.WithMaxRetries(0),
	)

	if err := p.verifyToken(ctx); err != nil {
		return err
	}

	if !looksLikeCloudflareAccountID(creds.accountID) {
		if creds.accountID != "" {
			p.log.Warn("Ignoring CLOUDFLARE_ACCOUNT_ID=%q — doesn't look like a real account id (expect 32 hex chars). Will try auto-discovery.", creds.accountID)
		}
		discovered, err := p.discoverAccountID(ctx)
		if err != nil {
			return fmt.Errorf("cloudflare account ID not provided and auto-discovery failed: %w (set CLOUDFLARE_ACCOUNT_ID env or cloudprovider.account_id in nextdeploy.yml)", err)
		}
		creds.accountID = discovered
		p.log.Info("Auto-discovered account ID: %s", creds.accountID)
	}
	p.accountID = creds.accountID

	// Long-lived R2 keys take precedence (preserves existing CI configs).
	// Otherwise the s3 client is built lazily in DeployStatic, after we
	// mint short-lived creds from the parent key.
	if p.r2AccessKey != "" && p.r2SecretKey != "" {
		p.r2s3 = newR2S3Client(p.accountID, p.r2AccessKey, p.r2SecretKey, "")
	}

	p.log.Info("Cloudflare session initialized (account: %s)", p.accountID)
	return nil
}

// verifyToken hits /user/tokens/verify and returns a clear error if the token
// is invalid or expired. The endpoint confirms the token is alive but does
// not enumerate scopes — scope failures will surface as 403s on the first
// API call that needs the missing permission. The error here lists the
// scopes a typical end-to-end deploy needs so users can sanity-check
// their token in the CF dashboard.
//
// The response also carries the token's `id`. Per Cloudflare's docs, that
// id is also the R2 S3-compat access-key-id half when the token's secret
// half is hashed via SHA-256. We stash the id on the provider so
// DeployStatic can derive R2 credentials from the API token alone — no
// separate R2 dashboard click required.
func (p *CloudflareProvider) verifyToken(ctx context.Context) error {
	var verify struct {
		Success bool `json:"success"`
		Result  struct {
			ID        string `json:"id"`
			Status    string `json:"status"`
			ExpiresOn string `json:"expires_on"`
		} `json:"result"`
		Errors []struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := p.cf.Get(ctx, "/user/tokens/verify", nil, &verify); err != nil {
		return fmt.Errorf("cloudflare token verification failed: %w\n\n%s", err, requiredScopesHint())
	}
	if !verify.Success {
		msg := "cloudflare API token is invalid"
		if len(verify.Errors) > 0 {
			msg = fmt.Sprintf("%s: %s (code %d)", msg, verify.Errors[0].Message, verify.Errors[0].Code)
		}
		return fmt.Errorf("%s\n\n%s", msg, requiredScopesHint())
	}
	if verify.Result.Status != "" && verify.Result.Status != "active" {
		return fmt.Errorf("cloudflare API token status is %q (need \"active\")", verify.Result.Status)
	}
	p.apiTokenID = verify.Result.ID
	return nil
}

// looksLikeCloudflareAccountID accepts what Cloudflare actually issues
// (32 lowercase hex characters) and rejects everything else, including
// the obvious placeholder strings that get pasted from sample configs:
// "YOUR_CLOUDFLARE_ACCOUNT_ID", "<account-id>", "REPLACE_ME", empty,
// dashboard URLs the user accidentally pasted, etc.
//
// Treating those as "no account id" lets the auto-discovery path kick
// in instead of letting them flow through to a 404 from /accounts/<bad>.
func looksLikeCloudflareAccountID(s string) bool {
	if len(s) != 32 {
		return false
	}
	for _, r := range s {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')) {
			return false
		}
	}
	return true
}

// deriveR2CredsFromAPIToken returns the (accessKeyId, secretAccessKey)
// pair that R2's S3 endpoint accepts for a given Cloudflare API token,
// without any extra dashboard step or API call.
//
// Per Cloudflare's R2 docs:
//
//	The Access Key ID corresponds to the API token's `id`,
//	and the Secret Access Key is the SHA-256 hash of the API token `value`.
//
// id is captured from /user/tokens/verify in verifyToken. value is the
// raw bearer string the user already exported as CLOUDFLARE_API_TOKEN.
//
// This is the zero-config path: a token with the right scopes
// (Workers R2 Storage: Edit) Just Works for both bucket management *and*
// object PUT, the same way an AWS IAM key handles both planes.
func deriveR2CredsFromAPIToken(tokenID, tokenValue string) (akid, secret string, ok bool) {
	if tokenID == "" || tokenValue == "" {
		return "", "", false
	}
	sum := sha256.Sum256([]byte(tokenValue))
	return tokenID, hex.EncodeToString(sum[:]), true
}

// requiredScopesHint lists the token permissions a full nextdeploy CF deploy
// uses. Surfaced in error messages so users know what to check when a deep
// API call fails 403.
func requiredScopesHint() string {
	return "Required token scopes (Account):\n" +
		"  • Workers Scripts: Edit\n" +
		"  • Workers Routes: Edit\n" +
		"  • Workers KV Storage: Edit (if using KV)\n" +
		"  • Workers R2 Storage: Edit\n" +
		"  • Workers Tail: Read\n" +
		"  • Account Settings: Read\n" +
		"  • D1: Edit (if using D1)\n" +
		"  • Hyperdrive: Edit (if using Hyperdrive)\n" +
		"  • Vectorize: Edit (if using Vectorize)\n" +
		"  • AI Gateway: Edit (if using AI Gateway)\n" +
		"Required token scopes (Zone, on each zone you deploy to):\n" +
		"  • Zone: Read\n" +
		"  • DNS: Edit\n" +
		"  • Cache Purge: Purge"
}

// discoverAccountID lists the accounts the API token can see. Returns the
// account ID if exactly one is visible. If multiple are visible, returns an
// error listing them so the user can pick one explicitly.
func (p *CloudflareProvider) discoverAccountID(ctx context.Context) (string, error) {
	var resp struct {
		Success bool `json:"success"`
		Result  []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"result"`
	}
	if err := p.cf.Get(ctx, "/accounts", nil, &resp); err != nil {
		return "", err
	}
	if !resp.Success || len(resp.Result) == 0 {
		return "", fmt.Errorf("no accounts visible to this token")
	}
	if len(resp.Result) == 1 {
		return resp.Result[0].ID, nil
	}
	names := make([]string, 0, len(resp.Result))
	for _, a := range resp.Result {
		names = append(names, fmt.Sprintf("  %s  (%s)", a.ID, a.Name))
	}
	return "", fmt.Errorf("token sees %d accounts, cannot pick one — set CLOUDFLARE_ACCOUNT_ID explicitly:\n%s",
		len(resp.Result), strings.Join(names, "\n"))
}

// newR2S3Client builds an S3 client configured against the R2 S3-compatible
// endpoint. Returns nil if R2 credentials are not present; callers must check
// before issuing object PUTs.
//
// sessionToken is the third element of an STS-style triple, set when the
// (akid, secret) pair was minted via /r2/temp-access-credentials. Pass ""
// for permanent R2 access keys.
//
// UsePathStyle MUST be true for the default R2 endpoint. R2's TLS cert is
// a wildcard *.r2.cloudflarestorage.com that only matches one subdomain
// level, so virtual-hosted-style URLs (bucket.account.r2.cloudflarestorage.com)
// fail TLS handshake. Path-style (account.r2.cloudflarestorage.com/bucket/key)
// matches the cert and works. Custom-domain buckets can opt back into
// virtual-hosted later, but that's out of scope here.
func newR2S3Client(accountID, akid, secret, sessionToken string) *s3.Client {
	if akid == "" || secret == "" {
		return nil
	}
	endpoint := fmt.Sprintf("https://%s.r2.cloudflarestorage.com", accountID)
	return s3.New(s3.Options{
		Region:       "auto",
		BaseEndpoint: awsv2.String(endpoint),
		Credentials:  credentials.NewStaticCredentialsProvider(akid, secret, sessionToken),
		UsePathStyle: true,
	})
}

// r2TempCreds is the body returned by /accounts/:id/r2/temp-access-credentials.
// CF wraps it in the standard {"success": bool, "result": {...}} envelope.
type r2TempCreds struct {
	AccessKeyID     string `json:"accessKeyId"`
	SecretAccessKey string `json:"secretAccessKey"`
	SessionToken    string `json:"sessionToken"`
}

// mintR2TempCreds asks Cloudflare for a short-lived R2 credential triple
// scoped to a single bucket. The returned creds are good for ttl seconds
// (clamped to [60, 36*3600] by the API; we ask for one hour).
//
// The endpoint is authenticated via the API token (Bearer) — the parent
// access key id is the *authority* for the temp creds, but the call itself
// rides the API token. So callers don't need the parent key's *secret*,
// only the id.
//
// permission is one of:
//
//	"object-read-only", "object-read-write",
//	"admin-read-only", "admin-read-write",
//	"admin-object-only-read-only", "admin-object-only-read-write".
//
// We use "object-read-write" — uploads but no bucket-level admin.
func (p *CloudflareProvider) mintR2TempCreds(ctx context.Context, bucket string) (r2TempCreds, error) {
	if p.r2ParentKeyID == "" {
		return r2TempCreds{}, fmt.Errorf("R2_PARENT_ACCESS_KEY_ID not set")
	}
	body := map[string]any{
		"bucket":            bucket,
		"parentAccessKeyId": p.r2ParentKeyID,
		"permission":        "object-read-write",
		"ttlSeconds":        3600,
	}
	var resp struct {
		Success bool        `json:"success"`
		Result  r2TempCreds `json:"result"`
		Errors  []struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"errors"`
	}
	path := fmt.Sprintf("/accounts/%s/r2/temp-access-credentials", p.accountID)
	if err := p.cf.Post(ctx, path, body, &resp); err != nil {
		return r2TempCreds{}, fmt.Errorf("mint r2 temp creds: %w", err)
	}
	if !resp.Success {
		msg := "mint r2 temp creds: api returned success=false"
		if len(resp.Errors) > 0 {
			msg = fmt.Sprintf("%s (%s, code %d)", msg, resp.Errors[0].Message, resp.Errors[0].Code)
		}
		return r2TempCreds{}, fmt.Errorf("%s", msg)
	}
	if resp.Result.AccessKeyID == "" || resp.Result.SecretAccessKey == "" {
		return r2TempCreds{}, fmt.Errorf("mint r2 temp creds: empty creds in response")
	}
	sensitive.Register(resp.Result.AccessKeyID, resp.Result.SecretAccessKey, resp.Result.SessionToken)
	return resp.Result, nil
}

// DeployStatic uploads the package's static assets to an R2 bucket via the
// S3-compatible endpoint. R2 management (bucket creation) goes through the
// official SDK.
//
// R2 object PUTs use S3-protocol HMAC auth, not Bearer; cloudflare-go
// doesn't expose object operations. Credential resolution order, designed
// so the zero-config path is the default:
//
//  1. Explicit long-lived pair (R2_ACCESS_KEY_ID + R2_SECRET_ACCESS_KEY in
//     env / credstore). Used as-is. Preserves existing CI configs.
//  2. **Auto-derived from the CF API token.** Per CF's R2 docs, the access
//     key id is the API token's `id` (captured in verifyToken via
//     /user/tokens/verify) and the secret access key is SHA-256 of the
//     token's `value`. No dashboard click, no extra scope beyond
//     "Workers R2 Storage: Edit", no minting endpoint. This is the
//     intended one-token-end-to-end path.
//  3. Parent-key fallback (R2_PARENT_ACCESS_KEY_ID set): mint a 1-hour
//     scoped child via /accounts/:id/r2/temp-access-credentials.
//     Useful when the API token deliberately doesn't have R2 scope but a
//     dedicated R2 parent key does.
//  4. None of the above — fail with dashboard URL.
func (p *CloudflareProvider) DeployStatic(ctx context.Context, pkg *packaging.PackageResult, cfg *config.NextDeployConfig, meta *nextcore.NextCorePayload) error {
	bucketName := p.getBucketName(cfg)

	if err := p.ensureR2BucketExists(ctx, bucketName); err != nil {
		return fmt.Errorf("failed to ensure R2 bucket: %w", err)
	}

	if p.r2s3 == nil {
		// Tier 2: derive R2 creds from the API token. Default path.
		if akid, secret, ok := deriveR2CredsFromAPIToken(p.apiTokenID, p.apiToken); ok {
			sensitive.Register(secret)
			p.log.Info("Using R2 credentials derived from API token (zero-config path)")
			p.r2s3 = newR2S3Client(p.accountID, akid, secret, "")
		} else if p.r2ParentKeyID != "" {
			// Tier 3: parent-key flow. Used when the user explicitly set
			// R2_PARENT_ACCESS_KEY_ID — usually because their main API
			// token doesn't have R2 scope.
			p.log.Info("Minting short-lived R2 creds for bucket %s (parent: %s…)", bucketName, p.r2ParentKeyID[:min(8, len(p.r2ParentKeyID))])
			creds, err := p.mintR2TempCreds(ctx, bucketName)
			if err != nil {
				return fmt.Errorf("R2 temp credential minting failed: %w", err)
			}
			p.r2s3 = newR2S3Client(p.accountID, creds.AccessKeyID, creds.SecretAccessKey, creds.SessionToken)
		}

		if p.r2s3 == nil {
			return fmt.Errorf(
				"R2 object uploads need credentials. The CLOUDFLARE_API_TOKEN you\n" +
					"provided didn't expose a token id — that's unexpected. Check that\n" +
					"the token has 'Workers R2 Storage: Edit' scope. As a fallback you\n" +
					"can export an explicit pair (R2_ACCESS_KEY_ID + R2_SECRET_ACCESS_KEY)\n" +
					"or just R2_PARENT_ACCESS_KEY_ID — see\n" +
					"https://dash.cloudflare.com/?to=/:account/r2/api-tokens",
			)
		}
	}

	p.log.Info("Uploading %d static assets to R2 bucket %s...", len(pkg.S3Assets), bucketName)

	const cfR2UploadConcurrency = 8
	sem := make(chan struct{}, cfR2UploadConcurrency)
	errs := make(chan error, len(pkg.S3Assets))
	var wg sync.WaitGroup

	var uploaded, skipped atomic.Int64

	for _, asset := range pkg.S3Assets {
		asset := asset
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			didUpload, err := p.uploadToR2IfChanged(ctx, bucketName, asset)
			if err != nil {
				errs <- fmt.Errorf("upload %s: %w", asset.S3Key, err)
				return
			}
			if didUpload {
				uploaded.Add(1)
			} else {
				skipped.Add(1)
			}
		}()
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			return err
		}
	}

	if skipped.Load() > 0 {
		p.log.Info("R2 asset sync: %d uploaded, %d skipped (content unchanged)", uploaded.Load(), skipped.Load())
	} else {
		p.log.Info("R2 asset sync: %d uploaded to %s", uploaded.Load(), bucketName)
	}
	return nil
}

// uploadToR2IfChanged HEADs the object first and skips the PUT when the
// remote ETag (R2 sets it to MD5 of the object body for non-multipart
// uploads) matches the local file's MD5. Returns (didUpload, err) so the
// caller can count uploaded vs. skipped.
//
// HeadObject costs one round trip but no body transfer; PUT pays the full
// transfer cost. For large asset sets that change rarely (the common case
// for /_next/static and /public), this turns N PUTs of N MB into N HEADs
// of zero bytes — the dominant savings on a redeploy.
func (p *CloudflareProvider) uploadToR2IfChanged(ctx context.Context, bucket string, asset packaging.S3Asset) (bool, error) {
	localETag, err := md5OfFile(asset.LocalPath)
	if err != nil {
		return false, fmt.Errorf("hash %s: %w", asset.LocalPath, err)
	}
	if remoteETag, ok := p.headR2ETag(ctx, bucket, asset.S3Key); ok && remoteETag == localETag {
		return false, nil
	}
	if err := p.putToR2(ctx, bucket, asset); err != nil {
		return false, err
	}
	return true, nil
}

// headR2ETag returns the remote ETag (with the surrounding quotes that S3
// returns trimmed off). Returns (_, false) on any error — including 404,
// permission errors, network errors — so callers fall through to PUT.
// Skipping a PUT we *should* have done just costs an extra round trip
// next time; doing one we shouldn't have wastes bandwidth. We err toward
// uploading.
func (p *CloudflareProvider) headR2ETag(ctx context.Context, bucket, key string) (string, bool) {
	out, err := p.r2s3.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: awsv2.String(bucket),
		Key:    awsv2.String(key),
	})
	if err != nil || out.ETag == nil {
		return "", false
	}
	return strings.Trim(*out.ETag, `"`), true
}

// md5OfFile streams a file through md5 and returns the lowercase hex
// digest. Used to compare against R2 ETags. md5 is not used for any
// security purpose here — R2's ETag scheme (S3-compatible) just happens
// to be the content MD5 for single-part uploads, so we have to match it.
func md5OfFile(path string) (string, error) {
	f, err := os.Open(path) // #nosec G304 — caller-validated path
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := md5.New() // #nosec G401 — see comment on import
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func (p *CloudflareProvider) putToR2(ctx context.Context, bucket string, asset packaging.S3Asset) error {
	f, err := os.Open(asset.LocalPath) // #nosec G304
	if err != nil {
		return err
	}
	defer f.Close()

	input := &s3.PutObjectInput{
		Bucket:      awsv2.String(bucket),
		Key:         awsv2.String(asset.S3Key),
		Body:        f,
		ContentType: awsv2.String(asset.ContentType),
	}
	if asset.CacheControl != "" {
		input.CacheControl = awsv2.String(asset.CacheControl)
	}
	_, err = p.r2s3.PutObject(ctx, input)
	return err
}

// deployHashTagPrefix marks Worker tags managed by nextdeploy. Anything
// with this prefix is owned by us; anything without is left alone so user
// tags set via dashboard / wrangler survive.
const deployHashTagPrefix = "nextdeploy-deploy-hash:"

// computeDeployHash returns a hex SHA-256 over everything that would
// change the running Worker: the bundle bytes, the binding metadata, and
// the secret values. If any of these change, the hash changes; if the
// hash matches the deployed one, re-uploading is a no-op and we skip.
//
// The metadata hash uses the SDK's JSON marshalling — same bytes the SDK
// would send to /scripts/{name}, modulo field ordering which Stainless
// emits deterministically. Secrets are hashed separately so the secret
// values don't end up in the metadata JSON before we hash it (they're
// already in scriptMeta as secret_text bindings, but we double-hash them
// to make the dependency explicit and keep tests deterministic).
func computeDeployHash(scriptBytes []byte, meta workers.ScriptUpdateParamsMetadata, secrets map[string]string) string {
	h := sha256.New()
	h.Write([]byte("nextdeploy-deploy-hash/v1\n"))

	bundleSum := sha256.Sum256(scriptBytes)
	h.Write([]byte("bundle="))
	h.Write([]byte(hex.EncodeToString(bundleSum[:])))
	h.Write([]byte{'\n'})

	if metaJSON, err := meta.MarshalJSON(); err == nil {
		metaSum := sha256.Sum256(metaJSON)
		h.Write([]byte("meta="))
		h.Write([]byte(hex.EncodeToString(metaSum[:])))
		h.Write([]byte{'\n'})
	}

	h.Write([]byte("secrets=\n"))
	for _, k := range sortedKeys(secrets) {
		h.Write([]byte(k))
		h.Write([]byte{'='})
		valSum := sha256.Sum256([]byte(secrets[k]))
		h.Write([]byte(hex.EncodeToString(valSum[:])))
		h.Write([]byte{'\n'})
	}

	return hex.EncodeToString(h.Sum(nil))
}

// workerAlreadyAtHash returns true when the named Worker's tags include
// `nextdeploy-deploy-hash:<hash>`. Returns false on any read error
// (script doesn't exist yet, permission issue, etc.) — safer to upload
// when in doubt than to skip and serve stale code.
func (p *CloudflareProvider) workerAlreadyAtHash(ctx context.Context, workerName, hash string) bool {
	settings, err := p.cf.Workers.Scripts.Settings.Get(ctx, workerName, workers.ScriptSettingGetParams{
		AccountID: cloudflare.F(p.accountID),
	})
	if err != nil || settings == nil {
		return false
	}
	want := deployHashTagPrefix + hash
	for _, t := range settings.Tags {
		if t == want {
			return true
		}
	}
	return false
}

// tagWorkerDeployHash writes our deploy-hash tag onto the Worker,
// preserving any user tags. Called after a successful upload so the next
// deploy can skip when nothing relevant changed.
//
// We Get current tags first, drop any old `nextdeploy-deploy-hash:` entry,
// append the fresh one, and PATCH back. This is safe even if the Worker
// has no tags yet (the slice just starts empty).
func (p *CloudflareProvider) tagWorkerDeployHash(ctx context.Context, workerName, hash string) error {
	current, err := p.cf.Workers.Scripts.Settings.Get(ctx, workerName, workers.ScriptSettingGetParams{
		AccountID: cloudflare.F(p.accountID),
	})
	if err != nil {
		return fmt.Errorf("read script settings: %w", err)
	}
	tags := make([]string, 0, len(current.Tags)+1)
	for _, t := range current.Tags {
		if !strings.HasPrefix(t, deployHashTagPrefix) {
			tags = append(tags, t)
		}
	}
	tags = append(tags, deployHashTagPrefix+hash)

	_, err = p.cf.Workers.Scripts.Settings.Edit(ctx, workerName, workers.ScriptSettingEditParams{
		AccountID: cloudflare.F(p.accountID),
		ScriptSetting: workers.ScriptSettingParam{
			Tags: cloudflare.F(tags),
		},
	})
	if err != nil {
		return fmt.Errorf("write script settings: %w", err)
	}
	return nil
}

// resolveStandaloneDir returns a path the Cloudflare adapter can read the
// raw Next.js standalone tree from, plus a cleanup closure.
//
// Preferred path: pkg.StandaloneTarPath is the target-agnostic artifact the
// packager produces — extract it to a temp dir so we work from a pristine
// copy and can't accidentally pollute the user's .next/standalone (the
// adapter writes _nextdeploy_worker.mjs into it). Fallback: the live
// standalone directory on disk, which is what older builds and non-Package
// callers hand us.
func resolveStandaloneDir(pkg *packaging.PackageResult, meta *nextcore.NextCorePayload, log *shared.Logger) (string, func(), error) {
	noop := func() {}

	if pkg != nil && pkg.StandaloneTarPath != "" {
		tmp, err := os.MkdirTemp("", "nextdeploy-cf-standalone-*")
		if err != nil {
			return "", noop, fmt.Errorf("create temp dir for standalone extract: %w", err)
		}
		if err := shared.ExtractTarGz(pkg.StandaloneTarPath, tmp); err != nil {
			_ = os.RemoveAll(tmp)
			return "", noop, fmt.Errorf("extract %s: %w", pkg.StandaloneTarPath, err)
		}
		log.Debug("Extracted standalone tarball to %s (%d bytes)", tmp, pkg.StandaloneTarSize)
		return tmp, func() { _ = os.RemoveAll(tmp) }, nil
	}

	projectDir, err := os.Getwd()
	if err != nil {
		return "", noop, fmt.Errorf("get project dir: %w", err)
	}
	distDir := ".next"
	if meta != nil && meta.DistDir != "" {
		distDir = meta.DistDir
	}
	standaloneDir := filepath.Join(projectDir, distDir, "standalone")
	log.Debug("Using live standalone dir: %s (no tarball in PackageResult)", standaloneDir)
	return standaloneDir, noop, nil
}

// DeployCompute adapts the Next.js standalone build into a Worker bundle
// (using esbuild + the embedded shim) and uploads it via the SDK.
//
// For static-export sites, no compute deploy is needed — DeployStatic + a
// catch-all R2 worker is sufficient. We skip in that case.
func (p *CloudflareProvider) DeployCompute(ctx context.Context, pkg *packaging.PackageResult, cfg *config.NextDeployConfig, meta *nextcore.NextCorePayload) error {
	if meta != nil && meta.OutputMode == nextcore.OutputModeExport {
		p.log.Info("Static-export build detected; skipping Worker deploy.")
		return nil
	}

	p.log.Info("Adapting Next.js standalone build for Cloudflare Workers...")

	standaloneDir, cleanup, err := resolveStandaloneDir(pkg, meta, p.log)
	if err != nil {
		return err
	}
	defer cleanup()

	bundlePath, err := BuildWorkerBundle(ctx, standaloneDir, meta, cfg, p.log)
	if err != nil {
		return fmt.Errorf("worker bundle build failed: %w", err)
	}

	scriptBytes, err := os.ReadFile(bundlePath) // #nosec G304
	if err != nil {
		return fmt.Errorf("failed to read worker bundle: %w", err)
	}

	workerName := p.getWorkerName(cfg)
	bucketName := p.getBucketName(cfg)

	const entryName = "worker.mjs"
	rawReader := bytes.NewReader(scriptBytes)
	// Wrap the reader with a progress tracker so the operator sees live
	// byte-counts during the multipart upload. Without this the CLI hangs
	// silently for the duration of the upload — there's no way to tell
	// "still streaming" from "stuck" — and we've watched 3MB Worker
	// uploads run for several minutes on residential connections.
	tracked := newProgressReader(rawReader, int64(len(scriptBytes)), p.log, "worker upload")
	scriptReader := newNamedFile(tracked, entryName, "application/javascript+module")

	var cfBlock *config.CloudflareConfig
	if cfg.Serverless != nil {
		cfBlock = cfg.Serverless.Cloudflare
	}
	var resolve refResolver = noResolver
	if p.provisioned != nil {
		resolve = p.provisioned.get
	}
	scriptMeta, err := buildScriptMetadata(cfBlock, bucketName, entryName, resolve, p.pendingSecrets)
	if err != nil {
		return fmt.Errorf("build script metadata: %w", err)
	}
	if len(p.pendingSecrets) > 0 {
		p.log.Info("Folding %d secret_text bindings into Worker upload", len(p.pendingSecrets))
	}

	deployHash := computeDeployHash(scriptBytes, scriptMeta, p.pendingSecrets)
	if p.workerAlreadyAtHash(ctx, workerName, deployHash) {
		p.log.Info("Worker bundle and bindings unchanged (hash %s) — skipping upload", deployHash[:12])
		// Still drop the secret stash even on skip — the deployed worker
		// already carries them, and keeping pendingSecrets around would
		// fold them again on the next call.
		p.pendingSecrets = nil
	} else {
		params := workers.ScriptUpdateParams{
			AccountID: cloudflare.F(p.accountID),
			Metadata:  cloudflare.F(scriptMeta),
			Files:     cloudflare.F([]io.Reader{scriptReader}),
		}
		if _, err := p.cf.Workers.Scripts.Update(ctx, workerName, params); err != nil {
			return fmt.Errorf("worker upload failed: %w", err)
		}
		p.log.Info("Worker deployed: %s", workerName)
		// Stamp the deployed hash on the worker so the next deploy can
		// compare without re-uploading.
		if err := p.tagWorkerDeployHash(ctx, workerName, deployHash); err != nil {
			// Non-fatal: failure here just means next deploy can't skip,
			// it has to re-upload. The deploy itself succeeded.
			p.log.Warn("Failed to stamp deploy-hash tag (non-fatal — next deploy won't be able to skip): %v", err)
		}
		p.pendingSecrets = nil
	}

	if err := p.applyWorkerTriggers(ctx, workerName, cfBlock); err != nil {
		return err
	}
	if err := p.wireQueueConsumers(ctx, workerName, cfBlock); err != nil {
		return err
	}
	p.attachEdgeRoutes(ctx, workerName, cfg, cfBlock)

	return nil
}

// applyWorkerTriggers applies cron schedules declared under the cloudflare
// block. Nil means "leave whatever's in the dashboard alone".
func (p *CloudflareProvider) applyWorkerTriggers(ctx context.Context, workerName string, cfBlock *config.CloudflareConfig) error {
	if cfBlock == nil || cfBlock.Triggers == nil {
		return nil
	}
	if err := p.applyCronTriggers(ctx, workerName, cfBlock.Triggers.Crons); err != nil {
		return fmt.Errorf("apply cron triggers: %w", err)
	}
	return nil
}

// wireQueueConsumers attaches each declared queue consumer to the worker.
// Producer queues themselves are created by ProvisionResources; consumers
// connect them to the script.
func (p *CloudflareProvider) wireQueueConsumers(ctx context.Context, workerName string, cfBlock *config.CloudflareConfig) error {
	if cfBlock == nil || cfBlock.Bindings == nil || cfBlock.Bindings.Queues == nil {
		return nil
	}
	for _, c := range cfBlock.Bindings.Queues.Consumers {
		if err := p.ensureQueueConsumer(ctx, workerName, c); err != nil {
			return fmt.Errorf("wire queue consumer for %q: %w", c.Queue, err)
		}
	}
	return nil
}

// attachEdgeRoutes wires custom domains + explicit routes. Each attempt is
// independent and non-fatal so one bad hostname doesn't sink the deploy.
//
// When the user has not declared an explicit cloudflare.custom_domains or
// cloudflare.routes block, we auto-promote cfg.App.Domain to a Custom
// Domain attachment (Workers.Domains.Update). Custom Domains provision
// DNS + worker route + SSL atomically — the user does not need to create
// any DNS records by hand. This replaces an older fallback that called
// ensureWorkerRoute, which only set up the route pattern and left the
// user wondering why their domain didn't resolve.
//
// Failure is logged but non-fatal: the Worker still serves on its
// *.workers.dev URL, and the deployment report includes the manual DNS
// instructions as a fallback for permission-denied or rate-limited
// cases (commonly: API token missing Workers Routes:Edit / DNS:Edit).
func (p *CloudflareProvider) attachEdgeRoutes(ctx context.Context, workerName string, cfg *config.NextDeployConfig, cfBlock *config.CloudflareConfig) {
	if cfBlock != nil {
		for _, cd := range cfBlock.CustomDomains {
			if err := p.ensureCustomDomain(ctx, workerName, cd); err != nil {
				p.log.Warn("Failed to attach custom domain %s (non-fatal): %v", cd.Hostname, err)
			}
		}
		for _, rt := range cfBlock.Routes {
			if err := p.ensureWorkerRouteForZone(ctx, workerName, rt.Pattern, rt.Zone); err != nil {
				p.log.Warn("Failed to set worker route %s (non-fatal): %v", rt.Pattern, err)
			}
		}
	}
	// Auto-attach: when the user gave us cfg.App.Domain but no explicit
	// edge block, treat it as a Custom Domain. We attach the apex + the
	// www subdomain so a typical "https://example.com" / "https://www.example.com"
	// pair works out of the box. Each call is idempotent.
	if cfg.App.Domain != "" && hasNoExplicitEdge(cfBlock) {
		hostnames := autoCustomDomainHostnames(cfg.App.Domain)
		for _, hostname := range hostnames {
			if err := p.ensureCustomDomain(ctx, workerName, config.CFCustomDomain{Hostname: hostname}); err != nil {
				p.log.Warn(
					"Auto-attach custom domain %s failed (non-fatal): %v — "+
						"check that the API token has Zone:DNS:Edit + Account:Workers Scripts:Edit, "+
						"then re-run `nextdeploy ship`. The deployment report has manual DNS steps as a fallback.",
					hostname, err,
				)
			}
		}
	}
}

// autoCustomDomainHostnames returns the set of hostnames to auto-attach
// when only cfg.App.Domain is set. For an apex (example.com) we attach
// both apex and www. For a subdomain (app.example.com) we attach only
// the subdomain — adding `www.app.example.com` is rarely what users
// want.
func autoCustomDomainHostnames(domain string) []string {
	domain = strings.TrimSpace(strings.TrimSuffix(domain, "."))
	if domain == "" {
		return nil
	}
	parts := strings.Split(domain, ".")
	isApex := len(parts) == 2
	if isApex {
		return []string{domain, "www." + domain}
	}
	return []string{domain}
}

func hasNoExplicitEdge(cfBlock *config.CloudflareConfig) bool {
	if cfBlock == nil {
		return true
	}
	return len(cfBlock.CustomDomains) == 0 && len(cfBlock.Routes) == 0
}

// ensureCustomDomain attaches a hostname to the worker via Workers.Domains.Update.
// The endpoint is upsert-style — calling repeatedly with the same hostname is
// safe and idempotent. Zone is resolved from cd.ZoneID if set, else derived
// from the hostname's apex.
func (p *CloudflareProvider) ensureCustomDomain(ctx context.Context, workerName string, cd config.CFCustomDomain) error {
	params := workers.DomainUpdateParams{
		AccountID: cloudflare.F(p.accountID),
		Hostname:  cloudflare.F(cd.Hostname),
		Service:   cloudflare.F(workerName),
	}
	switch {
	case cd.ZoneID != "":
		params.ZoneID = cloudflare.F(cd.ZoneID)
	default:
		params.ZoneName = cloudflare.F(zoneNameFromHostname(cd.Hostname))
	}
	if _, err := p.cf.Workers.Domains.Update(ctx, params); err != nil {
		return err
	}
	p.log.Info("Custom domain attached: %s → %s", cd.Hostname, workerName)
	return nil
}

// zoneNameFromHostname returns the apex zone for a hostname. Naive heuristic:
// last two DNS labels. Works for example.com, sub.example.com — does not
// handle public suffix exceptions like .co.uk. Users with multi-label TLDs
// should set zone_id explicitly.
func zoneNameFromHostname(host string) string {
	parts := strings.Split(host, ".")
	if len(parts) <= 2 {
		return host
	}
	return strings.Join(parts[len(parts)-2:], ".")
}

// applyCronTriggers replaces the worker's full cron schedule with the given
// list. The CF Schedules.Update endpoint is not additive — it overwrites.
// An empty list intentionally clears all crons (this is opt-in: caller must
// have already determined the user explicitly wants schedule management).
func (p *CloudflareProvider) applyCronTriggers(ctx context.Context, workerName string, crons []string) error {
	body := make([]workers.ScriptScheduleUpdateParamsBody, len(crons))
	for i, c := range crons {
		body[i] = workers.ScriptScheduleUpdateParamsBody{
			Cron: cloudflare.F(c),
		}
	}
	_, err := p.cf.Workers.Scripts.Schedules.Update(ctx, workerName, workers.ScriptScheduleUpdateParams{
		AccountID: cloudflare.F(p.accountID),
		Body:      body,
	})
	if err != nil {
		return err
	}
	if len(crons) == 0 {
		p.log.Info("Cleared cron triggers for worker %s", workerName)
	} else {
		p.log.Info("Applied %d cron trigger(s) to worker %s", len(crons), workerName)
	}
	return nil
}

// UpdateSecrets stashes a batch of secrets onto the provider so DeployCompute
// can fold them into the next Workers.Scripts.Update call as secret_text
// bindings. This bypasses CF's per-secret PUT endpoint (rate-limited at ~13
// req/s, error 10013) and lands every secret atomically in a single upload.
//
// Trade-off: secret rotation requires a Worker re-upload. That re-upload is
// cheap because the bundle is content-hashed and reused — only the metadata
// changes — but it does mean `nextdeploy secrets set FOO=bar` outside of a
// full deploy uses the per-secret path (SetSecret/UnsetSecret) instead of
// going through here.
//
// Calling UpdateSecrets with an empty map clears the staged set.
func (p *CloudflareProvider) UpdateSecrets(ctx context.Context, appName string, secrets map[string]string) error {
	_ = ctx
	_ = appName
	if len(secrets) == 0 {
		p.pendingSecrets = nil
		return nil
	}
	staged := make(map[string]string, len(secrets))
	for k, v := range secrets {
		staged[k] = v
	}
	p.pendingSecrets = staged
	p.log.Info("Staged %d secrets to fold into the next Worker upload", len(staged))
	return nil
}

// GetSecrets lists secret names. The CF API never returns secret values.
func (p *CloudflareProvider) GetSecrets(ctx context.Context, appName string) (map[string]string, error) {
	workerName := p.workerName(appName)
	iter := p.cf.Workers.Scripts.Secrets.ListAutoPaging(ctx, workerName, workers.ScriptSecretListParams{
		AccountID: cloudflare.F(p.accountID),
	})

	out := map[string]string{}
	for iter.Next() {
		out[iter.Current().Name] = "[secret]"
	}
	if err := iter.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (p *CloudflareProvider) SetSecret(ctx context.Context, appName, key, value string) error {
	return p.putWorkerSecret(ctx, p.workerName(appName), key, value)
}

func (p *CloudflareProvider) UnsetSecret(ctx context.Context, appName, key string) error {
	workerName := p.workerName(appName)
	_, err := p.cf.Workers.Scripts.Secrets.Delete(ctx, workerName, key, workers.ScriptSecretDeleteParams{
		AccountID: cloudflare.F(p.accountID),
	})
	return err
}

func (p *CloudflareProvider) putWorkerSecret(ctx context.Context, workerName, key, value string) error {
	body := workers.ScriptSecretUpdateParamsBodyWorkersBindingKindSecretText{
		Name: cloudflare.F(key),
		Text: cloudflare.F(value),
		Type: cloudflare.F(workers.ScriptSecretUpdateParamsBodyWorkersBindingKindSecretTextTypeSecretText),
	}
	_, err := p.cf.Workers.Scripts.Secrets.Update(ctx, workerName, workers.ScriptSecretUpdateParams{
		AccountID: cloudflare.F(p.accountID),
		Body:      body,
	})
	return err
}

// InvalidateCache purges the Cloudflare zone cache for the configured domain.
func (p *CloudflareProvider) InvalidateCache(ctx context.Context, cfg *config.NextDeployConfig) error {
	if cfg.App.Domain == "" {
		p.log.Info("No domain configured, skipping cache purge.")
		return nil
	}

	zoneID, err := p.getZoneID(ctx, cfg.App.Domain)
	if err != nil {
		return fmt.Errorf("failed to find zone for %s: %w", cfg.App.Domain, err)
	}

	_, err = p.cf.Cache.Purge(ctx, cache.CachePurgeParams{
		ZoneID: cloudflare.F(zoneID),
		Body: cache.CachePurgeParamsBodyCachePurgeEverything{
			PurgeEverything: cloudflare.F(true),
		},
	})
	if err != nil {
		return fmt.Errorf("cache purge failed: %w", err)
	}

	p.log.Info("Cloudflare cache purged for zone %s", zoneID)
	return nil
}

// Rollback reverts the Worker to a previous deployment version.
// Cloudflare's deployment API does not surface git commit metadata, so
// --to <commit> is unsupported and falls back to step-based rollback.
func (p *CloudflareProvider) Rollback(ctx context.Context, cfg *config.NextDeployConfig, opts RollbackOptions) error {
	if opts.ToCommit != "" {
		p.log.Warn("Cloudflare rollback does not support --to <commit>; using step-based rollback instead")
	}
	steps := opts.Steps
	if steps <= 0 {
		steps = 1
	}
	workerName := p.getWorkerName(cfg)
	p.log.Info("Fetching deployment history for worker: %s...", workerName)

	list, err := p.cf.Workers.Scripts.Deployments.List(ctx, workerName, workers.ScriptDeploymentListParams{
		AccountID: cloudflare.F(p.accountID),
	})
	if err != nil {
		return fmt.Errorf("failed to list deployments: %w", err)
	}

	deployments := list.Deployments
	if len(deployments) < steps+1 {
		return fmt.Errorf("not enough deployment history to rollback %d step(s) (found %d, need at least %d)",
			steps, len(deployments), steps+1)
	}
	target := deployments[steps]
	if len(target.Versions) == 0 {
		return fmt.Errorf("rollback target deployment %s has no versions", target.ID)
	}
	previousVersionID := target.Versions[0].VersionID
	p.log.Info("Rolling back to version: %s", previousVersionID)

	_, err = p.cf.Workers.Scripts.Deployments.New(ctx, workerName, workers.ScriptDeploymentNewParams{
		AccountID: cloudflare.F(p.accountID),
		Deployment: workers.DeploymentParam{
			Strategy: cloudflare.F(workers.DeploymentStrategyPercentage),
			Versions: cloudflare.F([]workers.DeploymentVersionParam{
				{
					VersionID:  cloudflare.F(previousVersionID),
					Percentage: cloudflare.F(100.0),
				},
			}),
		},
	})
	if err != nil {
		return fmt.Errorf("failed to activate previous deployment: %w", err)
	}

	p.log.Info("Rollback complete. Worker is now running version %s", previousVersionID)
	return nil
}

// Destroy removes the Worker and the R2 bucket. Bucket delete will fail if
// the bucket still has objects; we don't sweep them yet.
func (p *CloudflareProvider) Destroy(ctx context.Context, cfg *config.NextDeployConfig) error {
	workerName := p.getWorkerName(cfg)
	bucketName := p.getBucketName(cfg)

	p.log.Info("Deleting Worker: %s...", workerName)
	if _, err := p.cf.Workers.Scripts.Delete(ctx, workerName, workers.ScriptDeleteParams{
		AccountID: cloudflare.F(p.accountID),
	}); err != nil {
		var apiErr *cloudflare.Error
		if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusNotFound {
			p.log.Warn("Worker delete failed (non-fatal): %v", err)
		}
	}

	p.log.Info("Deleting R2 bucket: %s...", bucketName)
	if _, err := p.cf.R2.Buckets.Delete(ctx, bucketName, r2.BucketDeleteParams{
		AccountID: cloudflare.F(p.accountID),
	}); err != nil {
		p.log.Warn("R2 bucket delete failed (non-fatal — may still contain objects): %v", err)
	}

	p.log.Info("Cloudflare resources destroyed.")
	return nil
}

func (p *CloudflareProvider) GetResourceMap(ctx context.Context, cfg *config.NextDeployConfig) (ServerlessResourceMap, error) {
	return ServerlessResourceMap{
		AppName:        cfg.App.Name,
		Environment:    cfg.App.Environment,
		Region:         "global",
		S3BucketName:   p.getBucketName(cfg),
		CustomDomain:   cfg.App.Domain,
		DeploymentTime: time.Now(),
	}, nil
}

// ── Helpers ──────────────────────────────────────────────────────────────────

func (p *CloudflareProvider) getWorkerName(cfg *config.NextDeployConfig) string {
	return p.workerName(cfg.App.Name)
}

func (p *CloudflareProvider) getBucketName(cfg *config.NextDeployConfig) string {
	return p.bucketNameFromApp(cfg.App.Name)
}

// ensureR2BucketExists checks for the bucket and creates it on 404.
// Other API errors propagate, with one well-known case translated into
// a friendlier message: code 10042 ("Please enable R2 through the
// Cloudflare Dashboard") fires before R2 is opted in on the account
// and there is nothing the API token can do about it — the user has
// to click through the dashboard once to add a payment method and
// accept R2's terms. Same shape as AWS's S3 ToS gate.
func (p *CloudflareProvider) ensureR2BucketExists(ctx context.Context, name string) error {
	_, err := p.cf.R2.Buckets.Get(ctx, name, r2.BucketGetParams{
		AccountID: cloudflare.F(p.accountID),
	})
	if err == nil {
		return nil
	}
	if isR2NotEnabled(err) {
		return r2NotEnabledError(p.accountID)
	}
	var apiErr *cloudflare.Error
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusNotFound {
		return fmt.Errorf("get bucket: %w", err)
	}
	_, err = p.cf.R2.Buckets.New(ctx, r2.BucketNewParams{
		AccountID: cloudflare.F(p.accountID),
		Name:      cloudflare.F(name),
	})
	if isR2NotEnabled(err) {
		return r2NotEnabledError(p.accountID)
	}
	return err
}

// isR2NotEnabled returns true when an error is CF code 10042, which
// means the account hasn't enabled R2 yet. The SDK doesn't expose error
// codes as constants; we string-match on the well-known number.
func isR2NotEnabled(err error) bool {
	return err != nil && strings.Contains(err.Error(), "10042")
}

// r2NotEnabledError returns a clear, actionable error for the one-time
// R2 enablement gate. Includes the dashboard URL so the user goes
// straight to the right page instead of digging through CF's nav.
func r2NotEnabledError(accountID string) error {
	return fmt.Errorf(
		"R2 is not enabled on your Cloudflare account yet.\n\n"+
			"This is a one-time per-account step (free tier exists, but a payment\n"+
			"method must be on file). Open this URL, click 'Purchase R2 Plan',\n"+
			"add payment, accept terms — then re-run nextdeploy ship:\n\n"+
			"  https://dash.cloudflare.com/%s/r2/overview\n",
		accountID,
	)
}

// ensureWorkerRoute creates a route `<domain>/*` for the worker, deriving the
// zone from the domain. Convenience wrapper around ensureWorkerRouteForZone.
func (p *CloudflareProvider) ensureWorkerRoute(ctx context.Context, workerName, domain string) error {
	return p.ensureWorkerRouteForZone(ctx, workerName, domain+"/*", domain)
}

// ensureWorkerRouteForZone creates the given route pattern for the worker in
// the named zone. Skips creation if an identical route already exists.
// Resolves zoneName via Zones.List; if zoneName is empty, derives it from the
// pattern using zoneNameFromPattern.
func (p *CloudflareProvider) ensureWorkerRouteForZone(ctx context.Context, workerName, pattern, zoneName string) error {
	if zoneName == "" {
		zoneName = zoneNameFromPattern(pattern)
	}
	zoneID, err := p.getZoneID(ctx, zoneName)
	if err != nil {
		return err
	}
	iter := p.cf.Workers.Routes.ListAutoPaging(ctx, workers.RouteListParams{
		ZoneID: cloudflare.F(zoneID),
	})
	for iter.Next() {
		r := iter.Current()
		if r.Pattern == pattern && r.Script == workerName {
			return nil
		}
	}
	if err := iter.Err(); err != nil {
		return fmt.Errorf("list routes: %w", err)
	}

	_, err = p.cf.Workers.Routes.New(ctx, workers.RouteNewParams{
		ZoneID:  cloudflare.F(zoneID),
		Pattern: cloudflare.F(pattern),
		Script:  cloudflare.F(workerName),
	})
	if err == nil {
		p.log.Info("Worker route attached: %s → %s", pattern, workerName)
	}
	return err
}

// zoneNameFromPattern strips the trailing /* and any wildcard subdomain to
// extract the apex zone (e.g. "*.example.com/*" → "example.com"). Used as a
// fallback when zone is not explicitly set on a route.
func zoneNameFromPattern(pattern string) string {
	host := pattern
	if i := strings.Index(host, "/"); i >= 0 {
		host = host[:i]
	}
	host = strings.TrimPrefix(host, "*.")
	return zoneNameFromHostname(host)
}

func (p *CloudflareProvider) getZoneID(ctx context.Context, domain string) (string, error) {
	page, err := p.cf.Zones.List(ctx, zones.ZoneListParams{
		Name: cloudflare.F(domain),
	})
	if err != nil {
		return "", err
	}
	if len(page.Result) == 0 {
		return "", fmt.Errorf("no Cloudflare zone found for domain: %s", domain)
	}
	return page.Result[0].ID, nil
}

// progressReader wraps an io.Reader and logs cumulative bytes-read at
// regular intervals. Used to give the operator visibility into a
// long-running multipart upload that would otherwise look indistinguishable
// from a hung connection.
//
// Logging is throttled to one line per second + final summary, so a fast
// upload doesn't spam the terminal and a slow one doesn't go silent.
type progressReader struct {
	r       io.Reader
	total   int64
	read    int64
	lastLog time.Time
	log     *shared.Logger
	label   string
}

func newProgressReader(r io.Reader, total int64, log *shared.Logger, label string) *progressReader {
	return &progressReader{r: r, total: total, log: log, label: label, lastLog: time.Now()}
}

func (pr *progressReader) Read(p []byte) (int, error) {
	n, err := pr.r.Read(p)
	pr.read += int64(n)
	if err == io.EOF || time.Since(pr.lastLog) >= time.Second {
		pct := 0
		if pr.total > 0 {
			pct = int(pr.read * 100 / pr.total)
		}
		pr.log.Info("  %s: %d/%d KB (%d%%)", pr.label, pr.read/1024, pr.total/1024, pct)
		pr.lastLog = time.Now()
	}
	return n, err
}

// namedFile is an io.Reader that the CF SDK's multipart marshaller can name.
// The SDK reflects on Filename() / ContentType() when assembling form parts.
type namedFile struct {
	io.Reader
	filename    string
	contentType string
}

func newNamedFile(r io.Reader, filename, contentType string) *namedFile {
	return &namedFile{Reader: r, filename: filename, contentType: contentType}
}

func (f *namedFile) Filename() string    { return f.filename }
func (f *namedFile) ContentType() string { return f.contentType }
