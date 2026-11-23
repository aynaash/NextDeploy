package secrets

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
)

func Encrypt(plaintext []byte, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, ErrEncryptFailed
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, ErrEncryptFailed
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, ErrEncryptFailed
	}
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return []byte(base64.StdEncoding.EncodeToString(ciphertext)), nil
}

func Decrypt(ciphertext []byte, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, ErrDecryptFailed
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, ErrDecryptFailed
	}
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, ErrDecryptFailed
	}
	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, ErrDecryptFailed
	}
	return plaintext, nil
}

// DeriveKey creates a SHA-256 hash of the provided key.
func DeriveKey(key string) ([]byte, error) {
	if key == "" {
		return nil, errors.New("key cannot be empty")
	}
	hash := sha256.Sum256([]byte(key))
	return hash[:], nil
}

const (
	secretSaltLen = 16
	secretIVLen   = 12
)

func deriveSaltedKey(baseKey, salt []byte) []byte {
	h := sha256.New()
	h.Write(baseKey)
	h.Write(salt)
	return h.Sum(nil)
}

func decodeSecretCiphertext(encoded string) []byte {
	if decoded, err := base64.StdEncoding.DecodeString(encoded); err == nil {
		return decoded
	}
	return []byte(encoded)
}

func encryptSecretValue(plaintext, baseKey []byte) (cipherB64, alg, saltB64, ivB64 string, err error) {
	salt := make([]byte, secretSaltLen)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return "", "", "", "", ErrEncryptFailed
	}
	derivedKey := deriveSaltedKey(baseKey, salt)
	encrypted, err := Encrypt(plaintext, derivedKey)
	if err != nil {
		return "", "", "", "", err
	}
	raw, err := base64.StdEncoding.DecodeString(string(encrypted))
	if err != nil || len(raw) < secretIVLen {
		return "", "", "", "", ErrEncryptFailed
	}
	return string(encrypted), CurrentEncryptionAlg,
		base64.StdEncoding.EncodeToString(salt),
		base64.StdEncoding.EncodeToString(raw[:secretIVLen]), nil
}

func decryptSecretValue(secret *Secret, baseKey []byte) ([]byte, error) {
	if secret.Alg != "" && secret.Alg != CurrentEncryptionAlg {
		return nil, fmt.Errorf("%w: unsupported algorithm %q", ErrDecryptFailed, secret.Alg)
	}
	cipherBytes := decodeSecretCiphertext(secret.Value)
	key := baseKey
	if secret.Salt != "" {
		salt, err := base64.StdEncoding.DecodeString(secret.Salt)
		if err != nil {
			return nil, ErrDecryptFailed
		}
		key = deriveSaltedKey(baseKey, salt)
	}
	if secret.IV != "" {
		iv, err := base64.StdEncoding.DecodeString(secret.IV)
		if err != nil || len(iv) == 0 || len(cipherBytes) < len(iv) || !bytes.Equal(cipherBytes[:len(iv)], iv) {
			return nil, ErrDecryptFailed
		}
	}
	return Decrypt(cipherBytes, key)
}
