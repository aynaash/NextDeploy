package serverless

import (
	"crypto/sha256"
	"os"
	"path/filepath"
	"testing"

	"github.com/cloudflare/cloudflare-go/v6"
	"github.com/cloudflare/cloudflare-go/v6/workers"
)

// bundleSum mirrors how DeployCompute fingerprints the worker bundle (SHA-256
// of its bytes) so the tests feed computeDeployHash the same shape prod does.
func bundleSum(s string) [32]byte { return sha256.Sum256([]byte(s)) }

// computeDeployHash determinism — same inputs in any order produce the same
// digest. If this breaks, every redeploy will look "changed" and re-upload
// for no reason, defeating the whole point.
func TestComputeDeployHash_Deterministic(t *testing.T) {
	bytes1 := bundleSum("worker bundle")
	meta1 := workers.ScriptUpdateParamsMetadata{
		MainModule: cloudflare.F("worker.mjs"),
	}
	secrets1 := map[string]string{
		"DATABASE_URL": "postgres://u:p@h/db",
		"API_KEY":      "sk-live-xxx",
	}

	a := computeDeployHash(bytes1, meta1, secrets1)

	// Same secrets, different map iteration order. Go map iteration is
	// randomized so this isn't a synthetic test — re-running with a fresh
	// map literally does iterate in different orders.
	secrets2 := map[string]string{
		"API_KEY":      "sk-live-xxx",
		"DATABASE_URL": "postgres://u:p@h/db",
	}
	b := computeDeployHash(bytes1, meta1, secrets2)

	if a != b {
		t.Fatalf("hash not deterministic across map ordering:\n  a=%s\n  b=%s", a, b)
	}
}

// computeDeployHash sensitivity — the three inputs must each independently
// flip the hash, otherwise we'd skip uploads when we shouldn't.
func TestComputeDeployHash_DetectsEachInputChange(t *testing.T) {
	base := bundleSum("worker bundle")
	meta := workers.ScriptUpdateParamsMetadata{MainModule: cloudflare.F("worker.mjs")}
	secrets := map[string]string{"K": "v"}

	original := computeDeployHash(base, meta, secrets)

	bundleChanged := computeDeployHash(bundleSum("worker bundle v2"), meta, secrets)
	if bundleChanged == original {
		t.Error("bundle change didn't flip hash — would cause stale-code skips")
	}

	metaChanged := computeDeployHash(base, workers.ScriptUpdateParamsMetadata{
		MainModule: cloudflare.F("other.mjs"),
	}, secrets)
	if metaChanged == original {
		t.Error("metadata change didn't flip hash — would cause binding-drift skips")
	}

	secretsChanged := computeDeployHash(base, meta, map[string]string{"K": "v2"})
	if secretsChanged == original {
		t.Error("secret change didn't flip hash — would cause stale-secret skips")
	}

	secretAdded := computeDeployHash(base, meta, map[string]string{"K": "v", "X": "y"})
	if secretAdded == original {
		t.Error("adding a secret didn't flip hash")
	}

	secretRemoved := computeDeployHash(base, meta, map[string]string{})
	if secretRemoved == original {
		t.Error("removing all secrets didn't flip hash")
	}
}

// md5OfFile contract: identical bytes → identical digest, independent of
// file path. R2 ETag matching depends on this.
func TestMd5OfFile_StableAcrossPaths(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.bin")
	b := filepath.Join(dir, "subdir", "b.bin")
	if err := os.MkdirAll(filepath.Dir(b), 0o755); err != nil {
		t.Fatal(err)
	}

	content := []byte("the same bytes")
	if err := os.WriteFile(a, content, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(b, content, 0o644); err != nil {
		t.Fatal(err)
	}

	ha, err := md5OfFile(a)
	if err != nil {
		t.Fatal(err)
	}
	hb, err := md5OfFile(b)
	if err != nil {
		t.Fatal(err)
	}
	if ha != hb {
		t.Errorf("identical bytes hashed differently: %s vs %s", ha, hb)
	}

	// Sanity: a known-content's md5 matches the documented value.
	// Empty file → md5("") = d41d8cd98f00b204e9800998ecf8427e
	empty := filepath.Join(dir, "empty.bin")
	if err := os.WriteFile(empty, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	hempty, err := md5OfFile(empty)
	if err != nil {
		t.Fatal(err)
	}
	if hempty != "d41d8cd98f00b204e9800998ecf8427e" {
		t.Errorf("empty-file md5 = %s, want d41d8cd98f00b204e9800998ecf8427e", hempty)
	}
}

func TestLooksLikeCloudflareAccountID(t *testing.T) {
	cases := map[string]bool{
		"a7a222ece3a9a56fa8e88a442f2ab46b":  true,  // real-shape (32 hex)
		"A7A222ECE3A9A56FA8E88A442F2AB46B":  false, // upper case rejected
		"YOUR_CLOUDFLARE_ACCOUNT_ID":        false, // placeholder
		"":                                  false,
		"a7a222ece3a9a56fa8e88a442f2ab46":   false, // 31 chars
		"a7a222ece3a9a56fa8e88a442f2ab46bb": false, // 33 chars
		"a7a222ece3a9a56fa8e88a442f2ab46g":  false, // non-hex char
	}
	for in, want := range cases {
		if got := looksLikeCloudflareAccountID(in); got != want {
			t.Errorf("looksLikeCloudflareAccountID(%q) = %v, want %v", in, got, want)
		}
	}
}
