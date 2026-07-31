package serverless

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/aynaash/nextdeploy/shared/config"

	"github.com/cloudflare/cloudflare-go/v6"
	"github.com/cloudflare/cloudflare-go/v6/d1"
	"github.com/cloudflare/cloudflare-go/v6/kv"
	"github.com/cloudflare/cloudflare-go/v6/queues"
)

// Adoption and orphan safety for accounts that already have resources.
//
// Provisioning is idempotent by NAME: `ensure*` lists the account's resources
// and matches on the declared name, so a resource that already exists is
// ADOPTED rather than recreated. There is no state file, which is what makes a
// pre-provisioned account work at all — you can point nextdeploy at an existing
// D1/KV/R2/Queue and it binds to it.
//
// The flip side is that name IS identity, and two edits are silently
// destructive:
//
//  1. RENAME. Changing `resources.d1[].name` does not rename anything. It
//     provisions a NEW empty resource under the new name and abandons the old
//     one — still present, still billed, still holding all your data, no longer
//     referenced. The deploy succeeds and the app comes up empty.
//
//  2. REMOVAL. Deleting a resource from the config does not delete it from the
//     account. It just stops being managed.
//
// Neither is detectable from the config alone: "renamed" and "added a new one"
// are the same diff. What distinguishes them is the account — an existing
// resource that matches this app's naming prefix but is no longer declared.
// That's what scanUnmanaged finds, and why plan reports it.

// UnmanagedResource is an account resource that looks like it belongs to this
// app (it matches the app's name prefix) but is not declared in nextdeploy.yml.
type UnmanagedResource struct {
	Kind string
	Name string
}

// scanUnmanaged lists the account's D1 databases, KV namespaces and queues,
// and returns those whose name matches the app prefix but is absent from the
// declared set.
//
// Deliberately prefix-scoped rather than account-wide: an account may host
// many apps, and reporting every unrelated resource as "unmanaged" would be
// noise that trains users to ignore the warning. The cost is that a resource
// named outside the convention is not reported — a miss is better than a
// false alarm here.
//
// Read-only, and every error is swallowed into the returned error rather than
// failing the plan: this is advisory, and a token without list permission on
// one product should not block planning the others.
func (p *CloudflareProvider) scanUnmanaged(ctx context.Context, cfg *config.NextDeployConfig) ([]UnmanagedResource, error) {
	prefix := unmanagedScanPrefix(cfg.App.Name)
	if prefix == "" {
		return nil, nil
	}

	// A nil resources block means nothing is declared, not that nothing
	// exists — scan against empty declared sets so a hand-provisioned account
	// still gets reported.
	var declaredD1, declaredKV, declaredQueues map[string]struct{}
	if res := cfg.Serverless.Cloudflare.Resources; res != nil {
		declaredD1 = declaredNames(len(res.D1), func(i int) string { return res.D1[i].Name })
		declaredKV = declaredNames(len(res.KV), func(i int) string { return res.KV[i].Name })
		declaredQueues = declaredNames(len(res.Queues), func(i int) string { return res.Queues[i].Name })
	}

	var (
		out  []UnmanagedResource
		errs []error
	)

	d1Iter := p.cf.D1.Database.ListAutoPaging(ctx, d1.DatabaseListParams{
		AccountID: cloudflare.F(p.accountID),
	})
	for d1Iter.Next() {
		name := d1Iter.Current().Name
		if isUnmanaged(name, prefix, declaredD1) {
			out = append(out, UnmanagedResource{Kind: "d1", Name: name})
		}
	}
	if err := ignoreNotFound(d1Iter.Err()); err != nil {
		errs = append(errs, fmt.Errorf("list d1: %w", err))
	}

	kvIter := p.cf.KV.Namespaces.ListAutoPaging(ctx, kv.NamespaceListParams{
		AccountID: cloudflare.F(p.accountID),
	})
	for kvIter.Next() {
		title := kvIter.Current().Title
		if isUnmanaged(title, prefix, declaredKV) {
			out = append(out, UnmanagedResource{Kind: "kv", Name: title})
		}
	}
	if err := ignoreNotFound(kvIter.Err()); err != nil {
		errs = append(errs, fmt.Errorf("list kv: %w", err))
	}

	qIter := p.cf.Queues.ListAutoPaging(ctx, queues.QueueListParams{
		AccountID: cloudflare.F(p.accountID),
	})
	for qIter.Next() {
		name := qIter.Current().QueueName
		if isUnmanaged(name, prefix, declaredQueues) {
			out = append(out, UnmanagedResource{Kind: "queue", Name: name})
		}
	}
	if err := ignoreNotFound(qIter.Err()); err != nil {
		errs = append(errs, fmt.Errorf("list queues: %w", err))
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Kind != out[j].Kind {
			return out[i].Kind < out[j].Kind
		}
		return out[i].Name < out[j].Name
	})
	return out, errors.Join(errs...)
}

