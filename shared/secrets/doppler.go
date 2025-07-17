package secrets

func (sm *SecretManager) IsDopplerEnabled() bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	if sm.manager == nil {
		return false
	}

	_, exists := sm.manager.providers["doppler"]
	return exists
}
func (sm *SecretManager) GetDopplerProvider() (SecretProvider, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	if sm.manager == nil {
		return nil, false
	}

	provider, exists := sm.manager.providers["doppler"]
	return provider, exists
}
