package secrets

import (
	"encoding/base64"
	"fmt"
)

func (sm *SecretManager) MigrateToProvider(providerName string) error {
	if sm.manager == nil {
		return ErrProviderNotConfigured
	}
	provider, exists := sm.manager.providers[providerName]
	if !exists {
		return fmt.Errorf("%w: %s", ErrInvalidProvider, providerName)
	}
	sm.mu.Lock()
	defer sm.mu.Unlock()
	for name, secret := range sm.secrets {
		value := secret.Value
		if secret.IsEncrypted {
			key, err := sm.getCachedKey(name)
			if err != nil {
				SLogs.Warn("Failed to get key for %s during migration: %v", name, err)
				continue
			}
			var cipherBytes []byte
			if decoded, err := base64.StdEncoding.DecodeString(value); err == nil {
				cipherBytes = decoded
			} else {
				cipherBytes = []byte(value)
			}
			decrypted, err := Decrypt(cipherBytes, []byte(key))
			if err != nil {
				SLogs.Warn("Failed to decrypt %s during migration: %v", name, err)
				continue
			}
			value = string(decrypted)
		}
		if err := provider.SetSecret(name, value); err != nil {
			return fmt.Errorf("failed to migrate secret %s: %w", name, err)
		}
	}

	return nil
}
