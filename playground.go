//go:build ignore
// +build ignore


package secrets

import (
	"encoding/base64"
	"strings"
)

// Interfaces for better testability and extensibility
type SecretManager interface {
	GetSecret(path string) (string, error)
	UpdateSecret(path string, value string, encrypt bool) error
	DeleteSecret(path string) error
	EncryptValue(value string) (string, error)
	DecryptValue(encryptedValue string) (string, error)
	LoadConfig() (*Config, error)
}

type DopplerManager interface {
	PushSecrets(config *Config) error
}

type NDeploy struct {
	secretManager SecretManager
	config        *Config
	doppler       DopplerManager
	logger        Logger // Optional: interface for structured logging
}

// New creates a new NDeploy instance with a consistent dependency injection approach
func New(
	configPath string,
	masterKey string,
	logger Logger, // Optional: inject logger if needed
) (*NDeploy, error) {
	// Initialize config first (standalone operation)
	cfg, err := LoadConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Initialize secret manager with all required dependencies
	secretManager, err := NewSecretManager(configPath, masterKey, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create secret manager: %w", err)
	}

	// Initialize Doppler manager
	doppler := NewDopplerManager()

	// Setup cross-dependencies
	secretManager.SetDoppler(doppler)

	return &NDeploy{
		secretManager: secretManager,
		config:        cfg,
		doppler:       doppler,
		logger:        logger,
	}, nil
}

// Alternative constructor for full dependency injection
func NewWithDependencies(
	secretManager SecretManager,
	doppler DopplerManager,
	config *Config,
	logger Logger,
) *NDeploy {
	return &NDeploy{
		secretManager: secretManager,
		config:        config,
		doppler:       doppler,
		logger:        logger,
	}
}

func (nd *NDeploy) SyncWithDoppler() error {
	return nd.doppler.PushSecrets(nd.config)
}

func (nd *NDeploy) GetSecret(path string) (string, error) {
	return nd.secretManager.GetSecret(path)
}

func (nd *NDeploy) UpdateSecret(path string, value string, encrypt bool) error {
	return nd.secretManager.UpdateSecret(path, value, encrypt)
}

func (nd *NDeploy) DeleteSecret(path string) error {
	return nd.secretManager.DeleteSecret(path)
}

func (nd *NDeploy) EncryptValue(value string) (string, error) {
	// Delegate to secret manager's encryption
	return nd.secretManager.EncryptValue(value)
}

func (nd *NDeploy) DecryptValue(encryptedValue string) (string, error) {
	// Delegate to secret manager's decryption
	return nd.secretManager.DecryptValue(encryptedValue)
}

// Helper functions moved to appropriate packages

func LoadConfig(path string) (*Config, error) {
	// Implementation moved from SecretManager
}

// Encryption/Deco
//
//
//
//
//
//

// nDeploty Copyright (c) 2025
//// ryption moved to SecretManager or dedicated crypto package
uAuthor. All Rights Reserved.
package secrets

// TODO: Choose a single architecture approach for New() — either:
//       a) Fully inject all dependencies (SecretManager, DopplerManager, Config), or
//       b) Construct all dependencies inside using configPath and masterKey.
//       Mixing both leads to brittle, hard-to-test code.

// TODO: Remove redundant nil checks for secretManager and doppler.
//       If NewSecretManager or any prior step fails, it already returns an error.

// TODO: Avoid reinitializing SecretManager inside New() if one is passed in.
//       Respect the injected instance or drop the parameter altogether.

// TODO: Remove commented-out code like `NewDopplerManager(doppler)`.
//       Dead code should be deleted, not left behind as clutter.

// TODO: Replace all fmt.Printf and fmt.Println logging with structured logging.
//       Inject a logger if needed. Avoid stdout in constructors — it’s not testable.

// TODO: Abstract masterKey usage out of NDeploy.
//       Add Encrypt/Decrypt methods to SecretManager or use a dedicated Crypto service.

// TODO: Move crypto logic (EncryptValue, DecryptValue) into SecretManager.
//       NDeploy should not access secretManager.masterKey directly — that’s leaky encapsulation.

// TODO: Simplify config loading logic — consider making LoadConfig a standalone function.
//       Right now it’s weirdly tied to an input SecretManager that you overwrite.

// TODO: Add interfaces for SecretManager and DopplerManager.
//       This will drastically improve testability and future extensibility.
import (
	"encoding/base64"
	"fmt"
	"strings"
)

type NDeploy struct {
	secretManager *SecretManager
	config        *Config
	doppler       *DopplerManager
}

func New(sm *SecretManager, doppler *DopplerManager, configPath string, masterKey string) (*NDeploy, error) {
	fmt.Println("Creating NDeploy with configPath:", configPath, "and masterKey:", masterKey)
	fmt.Printf("The secret manager structure looks like this :%v", &SecretManager{})

	cfg, err := sm.LoadConfig()
	if err != nil {
		return nil, err
	}

	secretManager, err := NewSecretManager(configPath, masterKey, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create secret manager: %w", err)
	}

	// Remove this line as it's causing the type mismatch
	// doppler = NewDopplerManager(doppler)

	// Instead, just use the doppler manager passed in
	secretManager.doppler = doppler

	if secretManager == nil {
		return nil, fmt.Errorf("secret manager is nil")
	}
	if secretManager.doppler == nil {
		return nil, fmt.Errorf("doppler manager is nil")
	}

	return &NDeploy{
		secretManager: secretManager,
		config:        cfg,
		doppler:       doppler, // Store the doppler manager in NDeploy as well
	}, nil
}

func (nd *NDeploy) SyncWithDoppler() error {
	//NOTE: should take the nd.config to push secrets
	return nd.secretManager.doppler.PushSecrets()
}
func (nd *NDeploy) GetSecret(path string) (string, error) {
	return nd.secretManager.GetSecret(path)
}

func (nd *NDeploy) UpdateSecret(path string, value string, encrypt bool) error {
	return nd.secretManager.UpdateSecret(path, value, encrypt)
}

func (nd *NDeploy) DeleteSecret(path string) error {
	return nd.secretManager.DeleteSecret(path)
}

func (nd *NDeploy) EncryptValue(value string) (string, error) {
	encrypted, err := Encrypt([]byte(value), nd.secretManager.masterKey)
	if err != nil {
		return "", err
	}
	return "enc:" + base64.StdEncoding.EncodeToString(encrypted), nil
}

func (nd *NDeploy) DecryptValue(encryptedValue string) (string, error) {
	if !strings.HasPrefix(encryptedValue, "enc:") {
		return encryptedValue, nil
	}

	data, err := base64.StdEncoding.DecodeString(encryptedValue[4:])
	if err != nil {
		return "", err
	}

	decrypted, err := Decrypt(data, nd.secretManager.masterKey)
	if err != nil {
		return "", err
	}

	return string(decrypted), nil
}
