package secrets

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type (
	SecretManager interface {
		GetSecret(path string) (string, error)
		UpdateSecret(path, value string, encrypt bool) error
		DeleteSecret(path string) error
		EncryptValue(value string) (string, error)
		DecryptValue(encryptedValue string) (string, error)
		ListSecrets() ([]string, error)
		LoadConfig() (*Config, error)
		StoreToken(name, value string) error
		GetToken(name string) (string, error)
		SyncWithDoppler() error
	}

	DopplerManager interface {
		GetSecret(path string) (string, error)
		VerifySecrets(config *Config) error
		UpdateSecret(path, value string, encrypt bool) error
		PushSecrets(config *Config) error
	}

	Logger interface {
		Debug(msg string, args ...interface{})
		Info(msg string, args ...interface{})
		Error(msg string, args ...interface{})
	}
)

type secretsManager struct {
	configPath string
	masterKey  []byte
	config     *Config
	doppler    DopplerManager
	logger     Logger
}

func NewSecretManager(configPath, masterKey string, logger Logger, useDoppler bool) (SecretManager, error) {
	// Print out the  inputs
	fmt.Printf("Creating SecretManager with configPath: %s, masterKey: %s, useDoppler: %t\n", configPath, masterKey, useDoppler)
	if logger == nil {
		return nil, fmt.Errorf("logger cannot be nil")
	}

	derivedKey, err := DeriveKey(masterKey)
	if err != nil {
		return nil, fmt.Errorf("failed to derive key: %w", err)
	}

	cfg, err := loadConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	var doppler DopplerManager
	if useDoppler {
		doppler, err = NewDopplerManager(&cfg.Doppler, logger)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize Doppler manager: %w", err)
		}
	}

	return &secretsManager{
		configPath: configPath,
		config:     cfg,
		masterKey:  derivedKey,
		doppler:    doppler,
		logger:     logger,
	}, nil
}

func (s *secretsManager) LoadConfig() (*Config, error) {
	return loadConfig(s.configPath)
}

func loadConfig(configPath string) (*Config, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}
	return &cfg, nil
}

func (s *secretsManager) saveConfig(cfg *Config) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	configDir := filepath.Dir(s.configPath)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	if err := os.WriteFile(s.configPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}
	return nil
}

func (s *secretsManager) GetSecret(path string) (string, error) {
	cfg, err := s.LoadConfig()
	if err != nil {
		return "", fmt.Errorf("failed to load config: %w", err)
	}

	if s.doppler != nil {
		return s.doppler.GetSecret(path)
	}

	value, err := getNestedValue(cfg, path)
	if err != nil {
		return "", fmt.Errorf("failed to get secret: %w", err)
	}

	if strings.HasPrefix(value, "encrypted:") {
		return s.DecryptValue(strings.TrimPrefix(value, "encrypted:"))
	}
	return value, nil
}

func (s *secretsManager) UpdateSecret(path, value string, encrypt bool) error {
	cfg, err := s.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if encrypt {
		encryptedValue, err := s.EncryptValue(value)
		if err != nil {
			return fmt.Errorf("encryption failed: %w", err)
		}
		value = "encrypted:" + encryptedValue
	}

	if s.doppler != nil {
		return s.doppler.UpdateSecret(path, value, encrypt)
	}

	if err := setNestedValue(cfg, path, value); err != nil {
		return fmt.Errorf("failed to set secret: %w", err)
	}

	return s.saveConfig(cfg)
}

func (s *secretsManager) DeleteSecret(path string) error {
	cfg, err := s.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if s.doppler != nil {
		//FIX: add doppler delete secret
		// return s.doppler.DeleteSecret(path)
		return fmt.Errorf("Doppler integration not configured for delete operation")
	}

	if err := deleteNestedValue(cfg, path); err != nil {
		return fmt.Errorf("failed to delete secret: %w", err)
	}

	return s.saveConfig(cfg)
}

func (s *secretsManager) EncryptValue(value string) (string, error) {
	encrypted, err := Encrypt([]byte(value), s.masterKey)
	if err != nil {
		return "", fmt.Errorf("encryption failed: %w", err)
	}
	return base64.StdEncoding.EncodeToString(encrypted), nil
}

