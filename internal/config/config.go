package config

import (
	"bufio"
	"github.com/spf13/cobra"
	"nextdeploy/internal/logger"
	"nextdeploy/internal/nextdeploy"
	"strconv"
)

var (
	skipPrompts bool
)

func FirstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func HandleConfigSetup(cmd *cobra.Command, reader *bufio.Reader) error {
	//FIX: remove all prompts and use default values with NOTE to user to customize the generated file
	defaultConfig := true
	plog := logger.PackageLogger("Init", "NextDeploy")

	if defaultConfig {
		// If default config is requested, just generate the sample and exit
		//FIX: first check if nextdeploy.yml already exists and validate
		nextdeploy := nextdeploy.New()
		configExist, err := nextdeploy.ConfigOkay()
		if err != nil {
			plog.Error("failed to check configuration: %v", err)
			return nil
		}
		if configExist {
			plog.Warn("nextdeploy.yml already exists. Please use it to add the config values.")
			return nil
		}
		if err := GenerateSampleConfig(); err != nil {
			plog.Error("failed to generate sample config: %v", err)
			return nil
		}
		plog.Success("✅ nextdeploy.yml created")
		return nil
	}

	// Handle interactive flow when not using default config
	if !skipPrompts && PromptYesNo(reader, "Create customized nextdeploy.yml?") {
		cfg, err := InteractiveConfigPrompt(reader)
		//Print out the generated configuration
		if err != nil {
			plog.Error("failed to get configuration: %v", err)
			return nil
		}

		plog.Debug("Generated configuration: %+v", cfg)

		// Safely handle Database configuration
		if cfg.Database != nil && cfg.Database.Port != "" {
			port, err := strconv.Atoi(cfg.Database.Port)
			if err != nil {
				plog.Warn("⚠️ Invalid port number: %s, using default 5432\n", cfg.Database.Port)
				cfg.Database.Port = "5432"
			} else if port < 1 || port > 65535 {
				plog.Warn("⚠️ Port %d out of range, using default 5432\n", port)
				cfg.Database.Port = "5432"
			}
		} else if cfg.Database != nil {
			// Initialize with default port if Database exists but port is empty
			cfg.Database.Port = "5432"
		}

		if err := WriteConfig("nextdeploy.yml", cfg); err != nil {
			plog.Error("failed to write configuration: %v", err)
			return nil
		}
		plog.Success("✅ nextdeploy.yml created with your settings")

	} else {
		// Only generate sample config if user refused to create customized one
		if PromptYesNo(reader, "Generate sample configuration file for reference?") {
			if err := GenerateSampleConfig(); err != nil {
				plog.Error("failed to generate sample config: %v", err)
				return nil
			}
			plog.Success("✅ sample.nextdeploy.yml created")
		}
	}

	return nil
}
