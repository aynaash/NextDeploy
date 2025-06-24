//go:build ignore
// +build ignore

package playground

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"nextdeploy/internal/config"
	"nextdeploy/internal/logger"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

var (
	SLogs = logger.PackageLogger("Secrets", "🔐 Secrets Manager")
)

var (
	ErrEncryptionFailed      = errors.New("encryption failed")
	ErrDecryptionFailed      = errors.New("decryption failed")
	ErrSecretNotFound        = errors.New("secret not found")
	ErrInvalidSecretFormat   = errors.New("invalid secret format")
	ErrUnsupportedPlatform   = errors.New("unsupported platform")
	ErrKeyGenerationFailed   = errors.New("key generation failed")
	ErrSecretAlreadyExists   = errors.New("secret already exists")
	ErrPermissionDenied      = errors.New("permission denied")
	ErrInvalidProvider       = errors.New("invalid secret provider")
	ErrProviderNotConfigured = errors.New("provider not configured")
)

type SecretManager struct {
	cfg      *config.NextDeployConfig
	secrets  map[string]*Secret
	manager  *SecretsProviderManager
	mu       sync.RWMutex
	keyCache map[string][]byte // Cache for derived keys
}

type Secret struct {
	Value       string `json:"value"`
	Version     int    `json:"version"`
	CreatedAt   int64  `json:"created_at"`
	ModifiedAt  int64  `json:"modified_at"`
	IsEncrypted bool   `json:"is_encrypted"`
}

type SecretsProviderManager struct {
	providers map[string]SecretsProvider
}

type SecretsProvider interface {
	GetSecret(name string) (string, error)
	SetSecret(name, value string) error
	ListSecrets() ([]string, error)
}

type Option func(*SecretManager)

func WithConfig(cfg *config.NextDeployConfig) Option {
	return func(sm *SecretManager) {
		sm.cfg = cfg
	}
}

func WithProvider(name string, provider SecretsProvider) Option {
	return func(sm *SecretManager) {
		if sm.manager == nil {
			sm.manager = &SecretsProviderManager{
				providers: make(map[string]SecretsProvider),
			}
		}
		sm.manager.providers[name] = provider
	}
}

func NewSecretManager(opts ...Option) *SecretManager {
	sm := &SecretManager{
		secrets:  make(map[string]*Secret),
		keyCache: make(map[string][]byte),
	}

	for _, opt := range opts {
		opt(sm)
	}

	if sm.cfg == nil {
		cfg, err := config.Load()
		if err != nil {
			SLogs.Error("Failed to load configuration: %v", err)
			return nil
		}
		sm.cfg = cfg
	}

	return sm
}

// EncryptEnvFile encrypts all .env files in the current directory
func (sm *SecretManager) EncryptEnvFile() (map[string]string, error) {
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

		content, err := os.ReadFile(file)
		if err != nil {
			SLogs.Error("Failed to read .env file %s: %v", file, err)
			continue
		}

		masterKey, err := sm.GeneratePlatformKey()
		if err != nil {
			SLogs.Error("Failed to generate master key: %v", err)
			return nil, fmt.Errorf("%w: %v", ErrKeyGenerationFailed, err)
		}

		encrypted, err := Encrypt(content, masterKey)
		if err != nil {
			SLogs.Error("Failed to encrypt file %s: %v", file, err)
			return nil, fmt.Errorf("%w: %v", ErrEncryptionFailed, err)
		}

		encryptedFile := file + ".enc"
		if err := os.WriteFile(encryptedFile, encrypted, 0600); err != nil {
			SLogs.Error("Failed to write encrypted file %s: %v", encryptedFile, err)
			return nil, err
		}

		results[file] = encryptedFile
	}

	return results, nil
}

// GeneratePlatformKey generates a platform-specific master key
func (sm *SecretManager) GeneratePlatformKey() (string, error) {
	currentOS := runtime.GOOS
	switch currentOS {
	case "linux", "darwin":
		return sm.GenerateMasterKey()
	case "windows":
		return sm.GenerateWindowsKey()
	default:
		return "", fmt.Errorf("%w: %s", ErrUnsupportedPlatform, currentOS)
	}
}

// GenerateMasterKey generates a cryptographically secure random key
func (sm *SecretManager) GenerateMasterKey() (string, error) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return "", fmt.Errorf("%w: %v", ErrKeyGenerationFailed, err)
	}
	return hex.EncodeToString(key), nil
}

// GenerateWindowsKey generates a key for Windows platforms
func (sm *SecretManager) GenerateWindowsKey() (string, error) {
	// Windows-specific key generation logic could go here
	// For now, we'll use the same approach as other platforms
	return sm.GenerateMasterKey()
}

// SetSecret stores a secret with optional encryption
func (sm *SecretManager) SetSecret(name, value string, encrypt bool) error {
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
		encrypted, err := Encrypt([]byte(value), key)
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
	if err != nil {
		return "", err
	}

	decrypted, err := Decrypt([]byte(secret.Value), key)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrDecryptionFailed, err)
	}

	return string(decrypted), nil
}

// RotateSecrets re-encrypts all secrets with a new key
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

		decrypted, err := Decrypt([]byte(secret.Value), oldKey)
		if err != nil {
			SLogs.Warn("Failed to decrypt secret %s: %v", name, err)
			continue
		}

		// Encrypt with new key
		encrypted, err := Encrypt(decrypted, newKey)
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

// getCachedKey retrieves a cached key or generates a new one
func (sm *SecretManager) getCachedKey(name string) (string, error) {
	if key, exists := sm.keyCache[name]; exists {
		return string(key), nil
	}

	key, err := sm.GeneratePlatformKey()
	if err != nil {
		return "", err
	}

	sm.keyCache[name] = []byte(key)
	return key, nil
}

// MigrateToProvider migrates secrets to an external provider
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

			decrypted, err := Decrypt([]byte(value), key)
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

// ValidateSecret checks if a secret meets complexity requirements
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