func (s *secretsManager) DecryptValue(encryptedValue string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(encryptedValue)
	if err != nil {
		return "", fmt.Errorf("bled to create secret manager:/ase64 decode failed: %w", err)
	}

	decrypted, err := Decrypt(data, s.masterKey)
	if err != nil {
		return "", fmt.Errorf("decryption failed: %w", err)
	}
	return string(decrypted), nil
}

func (s *secretsManager) SyncWithDoppler() error {
	if s.doppler == nil {
		return fmt.Errorf("Doppler integration not configured")
	}

	cfg, err := s.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if err := s.doppler.VerifySecrets(cfg); err != nil {
		return fmt.Errorf("secrets verification failed: %w", err)
	}

	s.logger.Info("Successfully synced with Doppler")
	return nil
}

func (s *secretsManager) StoreToken(name, value string) error {
	cfg, err := s.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if cfg.Tokens == nil {
		cfg.Tokens = make(map[string]string)
	}

	cfg.Tokens[name] = value
	return s.saveConfig(cfg)
}

func (s *secretsManager) GetToken(name string) (string, error) {
	cfg, err := s.LoadConfig()
	if err != nil {
		return "", fmt.Errorf("failed to load config: %w", err)
	}

	if token, exists := cfg.Tokens[name]; exists {
		return token, nil
	}
	return "", fmt.Errorf("token not found: %s", name)
}
func (s *secretsManager) ListSecrets() ([]string, error) {
	cfg, err := s.LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	var secrets []string

	// Add repository secret
	if cfg.Repository.WebhookSecret != "" {
		secrets = append(secrets, "repository.webhook_secret")
	}

	// Add database secrets
	if cfg.Database.Username != "" {
		secrets = append(secrets, "database.username")
	}
	if cfg.Database.Password != "" {
		secrets = append(secrets, "database.password")
	}

	// Add other secrets
	for name := range cfg.Secrets {
		//secrets = append(secrets, "secrets."+strings.ToUpper(name))
		//secrets = append(secrets, "secrets:"+name)
		fmt.Printf("Adding secret: %s\n", name)

	}

	return secrets, nil
}

type dopplerManager struct {
	project string
	config  string
	token   string
	logger  Logger
}

func NewDopplerManager(cfg *Doppler, logger Logger) (DopplerManager, error) {
	// print out the inputs
	// check for nils to avoid panics
	if logger == nil {
		return nil, fmt.Errorf("logger cannot be nil")
	}
	if cfg == nil {
		return nil, fmt.Errorf("Doppler config cannot be nil")
	}
	if cfg.Project == "" || cfg.Config == "" || cfg.Token == "" {
		return nil, fmt.Errorf("Doppler project, config, and token are required")
	}
	// Print out the inputs
	fmt.Printf("Creating DopplerManager with project: %s, config: %s, token: %s\n", cfg.Project, cfg.Config, cfg.Token)
	if cfg == nil {
		return nil, fmt.Errorf("Doppler config is required")
	}

	requiredFields := []struct {
		value string
		name  string
	}{
		{cfg.Project, "project"},
		{cfg.Config, "config"},
		{cfg.Token, "token"},
	}

	for _, field := range requiredFields {
		if field.value == "" {
			return nil, fmt.Errorf("Doppler %s is required", field.name)
		}
	}

	return &dopplerManager{
		project: cfg.Project,
		config:  cfg.Config,
		token:   cfg.Token,
		logger:  logger,
	}, nil
}

func (dm *dopplerManager) PushSecrets(cfg *Config) error {
	dm.logger.Info("Pushing secrets to Doppler project %s, config %s", dm.project, dm.config)

	secretGroups := []struct {
		secrets map[string]string
		prefix  string
	}{
		{cfg.Secrets, ""},
		{cfg.Docker.Build.Args, "DOCKER_"},
	}

	for _, group := range secretGroups {
		for name, value := range group.secrets {
			if value == "" {
				dm.logger.Debug("Skipping empty secret %s", name)
				continue
			}

			fullName := strings.ToUpper(group.prefix + name)
			if err := dm.UpdateSecret(fullName, value, false); err != nil {
				return fmt.Errorf("failed to push secret %s: %w", fullName, err)
			}
			dm.logger.Info("Pushed secret %s", fullName)
		}
	}

	return nil
}

