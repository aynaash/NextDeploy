package core

import (
	"log/slog"
	"os"
	"time"
)

func SetupKeyManager(logger *slog.Logger, keyDir string, rotateFreq time.Duration) (*KeyManager, error) {
	logger.Info("initializing key manager", "key_dir", keyDir, "rotation_interval", rotateFreq)

	if err := os.MkdirAll(keyDir, 0700); err != nil {
		logger.Error("failed to create key directory", "error", err)
		return nil, err
	}

	keyManager, err := NewKeyManager(keyDir, rotateFreq)
	if err != nil {
		logger.Error("failed to initialize key manager", "error", err)
		return nil, err
	}

	keyManager.StartRotation()
	logger.Info("key manager initialized successfully")
	return keyManager, nil
}
