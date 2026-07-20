package config

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func sampleConfig() *NextDeployConfig {
	return &NextDeployConfig{
		Version:    "1",
		TargetType: "serverless",
		App: AppConfig{
			Name:               "demo",
			Port:               3000,
			Environment:        "production",
			DeletionProtection: true,
		},
	}
}

func TestSave_WritesAndReloads(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, ConfigFile)
	cfg := sampleConfig()

	if err := Save(cfg, p); err != nil {
		t.Fatalf("Save: %v", err)
	}

	info, err := os.Stat(p)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("file mode = %o, want 0600", perm)
	}

	t.Chdir(dir)
	got, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !reflect.DeepEqual(got, cfg) {
		t.Fatalf("round-trip mismatch:\n got %+v\nwant %+v", got, cfg)
	}
}

func TestLoad_Errors(t *testing.T) {
	t.Run("missing file", func(t *testing.T) {
		t.Chdir(t.TempDir())
		cfg, err := Load()
		if err == nil {
			t.Fatal("want error for missing file")
		}
		if !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("want os.ErrNotExist, got %v", err)
		}
		if cfg != nil {
			t.Fatalf("want nil cfg on error, got %+v", cfg)
		}
	})

	t.Run("invalid yaml", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, ConfigFile), []byte("::: not yaml :::"), 0o600); err != nil {
			t.Fatal(err)
		}
		t.Chdir(dir)
		cfg, err := Load()
		if err == nil {
			t.Fatal("want parse error for invalid yaml")
		}
		if cfg != nil {
			t.Fatalf("want nil cfg on parse error, got %+v", cfg)
		}
	})

	t.Run("empty file is a zero-value config", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, ConfigFile), []byte(""), 0o600); err != nil {
			t.Fatal(err)
		}
		t.Chdir(dir)
		cfg, err := Load()
		if err != nil {
			t.Fatalf("empty file: unexpected error %v", err)
		}
		if cfg == nil {
			t.Fatal("want non-nil cfg for empty doc")
		}
	})

	t.Run("valid minimal parses fields", func(t *testing.T) {
		dir := t.TempDir()
		yml := "version: \"1\"\napp:\n  name: demo\n  port: 3000\n"
		if err := os.WriteFile(filepath.Join(dir, ConfigFile), []byte(yml), 0o600); err != nil {
			t.Fatal(err)
		}
		t.Chdir(dir)
		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if cfg.App.Name != "demo" || cfg.App.Port != 3000 || cfg.Version != "1" {
			t.Fatalf("parsed fields wrong: %+v", cfg)
		}
	})
}

// LoadConfig duplicates Load; pin that they agree so an edit to one is caught.
func TestLoadConfig_MatchesLoad(t *testing.T) {
	dir := t.TempDir()
	if err := Save(sampleConfig(), filepath.Join(dir, ConfigFile)); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)

	a, errA := Load()
	b, errB := LoadConfig()
	if errA != nil || errB != nil {
		t.Fatalf("Load=%v LoadConfig=%v", errA, errB)
	}
	if !reflect.DeepEqual(a, b) {
		t.Fatalf("Load and LoadConfig disagree:\n%+v\n%+v", a, b)
	}
}

func TestSave_Errors(t *testing.T) {
	t.Run("unwritable path", func(t *testing.T) {
		dir := t.TempDir()
		// Make a regular file, then try to write "under" it as if it were a dir.
		blocker := filepath.Join(dir, "blocker")
		if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
		err := Save(sampleConfig(), filepath.Join(blocker, "nextdeploy.yml"))
		if err == nil {
			t.Fatal("want error writing under a file path")
		}
	})
}

