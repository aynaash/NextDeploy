package main

import (
	"fmt"
	"gopkg.in/yaml.v3"
	"nextdeploy/shared/config"
	"os"
)

func ReadConfigInServer(path string) (*config.NextDeployConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("Config file not found: %w", err)
	}
	var cfg config.NextDeployConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("Invalid config format: %w", err)
	}
	return &cfg, nil
}
