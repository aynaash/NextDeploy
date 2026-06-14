package secrets

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/pbkdf2"
)

// Encrypted file layout: salt(16) || nonce(12) || AES-256-GCM ciphertext.
// The key is derived from the password with PBKDF2-HMAC-SHA256. GCM is an
// authenticated cipher, so tampering is detected on decryption, and because
// encryption happens in-process the password never appears in any process's
// argument list (unlike the previous `openssl enc -pass pass:...` shell-out).
const (
	encSaltLen  = 16
	encNonceLen = 12
	encIter     = 600_000
	encKeyLen   = 32
)

func (sm *SecretManager) EncryptEnvFile(masterKey string) (map[string]string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get current working directory: %w", err)
	}

	files, err := filepath.Glob(filepath.Join(cwd, "*.env"))
	if err != nil {
		return nil, fmt.Errorf("failed to find .env files: %w", err)
	}

	if len(files) == 0 {
		SLogs.Warn("No .env files found in the current directory")
		return nil, nil
	}

	results := make(map[string]string)
	for _, file := range files {
		SLogs.Info("Encrypting .env file: %s", file)

		encryptedFile := file + ".enc"
		if err := sm.encryptToFile(file, encryptedFile, masterKey); err != nil {
			SLogs.Error("Failed to encrypt file %s: %v", file, err)
			return nil, fmt.Errorf("%w: %v", ErrEncryptionFailed, err)
		}

		results[file] = encryptedFile
	}

	return results, nil
}

func (sm *SecretManager) EncryptFile(filename string, key []byte) error {
	encryptedFilename := filename + ".enc"
	return sm.encryptToFile(filename, encryptedFilename, string(key))
}

// encryptToFile reads inputPath, encrypts it with AES-256-GCM (key derived from
// password via PBKDF2), and writes salt||nonce||ciphertext to outputPath (0600).
func (sm *SecretManager) encryptToFile(inputPath, outputPath, password string) error {
	// #nosec G304 — inputPath is an operator-selected local .env/config file.
	plaintext, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("read %s: %w", inputPath, err)
	}

	salt := make([]byte, encSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return fmt.Errorf("generate salt: %w", err)
	}
	gcm, err := newGCM(password, salt)
	if err != nil {
		return err
	}
	nonce := make([]byte, encNonceLen)
	if _, err := rand.Read(nonce); err != nil {
		return fmt.Errorf("generate nonce: %w", err)
	}

	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)

	out := make([]byte, 0, len(salt)+len(nonce)+len(ciphertext))
	out = append(out, salt...)
	out = append(out, nonce...)
	out = append(out, ciphertext...)

	if err := os.WriteFile(outputPath, out, 0600); err != nil {
		return fmt.Errorf("failed to write encrypted file: %w", err)
	}

	SLogs.Info("Successfully encrypted file %s to %s", inputPath, outputPath)
	return nil
}

// DecryptFile decrypts a .enc file produced by encryptToFile and returns the
// plaintext. A wrong key or any tampering surfaces as a decryption error
// (GCM authentication failure).
func (sm *SecretManager) DecryptFile(filename string, key []byte) (string, error) {
	if !strings.HasSuffix(filename, ".enc") {
		return "", fmt.Errorf("file %s is not an encrypted file", filename)
	}

	// #nosec G304 — filename is an operator-selected local encrypted file.
	data, err := os.ReadFile(filename)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", filename, err)
	}
	if len(data) < encSaltLen+encNonceLen {
		return "", fmt.Errorf("file %s is too short to be a valid encrypted file", filename)
	}

	salt := data[:encSaltLen]
	nonce := data[encSaltLen : encSaltLen+encNonceLen]
	ciphertext := data[encSaltLen+encNonceLen:]

	gcm, err := newGCM(string(key), salt)
	if err != nil {
		return "", err
	}
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt %s (wrong key or corrupted/tampered file): %w", filename, err)
	}

	SLogs.Info("Successfully decrypted file %s", filename)
	return string(plaintext), nil
}

// newGCM derives a 32-byte key from password+salt via PBKDF2-SHA256 and returns
// an AES-256-GCM AEAD.
func newGCM(password string, salt []byte) (cipher.AEAD, error) {
	key := pbkdf2.Key([]byte(password), salt, encIter, encKeyLen, sha256.New)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("aes cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("gcm: %w", err)
	}
	return gcm, nil
}