func (dm *dopplerManager) VerifySecrets(cfg *Config) error {
	cmd := exec.Command("doppler", "secrets", "download",
		"--project", dm.project,
		"--config", dm.config,
		"--format", "json",
		"--no-file",
	)

	if dm.token != "" {
		cmd.Env = append(os.Environ(), "DOPPLER_TOKEN="+dm.token)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("doppler command failed: %w\nOutput: %s", err, string(output))
	}

	var secrets map[string]interface{}
	if err := json.Unmarshal(output, &secrets); err != nil {
		return fmt.Errorf("failed to parse secrets: %w", err)
	}

	requiredSecrets := map[string]bool{
		"DATABASE_PASSWORD":         true,
		"DATABASE_USERNAME":         true,
		"REPOSITORY_WEBHOOK_SECRET": true,
	}

	for secret := range requiredSecrets {
		if _, exists := secrets[secret]; !exists {
			return fmt.Errorf("required secret %s is missing", secret)
		}
	}

	return nil
}

func (dm *dopplerManager) UpdateSecret(name, value string, encrypt bool) error {
	cmd := exec.Command("doppler", "secrets", "set",
		fmt.Sprintf("%s=%s", name, value),
		"--project", dm.project,
		"--config", dm.config,
		"--silent",
	)

	if dm.token != "" {
		cmd.Env = append(os.Environ(), "DOPPLER_TOKEN="+dm.token)
	}

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to set secret: %w\nOutput: %s", err, string(output))
	}

	dm.logger.Debug("Updated secret %s in Doppler", name)
	return nil
}

func (dm *dopplerManager) GetSecret(name string) (string, error) {
	cmd := exec.Command("doppler", "secrets", "get", name,
		"--project", dm.project,
		"--config", dm.config,
		"--plain",
	)

	if dm.token != "" {
		cmd.Env = append(os.Environ(), "DOPPLER_TOKEN="+dm.token)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to get secret: %w\nOutput: %s", err, string(output))
	}

	return strings.TrimSpace(string(output)), nil
}
func (dm *dopplerManager) ListSecrets() ([]string, error) {
	return nil, fmt.Errorf("ListSecrets not implemented for DopplerManager")

}
func (dm *dopplerManager) GetToken(name string) (string, error) {
	return "", fmt.Errorf("GetToken not implemented for DopplerManager")
}

func (dm *dopplerManager) DeleteSecret(name string) error {
	cmd := exec.Command("doppler", "secrets", "delete", name,
		"--project", dm.project,
		"--config", dm.config,
		"--silent",
	)

	if dm.token != "" {
		cmd.Env = append(os.Environ(), "DOPPLER_TOKEN="+dm.token)
	}

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to delete secret: %w\nOutput: %s", err, string(output))
	}

	dm.logger.Debug("Deleted secret %s from Doppler", name)
	return nil
}

// Helper functions
func getNestedValue(cfg *Config, path string) (string, error) {
	parts := strings.Split(path, ".")
	if len(parts) == 0 {
		return "", fmt.Errorf("invalid path")
	}

	switch parts[0] {
	case "repository":
		if len(parts) == 2 && parts[1] == "webhook_secret" {
			return cfg.Repository.WebhookSecret, nil
		}
	case "database":
		if len(parts) == 2 {
			switch parts[1] {
			case "password":
				return cfg.Database.Password, nil
			case "username":
				return cfg.Database.Username, nil
			}
		}
	case "secrets":
		if len(parts) == 2 {
			if secret, exists := cfg.Secrets[parts[1]]; exists {
				return secret, nil
			}
		}
	}

	return "", fmt.Errorf("path not found: %s", path)
}

func setNestedValue(cfg *Config, path, value string) error {
	parts := strings.Split(path, ".")
	if len(parts) == 0 {
		return fmt.Errorf("invalid path")
	}

	switch parts[0] {
	case "repository":
		if len(parts) == 2 && parts[1] == "webhook_secret" {
			cfg.Repository.WebhookSecret = value
			return nil
		}
	case "database":
		if len(parts) == 2 {
			switch parts[1] {
			case "password":
				cfg.Database.Password = value
				return nil
			case "username":
				cfg.Database.Username = value
				return nil
			}
		}
	case "secrets":
		if len(parts) == 2 {
			if cfg.Secrets == nil {
				cfg.Secrets = make(map[string]string)
			}
			cfg.Secrets[parts[1]] = value
			return nil
		}
	}

	return fmt.Errorf("path not found: %s", path)
}

func deleteNestedValue(cfg *Config, path string) error {
	return setNestedValue(cfg, path, "")
}
