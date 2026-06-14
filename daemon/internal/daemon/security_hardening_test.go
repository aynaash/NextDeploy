package daemon

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"
	"time"
)

func sign(payload, secret string) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(payload))
	return hex.EncodeToString(h.Sum(nil))
}

func TestVerifySignature_FailsClosedOnEmptySecret(t *testing.T) {
	// The pre-hardening bug: an empty secret returned true ("auth disabled").
	if VerifySignature([]byte("anything"), "", "") {
		t.Fatal("empty secret must NOT authenticate (must fail closed)")
	}
	if VerifySignature([]byte("anything"), sign("anything", ""), "") {
		t.Fatal("empty secret must fail closed regardless of signature")
	}
}

func TestVerifySignature_AcceptsValidRejectsInvalid(t *testing.T) {
	secret := "topsecret"
	payload := `{"type":"status"}`
	if !VerifySignature([]byte(payload), sign(payload, secret), secret) {
		t.Fatal("valid signature should verify")
	}
	if VerifySignature([]byte(payload), sign(payload, "wrong"), secret) {
		t.Fatal("signature under a different secret must be rejected")
	}
	if VerifySignature([]byte(payload), "deadbeef", secret) {
		t.Fatal("garbage signature must be rejected")
	}
}

func TestReplayGuard(t *testing.T) {
	rg := NewReplayGuard(5 * time.Minute)
	now := time.Now().Unix()

	if err := rg.Check(now, "nonce-1"); err != nil {
		t.Fatalf("first use of a fresh nonce should pass: %v", err)
	}
	if err := rg.Check(now, "nonce-1"); err == nil {
		t.Fatal("replayed nonce must be rejected")
	}
	if err := rg.Check(now, "nonce-2"); err != nil {
		t.Fatalf("a different fresh nonce should pass: %v", err)
	}

	// Freshness and presence checks.
	if err := rg.Check(time.Now().Add(-10*time.Minute).Unix(), "old"); err == nil {
		t.Fatal("stale timestamp must be rejected")
	}
	if err := rg.Check(0, "x"); err == nil {
		t.Fatal("missing timestamp must be rejected")
	}
	if err := rg.Check(now, ""); err == nil {
		t.Fatal("missing nonce must be rejected")
	}
}
