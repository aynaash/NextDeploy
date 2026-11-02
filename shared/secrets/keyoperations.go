package secrets

import (
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/zalando/go-keyring"
)

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

func (sm *SecretManager) GenerateMasterKey() ([]byte, error) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrKeyGenerationFailed, err)
	}
	return key, nil
}

func (sm *SecretManager) GenerateWindowsKey() (string, error) {
	key, err := sm.GenerateMasterKey()
	if err != nil {
		return "", fmt.Errorf("failed to generate master key for Windows: %w", err)
	}
	return string(key), nil
}

func (sm *SecretManager) GeneratePlatformKey() (string, error) {
	currentOS := runtime.GOOS
	switch currentOS {
	case "linux", "darwin":
		return sm.derivePlatformKey("linux-mac-salt")
	case "windows":
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
	h := sha256.Sum256(append(masterkey, []byte(salt)...))
	return string(h[:]), nil
}

func (sm *SecretManager) getOrCreateMasterKey() ([]byte, error) {
	key, err := sm.loadMasterKey()
	if err == nil {
		return key, nil
	}

	if !errors.Is(err, os.ErrNotExist) && !errors.Is(err, keyring.ErrNotFound) {
		return nil, fmt.Errorf("failed to load master key: %w", err)
	}

	newKey, err := sm.GenerateMasterKey()
	if err != nil {
		return nil, fmt.Errorf("failed to generate new master key: %w", err)
	}

	if err := sm.storeMasterKey(newKey); err != nil {
		return nil, fmt.Errorf("failed to store master key: %w", err)
	}

	return newKey, nil
}

func (sm *SecretManager) loadMasterKey() ([]byte, error) {
	switch runtime.GOOS {
	case "linux", "darwin":
		return sm.loadUnixKey()
	case "windows":
		return sm.loadWindowsKey()
	default:
		return nil, ErrUnsupportedPlatform
	}
}

func (sm *SecretManager) storeMasterKey(key []byte) error {
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
	if sm.cfg == nil || sm.cfg.App.Name == "" {
		return fmt.Errorf("invalid configuration: app name not set")
	}
	appname := sm.cfg.App.Name

	homedir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	appDir := filepath.Join(homedir, ".nextdeploy", appname)
	if err := os.MkdirAll(appDir, 0o700); err != nil {
		return fmt.Errorf("failed to create app directory: %w", err)
	}

	keyPath := filepath.Join(appDir, "master.key")
	if err := os.WriteFile(keyPath, key, 0o600); err != nil {
		return fmt.Errorf("failed to write master key to file: %w", err)
	}

	return nil
}

func (sm *SecretManager) IsKeyExist() bool {
	switch runtime.GOOS {
	case "linux", "darwin":
		return sm.isUnixKeyExist()
	case "windows":
		return sm.isWindowsKeyExist()
	default:
		return false
	}
}

func (sm *SecretManager) isWindowsKeyExist() bool {
	if sm.cfg == nil || sm.cfg.App.Name == "" {
		SLogs.Error("Invalid configuration: app name not set")
		return false
	}
	appname := sm.cfg.App.Name
	_, err := keyring.Get(appname, "master_key")
	if err != nil {
		return false
	}
	return true
}

func (sm *SecretManager) isUnixKeyExist() bool {
	const filename = "master.key"
	if sm.cfg == nil || sm.cfg.App.Name == "" {
		SLogs.Error("Invalid configuration: app name not set")
		return false
	}
	homedir, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	filePath := filepath.Join(homedir, ".nextdeploy", sm.cfg.App.Name, filename)
	file, err := os.Stat(filePath)
	if err != nil {
		return false
	}
	return file != nil && !file.IsDir()
}

func (sm *SecretManager) loadUnixKey() ([]byte, error) {
	const keyFilename = "master.key"
	if sm.cfg == nil || sm.cfg.App.Name == "" {
		return nil, fmt.Errorf("invalid configuration: app name not set")
	}
	homedir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}
	appDir := filepath.Join(homedir, ".nextdeploy", sm.cfg.App.Name)
	keyPath := filepath.Join(appDir, keyFilename)

	// #nosec G304
	keyData, err := os.ReadFile(keyPath)
	if err == nil {
		if len(keyData) == 0 {
			return nil, fmt.Errorf("empty master key")
		}
		return keyData, nil
	}

	if !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to read key file: %w", err)
	}

	newKey, err := sm.GenerateMasterKey()
	if err != nil {
		return nil, fmt.Errorf("failed to generate master key: %w", err)
	}

	if err := os.MkdirAll(appDir, 0o700); err != nil {
		return nil, fmt.Errorf("failed to create app directory: %w", err)
	}
	if err := os.WriteFile(keyPath, newKey, 0o600); err != nil {
		return nil, fmt.Errorf("failed to store master key: %w", err)
	}

	return newKey, nil
}

func (sm *SecretManager) storeWindowsKey(key []byte) error {
	if sm.cfg == nil || sm.cfg.App.Name == "" {
		return fmt.Errorf("invalid configuration: app name not set")
	}
	appname := sm.cfg.App.Name
	err := keyring.Set(appname, "master_key", string(key))
	if err != nil {
		return fmt.Errorf("failed to store master key in Windows keyring: %w", err)
	}
	return nil
}

func (sm *SecretManager) loadWindowsKey() ([]byte, error) {
	if sm.cfg == nil || sm.cfg.App.Name == "" {
		return nil, fmt.Errorf("invalid configuration: app name not set")
	}
	appname := sm.cfg.App.Name
	encryptedKey, err := keyring.Get(appname, "master_key")
	if err != nil {
		if errors.Is(err, keyring.ErrNotFound) {
			return nil, err
		}
		return nil, fmt.Errorf("failed to load master key from Windows keyring: %w", err)
	}
	return []byte(encryptedKey), nil
}

func (sm *SecretManager) GetKeyOsAgnosticPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	appname := sm.GetAppName()
	return filepath.Join(home, ".nextdeploy", appname, "master.key")
}
