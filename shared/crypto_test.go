package shared

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"testing"
)

func TestEd25519SignVerify(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey failed: %v", err)
	}

	if len(pub) != PublicKeySize {
		t.Fatalf("expected public key size %d, got %d", PublicKeySize, len(pub))
	}

	data := []byte("hello nextdeploy ed25519 verification")
	sig, err := Sign(data, priv)
	if err != nil {
		t.Fatalf("Sign failed: %v", err)
	}

	valid, err := Verify(data, sig, pub)
	if err != nil || !valid {
		t.Fatalf("Verify failed: valid=%v err=%v", valid, err)
	}
}

func TestSharedAESRoundTrip(t *testing.T) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("rand.Read failed: %v", err)
	}

	data := []byte("secret payload to test encryption")
	ciphertext, nonce, err := Encrypt(data, key)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	plaintext, err := Decrypt(ciphertext, key, nonce)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}

	if string(plaintext) != string(data) {
		t.Fatalf("roundtrip mismatch: got %q, want %q", plaintext, data)
	}
}

func TestECDHKeyExchange(t *testing.T) {
	kpA, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair A failed: %v", err)
	}
	kpB, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair B failed: %v", err)
	}

	sharedA, err := DeriveSharedKey(kpA.ECDHPrivate, kpB.ECDHPublic)
	if err != nil {
		t.Fatalf("DeriveSharedKey A->B failed: %v", err)
	}
	sharedB, err := DeriveSharedKey(kpB.ECDHPrivate, kpA.ECDHPublic)
	if err != nil {
		t.Fatalf("DeriveSharedKey B->A failed: %v", err)
	}
	if len(sharedA) != SharedKeySize || !bytes.Equal(sharedA, sharedB) {
		t.Fatalf("shared keys do not match or invalid length: len=%d", len(sharedA))
	}

	if _, err := DeriveSharedKey(nil, kpB.ECDHPublic); err == nil {
		t.Fatal("expected error for nil private key")
	}
	if _, err := DeriveSharedKey(kpA.ECDHPrivate, nil); err == nil {
		t.Fatal("expected error for nil public key")
	}

	if err := RunCryptoHealthChecks(); err != nil {
		t.Fatalf("RunCryptoHealthChecks failed: %v", err)
	}
}
