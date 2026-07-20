package serverless

import (
	"context"
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"slices"
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
	log            *shared.Logger
	cf             *cloudflare.Client
	r2s3           *s3.Client
	accountID      string
	apiToken       string
	apiTokenID     string
	r2AccessKey    string
	r2SecretKey    string
	r2ParentKeyID  string
	environment    string
	provisioned    *resourceMap
	pendingSecrets map[string]string
}

type cloudflareCreds struct {
	apiToken      string
	accountID     string
	r2AccessKey   string
	r2SecretKey   string
	r2ParentKeyID string
}

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
			log.Warn("Cloudflare API token loaded from nextdeploy.yml — committing this file leaks creds.")
			log.Warn("Recommended: 'nextdeploy creds set --provider cloudflare' (encrypted, mode 0600).")
		}
	}
	return c
}

func NewCloudflareProvider() *CloudflareProvider {
	return &CloudflareProvider{
		log: shared.PackageLogger("cloudflare", "☁️  CF::"),
	}
}

func sanitizeCFName(s string) string {
	var b strings.Builder
	lastHyphen := false
	for _, r := range strings.ToLower(s) {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
			lastHyphen = false
		case b.Len() > 0 && !lastHyphen:
			b.WriteByte('-')
			lastHyphen = true
		}
	}
	out := strings.Trim(b.String(), "-")
	if len(out) > 63 {
		out = strings.Trim(out[:63], "-")
	}
	return out
}

func (p *CloudflareProvider) workerName(appName string) string {
	env := p.environment
	if env == "" {
		env = "production"
	}
	return sanitizeCFName(fmt.Sprintf("%s-%s", appName, env))
}

func (p *CloudflareProvider) bucketNameFromApp(appName string) string {
	env := p.environment
	if env == "" {
		env = "production"
	}
	return sanitizeCFName(fmt.Sprintf("nextdeploy-%s-%s-assets", appName, env))
}

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

	if p.r2AccessKey != "" && p.r2SecretKey != "" {
		p.r2s3 = newR2S3Client(p.accountID, p.r2AccessKey, p.r2SecretKey, "")
	}

	p.log.Info("Cloudflare session initialized (account: %s)", p.accountID)
	return nil
}

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

func deriveR2CredsFromAPIToken(tokenID, tokenValue string) (akid, secret string, ok bool) {
	if tokenID == "" || tokenValue == "" {
		return "", "", false
	}
	sum := sha256.Sum256([]byte(tokenValue))
	return tokenID, hex.EncodeToString(sum[:]), true
}

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

type r2TempCreds struct {
	AccessKeyID     string `json:"accessKeyId"`
	SecretAccessKey string `json:"secretAccessKey"`
	SessionToken    string `json:"sessionToken"`
}

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

func (p *CloudflareProvider) DeployStatic(ctx context.Context, pkg *packaging.PackageResult, cfg *config.NextDeployConfig, meta *nextcore.NextCorePayload) error {
	bucketName := p.getBucketName(cfg)

	if err := p.ensureR2BucketExists(ctx, bucketName); err != nil {
		return fmt.Errorf("failed to ensure R2 bucket: %w", err)
	}

	if err := p.ensureR2Client(ctx, bucketName); err != nil {
		return err
	}

	p.log.Info("Uploading %d static assets to R2 bucket %s...", len(pkg.S3Assets), bucketName)

	immutable, mutable := partitionAssets(pkg.S3Assets)

	upImm, skImm, err := p.uploadBatch(ctx, bucketName, immutable)
	if err != nil {
		return fmt.Errorf("upload immutable assets: %w", err)
	}
	upMut, skMut, err := p.uploadBatch(ctx, bucketName, mutable)
	if err != nil {
		return fmt.Errorf("upload mutable assets: %w", err)
	}

	uploaded, skipped := upImm+upMut, skImm+skMut
	if skipped > 0 {
		p.log.Info("R2 asset sync: %d uploaded, %d skipped (content unchanged)", uploaded, skipped)
	} else {
		p.log.Info("R2 asset sync: %d uploaded to %s", uploaded, bucketName)
	}
	return nil
}

const immutableKeyPrefix = "_next/static/"

func partitionAssets(assets []packaging.S3Asset) (immutable, mutable []packaging.S3Asset) {
	for _, a := range assets {
		if strings.HasPrefix(a.S3Key, immutableKeyPrefix) {
			immutable = append(immutable, a)
		} else {
			mutable = append(mutable, a)
		}
	}
	return immutable, mutable
}