// fullCloudflareConfig populates every Cloudflare binding, resource, and
// protection field so the round-trip test proves none are silently dropped by
// Save→Load. Each slice/map has at least one element and every scalar is
// non-zero, so `omitempty` never turns a set value into a reload-time nil.
func fullCloudflareConfig() *NextDeployConfig {
	enabled := true
	return &NextDeployConfig{
		Version:    "1",
		TargetType: "serverless",
		App: AppConfig{
			Name:        "cf-app",
			Port:        3000,
			Environment: "production",
		},
		Serverless: &ServerlessConfig{
			Provider: "cloudflare",
			Region:   "auto",
			Cloudflare: &CloudflareConfig{
				CompatibilityDate:  "2025-04-01",
				CompatibilityFlags: []string{"nodejs_compat_v2"},
				CustomDomains:      []CFCustomDomain{{Hostname: "app.example.com", ZoneID: "zone123"}},
				Routes:             []CFRoute{{Pattern: "*.example.com/*", Zone: "example.com"}},
				Triggers:           &CFTriggers{Crons: []string{"0 * * * *"}},
				Bindings: &CFBindings{
					R2:         []CFR2Binding{{Name: "ASSETS", Bucket: "my-bucket"}},
					D1:         []CFD1Binding{{Name: "DB", ID: "d1-uuid"}, {Name: "DB2", Ref: "maindb"}},
					Hyperdrive: []CFHyperdriveBinding{{Name: "HYPERDRIVE_DB", Ref: "pg"}},
					Queues: &CFQueueBindings{
						Producers: []CFQueueProducer{{Name: "QUEUE", Queue: "jobs"}},
						Consumers: []CFQueueConsumer{{Queue: "jobs", MaxRetries: 3, MaxBatchSize: 10, MaxBatchTimeout: 30, DeadLetterQueue: "jobs-dlq"}},
					},
					Vectorize:      []CFVectorizeBinding{{Name: "VEC", Index: "memory"}},
					AI:             []CFAIBinding{{Name: "AI", Gateway: &CFAIGatewayID{ID: "gw-slug"}}},
					DurableObjects: []CFDOBinding{{Name: "DO", ClassName: "Counter", Script: "self"}},
					KV:             []CFKVBinding{{Name: "RATE_LIMIT", NamespaceID: "kv-uuid"}, {Name: "CACHE", Ref: "cachekv"}},
					PlainText:      []CFPlainTextBinding{{Name: "STAGE", Value: "prod"}},
					SecretsStore:   []CFSecretStoreBinding{{Name: "API_KEY", StoreID: "store1", SecretName: "api_key"}},
				},
				Migrations: []CFMigration{{
					Tag:                "v1",
					NewSQLiteClasses:   []string{"Counter"},
					NewClasses:         []string{"Legacy"},
					DeletedClasses:     []string{"Old"},
					RenamedClasses:     []CFRenamedDO{{From: "A", To: "B"}},
					TransferredClasses: []CFTransferredDO{{From: "X", FromScript: "other", To: "Y"}},
				}},
				Resources: &CFResources{
					D1:           []CFD1Resource{{Name: "maindb", MigrationsDir: "drizzle", LocationHint: "weur"}},
					KV:           []CFKVResource{{Name: "cachekv"}},
					Hyperdrive:   []CFHyperdriveResource{{Name: "pg", Origin: "postgres://x", OriginEnv: "PG_URL"}},
					Queues:       []CFQueueResource{{Name: "jobs"}},
					Vectorize:    []CFVectorizeResource{{Name: "memory", Dimensions: 768, Metric: "cosine"}},
					AIGateway:    []CFAIGatewayResource{{Slug: "gw-slug"}},
					DNS:          []CFDNSRecord{{Zone: "example.com", Name: "@", Type: "A", Content: "1.2.3.4", TTL: 300, Proxied: true}},
					ZoneSettings: &CFZoneSettings{Zone: "example.com", MinTTL: 60},
				},
				Protection: &CFProtection{
					Enabled:     true,
					PublicPaths: []string{"/", "/login"},
					Auth:        &CFAuth{SecretEnv: "AUTH_SECRET", CookieName: "session", ProtectedPaths: []string{"/app/*"}, LoginPath: "/login"},
					RateLimit:   &CFRateLimit{KVBinding: "RATE_LIMIT", RequestsPerMinute: 60, Paths: []string{"/api/*"}},
					Allow:       []string{"10.0.0.1"},
					Deny:        []string{"10.0.0.2"},
				},
				Observability:   &CFObservability{Enabled: &enabled, HeadSamplingRate: 1},
				AllowSecretWipe: true,
			},
		},
	}
}

// TestSaveLoad_CloudflareBindingsRoundTrip is the deploy-critical guard: a fully
// populated Cloudflare config must survive Save→Load byte-for-byte. A dropped
// binding here means a Worker that can't reach its R2/D1/KV/secret at runtime.
func TestSaveLoad_CloudflareBindingsRoundTrip(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, ConfigFile)
	want := fullCloudflareConfig()

	if err := Save(want, p); err != nil {
		t.Fatalf("Save: %v", err)
	}
	t.Chdir(dir)
	got, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("cloudflare config round-trip mismatch:\n got %+v\nwant %+v", got, want)
	}
}

// SaveConfig has reversed arg order vs Save: (path, cfg). Lock it.
func TestSaveConfig_ArgOrder(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, ConfigFile)
	if err := SaveConfig(p, sampleConfig()); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}
	t.Chdir(dir)
	got, err := Load()
	if err != nil {
		t.Fatalf("Load after SaveConfig: %v", err)
	}
	if got.App.Name != "demo" {
		t.Fatalf("SaveConfig round-trip wrong: %+v", got)
	}
}
