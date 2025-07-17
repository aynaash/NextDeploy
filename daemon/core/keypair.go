package core

import (
	"crypto/ecdh"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"nextdeploy/shared"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type SecureKeyPair struct {
	*shared.KeyPair
}

func (skp *SecureKeyPair) Close() {
	if skp.KeyPair == nil {
		return
	}

	// securely wipe private keys
	shared.ZeroKey(skp.ECDHPrivate.Bytes())
	shared.ZeroKey(skp.SignPrivate)
}

// NewSecureKeyManager creates a memory protected key manager

func NewSecureKeyManager(keyDir string, rotateFreq time.Duration) (*KeyManager, error) {
	km, err := NewKeyManager(keyDir, rotateFreq)
	if err != nil {
		return nil, err
	}

	if km.currentKey != nil {
		shared.SecureKeyMemory(km.currentKey.ECDHPrivate.Bytes())
		shared.SecureKeyMemory(km.currentKey.SignPrivate)
	}

	return km, nil
}

type KeyManager struct {
	keyDir       string
	rotateFreq   time.Duration
	currentKey   *shared.KeyPair
	previousKeys map[string]*shared.KeyPair
	mu           sync.RWMutex
	stopRotate   chan struct{}
}

func NewKeyManager(keyDir string, rotateFreq time.Duration) (*KeyManager, error) {
	km := &KeyManager{
		keyDir:       keyDir,
		rotateFreq:   rotateFreq,
		previousKeys: make(map[string]*shared.KeyPair),
		stopRotate:   make(chan struct{}),
	}

	// Try to load existing keys
	if err := km.loadKeys(); err != nil {
		// If no keys exist, generate a new one
		if errors.Is(err, os.ErrNotExist) {
			if err := km.generateNewKey(); err != nil {
				return nil, fmt.Errorf("failed to generate initial key: %v", err)
			}
		} else {
			return nil, fmt.Errorf("failed to load keys: %v", err)
		}
	}

	return km, nil
}

func (km *KeyManager) ValidateToken(token string) (valid bool, err error) {
	return false, fmt.Errorf("token validation not implemented")
}
func (km *KeyManager) generateNewKey() error {
	keyPair, err := shared.GenerateKeyPair()
	if err != nil {
		return fmt.Errorf("failed to generate key pair: %v", err)
	}

	// Save the new key
	if err := km.saveKey(keyPair); err != nil {
		return fmt.Errorf("failed to save key: %v", err)
	}

	km.mu.Lock()
	defer km.mu.Unlock()

	// Keep the previous current key in the history
	if km.currentKey != nil {
		km.previousKeys[km.currentKey.KeyID] = km.currentKey
	}

	// Set the new key as current
	km.currentKey = keyPair

	// Keep only the last 5 keys
	if len(km.previousKeys) > 5 {
		// Remove the oldest key (we don't track order, so just remove one)
		for k := range km.previousKeys {
			if k != km.currentKey.KeyID {
				delete(km.previousKeys, k)
				break
			}
		}
	}

	return nil
}

func (km *KeyManager) saveKey(keyPair *shared.KeyPair) error {
	keyFile := filepath.Join(km.keyDir, "current_key.json")

	// Create a temporary file first
	tmpFile, err := os.CreateTemp(km.keyDir, "tmp_key_")
	if err != nil {
		return err
	}
	defer os.Remove(tmpFile.Name())

	// Encode the key pair
	keyData := struct {
		KeyID       string `json:"key_id"`
		ECDHPrivate []byte `json:"ecdh_private"`
		SignPrivate []byte `json:"sign_private"`
	}{
		KeyID:       keyPair.KeyID,
		ECDHPrivate: keyPair.ECDHPrivate.Bytes(),
		SignPrivate: keyPair.SignPrivate,
	}

	if err := json.NewEncoder(tmpFile).Encode(keyData); err != nil {
		return err
	}

	if err := tmpFile.Close(); err != nil {
		return err
	}

	// Atomically rename the temporary file to the target file
	return os.Rename(tmpFile.Name(), keyFile)
}

func (km *KeyManager) loadKeys() error {
	keyFile := filepath.Join(km.keyDir, "current_key.json")

	file, err := os.Open(keyFile)
	if err != nil {
		return err
	}
	defer file.Close()

	var keyData struct {
		KeyID       string `json:"key_id"`
		ECDHPrivate []byte `json:"ecdh_private"`
		SignPrivate []byte `json:"sign_private"`
	}

	if err := json.NewDecoder(file).Decode(&keyData); err != nil {
		return err
	}

	curve := ecdh.X25519()
	ecdhPrivate, err := curve.NewPrivateKey(keyData.ECDHPrivate)
	if err != nil {
		return fmt.Errorf("failed to parse ECDH private key: %v", err)
	}

	signPrivate := ed25519.PrivateKey(keyData.SignPrivate)
	if len(signPrivate) != ed25519.PrivateKeySize {
		return fmt.Errorf("invalid Ed25519 private key size")
	}

	km.currentKey = &shared.KeyPair{
		KeyID:       keyData.KeyID,
		ECDHPrivate: ecdhPrivate,
		ECDHPublic:  ecdhPrivate.PublicKey(),
		SignPrivate: signPrivate,
		SignPublic:  signPrivate.Public().(ed25519.PublicKey),
	}

	return nil
}

func (km *KeyManager) GetCurrentKey() *shared.KeyPair {
	km.mu.RLock()
	defer km.mu.RUnlock()
	return km.currentKey
}

func (km *KeyManager) GetKey(keyID string) *shared.KeyPair {
	km.mu.RLock()
	defer km.mu.RUnlock()

	if km.currentKey != nil && km.currentKey.KeyID == keyID {
		return km.currentKey
	}

	return km.previousKeys[keyID]
}

func (km *KeyManager) StartRotation() {
	ticker := time.NewTicker(km.rotateFreq)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := km.generateNewKey(); err != nil {
				log.Printf("Failed to rotate key: %v", err)
			} else {
				log.Printf("Successfully rotated to new key: %s", km.currentKey.KeyID)
			}
		case <-km.stopRotate:
			return
		}
	}
}

func (km *KeyManager) StopRotation() {
	close(km.stopRotate)
}
