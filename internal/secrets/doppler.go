package secrets

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"gopkg.in/yaml.v3"
)

// LoadTokenFromConfig loads the webhook secret from nextdeploy.yml
func LoadTokenFromConfig() (string, error) {
	config, err := loadConfig()
	if err != nil {
		return "", err
	}

	if config.Repository.WebhookSecret == "" {
		return "", errors.New("webhook_secret not found in nextdeploy.yml")
	}

	return config.Repository.WebhookSecret, nil
}

// ExtractAndPushSecretsToDoppler extracts secrets from config and pushes them to Doppler
func ExtractAndPushSecretsToDoppler(dopplerProject, dopplerConfig string) error {
	config, err := loadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Collect all secrets
	secrets := []Secret{
		{"REPOSITORY_WEBHOOK_SECRET", config.Repository.WebhookSecret},
		{"DATABASE_PASSWORD", config.Database.Password},
		{"DATABASE_USERNAME", config.Database.Username},
		{"BACKUP_ACCESS_KEY", config.Backup.Storage.AccessKey},
		{"BACKUP_SECRET_KEY", config.Backup.Storage.SecretKey},
	}

	// Add docker build args if they exist
	for k, v := range config.Docker.Build.Args {
		secrets = append(secrets, Secret{
			Name:  fmt.Sprintf("DOCKER_BUILD_ARG_%s", k),
			Value: v,
		})
	}

	// Filter out empty secrets
	var nonEmptySecrets []Secret
	for _, s := range secrets {
		if s.Value != "" {
			nonEmptySecrets = append(nonEmptySecrets, s)
		}
	}

	// Push secrets to Doppler
	for _, secret := range nonEmptySecrets {
		cmd := exec.Command("doppler", "secrets", "set",
			secret.Name,
			"--config", dopplerConfig,
			"--project", dopplerProject,
			"--silent",
		)
		cmd.Env = append(os.Environ(), fmt.Sprintf("DOPPLER_VALUE=%s", secret.Value))

		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to set secret %s in Doppler: %w\nOutput: %s",
				secret.Name, err, string(output))
		}
	}

	return nil
}

// VerifyDopplerSecrets checks that required secrets exist in Doppler
func VerifyDopplerSecrets(dopplerProject, dopplerConfig string) error {
	// Get a list of secrets from Doppler
	cmd := exec.Command("doppler", "secrets", "download",
		"--project", dopplerProject,
		"--config", dopplerConfig,
		"--format", "json",
		"--no-file",
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to download secrets from Doppler: %w\nOutput: %s", err, string(output))
	}

	var dopplerSecrets map[string]interface{}
	if err := json.Unmarshal(output, &dopplerSecrets); err != nil {
		return fmt.Errorf("failed to parse Doppler secrets: %w", err)
	}

	// Check critical secrets exist in Doppler
	criticalSecrets := []string{
		"DATABASE_URL",
		"DATABASE_PASSWORD",
		"DATABASE_USERNAME",
	}

	for _, name := range criticalSecrets {
		if _, exists := dopplerSecrets[name]; !exists {
			return fmt.Errorf("critical secret %s not found in Doppler", name)
		}
	}

	return nil
}

// loadConfig loads and parses the nextdeploy.yml file
func loadConfig() (*Config, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get current working directory: %w", err)
	}

	configPath := filepath.Join(cwd, "nextdeploy.yml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read nextdeploy.yml: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse nextdeploy.yml: %w", err)
	}

	return &config, nil
}
