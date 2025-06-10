package secrets

//
// import (
// 	"encoding/json"
// 	"errors"
// 	"fmt"
// 	"gopkg.in/yaml.v3"
// 	"os"
// 	"os/exec"
// 	"path/filepath"
// )
//
// type Manager struct {
// 	Project string
// 	Config  string
// }
//
// func NewDopplerManager(dMMaannaaggeerr{
// u	return &Manager{
// 		Project: dp.Project,
// 		Config:  dp.Config,
// 	}
// }
//
// // PushSecrets pushes all secrets from nextdeploy.yml to Doppler
// func (dm *Manager)Manager error {
// 	return ExtractAndPushSecretsToDoppler(dm.Project, dm.Config)
// }
//
// // VerifySecrets verifies that all required secrets exist in Doppler
// func (dm *DopplerManager) VerifySecrets() error {
// 	return VerifyDopplerSecrets(dm.Project, dm.Config)
// }
//
// // Config represents the structure of nextdeploy.yml
// func LoadTokenFromConfig() (string, error) {
// 	cwd, err := os.Getwd()
// 	if err != nil {
// 		return "", fmt.Errorf("failed to get current working directory: %w", err)
// 	}
//
// 	configPath := filepath.Join(cwd, "nextdeploy.yml")
// 	data, err := os.ReadFile(configPath)
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
// // ExtractAndPushSecretsToDoppler extracts all secrets from nextdeploy.yml and pushes them to Doppler
// func ExtractAndPushSecretsToDoppler(dopplerProject string, dopplerConfig string) error {
// 	cwd, err := os.Getwd()
// 	if err != nil {
// 		return fmt.Errorf("failed to get current working directory: %w", err)
// 	}
//
// 	configPath := filepath.Join(cwd, "nextdeploy.yml")
// 	data, err := os.ReadFile(configPath)
// 	if err != nil {
// 		return fmt.Errorf("failed to read nextdeploy.yml: %w", err)
// 	}
//
// 	var config Config
// 	if err := yaml.Unmarshal(data, &config); err != nil {
// 		return fmt.Errorf("failed to parse nextdeploy.yml: %w", err)
// 	}
//
// 	// Collect all secrets from the config
// 	secrets := []Secret{
// 		{"REPOSITORY_WEBHOOK_SECRET", config.Repository.WebhookSecret},
// 		{"DATABASE_PASSWORD", config.Database.Password},
// 		{"DATABASE_USERNAME", config.Database.Username},
// 		{"BACKUP_ACCESS_KEY", config.Backup.Storage.AccessKey},
// 		{"BACKUP_SECRET_KEY", config.Backup.Storage.SecretKey},
// 	}
//
// 	// Add Docker build args if they exist
// 	for k, v := range config.Docker.Build.Args {
// 		secrets = append(secrets, Secret{
// 			Name:  fmt.Sprintf("DOCKER_BUILD_ARG_%s", k),
// 			Value: v,
// 		})
// 	}
//
// 	// Filter out empty secrets
// 	var nonEmptySecrets []Secret
// 	for _, secret := range secrets {
// 		if secret.Value != "" {
// 			nonEmptySecrets = append(nonEmptySecrets, secret)
// 		}
// 	}
//
// 	// Push secrets to Doppler
// 	for _, secret := range nonEmptySecrets {
// 		cmd := exec.Command("doppler", "secrets", "set",
// 			fmt.Sprintf("%s=%s", secret.Name, secret.Value),
// 			"--project", dopplerProject,
// 			"--config", dopplerConfig,
// 			"--silent",
// 		)
//
// 		output, err := cmd.CombinedOutput()
// 		if err != nil {
// 			return fmt.Errorf("failed to set secret %s in Doppler: %w\nOutput: %s", secret.Name, err, string(output))
// 		}
// 	}
//
// 	return nil
// }
//
// // VerifyDopplerSecrets verifies that all secrets exist in Doppler
// func VerifyDopplerSecrets(dopplerProject string, dopplerConfig string) error {
// 	// Get list of secrets from Doppler
// 	cmd := exec.Command("doppler", "secrets", "download",
// 		"--project", dopplerProject,
// 		"--config", dopplerConfig,
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
// 	// Load our expected secrets
// 	cwd, err := os.Getwd()
// 	if err != nil {
// 		return fmt.Errorf("failed to get current working directory: %w", err)
// 	}
//
// 	configPath := filepath.Join(cwd, "nextdeploy.yml")
// 	data, err := os.ReadFile(configPath)
// 	if err != nil {
// 		return fmt.Errorf("failed to read nextdeploy.yml: %w", err)
// 	}
//
// 	var config Config
// 	if err := yaml.Unmarshal(data, &config); err != nil {
// 		return fmt.Errorf("failed to parse nextdeploy.yml: %w", err)
// 	}
//
// 	// Check critical secrets exist in Doppler
// 	requiredSecrets := []string{
// 		"DATABASE_PASSWORD",
// 		"DATABASE_USERNAME",
// 	}
//
// 	for _, secretName := range requiredSecrets {
// 		if _, exists := dopplerSecrets[secretName]; !exists {
// 			return fmt.Errorf("required secret %s not found in Doppler", secretName)
// 		}
// 	}
//
// 	return nil
// }
// func (dm *DopplerManager) PushSecret(path string, value string) error {
// 	// Implementation for pushing to Doppler
// }
