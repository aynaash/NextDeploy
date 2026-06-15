package telemetry

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// isolateConfig points os.UserConfigDir at a temp dir (via XDG_CONFIG_HOME on
// Linux) and clears the opt-out env, so each test starts from a clean,
// enabled-by-default state without touching the real user config.
func isolateConfig(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("DO_NOT_TRACK", "")
	t.Setenv("NEXTDEPLOY_TELEMETRY", "")
	return filepath.Join(dir, "nextdeploy")
}

func TestEnabled_DefaultOptOut(t *testing.T) {
	isolateConfig(t)
	if !Enabled() {
		t.Error("telemetry should be ON by default (opt-out model)")
	}
}

func TestEnabled_RespectsEnv(t *testing.T) {
	isolateConfig(t)

	t.Setenv("DO_NOT_TRACK", "1")
	if Enabled() {
		t.Error("DO_NOT_TRACK=1 must disable telemetry")
	}
	t.Setenv("DO_NOT_TRACK", "")

	for _, v := range []string{"0", "false", "off", "no"} {
		t.Setenv("NEXTDEPLOY_TELEMETRY", v)
		if Enabled() {
			t.Errorf("NEXTDEPLOY_TELEMETRY=%s must disable telemetry", v)
		}
	}
}

func TestDisableEnable_Roundtrip(t *testing.T) {
	isolateConfig(t)

	if err := Disable(); err != nil {
		t.Fatalf("Disable: %v", err)
	}
	if Enabled() {
		t.Error("Enabled() should be false after Disable()")
	}
	if err := Enable(); err != nil {
		t.Fatalf("Enable: %v", err)
	}
	if !Enabled() {
		t.Error("Enabled() should be true after Enable()")
	}
	if err := Enable(); err != nil {
		t.Errorf("Enable() on already-enabled should be a no-op, got %v", err)
	}
}

func TestNormalizeTarget(t *testing.T) {
	cases := map[string]string{
		"cloudflare": "cloudflare",
		"CF":         "cloudflare",
		"aws":        "aws",
		"lambda":     "aws",
		"vps":        "vps",
		"weird":      "other",
		"":           "other",
	}
	for in, want := range cases {
		if got := normalizeTarget(in); got != want {
			t.Errorf("normalizeTarget(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestInstallID_StableAndAnonymous(t *testing.T) {
	isolateConfig(t)
	a := installID()
	b := installID()
	if a != b {
		t.Errorf("installID not stable across calls: %q vs %q", a, b)
	}
	if len(a) != 32 { // 16 random bytes hex-encoded
		t.Errorf("installID = %q, want 32 hex chars", a)
	}
}

func TestRecordShipSuccess_SendsAnonymousPayload(t *testing.T) {
	isolateConfig(t)

	var got Event
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &got)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	t.Setenv("NEXTDEPLOY_TELEMETRY_URL", srv.URL)

	RecordShipSuccess("cloudflare", "v0.12.2")

	if hits != 1 {
		t.Fatalf("expected exactly 1 telemetry request, got %d", hits)
	}
	if got.Event != "ship.success" || got.Target != "cloudflare" || got.Version != "v0.12.2" {
		t.Errorf("unexpected payload: %+v", got)
	}
	if got.ID == "" || got.TS == 0 || got.Nonce == "" {
		t.Errorf("payload missing id/ts/nonce: %+v", got)
	}

	// Anonymity guard: the raw JSON must not leak anything path/project-like.
	raw, _ := json.Marshal(got)
	for _, banned := range []string{"/", "home", "nextdeployfrontend"} {
		if strings.Contains(string(raw), banned) {
			t.Errorf("payload appears to contain %q: %s", banned, raw)
		}
	}
}

func TestRecordShipSuccess_SkipsWhenDisabled(t *testing.T) {
	cfgDir := isolateConfig(t)
	t.Setenv("DO_NOT_TRACK", "1")

	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits++
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	t.Setenv("NEXTDEPLOY_TELEMETRY_URL", srv.URL)

	RecordShipSuccess("cloudflare", "v0.12.2")
	if hits != 0 {
		t.Errorf("disabled telemetry must send nothing, got %d requests", hits)
	}
	if _, err := os.Stat(filepath.Join(cfgDir, idFile)); err == nil {
		t.Error("disabled telemetry should not create an install id")
	}
}

// A signed event must carry an Ed25519 signature the server can verify against
// the matching public key over the canonical message — this is what lets the
// backend reject forged "fake deploy" posts from clients without the key.
func TestSign_VerifiableWithPublicKey(t *testing.T) {
	isolateConfig(t)

	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	orig := signingKeyB64
	signingKeyB64 = base64.StdEncoding.EncodeToString(priv.Seed())
	t.Cleanup(func() { signingKeyB64 = orig })

	var sigHeader string
	var got Event
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sigHeader = r.Header.Get("X-NextDeploy-Signature")
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &got)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	t.Setenv("NEXTDEPLOY_TELEMETRY_URL", srv.URL)

	RecordShipSuccess("aws", "v0.12.2")

	const prefix = "ed25519="
	if !strings.HasPrefix(sigHeader, prefix) {
		t.Fatalf("missing/bad signature header: %q", sigHeader)
	}
	sig, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(sigHeader, prefix))
	if err != nil {
		t.Fatalf("signature not base64: %v", err)
	}
	if !ed25519.Verify(pub, []byte(canonical(&got)), sig) {
		t.Error("signature did not verify against the public key — server would reject a genuine event")
	}

	// Tamper check: a forged target must invalidate the signature.
	forged := got
	forged.Target = "vps"
	if ed25519.Verify(pub, []byte(canonical(&forged)), sig) {
		t.Error("signature verified after tampering with target — integrity not protected")
	}
}

// Without a build-injected key, no signature is attached (so the server can
// treat unsigned events as untrusted).
func TestSign_NoKeyMeansNoSignature(t *testing.T) {
	orig := signingKeyB64
	signingKeyB64 = ""
	t.Cleanup(func() { signingKeyB64 = orig })

	if sig := sign(&Event{ID: "x", Event: "ship.success"}); sig != "" {
		t.Errorf("expected empty signature without a key, got %q", sig)
	}
}
