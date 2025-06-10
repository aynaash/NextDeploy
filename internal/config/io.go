package config

import (
	"fmt"
	"gopkg.in/yaml.v3"
	"os"
)

func LoadConfig() (*NextDeployConfig, error) {
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
