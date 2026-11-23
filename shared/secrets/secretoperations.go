package secrets

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"strings"
	"time"
)

const maxSecretNameLen = 256

var reservedSecretNames = map[string]bool{
	"PATH":                  true,
	"HOME":                  true,
	"USER":                  true,
	"LOGNAME":               true,
	"SHELL":                 true,
	"PWD":                   true,
	"OLDPWD":                true,
	"TERM":                  true,
	"HOSTNAME":              true,
	"HOST":                  true,
	"PORT":                  true,
	"IFS":                   true,
	"LD_PRELOAD":            true,
	"LD_LIBRARY_PATH":       true,
	"LD_AUDIT":              true,
	"DYLD_INSERT_LIBRARIES": true,
	"DYLD_LIBRARY_PATH":     true,
	"DYLD_FRAMEWORK_PATH":   true,
	"NODE_OPTIONS":          true,
}

var reservedSecretPrefixes = []string{"ND_", "NEXTDEPLOY_", "DOPPLER_"}

// ValidateSecretName enforces POSIX environment identifier rules and blocks
// reserved system, runtime, and internal NextDeploy secret keys.
func ValidateSecretName(name string) error {
	if name == "" || len(name) > maxSecretNameLen {
		return fmt.Errorf("%w: name length must be between 1 and %d", ErrInvalidSecretName, maxSecretNameLen)
	}
	if !isValidSecretIdentifier(name) {
		return fmt.Errorf("%w: %q must match [A-Za-z_][A-Za-z0-9_]*", ErrInvalidSecretName, name)
	}
	upper := strings.ToUpper(name)
	if reservedSecretNames[upper] {
		return fmt.Errorf("%w: %q", ErrReservedSecretName, name)
	}
	for _, prefix := range reservedSecretPrefixes {
		if strings.HasPrefix(upper, prefix) {
			return fmt.Errorf("%w: %q", ErrReservedSecretName, name)
		}
	}
	return nil
}

func isValidSecretIdentifier(name string) bool {
	for i := 0; i < len(name); i++ {
		c := name[i]
		isLetter := (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || c == '_'
		isDigit := c >= '0' && c <= '9'
		if i == 0 && !isLetter {
			return false
		}
		if !isLetter && !isDigit {
			return false
		}
	}
	return true
}

// ValidateSecret validates a secret's name and minimum value length.
func (sm *SecretManager) ValidateSecret(name, value string) error {
	if err := ValidateSecretName(name); err != nil {
		return err
	}
	if len(value) < 12 {
		return errors.New("secret must be at least 12 characters long")
	}

	return nil
}

func (sm *SecretManager) SecureCompare(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

func (sm *SecretManager) RotateSecrets() error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	newKey, err := sm.GeneratePlatformKey()
	if err != nil {
		return err
	}

	for name, secret := range sm.secrets {
		if !secret.IsEncrypted {
			continue
		}

		oldKey, err := sm.getCachedKey(name)
		if err != nil {
			SLogs.Warn("Failed to get cached key for %s: %v", name, err)
			continue
		}

		decrypted, err := decryptSecretValue(secret, []byte(oldKey))
		if err != nil {
			SLogs.Warn("Failed to decrypt secret %s: %v", name, err)
			continue
		}

		encVal, alg, salt, iv, err := encryptSecretValue(decrypted, []byte(newKey))
		if err != nil {
			SLogs.Warn("Failed to re-encrypt secret %s: %v", name, err)
			continue
		}

		secret.Value = encVal
		secret.Alg = alg
		secret.Salt = salt
		secret.IV = iv
		secret.Version++
		secret.ModifiedAt = time.Now().Unix()
		sm.keyCache[name] = []byte(newKey)
	}

	return nil
}
func (sm *SecretManager) PrepareSecretsContext() error {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if sm.cfg == nil {
		return fmt.Errorf("configuration is not set")
	}
	if sm.manager == nil {
		sm.manager = &providerManager{
			providers: make(map[string]SecretProvider),
		}
	}

	for _, provider := range sm.manager.providers {
		if err := provider.ValidateSecretFormat(""); err != nil {
			return fmt.Errorf("invalid secret format: %w", err)
		}
	}

	return nil
}

// SetSecret validates the secret name and stores a new secret, optionally
// encrypting it at rest with per-secret encryption metadata.
func (sm *SecretManager) SetSecret(name, value string, encrypt bool) error {
	if err := ValidateSecretName(name); err != nil {
		return err
	}

	sm.mu.Lock()
	defer sm.mu.Unlock()

	if _, exists := sm.secrets[name]; exists {
		return ErrSecretAlreadyExists
	}

	now := time.Now().Unix()
	secret := &Secret{
		Value:       value,
		Version:     1,
		CreatedAt:   now,
		ModifiedAt:  now,
		IsEncrypted: encrypt,
	}

	if encrypt {
		key, err := sm.GeneratePlatformKey()
		if err != nil {
			return err
		}
		encVal, alg, salt, iv, err := encryptSecretValue([]byte(value), []byte(key))
		if err != nil {
			return err
		}
		secret.Value = encVal
		secret.Alg = alg
		secret.Salt = salt
		secret.IV = iv
	}

	sm.secrets[name] = secret
	return nil
}

// GetSecret retrieves and decrypts a secret by name.
func (sm *SecretManager) GetSecret(name string) (string, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	secret, exists := sm.secrets[name]
	if !exists {
		if sm.manager != nil {
			for _, provider := range sm.manager.providers {
				if value, err := provider.GetSecret(name); err == nil {
					return value, nil
				}
			}
		}
		return "", ErrSecretNotFound
	}

	if !secret.IsEncrypted {
		return secret.Value, nil
	}

	key, err := sm.GeneratePlatformKey()
	if err != nil {
		return "", err
	}

	decrypted, err := decryptSecretValue(secret, []byte(key))
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrDecryptionFailed, err)
	}

	return string(decrypted), nil
}

func (sm *SecretManager) ImportSecrets(filePath string) error {
	// #nosec G304
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read import file: %w", err)
	}

	var secrets map[string]*Secret
	if err := json.Unmarshal(data, &secrets); err != nil {
		return fmt.Errorf("failed to parse secrets: %w", err)
	}

	sm.mu.Lock()
	defer sm.mu.Unlock()

	maps.Copy(sm.secrets, secrets)

	return nil
}

func (sm *SecretManager) ExportSecrets(filePath string) error {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	data, err := json.MarshalIndent(sm.secrets, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal secrets: %w", err)
	}

	if err := os.WriteFile(filePath, data, 0600); err != nil {
		return fmt.Errorf("failed to write secrets file: %w", err)
	}

	return nil
}

// FlattenSecrets returns all secrets as a plain map[string]string,
// decrypting encrypted values in-place. Suitable for cloud injection.
func (sm *SecretManager) FlattenSecrets() map[string]string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	result := make(map[string]string, len(sm.secrets))
	for name, secret := range sm.secrets {
		if !secret.IsEncrypted {
			result[name] = secret.Value
			continue
		}

		key, err := sm.GeneratePlatformKey()
		if err != nil {
			SLogs.Warn("Skipping secret %s — failed to derive key: %v", name, err)
			continue
		}

		decrypted, err := decryptSecretValue(secret, []byte(key))
		if err != nil {
			SLogs.Warn("Skipping secret %s — decryption failed: %v", name, err)
			continue
		}

		result[name] = string(decrypted)
	}

	return result
}
