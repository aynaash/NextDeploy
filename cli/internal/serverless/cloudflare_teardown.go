package serverless

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"

	awsv2 "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3Types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/cloudflare/cloudflare-go/v6"
	"github.com/cloudflare/cloudflare-go/v6/ai_gateway"
	"github.com/cloudflare/cloudflare-go/v6/d1"
	"github.com/cloudflare/cloudflare-go/v6/dns"
	"github.com/cloudflare/cloudflare-go/v6/hyperdrive"
	"github.com/cloudflare/cloudflare-go/v6/kv"
	"github.com/cloudflare/cloudflare-go/v6/queues"
	"github.com/cloudflare/cloudflare-go/v6/vectorize"

	"github.com/aynaash/nextdeploy/cli/internal/serverless/cfstate"
	"github.com/aynaash/nextdeploy/shared/config"
	"github.com/aynaash/nextdeploy/shared/secrets"
)

// isCFNotFound reports whether err is a Cloudflare 404 — treated as
// already-gone (idempotent) during teardown.
func isCFNotFound(err error) bool {
	var apiErr *cloudflare.Error
	return errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound
}

// sweepR2Bucket empties an R2 bucket so it can be deleted (CF refuses to delete
// a non-empty bucket). Lists + batch-deletes every object via the S3-compatible
// client. A missing bucket is not an error.
func (p *CloudflareProvider) sweepR2Bucket(ctx context.Context, bucketName string) error {
	if err := p.ensureR2Client(ctx, bucketName); err != nil {
		return err
	}
	swept := 0
	pager := s3.NewListObjectsV2Paginator(p.r2s3, &s3.ListObjectsV2Input{
		Bucket: awsv2.String(bucketName),
	})
	for pager.HasMorePages() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			var noBucket *s3Types.NoSuchBucket
			if errors.As(err, &noBucket) {
				return nil // already gone
			}
			return fmt.Errorf("list objects: %w", err)
		}
		if len(page.Contents) == 0 {
			continue
		}
		ids := make([]s3Types.ObjectIdentifier, 0, len(page.Contents))
		for _, obj := range page.Contents {
			ids = append(ids, s3Types.ObjectIdentifier{Key: obj.Key})
		}
		if _, err := p.r2s3.DeleteObjects(ctx, &s3.DeleteObjectsInput{
			Bucket: awsv2.String(bucketName),
			Delete: &s3Types.Delete{Objects: ids},
		}); err != nil {
			return fmt.Errorf("delete objects: %w", err)
		}
		swept += len(ids)
	}
	if swept > 0 {
		p.log.Info("Swept %d object(s) from R2 bucket %s", swept, bucketName)
	}
	return nil
}

// deleteProvisionedResource deletes a single provisioned resource by kind + id.
// id is the value cfstate recorded (a UUID for D1/KV/Hyperdrive/Queues, the
// index name for Vectorize, the slug for AI Gateway).
func (p *CloudflareProvider) deleteProvisionedResource(ctx context.Context, kind, id string) error {
	acc := cloudflare.F(p.accountID)
	switch kind {
	case "d1":
		_, err := p.cf.D1.Database.Delete(ctx, id, d1.DatabaseDeleteParams{AccountID: acc})
		return err
	case "kv":
		_, err := p.cf.KV.Namespaces.Delete(ctx, id, kv.NamespaceDeleteParams{AccountID: acc})
		return err
	case "hyperdrive":
		_, err := p.cf.Hyperdrive.Configs.Delete(ctx, id, hyperdrive.ConfigDeleteParams{AccountID: acc})
		return err
	case "vectorize":
		_, err := p.cf.Vectorize.Indexes.Delete(ctx, id, vectorize.IndexDeleteParams{AccountID: acc})
		return err
	case "queue":
		_, err := p.cf.Queues.Delete(ctx, id, queues.QueueDeleteParams{AccountID: acc})
		return err
	case "ai_gateway":
		_, err := p.cf.AIGateway.Delete(ctx, id, ai_gateway.AIGatewayDeleteParams{AccountID: acc})
		return err
	default:
		return fmt.Errorf("unknown resource kind %q", kind)
	}
}

