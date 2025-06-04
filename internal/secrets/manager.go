package secrets

import (
	"encoding/base64"
	"errors"
	"fmt"
	"gopkg.in/yaml.v3"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type SecretManager struct {
	configPath string
	masterKey  []byte
	doppler    *DopplerManager
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

	// check if the value is encrypted(starts with "enc:")
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

	err = sm.setNestedValue(config, path, value)
	if err != nil {
		return err
	}

	if sm.doppler != nil {
		if err := sm.doppler.PushSecret(sm.configPath); err != nil {
			return fmt.Errorf("failed to sync with doppler: %w", err)
		}
	}
	return sm.saveConfig(config)
}

func (sm *SecretManager) LoadConfig() (*Config, error) {
	if _, err := os.Stat(sm.configPath); os.IsNotExist(err) {
		return config, nil
	}

	data, err := ioutil.ReadFile(sm.configPath)
	if err != nil {
		return nil, err
	}

	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}
	return config, nil
}

func (sm *SecretManager) saveConfig(config map[string]interface{}) error {
	data, err := yaml.Marshal(config)
	if err != nil {
		return err
	}

	dir := filepath.Dir(sm.configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return io.WriteFile(sm.configPath, data, 0644)
}

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

	err = sm.deleteNestedValue(config, path)
	if err != nil {
		return err
	}

	return sm.saveConfig(config)
}
