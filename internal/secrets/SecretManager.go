package secrets

// TODO: 🔥 Refactor into smaller packages: keymgmt, store, envcrypto, providers, migration.
// TODO: ✅ Inject interfaces for crypto engine and persistence (CryptoEngine, SecretStore).
// TODO: 🚨 Harden key management — avoid generating new keys per call, introduce key rotation policy.
// TODO: 🔐 Secure memory wiping for keys/secrets after usage (as much as Go allows).
// TODO: ⚙️ Write comprehensive unit tests with mocks for all interfaces.
// // TODO: 🌐 Add remote sync option — replicate secrets to remote backup or vault.
// TODO: 📄 Add support for secret expiration + auto-purge.
// TODO: 🧠 Integrate memory-based secret leases (e.g., valid for N mins).
// TODO: 🧪 Add CLI and test suite with full e2e coverage for: encrypt → store → rotate → export.

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/zalando/go-keyring"
	"golang.org/x/crypto/hkdf"
	"io"
	"nextdeploy/internal/config"

	"nextdeploy/internal/logger"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

var (
	ErrInvalidSecretFormat   = errors.New("invalid secret format")
	ErrUnsupportedPlatform   = errors.New("unsupported platform")
	ErrKeyGenerationFailed   = errors.New("key generation failed")
	ErrSecretAlreadyExists   = errors.New("secret already exists")
	ErrPermissionDenied      = errors.New("permission denied")
	ErrInvalidProvider       = errors.New("invalid secret provider")
	ErrProviderNotConfigured = errors.New("provider not configured")
)

var (
	SLogs = logger.PackageLogger("Secrets", "🔐 Secrets Manager")
)

type SecretManager struct {
	cfg     *config.NextDeployConfig
	secrets map[string]*Secret
	// TODO: 🚨 Replace this naive cache with persistent key storage or KMS integration.
	keyCache map[string][]byte // Cache for derived keys
	// TODO: ✅ Split provider manager into separate encryption and storage providers.
	manager *SecretsProviderManager
	// TODO: ✍️ Document locking strategy clearly — what's protected and what's not.
	mu sync.RWMutex // Mutex for thread-safe access
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
	GetSecret(key string) (string, error)
	SetSecret(key, value string) error
	DeleteSecret(key string) error
	ListSecrets() ([]string, error)
	Encrypt(data []byte, key string) ([]byte, error)
	Decrypt(data []byte, key string) ([]byte, error)
	GenerateMasterkey() (string, error)
	GenerateMasterkeyLinux() (string, error)
	DeriveKey(key string) ([]byte, error)
	ValidateSecretFormat(secret string) error
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

// Add this to your secrets package (likely near the provider-related methods)

// IsDopplerEnabled checks if Doppler is configured as a secrets provider
func (sm *SecretManager) IsDopplerEnabled() bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	if sm.manager == nil {
		return false
	}

	_, exists := sm.manager.providers["doppler"]
	return exists
}

// You might also want to add a Doppler-specific provider check:
func (sm *SecretManager) GetDopplerProvider() (SecretsProvider, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	if sm.manager == nil {
		return nil, false
	}

	provider, exists := sm.manager.providers["doppler"]
	return provider, exists
}

// TODO: ✅ Allow selection of which .env files to encrypt via pattern or CLI arg.
// TODO: 🔐 Don't overwrite original files by default — add backup logic or prompt.
// TODO: ⚠️ Add versioning or checksum for encrypted file verification.
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
		var masterKey string

		key, err := sm.GeneratePlatformKey()
		if err != nil {
			SLogs.Error("Failed to generate master key: %v", err)
			return nil, fmt.Errorf("%w: %v", ErrKeyGenerationFailed, err)
		}
		masterKey = key
		encrypted, err := Encrypt(content, []byte(masterKey))
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

