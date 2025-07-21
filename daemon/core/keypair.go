package core

import (
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/big"
	"nextdeploy/shared"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	DefaultKeyRotationInterval = 24 * time.Hour
	MaxKeyHistory              = 5
	KeyFilePerm                = 0600
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
	keyDir         string
	rotateFreq     time.Duration
	currentKey     *shared.KeyPair
	previousKeys   map[string]*shared.KeyPair
	mu             sync.RWMutex
	stopRotate     chan struct{}
	wsAuthKey      *ecdsa.PrivateKey
	rotationTicker *time.Ticker
}

func NewKeyManager(keyDir string, rotateFreq time.Duration) (*KeyManager, error) {

	if rotateFreq == 0 {
		rotateFreq = DefaultKeyRotationInterval
	}
	if err := os.MkdirAll(keyDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create key directory: %v", err)
	}
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
			if err := km.GenerateNewKey(); err != nil {
				return nil, fmt.Errorf("failed to generate initial key: %v", err)
			}
		} else {
			return nil, fmt.Errorf("failed to load keys: %v", err)
		}
	}

	// Generate dedicated WebSocket auth key if not exists
	if err := km.LoadOrGenerateWSAuthKey(); err != nil {
		return nil, fmt.Errorf("failed to load or generate WebSocket auth key: %v", err)
	}
	km.secureMemory()

	return km, nil
}

func (km *KeyManager) secureMemory() {
	km.mu.RLock()
	defer km.mu.RUnlock()

	if km.currentKey != nil {
		shared.SecureKeyMemory(km.currentKey.ECDHPrivate.Bytes())
		shared.SecureKeyMemory(km.currentKey.SignPrivate)
	}
	if km.wsAuthKey != nil {
		// Securely wipe the WebSocket auth key's private part
		shared.SecureKeyMemory(km.wsAuthKey.D.Bytes())
		// Note: Public key parts (X, Y) are not sensitive and do not need to be secured
	} else {
		log.Println("Warning: WebSocket auth key is nil, cannot secure memory")
	}
}

func (km *KeyManager) LoadOrGenerateWSAuthKey() error {
	kefile := filepath.Join(km.keyDir, "ws_auth_key.json")
	var wsKey *ecdsa.PrivateKey
	if _, err := os.Stat(kefile); err == nil {
		file, err := os.Open(kefile)
		if err != nil {
			return fmt.Errorf("failed to open WebSocket auth key file: %v", err)
		}
		defer file.Close()
		var keyData struct {
			D []byte `json:"d"`
			X []byte `json:"x"`
			Y []byte `json:"y"`
		}

		if err := json.NewDecoder(file).Decode(&keyData); err != nil {
			return fmt.Errorf("failed to decode WebSocket auth key: %v", err)
		}
		wsKey = &ecdsa.PrivateKey{
			PublicKey: ecdsa.PublicKey{
				Curve: elliptic.P224(),
				X:     new(big.Int).SetBytes(keyData.X),
				Y:     new(big.Int).SetBytes(keyData.Y),
			},
			D: new(big.Int).SetBytes(keyData.D),
		}
	} else if errors.Is(err, os.ErrNotExist) {
		// generate new key
		var err error
		wsKey, err := ecdsa.GenerateKey(elliptic.P224(), rand.Reader)
		if err != nil {
			return fmt.Errorf("failed to generate WebSocket auth key: %v", err)
		}
		// save new key
		tmpFile, err := os.CreateTemp(km.keyDir, "temp_ws_auth_key_")
		if err != nil {
			return fmt.Errorf("failed to create temporary file for WebSocket auth key: %v", err)
		}
		defer os.Remove(tmpFile.Name())
		keyData := struct {
			D []byte `json:"d"`
			X []byte `json:"x"`
			Y []byte `json:"y"`
		}{
			D: wsKey.D.Bytes(),
			X: wsKey.PublicKey.X.Bytes(),
			Y: wsKey.PublicKey.Y.Bytes(),
		}
		if err := json.NewEncoder(tmpFile).Encode(keyData); err != nil {
			return fmt.Errorf("failed to encode WebSocket auth key: %v", err)
		}

		if err := tmpFile.Close(); err != nil {
			return fmt.Errorf("failed to close temporary file for WebSocket auth key: %v", err)
		}
		if err := os.Rename(tmpFile.Name(), kefile); err != nil {
			return fmt.Errorf("failed to save WebSocket auth key: %v", err)
		}
	} else {
		return fmt.Errorf("failed to check WebSocket auth key file: %v", err)
	}
	km.wsAuthKey = wsKey
	return nil
}
func (km *KeyManager) ValidateToken(token string) (valid bool, err error) {
	return false, fmt.Errorf("token validation not implemented")
}
func (km *KeyManager) GenerateNewKey() error {
	keyPair, err := shared.GenerateKeyPair()
	if err != nil {
		return fmt.Errorf("failed to generate key pair: %v", err)
	}
	// add edcsa key for backward compatibility
	keyPair.ECDSAKey = km.wsAuthKey

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
	if len(km.previousKeys) > MaxKeyHistory {
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
func (km *KeyManager) GetWSAuthKey() *ecdsa.PrivateKey {
	km.mu.RLock()
	defer km.mu.RUnlock()
	return km.wsAuthKey
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
	km.rotationTicker = time.NewTicker(km.rotateFreq)
	defer km.rotationTicker.Stop()

	for {
		select {
		case <-km.rotationTicker.C:
			if err := km.GenerateNewKey(); err != nil {
				log.Printf("Key rotation failed: %v", err)
			} else {
				log.Printf("Successfully rotated to new key: %s", km.currentKey.KeyID)
			}
		case <-km.stopRotate:
			return
		}
	}
}

// func (km *KeyManager) GenerateWSToken(clientID string, expiresIn time.Duration) (string, error) {
// 	km.mu.RLock()
// 	defer km.mu.RUnlock()
//
// 	if km.wsAuthKey == nil {
// 		return "", errors.New("WebSocket auth key not initialized")
// 	}
//
// 	return shared.GenerateWSJWT(km.wsAuthKey, clientID, shared.GenerateSessionID(), expiresIn)
// }

//	func (km *KeyManager) VerifyWSToken(token string) (*shared.WSClaims, error) {
//		km.mu.RLock()
//		defer km.mu.RUnlock()
//
//		if km.wsAuthKey == nil {
//			return nil, errors.New("WebSocket auth key not initialized")
//		}
//
//		return shared.VerifyWSJWT(token, &km.wsAuthKey.PublicKey)
//	}
func (km *KeyManager) StopRotation() {
	close(km.stopRotate)
}
