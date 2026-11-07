package serverless

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aynaash/nextdeploy/cli/internal/serverless/cfstate"
	"github.com/aynaash/nextdeploy/shared"
	"github.com/aynaash/nextdeploy/shared/config"
	"github.com/aynaash/nextdeploy/shared/secrets"
)

func TestMergeResourceManifests(t *testing.T) {
	local := cfstate.New()
	local.Set("d1", "db", "local-new", "2026-11-06T10:00:00Z")
	local.Set("kv", "cache", "local-only", "2026-11-06T10:00:00Z")
	remote := cfstate.New()
	remote.Set("d1", "db", "remote-old", "2026-11-06T09:00:00Z")
	remote.Set("queue", "jobs", "remote-only", "2026-11-06T09:00:00Z")

	merged := mergeResourceManifests(local, remote)
	if record, ok := merged.Get("d1", "db"); !ok || record.ID != "local-new" {
		t.Fatalf("newer local record was not selected: %+v, %v", record, ok)
	}
	if _, ok := merged.Get("kv", "cache"); !ok {
		t.Fatal("local-only record was lost")
	}
	if _, ok := merged.Get("queue", "jobs"); !ok {
		t.Fatal("remote-only record was lost")
	}
}

func TestMergeResourceManifestsComparesFractionalTimestamps(t *testing.T) {
	local := cfstate.New()
	local.Set("d1", "db", "later", "2026-11-06T10:00:00.100Z")
	remote := cfstate.New()
	remote.Set("d1", "db", "earlier", "2026-11-06T10:00:00Z")

	merged := mergeResourceManifests(local, remote)
	if record, _ := merged.Get("d1", "db"); record.ID != "later" {
		t.Fatalf("fractional timestamp comparison selected %q", record.ID)
	}
}

func TestCFStateObjectKeyIsAppAndEnvironmentScoped(t *testing.T) {
	provider := &CloudflareProvider{environment: "preview"}
	cfg := &config.NextDeployConfig{}
	cfg.App.Name = "Shop"
	if got, want := provider.cfStateObjectKey(cfg), "_nextdeploy/state/shop/cf-state.preview.json.enc"; got != want {
		t.Fatalf("cfStateObjectKey() = %q, want %q", got, want)
	}
}

func TestCFStateEncryptionKeyUsesSharedSecret(t *testing.T) {
	sharedKey := strings.Repeat("x", 32)
	t.Setenv(cfStateKeyEnv, sharedKey)
	provider := &CloudflareProvider{apiToken: "account-token"}

	got, err := provider.cfStateEncryptionKey()
	if err != nil {
		t.Fatal(err)
	}
	want, err := secrets.DeriveKey(sharedKey)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatal("state key was not derived from the configured shared secret")
	}
}

func TestCFStateEncryptionKeyRequiresStrongSharedSecret(t *testing.T) {
	t.Setenv(cfStateKeyEnv, "too-short")
	provider := &CloudflareProvider{apiToken: "account-token"}
	if _, err := provider.cfStateEncryptionKey(); err == nil {
		t.Fatal("expected an error for a short shared state key")
	}
}

func TestRemoteStateIsOptIn(t *testing.T) {
	t.Setenv(cfStateKeyEnv, "")
	if (&CloudflareProvider{}).remoteStateEnabled() {
		t.Fatal("remote state should stay disabled without its shared secret")
	}
}

func TestLoadResourceManifest_InvalidatesCacheOnTokenRotationWithoutRemoteKey(t *testing.T) {
	t.Setenv(cfStateKeyEnv, "")
	oldToken := "old-token-from-doppler"
	oldKey, err := secrets.DeriveKey(oldToken)
	if err != nil {
		t.Fatal(err)
	}

	stateDir := t.TempDir()
	statePath := filepath.Join(stateDir, "cf-state.json.enc")

	m := cfstate.New()
	m.Set("d1", "app-db", "old-uuid", "2026-11-06T10:00:00Z")
	if err := cfstate.Save(statePath, m, oldKey); err != nil {
		t.Fatal(err)
	}

	// Provider initialized with rotated token
	newToken := "new-rotated-token"
	newKey, err := secrets.DeriveKey(newToken)
	if err != nil {
		t.Fatal(err)
	}

	p := &CloudflareProvider{
		log:      shared.PackageLogger("test", ""),
		apiToken: newToken,
	}

	// Loading state with rotated token should not return an error, but invalidate cache
	got, err := p.loadResourceManifest(statePath, newKey)
	if err != nil {
		t.Fatalf("expected nil error on token rotation fallback, got: %v", err)
	}
	if len(got.Resources) != 0 {
		t.Fatalf("expected empty manifest after cache invalidation, got %d resources", len(got.Resources))
	}
}

func TestLoadResourceManifest_ErrorsWhenSharedSecretKeyMismatch(t *testing.T) {
	sharedKey := strings.Repeat("x", 32)
	t.Setenv(cfStateKeyEnv, sharedKey)

	otherKeyBytes, err := secrets.DeriveKey(strings.Repeat("y", 32))
	if err != nil {
		t.Fatal(err)
	}

	stateDir := t.TempDir()
	statePath := filepath.Join(stateDir, "cf-state.json.enc")

	m := cfstate.New()
	m.Set("d1", "app-db", "uuid", "2026-11-06T10:00:00Z")
	if err := cfstate.Save(statePath, m, otherKeyBytes); err != nil {
		t.Fatal(err)
	}

	key, err := secrets.DeriveKey(sharedKey)
	if err != nil {
		t.Fatal(err)
	}

	p := &CloudflareProvider{
		log:      shared.PackageLogger("test", ""),
		apiToken: "some-token",
	}

	_, err = p.loadResourceManifest(statePath, key)
	if err == nil {
		t.Fatal("expected error when shared secret cannot decrypt state manifest")
	}
}

func TestRefResolverWithFallback_FindsLiveResourceWhenNotInManifest(t *testing.T) {
	m := &d1Mock{existingDBs: []map[string]string{
		{"uuid": "live-db-uuid", "name": "app-db"},
	}}
	p := newD1TestProvider(t, m)
	p.apiToken = "test-token"

	resolve := p.refResolverWithFallback()
	id, ok := resolve("d1", "app-db")
	if !ok {
		t.Fatal("expected ref to resolve live from Cloudflare")
	}
	if id != "live-db-uuid" {
		t.Fatalf("got id %q, want live-db-uuid", id)
	}
}
