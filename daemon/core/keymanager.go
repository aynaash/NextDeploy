package core

import (
	"log/slog"
	"os"
)

func setupKeyManager(logger *slog.Logger) (*KeyManager, error) {
	logger.Info("initializing key manager", "key_dir", config.keyDir, "rotation_interval", config.rotateFreq)

	if err := os.MkdirAll(config.keyDir, 0700); err != nil {
		logger.Error("failed to create key directory", "error", err)
		return nil, err
	}

	keyManager, err := core.NewKeyManager(config.keyDir, config.rotateFreq)
	if err != nil {
		logger.Error("failed to initialize key manager", "error", err)
		return nil, err
	}

	keyManager.StartRotation()
	logger.Info("key manager initialized successfully")
	return keyManager, nil
}