// unmanagedScanPrefix is the naming convention the scan keys off: resources
// belonging to app "shop" are expected to be named "shop-*" or "shop". An
// empty app name disables the scan rather than matching everything.
func unmanagedScanPrefix(appName string) string {
	return strings.TrimSpace(strings.ToLower(appName))
}

func isUnmanaged(name, prefix string, declared map[string]struct{}) bool {
	if name == "" {
		return false
	}
	if _, ok := declared[name]; ok {
		return false
	}
	lower := strings.ToLower(name)
	return lower == prefix || strings.HasPrefix(lower, prefix+"-")
}

func declaredNames(n int, at func(int) string) map[string]struct{} {
	out := make(map[string]struct{}, n)
	for i := range n {
		if name := at(i); name != "" {
			out[name] = struct{}{}
		}
	}
	return out
}

func ignoreNotFound(err error) error {
	if err == nil {
		return nil
	}
	var apiErr *cloudflare.Error
	if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
		return nil
	}
	return err
}

// unmanagedWarnings renders the scan result as plan warning lines.
func unmanagedWarnings(found []UnmanagedResource) []string {
	if len(found) == 0 {
		return nil
	}
	out := []string{
		fmt.Sprintf("%d resource(s) on this account match the app name but are not declared "+
			"in nextdeploy.yml:", len(found)),
	}
	for _, r := range found {
		out = append(out, fmt.Sprintf("    %-6s %s", r.Kind, r.Name))
	}
	out = append(out,
		"  To manage one of these, declare it under cloudflare.resources.* with EXACTLY this",
		"  name — provisioning matches on name, so an existing resource is adopted, not recreated.",
		"  If you renamed it in nextdeploy.yml, apply will NOT rename it: it creates a new empty",
		"  resource and leaves this one orphaned (still billed, still holding its data). Rename",
		"  it back, or migrate the data before shipping.",
		"  If you dropped it on purpose, delete it in the Cloudflare dashboard — removing it from",
		"  the config does not delete it.")
	return out
}

// queueConsumerWarnings reports consumers whose source queue is not declared
// under resources.queues.
//
// Registration happens after the script upload (ensureQueueConsumer) and hard-
// fails there if the queue doesn't exist. Surfacing it at plan time turns a
// mid-deploy error into something you can fix before shipping.
func queueConsumerWarnings(cf *config.CloudflareConfig) []string {
	if cf == nil || cf.Bindings == nil || cf.Bindings.Queues == nil || cf.Resources == nil {
		return nil
	}
	declared := declaredNames(len(cf.Resources.Queues), func(i int) string {
		return cf.Resources.Queues[i].Name
	})
	var out []string
	for _, c := range cf.Bindings.Queues.Consumers {
		if c.Queue == "" {
			continue
		}
		if _, ok := declared[c.Queue]; !ok {
			out = append(out, fmt.Sprintf(
				"queue consumer targets %q, which is not declared under cloudflare.resources.queues "+
					"— the deploy will fail when it registers the consumer", c.Queue))
		}
	}
	return out
}
