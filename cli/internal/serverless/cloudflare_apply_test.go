package serverless

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/aynaash/nextdeploy/shared/config"
)

func cfWithKVResource(name string) *config.NextDeployConfig {
	return &config.NextDeployConfig{
		Serverless: &config.ServerlessConfig{
			Cloudflare: &config.CloudflareConfig{
				Resources: &config.CFResources{KV: []config.CFKVResource{{Name: name}}},
			},
		},
	}
}

func TestApply_NoResourcesIsEmptyNoop(t *testing.T) {
	p := newKVTestProvider(t, &kvMock{})
	plan, err := p.Apply(context.Background(), &config.NextDeployConfig{})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if len(plan.Items) != 0 {
		t.Errorf("expected empty plan, got %+v", plan.Items)
	}
}

func TestApply_CreatesMissingResource(t *testing.T) {
	m := &kvMock{existing: nil} // namespace absent → must be created
	p := newKVTestProvider(t, m)

	plan, err := p.Apply(context.Background(), cfWithKVResource("app-rate-limit"))
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	// Plan reported a create...
	if len(plan.Items) != 1 || plan.Items[0].Action != PlanCreate || plan.Items[0].Kind != "kv" {
		t.Fatalf("expected one kv create in plan, got %+v", plan.Items)
	}
	// ...and provisioning actually created it.
	if len(m.created) != 1 || m.created[0] != "app-rate-limit" {
		t.Errorf("resource not recreated: created=%v", m.created)
	}
	// id is stashed for ref resolution.
	if id, ok := p.provisioned.get("kv", "app-rate-limit"); !ok || id == "" {
		t.Errorf("provisioned id not stashed: %q ok=%v", id, ok)
	}
}

func TestApply_ExistingResourceIsNoop(t *testing.T) {
	m := &kvMock{existing: []map[string]string{{"id": "x", "title": "app-rate-limit"}}}
	p := newKVTestProvider(t, m)

	plan, err := p.Apply(context.Background(), cfWithKVResource("app-rate-limit"))
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if len(plan.Items) != 1 || plan.Items[0].Action != PlanNoOp {
		t.Errorf("expected no-op plan, got %+v", plan.Items)
	}
	if len(m.created) != 0 {
		t.Errorf("existing resource should not be recreated, created=%v", m.created)
	}
}

// Secret Store bindings must reference the store, never inline a value.
func TestBuildScriptMetadata_SecretsStoreBinding(t *testing.T) {
	cf := &config.CloudflareConfig{
		Bindings: &config.CFBindings{
			SecretsStore: []config.CFSecretStoreBinding{
				{Name: "STRIPE_KEY", StoreID: "store-1", SecretName: "stripe_live"},
			},
		},
	}
	meta, err := buildScriptMetadata(cf, "bucket", "worker.mjs", noResolver, nil)
	if err != nil {
		t.Fatalf("buildScriptMetadata: %v", err)
	}
	raw, _ := json.Marshal(meta)
	got := string(raw)
	for _, frag := range []string{
		`"name":"STRIPE_KEY"`, `"store_id":"store-1"`,
		`"secret_name":"stripe_live"`, `"type":"secrets_store_secret"`,
	} {
		if !strings.Contains(got, frag) {
			t.Errorf("secrets_store binding missing %q\n%s", frag, got)
		}
	}
}

func TestBuildScriptMetadata_SecretsStoreValidation(t *testing.T) {
	cf := &config.CloudflareConfig{
		Bindings: &config.CFBindings{
			SecretsStore: []config.CFSecretStoreBinding{{Name: "X", SecretName: "s"}}, // missing store_id
		},
	}
	if _, err := buildScriptMetadata(cf, "bucket", "worker.mjs", noResolver, nil); err == nil {
		t.Error("expected error for secrets_store binding missing store_id")
	}
}
