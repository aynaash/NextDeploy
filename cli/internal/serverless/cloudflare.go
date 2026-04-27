package serverless

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Golangcodes/nextdeploy/internal/packaging"
	"github.com/Golangcodes/nextdeploy/shared"
	"github.com/Golangcodes/nextdeploy/shared/config"
	"github.com/Golangcodes/nextdeploy/shared/credstore"
	"github.com/Golangcodes/nextdeploy/shared/nextcore"
	"github.com/Golangcodes/nextdeploy/shared/sensitive"

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
	p.r2AccessKey = creds.r2AccessKey
	p.r2SecretKey = creds.r2SecretKey
	p.r2ParentKeyID = creds.r2ParentKeyID

	p.cf = cloudflare.NewClient(
		option.WithAPIToken(creds.apiToken),
		option.WithRequestTimeout(60*time.Second),
	)

	if err := p.verifyToken(ctx); err != nil {
		return err
	}

	if creds.accountID == "" {
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
func (p *CloudflareProvider) verifyToken(ctx context.Context) error {
	var verify struct {
		Success bool `json:"success"`
		Result  struct {
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
	return nil
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
func newR2S3Client(accountID, akid, secret, sessionToken string) *s3.Client {
	if akid == "" || secret == "" {
		return nil
	}
	endpoint := fmt.Sprintf("https://%s.r2.cloudflarestorage.com", accountID)
	return s3.New(s3.Options{
		Region:       "auto",
		BaseEndpoint: awsv2.String(endpoint),
		Credentials:  credentials.NewStaticCredentialsProvider(akid, secret, sessionToken),
		UsePathStyle: false,
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
// R2 object PUTs are S3-protocol — cloudflare-go doesn't expose them. Three
// credential paths, in order of preference:
//
//  1. Long-lived (R2_ACCESS_KEY_ID + R2_SECRET_ACCESS_KEY in env/credstore).
//     Used as-is. Fastest, no extra API call.
//  2. Short-lived from a parent (R2_PARENT_ACCESS_KEY_ID set). At deploy
//     time, mint a 1-hour scoped triple (akid, secret, sessionToken) via
//     POST /accounts/:id/r2/temp-access-credentials. No long-lived secret
//     ever lives on the deploy host.
//  3. Neither — fail loud with dashboard URL and roadmap pointer.
func (p *CloudflareProvider) DeployStatic(ctx context.Context, pkg *packaging.PackageResult, cfg *config.NextDeployConfig, meta *nextcore.NextCorePayload) error {
	bucketName := p.getBucketName(cfg)

	if err := p.ensureR2BucketExists(ctx, bucketName); err != nil {
		return fmt.Errorf("failed to ensure R2 bucket: %w", err)
	}

	if p.r2s3 == nil {
		if p.r2ParentKeyID == "" {
			return fmt.Errorf(
				"R2 object uploads need either:\n" +
					"  • R2_ACCESS_KEY_ID + R2_SECRET_ACCESS_KEY (long-lived), or\n" +
					"  • R2_PARENT_ACCESS_KEY_ID (we mint 1-hour temp creds via the API token).\n\n" +
					"Generate a key pair: https://dash.cloudflare.com/?to=/:account/r2/api-tokens\n" +
					"  (R2 → Manage R2 API Tokens → Create API token → Permissions: Object Read & Write)\n" +
					"Then either export both halves in env, or export only the access key id as\n" +
					"R2_PARENT_ACCESS_KEY_ID and let nextdeploy mint short-lived child creds.",
			)
		}
		p.log.Info("Minting short-lived R2 creds for bucket %s (parent: %s…)", bucketName, p.r2ParentKeyID[:min(8, len(p.r2ParentKeyID))])
		creds, err := p.mintR2TempCreds(ctx, bucketName)
		if err != nil {
			return fmt.Errorf("R2 temp credential minting failed: %w", err)
		}
		p.r2s3 = newR2S3Client(p.accountID, creds.AccessKeyID, creds.SecretAccessKey, creds.SessionToken)
		if p.r2s3 == nil {
			return fmt.Errorf("R2 temp credential minting returned an empty triple")
		}
	}

	p.log.Info("Uploading %d static assets to R2 bucket %s...", len(pkg.S3Assets), bucketName)

	const cfR2UploadConcurrency = 8
	sem := make(chan struct{}, cfR2UploadConcurrency)
	errs := make(chan error, len(pkg.S3Assets))
	var wg sync.WaitGroup

	for _, asset := range pkg.S3Assets {
		asset := asset
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			if err := p.uploadToR2(ctx, bucketName, asset); err != nil {
				errs <- fmt.Errorf("upload %s: %w", asset.S3Key, err)
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

	p.log.Info("Static assets uploaded to R2 bucket: %s", bucketName)
	return nil
}

func (p *CloudflareProvider) uploadToR2(ctx context.Context, bucket string, asset packaging.S3Asset) error {
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
	scriptReader := newNamedFile(bytes.NewReader(scriptBytes), entryName, "application/javascript+module")

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

	params := workers.ScriptUpdateParams{
		AccountID: cloudflare.F(p.accountID),
		Metadata:  cloudflare.F(scriptMeta),
		Files:     cloudflare.F([]io.Reader{scriptReader}),
	}

	if _, err := p.cf.Workers.Scripts.Update(ctx, workerName, params); err != nil {
		return fmt.Errorf("worker upload failed: %w", err)
	}
	p.log.Info("Worker deployed: %s", workerName)
	// Folded secrets landed atomically in the upload above; drop the stash
	// so a follow-up DeployCompute (e.g. a redeploy without new secrets)
	// doesn't re-emit them after the user had a chance to rotate via
	// SetSecret/UnsetSecret.
	p.pendingSecrets = nil

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
// Falls back to the legacy app.domain route when cfBlock has no explicit
// edge configuration.
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
	// Legacy single-domain fallback — only when the cloudflare block has
	// no explicit edge configuration of its own.
	if cfg.App.Domain != "" && hasNoExplicitEdge(cfBlock) {
		if err := p.ensureWorkerRoute(ctx, workerName, cfg.App.Domain); err != nil {
			p.log.Warn("Failed to set worker route for %s (non-fatal): %v", cfg.App.Domain, err)
		}
	}
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
// Other API errors propagate so we don't mask permission problems.
func (p *CloudflareProvider) ensureR2BucketExists(ctx context.Context, name string) error {
	_, err := p.cf.R2.Buckets.Get(ctx, name, r2.BucketGetParams{
		AccountID: cloudflare.F(p.accountID),
	})
	if err == nil {
		return nil
	}
	var apiErr *cloudflare.Error
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusNotFound {
		return fmt.Errorf("get bucket: %w", err)
	}
	_, err = p.cf.R2.Buckets.New(ctx, r2.BucketNewParams{
		AccountID: cloudflare.F(p.accountID),
		Name:      cloudflare.F(name),
	})
	return err
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
