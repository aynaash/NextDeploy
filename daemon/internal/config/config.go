package config

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/aynaash/nextdeploy/daemon/internal/types"
	"github.com/aynaash/nextdeploy/shared/config"

	"gopkg.in/yaml.v3"
)

func LoadConfig(filePath string) (*types.DaemonConfig, error) {
	// Default socket lives inside the RuntimeDirectory that systemd creates
	// (/run/nextdeployd/) so ProtectSystem=strict doesn't block writes.
	socketPath := "/run/nextdeployd/nextdeployd.sock"
	logDir := "/var/log/nextdeployd"

	if os.Geteuid() != 0 {
		home, err := os.UserHomeDir()
		if err == nil {
			socketPath = home + "/.nextdeploy/daemon.sock"
			logDir = home + "/.nextdeploy/log"
		}
	}

	config := &types.DaemonConfig{
		SocketPath:      socketPath,
		SocketMode:      "0666",
		DockerSocket:    "/var/run/docker.sock",
		ContainerPrefix: "nextdeploy_",
		LogLevel:        "info",
		LogDir:          logDir,
		LogMaxSize:      10,
		LogMaxBackups:   5,
		RateLimitRate:   10,
		RateLimitBurst:  20,
	}

	if filePath != "" {
		// #nosec G304
		file, err := os.Open(filePath)
		if err == nil {
			defer file.Close()
			decoder := json.NewDecoder(file)
			if err := decoder.Decode(config); err != nil {
				return nil, err
			}
		} else if !os.IsNotExist(err) {
			return nil, err
		}
	}
	return config, nil
}

// EnsureSecuritySecret guarantees the daemon has a non-empty HMAC secret.
// If cfg.SecuritySecret is empty it generates a cryptographically random
// 32-byte secret, sets it on cfg, and persists the full config to configPath
// (mode 0600) so the local CLI client — which reads the same file — signs with
// a matching secret. This MUST only be called from the daemon startup path,
// never from the client, so the client never invents its own secret.
//
// Returns true when a new secret was generated.
func EnsureSecuritySecret(configPath string, cfg *types.DaemonConfig) (bool, error) {
	if cfg.SecuritySecret != "" {
		return false, nil
	}

	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return false, fmt.Errorf("generate security secret: %w", err)
	}
	cfg.SecuritySecret = hex.EncodeToString(buf)

	if configPath == "" {
		// No file to persist to; the in-memory secret still secures this run.
		return true, nil
	}

	if err := os.MkdirAll(filepath.Dir(configPath), 0700); err != nil {
		return true, fmt.Errorf("create config dir: %w", err)
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return true, fmt.Errorf("marshal config: %w", err)
	}
	if err := os.WriteFile(configPath, data, 0600); err != nil {
		return true, fmt.Errorf("persist config with generated secret: %w", err)
	}
	return true, nil
}

func ReadConfigInServer(path string) (*config.NextDeployConfig, error) {
	// #nosec G304
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
