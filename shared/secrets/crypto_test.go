package secrets

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"testing"
)

// roundTrip mirrors how the only production caller pairs Encrypt/Decrypt:
// Encrypt base64-encodes its output, so the bytes must be decoded before they
// are handed to Decrypt. See cli/internal/serverless/cfstate/cfstate.go.
func roundTrip(t *testing.T, pt, key []byte) []byte {
	t.Helper()
	enc, err := Encrypt(pt, key)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	raw, err := base64.StdEncoding.DecodeString(string(enc))
	if err != nil {
		t.Fatalf("base64 decode: %v", err)
	}
	got, err := Decrypt(raw, key)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	return got
}

func TestDeriveKey(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		wantErr bool
	}{
		{"empty rejected", "", true},
		{"short key", "a", false},
		{"typical key", "correct horse battery staple", false},
		{"unicode key", "🔑pässwörd", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := DeriveKey(tt.in)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("DeriveKey(%q): want error, got nil", tt.in)
				}
				return
			}
			if err != nil {
				t.Fatalf("DeriveKey(%q): unexpected error %v", tt.in, err)
			}
			if len(got) != 32 {
				t.Fatalf("DeriveKey(%q): len=%d, want 32", tt.in, len(got))
			}
		})
	}
}

func TestDeriveKey_DeterministicAndDistinct(t *testing.T) {
	a1, _ := DeriveKey("x")
	a2, _ := DeriveKey("x")
	if !bytes.Equal(a1, a2) {
		t.Fatal("DeriveKey is not deterministic for the same input")
	}
	b, _ := DeriveKey("y")
	if bytes.Equal(a1, b) {
		t.Fatal("DeriveKey produced equal output for different inputs")
	}
}

func TestDeriveKey_KnownAnswer(t *testing.T) {
	got, err := DeriveKey("test")
	if err != nil {
		t.Fatalf("DeriveKey: %v", err)
	}
	want := sha256.Sum256([]byte("test"))
	if !bytes.Equal(got, want[:]) {
		t.Fatalf("DeriveKey(\"test\") != sha256(\"test\")")
	}
}

func TestEncryptDecrypt_RoundTrip(t *testing.T) {
	key, err := DeriveKey("round-trip-key")
	if err != nil {
		t.Fatalf("DeriveKey: %v", err)
	}
	tests := []struct {
		name string
		in   []byte
	}{
		{"empty", []byte{}},
		{"short", []byte("hi")},
		{"json blob", []byte(`{"a":1,"b":"x"}`)},
		{"binary", []byte{0x00, 0xFF, 0x10, 0x80}},
		{"large", bytes.Repeat([]byte("A"), 100_000)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := roundTrip(t, tt.in, key)
			if !bytes.Equal(got, tt.in) {
				t.Fatalf("round-trip mismatch: got %d bytes, want %d", len(got), len(tt.in))
			}
		})
	}
}

// TestEncryptDecrypt_Asymmetry pins the documented footgun: feeding Encrypt's
// base64 output straight into Decrypt (no decode) must fail. If someone makes
// the pair symmetric, this test should be updated deliberately.
func TestEncryptDecrypt_Asymmetry(t *testing.T) {
	key, _ := DeriveKey("asym")
	enc, err := Encrypt([]byte("payload"), key)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	if _, err := Decrypt(enc, key); !errors.Is(err, ErrDecryptFailed) {
		t.Fatalf("Decrypt(base64 bytes): want ErrDecryptFailed, got %v", err)
	}
}

func TestEncrypt_KeySizes(t *testing.T) {
	tests := []struct {
		name    string
		keyLen  int
		wantErr bool
	}{
		{"aes-128 ok", 16, false},
		{"aes-192 ok", 24, false},
		{"aes-256 ok", 32, false},
		{"bad 1 byte", 1, true},
		{"bad 31 bytes", 31, true},
		{"empty key", 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Encrypt([]byte("data"), make([]byte, tt.keyLen))
			if tt.wantErr {
				if !errors.Is(err, ErrEncryptFailed) {
					t.Fatalf("Encrypt with %d-byte key: want ErrEncryptFailed, got %v", tt.keyLen, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Encrypt with %d-byte key: unexpected error %v", tt.keyLen, err)
			}
		})
	}
}

func TestDecrypt_KeySizes(t *testing.T) {
	// A well-formed ciphertext made with a valid key; Decrypt should reject the
	// call on bad key sizes before it ever looks at the payload.
	key, _ := DeriveKey("k")
	enc, _ := Encrypt([]byte("data"), key)
	raw, _ := base64.StdEncoding.DecodeString(string(enc))

	for _, keyLen := range []int{1, 31, 0} {
		_, err := Decrypt(raw, make([]byte, keyLen))
		if !errors.Is(err, ErrDecryptFailed) {
			t.Fatalf("Decrypt with %d-byte key: want ErrDecryptFailed, got %v", keyLen, err)
		}
	}
}

func TestEncrypt_NonceUniqueness(t *testing.T) {
	key, _ := DeriveKey("nonce")
	pt := []byte("same plaintext")
	enc1, _ := Encrypt(pt, key)
	enc2, _ := Encrypt(pt, key)
	if bytes.Equal(enc1, enc2) {
		t.Fatal("Encrypt produced identical output for the same input (nonce reuse?)")
	}
	// Both must still decrypt back to the same plaintext.
	for _, enc := range [][]byte{enc1, enc2} {
		raw, _ := base64.StdEncoding.DecodeString(string(enc))
		got, err := Decrypt(raw, key)
		if err != nil || !bytes.Equal(got, pt) {
			t.Fatalf("decrypt of unique-nonce ciphertext failed: got %q err %v", got, err)
		}
	}
}

func TestDecrypt_Tampered(t *testing.T) {
	key, _ := DeriveKey("tamper")
	enc, _ := Encrypt([]byte("secret message"), key)
	valid, _ := base64.StdEncoding.DecodeString(string(enc))

	mutate := func(f func(b []byte) []byte) []byte {
		cp := append([]byte(nil), valid...)
		return f(cp)
	}

	tests := []struct {
		name string
		in   []byte
	}{
		{"flip payload byte", mutate(func(b []byte) []byte { b[len(b)-1] ^= 0xFF; return b })},
		{"flip nonce byte", mutate(func(b []byte) []byte { b[0] ^= 0xFF; return b })},
		{"truncated below nonce size", valid[:5]},
		{"empty", []byte{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := Decrypt(tt.in, key); !errors.Is(err, ErrDecryptFailed) {
				t.Fatalf("Decrypt(%s): want ErrDecryptFailed, got %v", tt.name, err)
			}
		})
	}

	// Wrong key on otherwise-valid ciphertext.
	t.Run("wrong key", func(t *testing.T) {
		other, _ := DeriveKey("a different key")
		if _, err := Decrypt(valid, other); !errors.Is(err, ErrDecryptFailed) {
			t.Fatalf("Decrypt with wrong key: want ErrDecryptFailed, got %v", err)
		}
	})
}