func (sm *SecretManager) GeneratePlatformKey() (string, error) {
	currentOS := runtime.GOOS
	switch currentOS {
	case "linux", "darwin": // Linux or macOS
		return sm.derivePlatformKey("linux-mac-salt")
	case "windows": // Windows
		return sm.derivePlatformKey("windows-salt")
	default:
		return "", fmt.Errorf("%w: %s", ErrUnsupportedPlatform, currentOS)
	}
}

func (sm *SecretManager) derivePlatformKey(salt string) (string, error) {
	masterkey, err := sm.getOrCreateMasterKey()
	if err != nil {
		return "", fmt.Errorf("failed to get or create master key: %w", err)
	}
	h := hkdf.New(sha256.New, masterkey, []byte(salt), nil)
	deriveky := make([]byte, 32) // AES-256 key NonceSize
	if _, err := io.ReadFull(h, deriveky); err != nil {
		return "", fmt.Errorf("failed to derive key: %w", err)
	}
	return base64.StdEncoding.EncodeToString(deriveky), nil
}

func (sm *SecretManager) getOrCreateMasterKey() ([]byte, error) {
	// Try to load existing key
	key, err := sm.loadMasterKey()
	if err == nil {
		return key, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("failed to load master key: %w", err)
	}
	// generate new key if none exists
	newKey, err := sm.GenerateMasterKey()
	if err != nil {
		return nil, fmt.Errorf("failed to generate new master key: %w", err)
	}
	return []byte(newKey), nil
}

func (sm *SecretManager) loadMasterKey() ([]byte, error) {
	// platform specifc secure storage
	switch runtime.GOOS {
	case "linux", "darwin":
		return sm.loadUnixKey()
	case "windows":
		key, err := sm.GenerateMasterKey()
		if err != nil {
			return nil, fmt.Errorf("failed to generate master key for Windows: %w", err)
		}

		return []byte(key), nil
	default:
		return nil, ErrUnsupportedPlatform
	}
}

func (sm *SecretManager) storeMasterKey(key []byte) error {
	// platform specific secure storage
	switch runtime.GOOS {
	case "linux", "darwin":
		return sm.storeUnixKey(key)
	case "windows":
		return sm.storeWindowsKey(key)
	default:
		return ErrUnsupportedPlatform
	}
}

func (sm *SecretManager) storeUnixKey(key []byte) error {
	appname := sm.cfg.App.Name
	user := os.Getenv("USER")
	err := keyring.Set(appname, user, string(key))
	if err != nil {
		return fmt.Errorf("failed to store master key in keyring: %w", err)
	}
	return nil
}
func (sm *SecretManager) loadUnixKey() ([]byte, error) {
	appname := sm.cfg.App.Name
	user := os.Getenv("USER")
	key, err := keyring.Get(appname, user)
	if err != nil {
		return nil, fmt.Errorf("failed to load master key from"+" keyring: %w", err)
	}
	return []byte(key), nil
}
func (sm *SecretManager) storeWindowsKey(key []byte) error {
	encrypted, err := winutil.Encrypt([]byte(key))
	if err != nil {
		return fmt.Errorf("failed to encrypt master key for Windows: %w", err)
	}
	appname := sm.cfg.App.Name
	err = key
	keyring.set(appname, "windows", string(encrypted))
	if err != nil {
		return fmt.Errorf("failed to store master key in Windows keyring: %w", err)
	}
	return nil

}

func (sm *SecretManager) loadWindowsKey() ([]byte, error) {
	appname := sm.cfg.App.Name
	encrypted, err := keyring.get(appname, "windows")
	if err != nil {
		return nil, fmt.Errorf("failed to load master key from Windows keyring: %w", err)
	}
	key, err := winutil.Decrypt([]byte(encrypted))
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt master key for Windows: %w", err)
	}
	return key, nil
}

