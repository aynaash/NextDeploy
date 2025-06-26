package secrets

import (
	"fmt"
	"os"
)

func (sm *SecretManager) PrepareAppContext(key string) (string, error) {
	// TODO: ✂️ Split into smaller interfaces: StorageProvider, CryptoProvider, ValidationProvider.
	// Get and validate working directory
	cwd, err := os.Getwd()
	if err != nil {
		SLogs.Error("Failed to get working directory: %v", err)
		return "", fmt.Errorf("failed to get working directory: %w", err)
	}
	SLogs.Debug("Current working directory: %s", cwd)

	// Generate encryption key

	// Process nextdeploy.yml
	// ndConfig, err := sm.ProcessConfigFile(cwd, key)
	// if err != nil {
	// 	return "", fmt.Errorf("config processing failed: %w", err)
	//}

	// Process environment files
	// envFiles, err := sm.processEnvFiles(cwd, key, ndConfig)
	// if err != nil {
	// 	return "", fmt.Errorf("env file processing failed: %w", err)
	// }

	// // Prepare cleanup defer function
	// var filesToCleanup []string
	// for _, envFile := range envFiles {
	// 	filesToCleanup = append(filesToCleanup, envFile.DecryptedPath)
	// }
	// filesToCleanup = append(filesToCleanup, ndConfig.DecryptedPath)
	// defer sm.cleanupDecryptedFiles(filesToCleanup)
	//
	// SLogs.Info("Application context prepared successfully")
	return cwd, nil
}
