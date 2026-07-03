package serverless

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/aynaash/nextdeploy/shared/config"

	"github.com/cloudflare/cloudflare-go/v6"
	"github.com/cloudflare/cloudflare-go/v6/dns"
)

func (p *CloudflareProvider) ensureDNSRecord(ctx context.Context, decl config.CFDNSRecord) error {
	if decl.Zone == "" {
		return errors.New("dns: zone is required")
	}
	if decl.Name == "" {
		return errors.New("dns: name is required")
	}
	if decl.Type == "" {
		return errors.New("dns: type is required")
	}
	if decl.Content == "" {
		return fmt.Errorf("dns %s/%s: content is required", decl.Type, decl.Name)
	}

	zoneID, err := p.getZoneID(ctx, decl.Zone)
	if err != nil {
		return fmt.Errorf("dns %s %s: resolve zone %q: %w", decl.Type, decl.Name, decl.Zone, err)
	}

	fqdn := dnsRecordFQDN(decl.Name, decl.Zone)
	recType := strings.ToUpper(decl.Type)
	ttl := decl.TTL
	if ttl <= 0 {
		ttl = 1 // CF "automatic"
	}

	newBody, editBody, err := buildDNSRecordBody(fqdn, recType, decl.Content, ttl, decl.Proxied)
	if err != nil {
		return fmt.Errorf("dns %s %s: %w", recType, fqdn, err)
	}

	existing, err := p.findDNSRecords(ctx, zoneID, fqdn, recType)
	if err != nil {
		return fmt.Errorf("dns %s %s: list: %w", recType, fqdn, err)
	}

	// Already-current if ANY existing record carries the declared value — never
	// rewrite a sibling to reach this state. Index to avoid copying the large
	// dns.RecordResponse per iteration.
	for i := range existing {
		if dnsRecordMatches(existing[i], decl.Content, ttl, decl.Proxied) {
			p.log.Info("DNS record already current: %s %s → %s", recType, fqdn, decl.Content)
			return nil
		}
	}

	// A single-valued type (CNAME) with exactly one record is safe to edit in
	// place. For multi-valued types (A/TXT) with no content match, CREATE —
	// editing would clobber a round-robin leg or an SPF/verification sibling.
	if isSingleValued(recType) && len(existing) == 1 {
		if _, err := p.cf.DNS.Records.Edit(ctx, existing[0].ID, dns.RecordEditParams{
			ZoneID: cloudflare.F(zoneID),
			Body:   editBody,
		}); err != nil {
			return fmt.Errorf("dns %s %s: update: %w", recType, fqdn, err)
		}
		p.log.Info("DNS record updated: %s %s → %s (ttl=%d, proxied=%v)", recType, fqdn, decl.Content, ttl, decl.Proxied)
		return nil
	}

	if _, err := p.cf.DNS.Records.New(ctx, dns.RecordNewParams{
		ZoneID: cloudflare.F(zoneID),
		Body:   newBody,
	}); err != nil {
		return fmt.Errorf("dns %s %s: create: %w", recType, fqdn, err)
	}
	p.log.Info("DNS record created: %s %s → %s (ttl=%d, proxied=%v)", recType, fqdn, decl.Content, ttl, decl.Proxied)
	return nil
}

// isSingleValued reports whether a record type holds at most one value per name,
// making in-place edit of the sole record correct. A/AAAA/TXT/MX/NS/CAA can hold
// several per name; CNAME cannot.
func isSingleValued(recType string) bool {
	return recType == "CNAME"
}

// findDNSRecords returns ALL records matching (name, type) in the zone. A
// (name, type) can legitimately hold several records — round-robin A/AAAA,
// multiple apex TXT (SPF + verification). Returning only the first let
// ensureDNSRecord rewrite a sibling in place.
func (p *CloudflareProvider) findDNSRecords(ctx context.Context, zoneID, fqdn, recType string) ([]dns.RecordResponse, error) {
	iter := p.cf.DNS.Records.ListAutoPaging(ctx, dns.RecordListParams{
		ZoneID: cloudflare.F(zoneID),
		Name: cloudflare.F(dns.RecordListParamsName{
			Exact: cloudflare.F(fqdn),
		}),
	})
	var out []dns.RecordResponse
	for iter.Next() {
		r := iter.Current()
		if string(r.Type) == recType {
			out = append(out, r)
		}
	}
	if err := iter.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// dnsRecordFQDN expands "@" → zone, "*" → "*.zone", and "sub" → "sub.zone".
// Names that already end in the zone (or contain a dot) are left alone.
func dnsRecordFQDN(name, zone string) string {
	switch name {
	case "@":
		return zone
	case "*":
		return "*." + zone
	}
	// Fully qualified only if it already ends with the zone. A bare label
	// ("api") AND a multi-label subdomain ("api.staging") both expand by
	// appending the zone — the old strings.Contains(".") shortcut wrongly
	// treated "api.staging" as already-qualified, so lookups missed and every
	// deploy created a duplicate.
	if name == zone || strings.HasSuffix(name, "."+zone) {
		return name
	}
	return name + "." + zone
}

// dnsRecordMatches reports whether the existing CF record already has the
// declared content/ttl/proxied values — used to skip no-op updates.
func dnsRecordMatches(r dns.RecordResponse, content string, ttl int, proxied bool) bool {
	return r.Content == content && int(r.TTL) == ttl && r.Proxied == proxied
}

func buildDNSRecordBody(name, recType, content string, ttl int, proxied bool) (dns.RecordNewParamsBodyUnion, dns.RecordEditParamsBodyUnion, error) {
	switch recType {
	case "A":
		v := dns.ARecordParam{
			Name:    cloudflare.F(name),
			Type:    cloudflare.F(dns.ARecordTypeA),
			Content: cloudflare.F(content),
			TTL:     cloudflare.F(dns.TTL(ttl)),
			Proxied: cloudflare.F(proxied),
		}
		return v, v, nil
	case "AAAA":
		v := dns.AAAARecordParam{
			Name:    cloudflare.F(name),
			Type:    cloudflare.F(dns.AAAARecordTypeAAAA),
			Content: cloudflare.F(content),
			TTL:     cloudflare.F(dns.TTL(ttl)),
			Proxied: cloudflare.F(proxied),
		}
		return v, v, nil
	case "CNAME":
		v := dns.CNAMERecordParam{
			Name:    cloudflare.F(name),
			Type:    cloudflare.F(dns.CNAMERecordTypeCNAME),
			Content: cloudflare.F(content),
			TTL:     cloudflare.F(dns.TTL(ttl)),
			Proxied: cloudflare.F(proxied),
		}
		return v, v, nil
	case "TXT":
		v := dns.TXTRecordParam{
			Name:    cloudflare.F(name),
			Type:    cloudflare.F(dns.TXTRecordTypeTXT),
			Content: cloudflare.F(content),
			TTL:     cloudflare.F(dns.TTL(ttl)),
		}
		return v, v, nil
	default:
		return nil, nil, fmt.Errorf("unsupported DNS record type %q (supported: A, AAAA, CNAME, TXT)", recType)
	}
}
