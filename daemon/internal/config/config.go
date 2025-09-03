package config

import (
	"encoding/json"
	"fmt"
	"gopkg.in/yaml.v3"
	"nextdeploy/daemon/internal/types"
	"nextdeploy/shared/config"
	"os"
)

func LoadConfig(filePath string) (*types.DaemonConfig, error) {
	config := &types.DaemonConfig{
		SocketPath:      "/var/run/nextdeployd.sock",
		SocketMode:      "0666",
		DockerSocket:    "/var/run/docker.sock",
		ContainerPrefix: "nextdeploy_",
		LogLevel:        "info",
		LogDir:          "/var/log/nextdeploy",
		LogMaxSize:      10,
		LogMaxBackups:   5,
	}

	if filePath == "" {
		file, err := os.Open(filePath)
		if err != nil {
			return nil, err
		}
		defer file.Close()

		decoder := json.NewDecoder(file)

		if err := decoder.Decode(config); err != nil {
			return nil, err
		}
	}
	return config, nil
}

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