func (p *CloudflareProvider) uploadBatch(ctx context.Context, bucket string, assets []packaging.S3Asset) (uploaded, skipped int64, err error) {
	const cfR2UploadConcurrency = 8
	sem := make(chan struct{}, cfR2UploadConcurrency)
	errs := make(chan error, len(assets))
	var wg sync.WaitGroup

	var up, sk atomic.Int64

	for _, asset := range assets {
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			didUpload, uploadErr := p.uploadToR2IfChanged(ctx, bucket, asset)
			if uploadErr != nil {
				errs <- fmt.Errorf("upload %s: %w", asset.S3Key, uploadErr)
				return
			}
			if didUpload {
				up.Add(1)
			} else {
				sk.Add(1)
			}
		}()
	}
	wg.Wait()
	close(errs)

	for e := range errs {
		if e != nil {
			return up.Load(), sk.Load(), e
		}
	}
	return up.Load(), sk.Load(), nil
}

func (p *CloudflareProvider) ensureR2Client(ctx context.Context, bucketName string) error {
	if p.r2s3 != nil {
		return nil
	}
	switch {
	case p.r2ParentKeyID != "":
		p.log.Info("Minting short-lived R2 creds for bucket %s (parent: %s…)", bucketName, p.r2ParentKeyID[:min(8, len(p.r2ParentKeyID))])
		creds, err := p.mintR2TempCreds(ctx, bucketName)
		if err != nil {
			return fmt.Errorf("R2 temp credential minting failed: %w", err)
		}
		p.r2s3 = newR2S3Client(p.accountID, creds.AccessKeyID, creds.SecretAccessKey, creds.SessionToken)
	default:
		// Tier 2: derive R2 creds from the API token. Default zero-config path.
		if akid, secret, ok := deriveR2CredsFromAPIToken(p.apiTokenID, p.apiToken); ok {
			sensitive.Register(secret)
			p.log.Info("Using R2 credentials derived from API token (zero-config path)")
			p.r2s3 = newR2S3Client(p.accountID, akid, secret, "")
		}
	}
	if p.r2s3 == nil {
		return fmt.Errorf(
			"R2 access needs credentials. The CLOUDFLARE_API_TOKEN you\n" +
				"provided didn't expose a token id — that's unexpected. Check that\n" +
				"the token has 'Workers R2 Storage: Edit' scope. As a fallback you\n" +
				"can export an explicit pair (R2_ACCESS_KEY_ID + R2_SECRET_ACCESS_KEY)\n" +
				"or just R2_PARENT_ACCESS_KEY_ID — see\n" +
				"https://dash.cloudflare.com/?to=/:account/r2/api-tokens",
		)
	}
	return nil
}

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

func md5OfFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := md5.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func sha256OfFile(path string) ([32]byte, error) {
	var sum [32]byte
	f, err := os.Open(path)
	if err != nil {
		return sum, err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return sum, err
	}
	copy(sum[:], h.Sum(nil))
	return sum, nil
}

