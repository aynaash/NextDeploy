package config

import (
	"encoding/json"
	"fmt"
	"gopkg.in/yaml.v3"
	"os"
)

func Save(cfg *NextDeployConfig, path string) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
func Load() (*NextDeployConfig, error) {
	data, err := os.ReadFile(ConfigFile)
	if err != nil {
		return nil, fmt.Errorf("%s Config file not found: %w", EmojiWarning, err)
	}

	var cfg NextDeployConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("%s Invalid config format: %w", EmojiWarning, err)
	}

	fmt.Printf("%s Configuration loaded successfully\n", EmojiSuccess)
	return &cfg, nil
}

func ReadConfigInServer(path string) (*NextDeployConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("Config file not found: %w", err)
	}
	var cfg NextDeployConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("Invalid config format: %w", err)
	}
	return &cfg, nil
}