// PrepareAppContext prepares the application context by decrypting configuration and environment files
func (sm *SecretManager) PrepareAppContext() (string, error) {
	// TODO: ✂️ Split into smaller interfaces: StorageProvider, CryptoProvider, ValidationProvider.
	// Get and validate working directory
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get working directory: %w", err)
	}

	// Generate encryption key
	key, err := sm.GeneratePlatformKey()
	if err != nil {
		return "", fmt.Errorf("failed to generate encryption key: %w", err)
	}

	// Process nextdeploy.yml
	ndConfig, err := sm.ProcessConfigFile(cwd, key)
	if err != nil {
		return "", fmt.Errorf("config processing failed: %w", err)
	}

	// Process environment files
	envFiles, err := sm.processEnvFiles(cwd, key, ndConfig)
	if err != nil {
		return "", fmt.Errorf("env file processing failed: %w", err)
	}

	// Prepare cleanup defer function
	var filesToCleanup []string
	for _, envFile := range envFiles {
		filesToCleanup = append(filesToCleanup, envFile.DecryptedPath)
	}
	filesToCleanup = append(filesToCleanup, ndConfig.DecryptedPath)
	defer sm.cleanupDecryptedFiles(filesToCleanup)

	SLogs.Info("Application context prepared successfully")
	return cwd, nil
}

// processConfigFile handles the decryption of the configuration file
func (sm *SecretManager) ProcessConfigFile(cwd, key string) (*ConfigFile, error) {
	const configFileName = "nextdeploy.yml"
	encryptedPath := filepath.Join(cwd, configFileName+".enc")

	// Verify config file exists
	if _, err := os.Stat(encryptedPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("encrypted config file not found: %w", err)
	}

	// Read and decrypt config
	encContent, err := os.ReadFile(encryptedPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read encrypted config: %w", err)
	}

	decryptedContent, err := Decrypt(encContent, []byte(key))
	if err != nil {
		return nil, fmt.Errorf("config decryption failed: %w", err)
	}

	// Write decrypted file
	decryptedPath := filepath.Join(cwd, configFileName)
	if err := secureWriteFile(decryptedPath, decryptedContent); err != nil {
		return nil, fmt.Errorf("failed to write decrypted config: %w", err)
	}

	SLogs.Info("Config file decrypted to %s", decryptedPath)
	return &ConfigFile{
		EncryptedPath: encryptedPath,
		DecryptedPath: decryptedPath,
		Content:       decryptedContent,
	}, nil
}

// processEnvFiles handles the decryption of all environment files
func (sm *SecretManager) processEnvFiles(cwd, key string, config *ConfigFile) ([]*EnvFile, error) {
	var processedFiles []*EnvFile

	// Find all encrypted env files
	files, err := filepath.Glob(filepath.Join(cwd, "*.env.enc"))
	if err != nil {
		return nil, fmt.Errorf("failed to find env files: %w", err)
	}

	if len(files) == 0 {
		SLogs.Warn("No encrypted .env files found in directory")
		return nil, nil
	}

	// Process each file
	for _, file := range files {
		envFile, err := sm.processSingleEnvFile(file, key)
		if err != nil {
			SLogs.Error("Failed to process %s: %v", file, err)
			continue
		}
		processedFiles = append(processedFiles, envFile)
	}

	return processedFiles, nil
}

// processSingleEnvFile handles decryption of a single environment file
func (sm *SecretManager) processSingleEnvFile(path, key string) (*EnvFile, error) {
	SLogs.Debug("Processing env file: %s", path)

	encContent, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read failed: %w", err)
	}

	decryptedContent, err := Decrypt(encContent, []byte(key))
	if err != nil {
		return nil, fmt.Errorf("decryption failed: %w", err)
	}

	decryptedPath := strings.TrimSuffix(path, ".enc")
	if err := secureWriteFile(decryptedPath, decryptedContent); err != nil {
		return nil, fmt.Errorf("write failed: %w", err)
	}

	SLogs.Info("Decrypted env file written to %s", decryptedPath)
	return &EnvFile{
		EncryptedPath: path,
		DecryptedPath: decryptedPath,
		Content:       decryptedContent,
	}, nil
}

