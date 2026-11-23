package secrets

import (
	"fmt"
)

// MigrateToProvider exports all secrets into the named SecretProvider.
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
			decrypted, err := decryptSecretValue(secret, []byte(key))
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
