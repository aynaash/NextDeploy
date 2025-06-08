package secrets

import (
	"encoding/base64"
	"errors"
	"fmt"
	"gopkg.in/yaml.v3"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

type SecretManager struct {
	configPath string
	masterKey  []byte
	doppler    *DopplerManager
}
type DopplerConfig struct {
	Token   string `yaml:"token"`
	Project string `yaml:"project"`
	Config  string `yaml:"config"`
}

// NewSecretManager creates a new SecretManager
func NewSecretManager(configPath string, masterKey string, cfg *Config) (*SecretManager, error) {
	derivedKey, err := DeriveKey(masterKey)
	if err != nil {
		return nil, fmt.Errorf("failed to derive master key: %w", err)
	}
	return &SecretManager{
		configPath: configPath,
		masterKey:  derivedKey,
		doppler:    NewDopplerManager(&cfg.Doppler),
	}, nil
}

func (sm *SecretManager) GetSecret(path string) (string, error) {
	value, err := sm.getNestedValue(path)
	if err != nil {
		return "", err
	}

	if strings.HasPrefix(value, "enc:") {
		encrypted, err := base64.StdEncoding.DecodeString(value[4:])
		if err != nil {
			return "", ErrDecryptFailed
		}
		decrypted, err := Decrypt(encrypted, sm.masterKey)
		if err != nil {
			return "", ErrDecryptFailed
		}
		return string(decrypted), nil
	}
	return value, nil
}

func (sm *SecretManager) UpdateSecret(path string, value string, encrypt bool) error {
	config, err := sm.LoadConfig()
	if err != nil {
		return err
	}

	if encrypt {
		encrypted, err := Encrypt([]byte(value), sm.masterKey)
		if err != nil {
			return ErrEncryptFailed
		}
		value = "enc:" + base64.StdEncoding.EncodeToString(encrypted)
	}

	// Convert config to map[string]interface{} for nested operations
	configMap, ok := config.ToMap()
	if !ok {
		return errors.New("failed to convert config to map")
	}

	err = sm.setNestedValue(configMap, path, value)
	if err != nil {
		return err
	}

	// Update the original config with the modified map
	if err := config.FromMap(configMap); err != nil {
		return err
	}

	if sm.doppler != nil {
		if err := sm.doppler.PushSecret(path, value); err != nil {
			return fmt.Errorf("failed to sync with doppler: %w", err)
		}
	}
	return sm.saveConfig(config)
}

// StoreToken stores a secret name/value pair (encrypted by default)
func (sm *SecretManager) StoreToken(name, value string) error {
	return sm.UpdateSecret(name, value, true) // true = encrypt the value
}

// StoreToken creates a SecretManager and stores a name/value pair
func StoreToken(name, value string) error {
	// Create a basic configuration
	cfg := &Config{
		Doppler: Doppler{
			// These can be empty if not using Doppler
		},
	}

	// Create secret manager with empty path and master key for now
	// You might want to make these configurable
	sm, err := NewSecretManager("", "default-master-key", cfg)
	if err != nil {
		return fmt.Errorf("failed to create secret manager: %w", err)
	}

	return sm.StoreToken(name, value)
}
func (sm *SecretManager) LoadConfig() (*Config, error) {
	if _, err := os.Stat(sm.configPath); os.IsNotExist(err) {
		return &Config{}, nil // Return empty config instead of undefined variable
	}

	data, err := ioutil.ReadFile(sm.configPath)
	if err != nil {
		return nil, err
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}
	return &config, nil
}

func (sm *SecretManager) saveConfig(config *Config) error {
	data, err := yaml.Marshal(config)
	if err != nil {
		return err
	}

	dir := filepath.Dir(sm.configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return ioutil.WriteFile(sm.configPath, data, 0644)
}

// ... rest of the methods remain the same ...

func (sm *SecretManager) getNestedValue(path string) (string, error) {
	config, err := sm.LoadConfig()
	if err != nil {
		return "", err
	}

	parts := strings.Split(path, ".")
	var value interface{} = config

	for _, part := range parts {
		if m, ok := value.(map[string]interface{}); ok {
			value = m[part]
		} else {
			return "", errors.New("invalid path: " + path)
		}
	}

	if strValue, ok := value.(string); ok {
		return strValue, nil
	}
	return "", errors.New("value is not a string: " + path)
}

func (sm *SecretManager) setNestedValue(config map[string]interface{}, path string, value string) error {
	parts := strings.Split(path, ".")
	var current map[string]interface{} = config

	for i, part := range parts {
		if i == len(parts)-1 {
			current[part] = value
			return nil
		}
		if _, ok := current[part]; !ok {
			current[part] = make(map[string]interface{})
		}
		current = current[part].(map[string]interface{})
	}
	return nil
}

func (sm *SecretManager) deleteNestedValue(config map[string]interface{}, path string) error {
	parts := strings.Split(path, ".")
	var current map[string]interface{} = config

	for i, part := range parts {
		if i == len(parts)-1 {
			delete(current, part)
			return nil
		}
		if next, ok := current[part].(map[string]interface{}); ok {
			current = next
		} else {
			return errors.New("invalid path: " + path)
		}
	}
	return nil
}

func (sm *SecretManager) DeleteSecret(path string) error {
	config, err := sm.LoadConfig()
	if err != nil {
		return err
	}
	configMap, ok := config.ToMap()
	if !ok {
		return errors.New("failed to convert config to map")
	}
	err = sm.deleteNestedValue(configMap, path)
	if err != nil {
		return err
	}

	return sm.saveConfig(config)
}
