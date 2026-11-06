package serverless

import (
	"bytes"
	"strings"
	"testing"

	"github.com/aynaash/nextdeploy/cli/internal/serverless/cfstate"
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