// secureWriteFile safely writes content to a file
func secureWriteFile(path string, content []byte) error {
	// TODO: 🧼 Memory wipe old temp file securely before overwriting, if applicable.
	// TODO: 🧪 Check file integrity post-write (hash comparison).
	// 1. Write to temp file first
	tempPath := path + ".tmp"
	if err := os.WriteFile(tempPath, content, 0600); err != nil {
		return err
	}

	// 2. Verify the temp file
	if _, err := os.Stat(tempPath); os.IsNotExist(err) {
		return errors.New("temp file not created")
	}

	// 3. Rename (atomic operation)
	return os.Rename(tempPath, path)
}

// cleanupDecryptedFiles removes decrypted files
func (sm *SecretManager) cleanupDecryptedFiles(paths []string) {
	// TODO: 🔐 Use secure delete (overwrite + unlink) if applicable.
	for _, path := range paths {
		if path == "" {
			continue
		}
		if err := os.Remove(path); err != nil {
			SLogs.Warn("Failed to cleanup %s: %v", path, err)
		} else {
			SLogs.Debug("Cleaned up decrypted file: %s", path)
		}
	}
}

// Secure file writing helper
// Supporting types
type ConfigFile struct {
	EncryptedPath string
	DecryptedPath string
	Content       []byte
}

type EnvFile struct {
	EncryptedPath string
	DecryptedPath string
	Content       []byte
}

func (sm *SecretManager) GenerateMasterKey() (string, error) {
	//TODO: this should check for existence of a master key
	//     -- if not in existence save key some where save
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
	if err != nil {
		return "", err
	}

	decrypted, err := Decrypt([]byte(secret.Value), []byte(key))
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrDecryptionFailed, err)
	}

	return string(decrypted), nil
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
		sm.manager = &SecretsProviderManager{
			providers: make(map[string]SecretsProvider),
		}
	}

	for _, provider := range sm.manager.providers {
		if err := provider.ValidateSecretFormat(""); err != nil {
			return fmt.Errorf("invalid secret format: %w", err)
		}
	}

	return nil
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

func (sm *SecretManager) EncryptFile(filename string, key []byte) error {
	// read the file content
	content, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("failed to read nextdeploy.yml: %w", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return fmt.Errorf("failed to create cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return fmt.Errorf("failed to create GCM: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return fmt.Errorf("failed to generate nonce: %w", err)
	}
	ciphertext := gcm.Seal(nonce, nonce, content, nil)
	// Write the encrypted content to a new file
	err = os.WriteFile(filename+".enc", ciphertext, 0644)
	if err != nil {
		return fmt.Errorf("failed to write encrypted file: %w", err)
	}
	//TODO here we should remove the original file or add to git ingore to untracted by git
	SLogs.Info("Encrypted file written to %s.enc", filename)
	return nil

}

func (sm *SecretManager) DecryptFile(filename string, key []byte) error {
	if !strings.HasSuffix(filename, ".enc") {
		return fmt.Errorf("file %s is not an encrypted file", filename)
	}
	// Read the encrypted file content
	content, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("failed to read encrypted file %s: %w", filename, err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return fmt.Errorf("failed to create cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return fmt.Errorf("failed to create GCM: %w", err)
	}
	nonceSize := gcm.NonceSize()
	if len(content) < nonceSize {
		return fmt.Errorf("encrypted file %s is too short", filename)
	}
	nonce, ciphertext := content[:nonceSize], content[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return fmt.Errorf("failed to decrypt file %s: %w", filename, err)
	}
	// Write the decrypted content to a new filename
	decryptedFilename := strings.TrimSuffix(filename, ".enc")
	if err := os.WriteFile(decryptedFilename, plaintext, 0644); err != nil {
		return fmt.Errorf("failed to write decrypted file %s: %w", decryptedFilename, err)
	}
	SLogs.Info("Decrypted file written to %s", decryptedFilename)
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

			decrypted, err := Decrypt([]byte(value), []byte(key))
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
