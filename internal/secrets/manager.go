package secrets

//
// import (
// 	"encoding/base64"
// 	"encoding/json"
// 	"errors"
// 	"fmt"
// 	"gopkg.in/yaml.v3"
// 	"io/ioutil"
// 	"os"
// 	"os/exec"
// 	"path/filepath"
// 	"strings"
// )
//
// // Interfaces for dependency injection and testing
// type (
// 	SecretManager interface {
// 		GetSecret(path string) (string, error)
// 		UpdateSecret(path string, value string, encrypt bool) error
// 		DeleteSecret(path string) error
// 		EncryptValue(value string) (string, error)
// 		DecryptValue(encryptedValue string) (string, error)
// 		LoadConfig() (*Config, error)
// 		StoreToken(name, value string) error
// 	}
//
// 	DopplerManager interface {
// 		PushSecrets() error
// 		VerifySecrets() error
// 		PushSecret(path string, value string) error
// 	}
//
// 	Logger interface {
// 		Debug(msg string, args ...interface{})
// 		Info(msg string, args ...interface{})
// 		Error(msg string, args ...interface{})
// 	}
// )
//
// // Config represents the application configuration
// type Config struct {
// 	Repository struct {
// 		WebhookSecret string `yaml:"webhook_secret"`
// 	} `yaml:"repository"`
// 	Database struct {
// 		Username string `yaml:"username"`
// 		Password string `yaml:"password"`
// 	} `yaml:"database"`
// 	Backup struct {
// 		Storage struct {
// 			AccessKey string `yaml:"access_key"`
// 			SecretKey string `yaml:"secret_key"`
// 		} `yaml:"storage"`
// 	} `yaml:"backup"`
// 	Docker struct {
// 		Build struct {
// 			Args map[string]string `yaml:"args"`
// 		} `yaml:"build"`
// 	} `yaml:"docker"`
// 	Doppler DopplerConfig `yaml:"doppler"`
// }
//
// type DopplerConfig struct {
// 	Token   string `yaml:"token"`
// 	Project string `yaml:"project"`
// 	Config  string `yaml:"config"`
// }
//
// type Secret struct {
// 	Name  string
// 	Value string
// }
//
// // DopplerManager implementation
// type dopplerManager struct {
// 	project string
// 	config  string
// 	logger  Logger
// }
//
// func NewDopplerManager(cfg *DopplerConfig, logger Logger) DopplerManager {
// 	return &dopplerManager{
// 		project: cfg.Project,
// 		config:  cfg.Config,
// 		logger:  logger,
// 	}
// }
//
// func (dm *dopplerManager) PushSecrets() error {
// 	return dm.extractAndPushSecretsToDoppler()
// }
//
// func (dm *dopplerManager) VerifySecrets() error {
// 	return dm.verifyDopplerSecrets()
// }
//
// func (dm *dopplerManager) PushSecret(path string, value string) error {
// 	cmd := exec.Command("doppler", "secrets", "set",
// 		fmt.Sprintf("%s=%s", path, value),
// 		"--project", dm.project,
// 		"--config", dm.config,
// 		"--silent",
// 	)
//
// 	output, err := cmd.CombinedOutput()
// 	if err != nil {
// 		return fmt.Errorf("failed to set secret %s in Doppler: %w\nOutput: %s", path, err, string(output))
// 	}
// 	return nil
// }
//
// func (dm *dopplerManager) extractAndPushSecretsToDoppler() error {
// 	cwd, err := os.Getwd()
// 	if err != nil {
// 		return fmt.Errorf("failed to get current working directory: %w", err)
// 	}
//
// 	configPath := filepath.Join(cwd, "nextdeploy.yml")
// 	data, err := ioutil.ReadFile(configPath)
// 	if err != nil {
// 		return fmt.Errorf("failed to read nextdeploy.yml: %w", err)
// 	}
//
// 	var config Config
// 	if err := yaml.Unmarshal(data, &config); err != nil {
// 		return fmt.Errorf("failed to parse nextdeploy.yml: %w", err)
// 	}
//
// 	secrets := []Secret{
// 		{"REPOSITORY_WEBHOOK_SECRET", config.Repository.WebhookSecret},
// 		{"DATABASE_PASSWORD", config.Database.Password},
// 		{"DATABASE_USERNAME", config.Database.Username},
// 		{"BACKUP_ACCESS_KEY", config.Backup.Storage.AccessKey},
// 		{"BACKUP_SECRET_KEY", config.Backup.Storage.SecretKey},
// 	}
//
// 	for k, v := range config.Docker.Build.Args {
// 		secrets = append(secrets, Secret{
// 			Name:  fmt.Sprintf("DOCKER_BUILD_ARG_%s", k),
// 			Value: v,
// 		})
// 	}
//
// 	for _, secret := range secrets {
// 		if secret.Value != "" {
// 			if err := dm.PushSecret(secret.Name, secret.Value); err != nil {
// 				return err
// 			}
// 		}
// 	}
//
// 	return nil
// }
//
// func (dm *dopplerManager) verifyDopplerSecrets() error {
// 	cmd := exec.Command("doppler", "secrets", "download",
// 		"--project", dm.project,
// 		"--config", dm.config,
// 		"--format", "json",
// 		"--no-file",
// 	)
//
// 	output, err := cmd.CombinedOutput()
// 	if err != nil {
// 		return fmt.Errorf("failed to download secrets from Doppler: %w\nOutput: %s", err, string(output))
// 	}
//
// 	var dopplerSecrets map[string]interface{}
// 	if err := json.Unmarshal(output, &dopplerSecrets); err != nil {
// 		return fmt.Errorf("failed to parse Doppler secrets: %w", err)
// 	}
//
// 	requiredSecrets := []string{"DATABASE_PASSWORD", "DATABASE_USERNAME"}
// 	for _, secretName := range requiredSecrets {
// 		if _, exists := dopplerSecrets[secretName]; !exists {
// 			return fmt.Errorf("required secret %s not found in Doppler", secretName)
// 		}
// 	}
//
// 	return nil
// }
//
// // SecretManager implementation
// type secretManager struct {
// 	configPath string
// 	masterKey  []byte
// 	doppler    DopplerManager
// 	logger     Logger
// }
//
// func NewSecretManager(configPath string, masterKey string, cfg *Config, logger Logger) (SecretManager, error) {
// 	derivedKey, err := DeriveKey(masterKey)
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to derive master key: %w", err)
// 	}
//
// 	return &secretManager{
// 		configPath: configPath,
// 		masterKey:  derivedKey,
// 		doppler:    NewDopplerManager(&cfg.Doppler, logger),
// 		logger:     logger,
// 	}, nil
// }
//
// func (sm *secretManager) GetSecret(path string) (string, error) {
// 	value, err := sm.getNestedValue(path)
// 	if err != nil {
// 		return "", err
// 	}
//
// 	if strings.HasPrefix(value, "enc:") {
// 		return sm.DecryptValue(value)
// 	}
// 	return value, nil
// }
//
// func (sm *secretManager) UpdateSecret(path string, value string, encrypt bool) error {
// 	config, err := sm.LoadConfig()
// 	if err != nil {
// 		return err
// 	}
//
// 	if encrypt {
// 		value, err = sm.EncryptValue(value)
// 		if err != nil {
// 			return err
// 		}
// 	}
//
// 	configMap, ok := config.ToMap()
// 	if !ok {
// 		return errors.New("failed to convert config to map")
// 	}
//
// 	if err := sm.setNestedValue(configMap, path, value); err != nil {
// 		return err
// 	}
//
// 	if err := config.FromMap(configMap); err != nil {
// 		return err
// 	}
//
// 	if sm.doppler != nil {
// 		if err := sm.doppler.PushSecret(path, value); err != nil {
// 			return fmt.Errorf("failed to sync with doppler: %w", err)
// 		}
// 	}
// 	return sm.saveConfig(config)
// }
//
// func (sm *secretManager) StoreToken(name, value string) error {
// 	return sm.UpdateSecret(name, value, true)
// }
//
// func (sm *secretManager) EncryptValue(value string) (string, error) {
// 	encrypted, err := Encrypt([]byte(value), sm.masterKey)
// 	if err != nil {
// 		return "", ErrEncryptFailed
// 	}
// 	return "enc:" + base64.StdEncoding.EncodeToString(encrypted), nil
// }
//
// func (sm *secretManager) DecryptValue(encryptedValue string) (string, error) {
// 	if !strings.HasPrefix(encryptedValue, "enc:") {
// 		return encryptedValue, nil
// 	}
//
// 	data, err := base64.StdEncoding.DecodeString(encryptedValue[4:])
// 	if err != nil {
// 		return "", ErrDecryptFailed
// 	}
//
// 	decrypted, err := Decrypt(data, sm.masterKey)
// 	if err != nil {
// 		return "", ErrDecryptFailed
// 	}
//
// 	return string(decrypted), nil
// }
//
// func (sm *secretManager) DeleteSecret(path string) error {
// 	config, err := sm.LoadConfig()
// 	if err != nil {
// 		return err
// 	}
// 	configMap, ok := config.ToMap()
// 	if !ok {
// 		return errors.New("failed to convert config to map")
// 	}
// 	if err := sm.deleteNestedValue(configMap, path); err != nil {
// 		return err
// 	}
// 	return sm.saveConfig(config)
// }
//
// func (sm *secretManager) LoadConfig() (*Config, error) {
// 	if _, err := os.Stat(sm.configPath); os.IsNotExist(err) {
// 		return &Config{}, nil
// 	}
//
// 	data, err := ioutil.ReadFile(sm.configPath)
// 	if err != nil {
// 		return nil, err
// 	}
//
// 	var config Config
// 	if err := yaml.Unmarshal(data, &config); err != nil {
// 		return nil, err
// 	}
// 	return &config, nil
// }
//
// func (sm *secretManager) saveConfig(config *Config) error {
// 	data, err := yaml.Marshal(config)
// 	if err != nil {
// 		return err
// 	}
//
// 	dir := filepath.Dir(sm.configPath)
// 	if err := os.MkdirAll(dir, 0755); err != nil {
// 		return err
// 	}
// 	return ioutil.WriteFile(sm.configPath, data, 0644)
// }
//
// func (sm *secretManager) getNestedValue(path string) (string, error) {
// 	config, err := sm.LoadConfig()
// 	if err != nil {
// 		return "", err
// 	}
//
// 	parts := strings.Split(path, ".")
// 	var value interface{} = config
//
// 	for _, part := range parts {
// 		if m, ok := value.(map[string]interface{}); ok {
// 			value = m[part]
// 		} else {
// 			return "", errors.New("invalid path: " + path)
// 		}
// 	}
//
// 	if strValue, ok := value.(string); ok {
// 		return strValue, nil
// 	}
// 	return "", errors.New("value is not a string: " + path)
// }
//
// func (sm *secretManager) setNestedValue(config map[string]interface{}, path string, value string) error {
// 	parts := strings.Split(path, ".")
// 	var current map[string]interface{} = config
//
// 	for i, part := range parts {
// 		if i == len(parts)-1 {
// 			current[part] = value
// 			return nil
// 		}
// 		if _, ok := current[part]; !ok {
// 			current[part] = make(map[string]interface{})
// 		}
// 		current = current[part].(map[string]interface{})
// 	}
// 	return nil
// }
//
// func (sm *secretManager) deleteNestedValue(config map[string]interface{}, path string) error {
// 	parts := strings.Split(path, ".")
// 	var current map[string]interface{} = config
//
// 	for i, part := range parts {
// 		if i == len(parts)-1 {
// 			delete(current, part)
// 			return nil
// 		}
// 		if next, ok := current[part].(map[string]interface{}); ok {
// 			current = next
// 		} else {
// 			return errors.New("invalid path: " + path)
// 		}
// 	}
// 	return nil
// }
//
// // Helper functions
// func LoadTokenFromConfig() (string, error) {
// 	cwd, err := os.Getwd()
// 	if err != nil {
// 		return "", fmt.Errorf("failed to get current working directory: %w", err)
// 	}
//
// 	configPath := filepath.Join(cwd, "nextdeploy.yml")
// 	data, err := ioutil.ReadFile(configPath)
// 	if err != nil {
// 		return "", fmt.Errorf("failed to read nextdeploy.yml: %w", err)
// 	}
//
// 	var config Config
// 	if err := yaml.Unmarshal(data, &config); err != nil {
// 		return "", fmt.Errorf("failed to parse nextdeploy.yml: %w", err)
// 	}
//
// 	if config.Repository.WebhookSecret == "" {
// 		return "", errors.New("webhook_secret not found in nextdeploy.yml")
// 	}
//
// 	return config.Repository.WebhookSecret, nil
// }
//
// func StoreToken(name, value string) error {
// 	cfg := &Config{
// 		Doppler: DopplerConfig{
// 			// These can be empty if not using Doppler
// 		},
// 	}
//
// 	// Create secret manager with empty path and default master key
// 	sm, err := NewSecretManager("", "default-master-key", cfg, nil)
// 	if err != nil {
// 		return fmt.Errorf("failed to create secret manager: %w", err)
// 	}
//
// 	return sm.StoreToken(name, value)
// }
