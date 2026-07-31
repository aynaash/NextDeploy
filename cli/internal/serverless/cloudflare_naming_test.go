package serverless

import (
	"strings"
	"testing"
)

// sanitizeCFName must produce names R2 and the Workers API accept: lowercase,
// [a-z0-9-] only, no leading/trailing/repeated hyphens, ≤63 chars. The first
// case is the real failure — app "NextDeploy" yielded the invalid bucket
// "nextdeploy-NextDeploy-production-assets" (Cloudflare error 10005).
func TestSanitizeCFName(t *testing.T) {
	cases := map[string]string{
		"nextdeploy-NextDeploy-production-assets": "nextdeploy-nextdeploy-production-assets",
		"NextDeploy-production":                   "nextdeploy-production",
		"pesastream-assets":                       "pesastream-assets", // already valid → unchanged
		"My App!":                                 "my-app",
		"a__b  c":                                 "a-b-c", // collapse runs of invalid chars
		"--leading-and-trailing--":                "leading-and-trailing",
		"UPPER_Snake_Case":                        "upper-snake-case",
	}
	for in, want := range cases {
		if got := sanitizeCFName(in); got != want {
			t.Errorf("sanitizeCFName(%q) = %q, want %q", in, got, want)
		}
	}
}

// The output must never exceed R2's 63-char limit and must not end on a hyphen
// after truncation.
func TestSanitizeCFName_LengthCap(t *testing.T) {
	got := sanitizeCFName(strings.Repeat("a", 80))
	if len(got) != 63 {
		t.Errorf("len = %d, want 63", len(got))
	}
	if strings.HasPrefix(got, "-") || strings.HasSuffix(got, "-") {
		t.Errorf("result has a boundary hyphen: %q", got)
	}
}

func TestCloudflareWorkerName_Environment(t *testing.T) {
	cases := map[string]string{
		"":        "myapp-production",
		"pr-42":   "myapp-pr-42",
		"staging": "myapp-staging",
	}
	for env, want := range cases {
		if got := CloudflareWorkerName("myapp", env); got != want {
			t.Errorf("env %q: got %q, want %q", env, got, want)
		}
	}
}

// The environment is the namespace: a preview and production must never
// resolve to the same Worker or the same asset bucket.
func TestCloudflareNames_EnvironmentsAreDisjoint(t *testing.T) {
	prodWorker := CloudflareWorkerName("myapp", "production")
	prWorker := CloudflareWorkerName("myapp", "pr-42")
	if prodWorker == prWorker {
		t.Fatalf("preview and production share a Worker name: %q", prodWorker)
	}
	prodBucket := CloudflareAssetBucketName("myapp", "production")
	prBucket := CloudflareAssetBucketName("myapp", "pr-42")
	if prodBucket == prBucket {
		t.Fatalf("preview and production share an asset bucket: %q", prodBucket)
	}
	if prodBucket != "nextdeploy-myapp-production-assets" {
		t.Errorf("bucket = %q", prodBucket)
	}
}

// The exported helpers must agree with what the provider actually deploys —
// the preview URL is composed from them.
func TestCloudflareWorkerName_MatchesProvider(t *testing.T) {
	p := &CloudflareProvider{environment: "pr-7"}
	if got, want := p.workerName("myapp"), CloudflareWorkerName("myapp", "pr-7"); got != want {
		t.Errorf("provider workerName = %q, exported = %q", got, want)
	}
	if got, want := p.bucketNameFromApp("myapp"), CloudflareAssetBucketName("myapp", "pr-7"); got != want {
		t.Errorf("provider bucketName = %q, exported = %q", got, want)
	}
}
