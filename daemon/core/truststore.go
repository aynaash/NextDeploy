package core

import (
	"encoding/json"
	"os"
	"sync"

	"nextdeploy/shared"
)

type TrustStoreManager struct {
	path       string
	trustStore *shared.TrustStore
	mu         sync.RWMutex
}

func NewTrustStoreManager(path string) *TrustStoreManager {
	tsm := &TrustStoreManager{
		path:       path,
		trustStore: &shared.TrustStore{},
	}
	if err := tsm.load(); err != nil {
		shared.SharedLogger.Error("Failed to load trust store: %v", err)
	}

	return tsm
}

func (tsm *TrustStoreManager) load() error {
	tsm.mu.Lock()
	defer tsm.mu.Unlock()

	file, err := os.Open(tsm.path)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist, initialize an empty trust store
			tsm.trustStore = &shared.TrustStore{
				Identities: []shared.Identity{},
			}
			return nil
		}
		return err // Other errors
	}

	defer file.Close()
	return json.NewDecoder(file).Decode(tsm.trustStore)

}

func (tsm *TrustStoreManager) Save() error {
	tsm.mu.Lock()
	defer tsm.mu.Unlock()

	// Create temp file first
	tmpPath := tsm.path + ".tmp"
	file, err := os.Create(tmpPath)
	if err != nil {
		return err
	}
	defer file.Close()

	if err := json.NewEncoder(file).Encode(tsm.trustStore); err != nil {
		os.Remove(tmpPath)
		return err
	}

	// Atomically rename
	return os.Rename(tmpPath, tsm.path)
}

func (tsm *TrustStoreManager) GetTrustStore() *shared.TrustStore {
	tsm.mu.RLock()
	defer tsm.mu.RUnlock()
	return tsm.trustStore
}

func (tsm *TrustStoreManager) AddIdentity(identity shared.Identity) error {
	tsm.mu.Lock()
	defer tsm.mu.Unlock()

	for _, id := range tsm.trustStore.Identities {
		if id.Fingerprint == identity.Fingerprint {
			return nil // Identity already exists
		}
	}

	tsm.trustStore.Identities = append(tsm.trustStore.Identities, identity)
	return tsm.Save()
}

func (tsm *TrustStoreManager) RemoveIdentity(fingerprint string) error {
	tsm.mu.Lock()
	defer tsm.mu.Unlock()

	for i, id := range tsm.trustStore.Identities {
		if id.Fingerprint == fingerprint {
			// Remove the identity
			tsm.trustStore.Identities = append(tsm.trustStore.Identities[:i], tsm.trustStore.Identities[i+1:]...)
			return tsm.Save()
		}
	}

	return nil // Identity not found, nothing to remove
}