// teardownProvisionedResources deletes every resource recorded in the encrypted
// cfstate manifest (D1, KV, Hyperdrive, Queues, Vectorize, AI Gateway),
// including orphans. Successfully-deleted (or already-gone) records are dropped
// from state; records that fail to delete are kept so a re-run can retry.
// Returns the failed "kind:name" list.
func (p *CloudflareProvider) teardownProvisionedResources(ctx context.Context) []string {
	if p.apiToken == "" {
		return nil
	}
	key, err := secrets.DeriveKey(p.apiToken)
	if err != nil {
		p.log.Warn("teardown: derive state key failed (skipping resource cleanup): %v", err)
		return nil
	}
	manifest, err := cfstate.Load(cfStatePath, key)
	if err != nil {
		p.log.Warn("teardown: could not read state manifest (skipping resource cleanup): %v", err)
		return nil
	}
	records := manifest.List()
	if len(records) == 0 {
		return nil
	}

	var failed []string
	survivors := cfstate.New()
	for _, r := range records {
		p.log.Info("Deleting %s: %s (id=%s)...", r.Kind, r.Name, r.ID)
		if err := p.deleteProvisionedResource(ctx, r.Kind, r.ID); err != nil && !isCFNotFound(err) {
			p.log.Warn("  failed to delete %s %q: %v", r.Kind, r.Name, err)
			failed = append(failed, r.Kind+":"+r.Name)
			survivors.Set(r.Kind, r.Name, r.ID, r.UpdatedAt)
		}
	}

	if len(survivors.Resources) == 0 {
		if err := os.Remove(cfStatePath); err != nil && !os.IsNotExist(err) {
			p.log.Warn("teardown: could not remove state file %s: %v", cfStatePath, err)
		}
	} else if err := cfstate.Save(cfStatePath, survivors, key); err != nil {
		p.log.Warn("teardown: could not persist remaining state: %v", err)
	}
	return failed
}

// teardownDeclaredDNS deletes the DNS records nextdeploy provisioned — matched
// by (name, type, content) so unrelated sibling records in the zone are never
// touched. Returns the failed record descriptors.
func (p *CloudflareProvider) teardownDeclaredDNS(ctx context.Context, cfg *config.NextDeployConfig) []string {
	if cfg.Serverless == nil || cfg.Serverless.Cloudflare == nil || cfg.Serverless.Cloudflare.Resources == nil {
		return nil
	}
	var failed []string
	for _, decl := range cfg.Serverless.Cloudflare.Resources.DNS {
		if decl.Zone == "" || decl.Name == "" || decl.Type == "" {
			continue
		}
		failed = append(failed, p.teardownDNSRecord(ctx, &decl)...)
	}
	return failed
}

// teardownDNSRecord deletes the record(s) matching one DNS declaration by value,
// leaving unrelated siblings in the zone untouched. Returns failure descriptors.
func (p *CloudflareProvider) teardownDNSRecord(ctx context.Context, decl *config.CFDNSRecord) []string {
	zoneID, err := p.getZoneID(ctx, decl.Zone)
	if err != nil {
		p.log.Warn("teardown dns: zone %q lookup failed: %v", decl.Zone, err)
		return []string{"dns " + decl.Type + "/" + decl.Name}
	}
	fqdn := dnsRecordFQDN(decl.Name, decl.Zone)
	recType := strings.ToUpper(decl.Type)
	ttl := decl.TTL
	if ttl <= 0 {
		ttl = 1
	}
	records, err := p.findDNSRecords(ctx, zoneID, fqdn, recType)
	if err != nil {
		p.log.Warn("teardown dns: list %s %s failed: %v", recType, fqdn, err)
		return []string{"dns " + recType + "/" + fqdn}
	}
	var failed []string
	for i := range records {
		// Only delete the record whose value nextdeploy set — never a sibling.
		if !dnsRecordMatches(records[i], decl.Content, ttl, decl.Proxied) {
			continue
		}
		p.log.Info("Deleting DNS record: %s %s → %s...", recType, fqdn, decl.Content)
		if _, err := p.cf.DNS.Records.Delete(ctx, records[i].ID, dns.RecordDeleteParams{
			ZoneID: cloudflare.F(zoneID),
		}); err != nil && !isCFNotFound(err) {
			p.log.Warn("  failed to delete DNS %s %s: %v", recType, fqdn, err)
			failed = append(failed, "dns "+recType+"/"+fqdn)
		}
	}
	return failed
}
