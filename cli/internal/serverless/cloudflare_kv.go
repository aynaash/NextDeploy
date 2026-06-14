package serverless

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/cloudflare/cloudflare-go/v6"
	"github.com/cloudflare/cloudflare-go/v6/kv"
)

// ensureKVNamespace creates a KV namespace with the given title if one doesn't
// already exist (titles are unique per account). Returns the namespace UUID.
//
// KV namespaces back the protection guard's per-IP rate limiter, so this is how
// `cloudflare.protection.rate_limit` becomes turnkey: declare a resources.kv
// entry + a ref binding and the namespace is provisioned automatically.
func (p *CloudflareProvider) ensureKVNamespace(ctx context.Context, title string) (string, error) {
	id, err := p.findKVNamespaceID(ctx, title)
	if err != nil {
		return "", fmt.Errorf("list kv namespaces: %w", err)
	}
	if id != "" {
		p.log.Info("KV namespace already exists: %s (id=%s)", title, id)
		return id, nil
	}
	created, err := p.cf.KV.Namespaces.New(ctx, kv.NamespaceNewParams{
		AccountID: cloudflare.F(p.accountID),
		Title:     cloudflare.F(title),
	})
	if err != nil {
		return "", fmt.Errorf("create kv namespace %q: %w", title, err)
	}
	p.log.Info("KV namespace created: %s (id=%s)", title, created.ID)
	return created.ID, nil
}

// findKVNamespaceID returns the UUID of the namespace with the given title, or
// "" if none. Titles are the unique key; there is no get-by-title endpoint.
func (p *CloudflareProvider) findKVNamespaceID(ctx context.Context, title string) (string, error) {
	iter := p.cf.KV.Namespaces.ListAutoPaging(ctx, kv.NamespaceListParams{
		AccountID: cloudflare.F(p.accountID),
	})
	for iter.Next() {
		ns := iter.Current()
		if ns.Title == title {
			return ns.ID, nil
		}
	}
	if err := iter.Err(); err != nil {
		var apiErr *cloudflare.Error
		if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
			return "", nil
		}
		return "", err
	}
	return "", nil
}
