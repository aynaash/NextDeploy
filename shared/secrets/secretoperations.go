package secrets

import (
	"crypto/subtle"
	"encoding/json"

	"errors"
	"fmt"
	"os"
	"time"
)

func (sm *SecretManager) ValidateSecret(name, value string) error {
	if len(value) < 12 {
		return errors.New("secret must be at least 12 characters long")
	}

	// Add more validation rules as needed
	return nil
}

// SecureCompare performs constant-time comparison of secrets
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

		// Decrypt with old key
		oldKey, err := sm.getCachedKey(name)
		if err != nil {
			SLogs.Warn("Failed to get cached key for %s: %v", name, err)
			continue
		}

		decrypted, err := Decrypt([]byte(secret.Value), []byte(oldKey))
		if err != nil {
			SLogs.Warn("Failed to decrypt secret %s: %v", name, err)
			continue
		}

		// Encrypt with new key
		encrypted, err := Encrypt(decrypted, []byte(newKey))
		if err != nil {
			SLogs.Warn("Failed to re-encrypt secret %s: %v", name, err)
			continue
		}

		secret.Value = string(encrypted)
		secret.Version++
		secret.ModifiedAt = time.Now().Unix()
		sm.keyCache[name] = []byte(newKey)
	}

	return nil
}
func (sm *SecretManager) PrepareSecretsContext() error {
	sm.mu.Lock()
	//ensure config is load properly on sm
	defer sm.mu.Unlock()
	if sm.cfg == nil {
		return fmt.Errorf("configuration is not set")
	}
	// ensure manager is initialized
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

func (sm *SecretManager) SetSecret(name, value string, encrypt bool) error {
	// TODO: ❗ Enforce naming rules, avoid reserved/system keys.
	// TODO: 🔐 Store encryption metadata (alg version, salt, iv) with each secret.

	sm.mu.Lock()
	defer sm.mu.Unlock()

	if _, exists := sm.secrets[name]; exists {
		return ErrSecretAlreadyExists
	}

	secret := &Secret{
		Value:       value,
		Version:     1,
		CreatedAt:   time.Now().Unix(),
		ModifiedAt:  time.Now().Unix(),
		IsEncrypted: encrypt,
	}

	if encrypt {
		key, err := sm.GeneratePlatformKey()
		if err != nil {
			return err
		}
		encrypted, err := Encrypt([]byte(value), []byte(key))
		if err != nil {
			return err
		}
		secret.Value = string(encrypted)
	}

	sm.secrets[name] = secret
	return nil
}

// GetSecret retrieves a secret, decrypting if necessary
func (sm *SecretManager) GetSecret(name string) (string, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	secret, exists := sm.secrets[name]
	if !exists {
		// Try external providers
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
	SLogs.Debug("The key from get secret is: %s", key)
	if err != nil {
		return "", err
	}

	decrypted, err := Decrypt([]byte(secret.Value), []byte(key))
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrDecryptionFailed, err)
	}

	return string(decrypted), nil
}

// ImportSecrets imports secrets from a JSON file
func (sm *SecretManager) ImportSecrets(filePath string) error {
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

	for name, secret := range secrets {
		sm.secrets[name] = secret
	}

	return nil
}

// ExportSecrets exports secrets to a JSON file
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
