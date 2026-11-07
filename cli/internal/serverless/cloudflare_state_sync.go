package serverless

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	awsv2 "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3Types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"

	"github.com/aynaash/nextdeploy/cli/internal/serverless/cfstate"
	"github.com/aynaash/nextdeploy/shared/config"
	"github.com/aynaash/nextdeploy/shared/secrets"
	"github.com/aynaash/nextdeploy/shared/sensitive"
)

const (
	cfStateKeyEnv    = "NEXTDEPLOY_CF_STATE_KEY"
	cfStateObjectDir = "_nextdeploy/state/"
)

func (p *CloudflareProvider) remoteStateEnabled() bool {
	return os.Getenv(cfStateKeyEnv) != ""
}

func (p *CloudflareProvider) cfStateEncryptionKey() ([]byte, error) {
	masterKey := os.Getenv(cfStateKeyEnv)
	if masterKey == "" {
		masterKey = p.apiToken
	} else {
		if len(masterKey) < 32 {
			return nil, fmt.Errorf("%s must contain at least 32 bytes", cfStateKeyEnv)
		}
		sensitive.Register(masterKey)
	}
	return secrets.DeriveKey(masterKey)
}

func (p *CloudflareProvider) cfStateObjectKey(cfg *config.NextDeployConfig) string {
	return cfStateObjectDir + sanitizeCFName(cfg.App.Name) + "/" + filepath.Base(p.cfStatePath())
}

// SyncResourceState pulls the encrypted resource manifest from R2 and merges
// it with local state. Remote sync is enabled by setting NEXTDEPLOY_CF_STATE_KEY
// to the same secret on each developer machine and CI runner.
func (p *CloudflareProvider) SyncResourceState(ctx context.Context, cfg *config.NextDeployConfig) error {
	if !p.remoteStateEnabled() {
		return nil
	}
	key, err := p.cfStateEncryptionKey()
	if err != nil {
		return err
	}
	bucket := p.getBucketName(cfg)
	if err := p.ensureR2Client(ctx, bucket); err != nil {
		return fmt.Errorf("initialize R2 state sync: %w", err)
	}
	if err := p.syncRemoteState(ctx, bucket, p.cfStateObjectKey(cfg), key); err != nil {
		return fmt.Errorf("sync encrypted resource state: %w", err)
	}
	return nil
}

func (p *CloudflareProvider) syncRemoteState(ctx context.Context, bucket, objectKey string, key []byte) error {
	local, err := p.loadResourceManifest(p.cfStatePath(), key)
	if err != nil {
		return fmt.Errorf("load local state: %w", err)
	}

	const attempts = 5
	for attempt := 0; attempt < attempts; attempt++ {
		remote, etag, err := p.loadRemoteResourceManifest(ctx, bucket, objectKey, key)
		if err != nil {
			return err
		}
		merged := mergeResourceManifests(local, remote)
		if !sameResourceManifest(merged, local) {
			if err := cfstate.Save(p.cfStatePath(), merged, key); err != nil {
				return fmt.Errorf("save merged local state: %w", err)
			}
			local = merged
		}
		if sameResourceManifest(merged, remote) {
			return nil
		}

		data, err := cfstate.Encode(merged, key)
		if err != nil {
			return err
		}
		input := &s3.PutObjectInput{
			Bucket:      awsv2.String(bucket),
			Key:         awsv2.String(objectKey),
			Body:        bytes.NewReader(data),
			ContentType: awsv2.String("application/octet-stream"),
		}
		if etag != nil {
			input.IfMatch = etag
		} else {
			input.IfNoneMatch = awsv2.String("*")
		}
		if _, err := p.r2s3.PutObject(ctx, input); err == nil {
			p.log.Info("Synchronized encrypted resource state to R2 (%s)", objectKey)
			return nil
		}
		if attempt == attempts-1 {
			return fmt.Errorf("write remote state after %d concurrent update retries: %w", attempts, err)
		}
		// A conditional write may lose to another deploy. Reload and merge its
		// state before retrying; transient failures are also surfaced if retries
		// are exhausted.
	}
	return errors.New("remote state synchronization exhausted retries")
}

