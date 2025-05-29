
package config

import (
	"fmt"
	"os"
	"gopkg.in/yaml.v3"
)



const configFile = ".nextdeploy.yml"

func Load() (*AppConfig, error) {
	data, err := os.ReadFile(configFile)
	if err != nil {
		return nil, fmt.Errorf("cannot read config file: %w", err)
	}

	var cfg AppConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("cannot parse config: %w", err)
	}

	return &cfg, nil
}

func Save(cfg *AppConfig) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("cannot marshal config: %w", err)
	}
	return os.WriteFile(configFile, data, 0644)
}
