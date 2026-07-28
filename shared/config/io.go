package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
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

	// Same boundary check as Load — LoadConfig is a duplicate entry point and
	// must not be a way around the app-name rule.
	if err := ValidateAppName(cfg.App.Name); err != nil {
		return nil, err
	}

	fmt.Printf("%s Configuration loaded successfully\n", EmojiSuccess)
	return &cfg, nil
}
