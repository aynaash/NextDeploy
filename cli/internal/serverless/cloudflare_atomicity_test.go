package serverless

import (
	"strings"
	"testing"

	"github.com/aynaash/nextdeploy/internal/packaging"
)


func TestPartitionSecrets(t *testing.T) {
	assets := []packaging.S3Asset{
		{S3Key:"index.html", CacheControl:"public, max-age=0, must-revalidate"},
		{S3Key:"_next/static/chunks/main-abcd12.js", CacheControl: "public, max-age=3153600, immutable"},
		{S3Key: "about.rsc", CacheControl:"public, max-age=0, must-revalidate"},
		{S3Key:"_next/static/css/app-def456.css", CacheControl: "public, max-age=3153600, immutable"},
		{S3Key: "favicon.ico", CacheControl: "public, max-age=3600"},
	}
	immutable, mutable := partitionAssets(assets)

	if len(immutable) != 2 {
		t.Fatalf("expected 2 immutables assets got %d:%v", len(immutable), immutable)
	}

	for _, a := range immutable {
		if !strings.HasPrefix(a.S3Key, "_next/static/"){
			t.Errorf("Immutable partition contains non-hashed key %q", a.S3Key)
		}
	}

	if len(mutable) != 3 {
		t.Fatalf("expected 3 mutable assets got %d:%v", len(mutable), mutable)
		}
		for _, a := range mutable {
		if strings.HasPrefix(a.S3Key, "_next/static/"){
			t.Errorf("Mutable partition contains hashed key %q", a.S3Key)
		}
	}
}

func TestPartitionAssets_ImmutableSeperatedFromMutable(t *testing.T){
	assets := []packaging.S3Asset{
		{S3Key: "index.html", CacheControl: "public, max-age=0, must-revalidate"},
		{S3Key: "_next/static/chunks/main-abc123.js", CacheControl: "public, max-age=31536000, immutable"},
		{S3Key: "about.rsc", CacheControl: "public, max-age=0, must-revalidate"},
		{S3Key: "_next/static/css/app-def456.css", CacheControl: "public, max-age=31536000, immutable"},
		{S3Key: "favicon.ico", CacheControl: "public, max-age=3600"},
	}

	immutable, mutable := partitionAssets(assets)


	if len(immutable) != 2 {
		t.Fatalf("expected 2 immutable assets, got %d: %+v", len(immutable), immutable)
	}

	for _, a := range immutable {
		if !strings.HasPrefix(a.S3Key, "_next/static/"){
			t.Errorf("immutable partitions contains non-hashed key %q", a.S3Key)
		}
	}

	if len(mutable) != 3 {
		t.Fatalf("expected 3 mutable assets, got %d: %+v", len(mutable), mutable)
	}
	for _, a := range mutable {
		if strings.HasPrefix(a.S3Key, "_next/static/"){
			t.Errorf("mutable partition contains hashed key %q (reintroduces the race)", a.S3Key)
		}
	}
}