func (p *CloudflareProvider) putToR2(ctx context.Context, bucket string, asset packaging.S3Asset) error {
	f, err := os.Open(asset.LocalPath)
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

const deployHashTagPrefix = "nextdeploy-deploy-hash:"

func computeDeployHash(bundleSum [32]byte, meta workers.ScriptUpdateParamsMetadata, secrets map[string]string) string {
	h := sha256.New()
	h.Write([]byte("nextdeploy-deploy-hash/v1\n"))

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

func (p *CloudflareProvider) workerAlreadyAtHash(ctx context.Context, workerName, hash string) bool {
	settings, err := p.cf.Workers.Scripts.Settings.Get(ctx, workerName, workers.ScriptSettingGetParams{
		AccountID: cloudflare.F(p.accountID),
	})
	if err != nil || settings == nil {
		return false
	}
	want := deployHashTagPrefix + hash
	return slices.Contains(settings.Tags, want)
}

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

func (p *CloudflareProvider) DeployCompute(ctx context.Context, pkg *packaging.PackageResult, cfg *config.NextDeployConfig, meta *nextcore.NextCorePayload) error {
	if meta != nil && meta.OutputMode == nextcore.OutputModeExport {
		if len(p.pendingSecrets) > 0 {
			p.log.Warn("Static-export build has no server runtime — %d staged secret(s) "+
				"will NOT be deployed. Move runtime logic to SSR, or drop them from config.",
				len(p.pendingSecrets))
			p.pendingSecrets = nil
		}
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

	bundleSum, err := sha256OfFile(bundlePath)
	if err != nil {
		return fmt.Errorf("failed to fingerprint worker bundle: %w", err)
	}
	bundleInfo, err := os.Stat(bundlePath)
	if err != nil {
		return fmt.Errorf("failed to stat worker bundle: %w", err)
	}

	workerName := p.getWorkerName(cfg)
	bucketName := p.getBucketName(cfg)

	const entryName = "worker.mjs"
	var cfBlock *config.CloudflareConfig
	if cfg.Serverless != nil {
		cfBlock = cfg.Serverless.Cloudflare
	}
	var resolve refResolver = noResolver
	if p.provisioned != nil {
		resolve = p.provisioned.get
	}

	if cfBlock == nil || !cfBlock.AllowSecretWipe {
		live, err := p.GetSecrets(ctx, cfg.App.Name)
		if err != nil {
			p.log.Warn("Could not list live Worker secrets to check for an accidental wipe (%v) — proceeding without the guard", err)
		} else if err := refuseSecretWipe(p.pendingSecrets, live, false); err != nil {
			return err
		}
	}

	scriptMeta, err := buildScriptMetadata(cfBlock, bucketName, entryName, resolve, p.pendingSecrets)
	if err != nil {
		return fmt.Errorf("build script metadata: %w", err)
	}
	if len(p.pendingSecrets) > 0 {
		p.log.Info("Folding %d secret_text bindings into Worker upload", len(p.pendingSecrets))
	}

	deployHash := computeDeployHash(bundleSum, scriptMeta, p.pendingSecrets)
	if p.workerAlreadyAtHash(ctx, workerName, deployHash) {
		p.log.Info("Worker bundle and bindings unchanged (hash %s) — skipping upload", deployHash[:12])
		p.pendingSecrets = nil
	} else {
		bundleFile, err := os.Open(bundlePath) // #nosec G304 — packager output path
		if err != nil {
			return fmt.Errorf("failed to open worker bundle: %w", err)
		}
		defer bundleFile.Close()
		tracked := newProgressReader(bundleFile, bundleInfo.Size(), p.log, "worker upload")
		scriptReader := newNamedFile(tracked, entryName, "application/javascript+module")

		params := workers.ScriptUpdateParams{
			AccountID: cloudflare.F(p.accountID),
			Metadata:  cloudflare.F(scriptMeta),
			Files:     cloudflare.F([]io.Reader{scriptReader}),
		}
		if _, err := p.cf.Workers.Scripts.Update(ctx, workerName, params); err != nil {
			return fmt.Errorf("worker upload failed: %w", err)
		}
		p.log.Info("Worker deployed: %s", workerName)
		if err := p.tagWorkerDeployHash(ctx, workerName, deployHash); err != nil {
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

func (p *CloudflareProvider) applyWorkerTriggers(ctx context.Context, workerName string, cfBlock *config.CloudflareConfig) error {
	if cfBlock == nil || cfBlock.Triggers == nil {
		return nil
	}
	if err := p.applyCronTriggers(ctx, workerName, cfBlock.Triggers.Crons); err != nil {
		return fmt.Errorf("apply cron triggers: %w", err)
	}
	return nil
}

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
	if cfg.App.Domain.Name != "" && hasNoExplicitEdge(cfBlock) {
		hostnames := autoCustomDomainHostnames(cfg.App.Domain.Name)
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
	overrides := []option.RequestOption{
		option.WithJSONSet("override_existing_origin", true),
		option.WithJSONSet("override_existing_dns_record", true),
	}
	if _, err := p.cf.Workers.Domains.Update(ctx, params, overrides...); err != nil {
		return err
	}
	p.log.Info("Custom domain attached: %s → %s", cd.Hostname, workerName)
	return nil
}

func zoneNameFromHostname(host string) string {
	parts := strings.Split(host, ".")
	if len(parts) <= 2 {
		return host
	}
	return strings.Join(parts[len(parts)-2:], ".")
}

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

func (p *CloudflareProvider) UpdateSecrets(ctx context.Context, appName string, secrets map[string]string) error {
	_ = ctx
	_ = appName
	if len(secrets) == 0 {
		p.pendingSecrets = nil
		return nil
	}
	staged := make(map[string]string, len(secrets))
	maps.Copy(staged, secrets)
	p.pendingSecrets = staged
	p.log.Info("Staged %d secrets to fold into the next Worker upload", len(staged))
	return nil
}

func refuseSecretWipe(pending, live map[string]string, allow bool) error {
	if allow || len(live) == 0 {
		return nil
	}
	dropped := map[string]string{}
	for name := range live {
		if _, keep := pending[name]; !keep {
			dropped[name] = ""
		}
	}
	if len(dropped) == 0 {
		return nil
	}
	return fmt.Errorf("refusing to upload a Worker that would strip %d live secret(s) (%s): "+
		"the incoming secret set omits them and CF uploads are replace-not-merge; "+
		"set cloudflare.allow_secret_wipe: true to override",
		len(dropped), strings.Join(sortedKeys(dropped), ", "))
}

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

func (p *CloudflareProvider) InvalidateCache(ctx context.Context, cfg *config.NextDeployConfig) error {
	if cfg.App.Domain.Name == "" {
		p.log.Info("No domain configured, skipping cache purge.")
		return nil
	}

	zoneID, err := p.getZoneID(ctx, cfg.App.Domain.Name)
	if err != nil {
		return fmt.Errorf("failed to find zone for %s: %w", cfg.App.Domain.Name, err)
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
	previousVersionID := pickActiveVersion(target.Versions)
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

	if err := p.InvalidateCache(ctx, cfg); err != nil {
		p.log.Warn("Rollback: cache invalidation failed (edge may serve stale HTML): %v", err)
	}
	p.log.Warn("Rollback re-pointed the Worker to %s, but R2 HTML/RSC assets are NOT "+
		"version-restored. If this deploy changed prerendered pages, redeploy the "+
		"known-good commit to realign assets with server code.", previousVersionID)

	p.log.Info("Rollback complete. Worker is now running version %s", previousVersionID)
	return nil
}

func pickActiveVersion(versions []workers.DeploymentVersion) string {
	best := versions[0]
	for _, v := range versions[1:] {
		if v.Percentage > best.Percentage {
			best = v
		}
	}
	return best.VersionID
}

func (p *CloudflareProvider) Destroy(ctx context.Context, cfg *config.NextDeployConfig) error {
	workerName := p.getWorkerName(cfg)
	bucketName := p.getBucketName(cfg)

	var problems []string

	p.log.Info("Deleting Worker: %s...", workerName)
	if _, err := p.cf.Workers.Scripts.Delete(ctx, workerName, workers.ScriptDeleteParams{
		AccountID: cloudflare.F(p.accountID),
	}); err != nil && !isCFNotFound(err) {
		p.log.Warn("Worker delete failed: %v", err)
		problems = append(problems, "worker "+workerName)
	}

	// 2. R2 — empty the bucket, then delete it.
	p.log.Info("Emptying + deleting R2 bucket: %s...", bucketName)
	if err := p.sweepR2Bucket(ctx, bucketName); err != nil {
		p.log.Warn("R2 sweep failed (bucket not deleted, still holds objects): %v", err)
		problems = append(problems, "r2 bucket "+bucketName+" (objects remain)")
	} else if _, err := p.cf.R2.Buckets.Delete(ctx, bucketName, r2.BucketDeleteParams{
		AccountID: cloudflare.F(p.accountID),
	}); err != nil && !isCFNotFound(err) {
		p.log.Warn("R2 bucket delete failed: %v", err)
		problems = append(problems, "r2 bucket "+bucketName)
	}

	problems = append(problems, p.teardownProvisionedResources(ctx)...)

	problems = append(problems, p.teardownDeclaredDNS(ctx, cfg)...)

	if len(problems) > 0 {
		return fmt.Errorf("destroy incomplete — these still exist and need manual cleanup "+
			"(check the Cloudflare dashboard — they may still bill): %s", strings.Join(problems, ", "))
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
		CustomDomain:   cfg.App.Domain.Name,
		DeploymentTime: time.Now(),
	}, nil
}

func (p *CloudflareProvider) getWorkerName(cfg *config.NextDeployConfig) string {
	return p.workerName(cfg.App.Name)
}

func (p *CloudflareProvider) getBucketName(cfg *config.NextDeployConfig) string {
	return p.bucketNameFromApp(cfg.App.Name)
}

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

func isR2NotEnabled(err error) bool {
	return err != nil && strings.Contains(err.Error(), "10042")
}

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

func (p *CloudflareProvider) ensureWorkerRoute(ctx context.Context, workerName, domain string) error {
	return p.ensureWorkerRouteForZone(ctx, workerName, domain+"/*", domain)
}

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