func (p *CloudflareProvider) loadRemoteResourceManifest(ctx context.Context, bucket, objectKey string, key []byte) (*cfstate.Manifest, *string, error) {
	out, err := p.r2s3.GetObject(ctx, &s3.GetObjectInput{
		Bucket: awsv2.String(bucket),
		Key:    awsv2.String(objectKey),
	})
	if err != nil {
		if isMissingRemoteState(err) {
			return cfstate.New(), nil, nil
		}
		return nil, nil, fmt.Errorf("read R2 state object: %w", err)
	}
	defer out.Body.Close()

	data, err := io.ReadAll(out.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("read R2 state body: %w", err)
	}
	manifest, err := cfstate.Decode(data, key)
	if err != nil {
		return nil, nil, fmt.Errorf("decode R2 state object: %w", err)
	}
	if out.ETag == nil || *out.ETag == "" {
		return nil, nil, errors.New("R2 state object has no ETag; refusing an unconditional overwrite")
	}
	return manifest, awsv2.String(*out.ETag), nil
}

func isMissingRemoteState(err error) bool {
	var noBucket *s3Types.NoSuchBucket
	var noKey *s3Types.NoSuchKey
	if errors.As(err, &noBucket) || errors.As(err, &noKey) {
		return true
	}
	var apiErr smithy.APIError
	return errors.As(err, &apiErr) && (apiErr.ErrorCode() == "NoSuchBucket" || apiErr.ErrorCode() == "NoSuchKey")
}

func (p *CloudflareProvider) loadResourceManifest(path string, key []byte) (*cfstate.Manifest, error) {
	manifest, err := cfstate.Load(path, key)
	if err == nil {
		return manifest, nil
	}
	if !p.remoteStateEnabled() {
		// Key was derived from fallback p.apiToken. When token rotation occurs or
		// local state is encrypted with an older token, decryption fails. Treat
		// the local cache as stale/invalidated, warn, and refresh state from
		// Cloudflare API instead of blocking the entire ship process.
		p.log.Warn("Local Cloudflare state (%s) could not be decrypted with current API token (token rotated or state invalid); invalidating local cache and refreshing from Cloudflare: %v", path, err)
		return cfstate.New(), nil
	}
	if p.apiToken == "" {
		return manifest, err
	}
	legacyKey, keyErr := secrets.DeriveKey(p.apiToken)
	if keyErr != nil {
		return nil, keyErr
	}
	if bytes.Equal(key, legacyKey) {
		return nil, err
	}
	legacy, legacyErr := cfstate.Load(path, legacyKey)
	if legacyErr != nil {
		return nil, errors.Join(err, fmt.Errorf("load legacy token-encrypted state: %w", legacyErr))
	}
	if err := cfstate.Save(path, legacy, key); err != nil {
		return nil, fmt.Errorf("migrate local state to shared encryption key: %w", err)
	}
	p.log.Info("Migrated local Cloudflare state to shared encryption key")
	return legacy, nil
}

func mergeResourceManifests(local, remote *cfstate.Manifest) *cfstate.Manifest {
	merged := cfstate.New()
	if local.Version > merged.Version {
		merged.Version = local.Version
	}
	if remote.Version > merged.Version {
		merged.Version = remote.Version
	}
	for _, source := range []*cfstate.Manifest{remote, local} {
		for _, record := range source.List() {
			current, exists := merged.Get(record.Kind, record.Name)
			if !exists || resourceRecordNewerOrEqual(record, current) {
				merged.Set(record.Kind, record.Name, record.ID, record.UpdatedAt)
			}
		}
	}
	return merged
}

func resourceRecordNewerOrEqual(a, b cfstate.Record) bool {
	aTime, aErr := time.Parse(time.RFC3339Nano, a.UpdatedAt)
	bTime, bErr := time.Parse(time.RFC3339Nano, b.UpdatedAt)
	if aErr == nil && bErr == nil {
		return !aTime.Before(bTime)
	}
	return a.UpdatedAt >= b.UpdatedAt
}

func sameResourceManifest(a, b *cfstate.Manifest) bool {
	if a.Version != b.Version || len(a.Resources) != len(b.Resources) {
		return false
	}
	for key, record := range a.Resources {
		if b.Resources[key] != record {
			return false
		}
	}
	return true
}

func stateRecordTimestamp() string {
	return time.Now().UTC().Format(time.RFC3339Nano)
}
