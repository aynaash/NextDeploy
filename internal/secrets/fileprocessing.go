package secrets

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// TODO: ✅ Allow selection of which .env files to encrypt via pattern or CLI arg.
// TODO: 🔐 Don't overwrite original files by default — add backup logic or prompt.
// TODO: ⚠️ Add versioning or checksum for encrypted file verification.
func (sm *SecretManager) EncryptEnvFile(masterKey string) (map[string]string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get current working directory: %w", err)
	}

	files, err := filepath.Glob(filepath.Join(cwd, "*.env"))
	if err != nil {
		return nil, fmt.Errorf("failed to find .env files: %w", err)
	}

	if len(files) == 0 {
		SLogs.Warn("No .env files found in the current directory")
		return nil, nil
	}

	results := make(map[string]string)
	for _, file := range files {
		SLogs.Info("Encrypting .env file: %s", file)

		content, err := os.ReadFile(file)
		if err != nil {
			SLogs.Error("Failed to read .env file %s: %v", file, err)
			continue
		}
		var masterKey string

		encrypted, err := Encrypt(content, []byte(masterKey))
		if err != nil {
			SLogs.Error("Failed to encrypt file %s: %v", file, err)
			return nil, fmt.Errorf("%w: %v", ErrEncryptionFailed, err)
		}

		encryptedFile := file + ".enc"
		if err := os.WriteFile(encryptedFile, encrypted, 0600); err != nil {
			SLogs.Error("Failed to write encrypted file %s: %v", encryptedFile, err)
			return nil, err
		}

		results[file] = encryptedFile
	}

	return results, nil
}
func (sm *SecretManager) EncryptFile(filename string, key []byte) error {
	// Validate key size first
	if len(key) != 16 && len(key) != 24 && len(key) != 32 {
		SLogs.Debug("The at encrypting look like this: %s", key)
		return fmt.Errorf("invalid AES key size: %d bytes (must be 16, 24, or 32 bytes)", len(key))
	}

	// Read the file content
	content, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("failed to read file %s: %w", filename, err)
	}

	// Create cipher block
	block, err := aes.NewCipher(key)
	if err != nil {
		return fmt.Errorf("failed to create cipher: %w", err)
	}

	// Create GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return fmt.Errorf("failed to create GCM: %w", err)
	}

	// Generate nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Encrypt and prepend nonce
	ciphertext := gcm.Seal(nonce, nonce, content, nil)

	// Write encrypted file
	encryptedFilename := filename + ".enc"
	if err := os.WriteFile(encryptedFilename, ciphertext, 0600); err != nil {
		return fmt.Errorf("failed to write encrypted file %s: %w", encryptedFilename, err)
	}

	// Consider adding original file to .gitignore
	if err := addToGitignore(); err != nil {
		SLogs.Warn("Failed to add %s to .gitignore: %v", filename, err)
	}

	// Optionally remove original file (uncomment if needed)
	// if err := os.Remove(filename); err != nil {
	//     return fmt.Errorf("failed to remove original file: %w", err)
	// }

	SLogs.Info("Successfully encrypted file %s to %s", filename, encryptedFilename)
	return nil
}

// Helper function to add file to .gitignore
func addToGitignore() error {
	// Implement logic to add filename to .gitignore
	// Example: append to .gitignore file in same directory
	return nil
}

func (sm *SecretManager) DecryptFile(filename string, key []byte) error {
	if !strings.HasSuffix(filename, ".enc") {
		return fmt.Errorf("file %s is not an encrypted file", filename)
	}
	// Read the encrypted file content
	content, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("failed to read encrypted file %s: %w", filename, err)
	}
	SLogs.Debug("The key passed to new cipher function is:%s", key)
	block, err := aes.NewCipher(key)
	if err != nil {
		return fmt.Errorf("failed to create cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return fmt.Errorf("failed to create GCM: %w", err)
	}
	nonceSize := gcm.NonceSize()
	if len(content) < nonceSize {
		return fmt.Errorf("encrypted file %s is too short", filename)
	}
	nonce, ciphertext := content[:nonceSize], content[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return fmt.Errorf("failed to decrypt file %s: %w", filename, err)
	}
	// Write the decrypted content to a new filename
	decryptedFilename := strings.TrimSuffix(filename, ".enc")
	if err := os.WriteFile(decryptedFilename, plaintext, 0644); err != nil {
		return fmt.Errorf("failed to write decrypted file %s: %w", decryptedFilename, err)
	}
	SLogs.Info("Decrypted file written to %s", decryptedFilename)
	return nil
}

func (sm *SecretManager) processEnvFiles(cwd, key string, config *ConfigFile) ([]*EnvFile, error) {
	var processedFiles []*EnvFile

	// Find all encrypted env files
	files, err := filepath.Glob(filepath.Join(cwd, "*.env.enc"))
	if err != nil {
		return nil, fmt.Errorf("failed to find env files: %w", err)
	}

	if len(files) == 0 {
		SLogs.Warn("No encrypted .env files found in directory")
		return nil, nil
	}

	// Process each file
	for _, file := range files {
		envFile, err := sm.processSingleEnvFile(file, key)
		if err != nil {
			SLogs.Error("Failed to process %s: %v", file, err)
			continue
		}
		processedFiles = append(processedFiles, envFile)
	}

	return processedFiles, nil
}

// processSingleEnvFile handles decryption of a single environment file
func (sm *SecretManager) processSingleEnvFile(path, key string) (*EnvFile, error) {
	SLogs.Debug("Processing env file: %s", path)

	encContent, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read failed: %w", err)
	}

	decryptedContent, err := Decrypt(encContent, []byte(key))
	if err != nil {
		return nil, fmt.Errorf("decryption failed: %w", err)
	}

	decryptedPath := strings.TrimSuffix(path, ".enc")
	if err := secureWriteFile(decryptedPath, decryptedContent); err != nil {
		return nil, fmt.Errorf("write failed: %w", err)
	}

	SLogs.Info("Decrypted env file written to %s", decryptedPath)
	return &EnvFile{
		EncryptedPath: path,
		DecryptedPath: decryptedPath,
		Content:       decryptedContent,
	}, nil
}

// secureWriteFile safely writes content to a file
func secureWriteFile(path string, content []byte) error {
	// TODO: 🧼 Memory wipe old temp file securely before overwriting, if applicable.
	// TODO: 🧪 Check file integrity post-write (hash comparison).
	// 1. Write to temp file first
	tempPath := path + ".tmp"
	if err := os.WriteFile(tempPath, content, 0600); err != nil {
		return err
	}

	// 2. Verify the temp file
	if _, err := os.Stat(tempPath); os.IsNotExist(err) {
		return errors.New("temp file not created")
	}

	// 3. Rename (atomic operation)
	return os.Rename(tempPath, path)
}

// cleanupDecryptedFiles removes decrypted files
func (sm *SecretManager) cleanupDecryptedFiles(paths []string) {
	// TODO: 🔐 Use secure delete (overwrite + unlink) if applicable.
	for _, path := range paths {
		if path == "" {
			continue
		}
		if err := os.Remove(path); err != nil {
			SLogs.Warn("Failed to cleanup %s: %v", path, err)
		} else {
			SLogs.Debug("Cleaned up decrypted file: %s", path)
		}
	}
}
func (sm *SecretManager) ProcessConfigFile(cwd, key string) (*ConfigFile, error) {
	const configFileName = "nextdeploy.yml"
	encryptedPath := filepath.Join(cwd, configFileName+".enc")

	// Verify config file exists
	if _, err := os.Stat(encryptedPath); os.IsNotExist(err) {
		// We can encrypt the file here
		_, err := os.ReadFile(filepath.Join(cwd, configFileName))
		if err != nil {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
	}

	// Read and decrypt config
	err := sm.DecryptFile("nextdeploy.yml.enc", []byte(key))
	if err != nil {
		return nil, fmt.Errorf("config decryption failed: %w", err)
	}

	return &ConfigFile{
		EncryptedPath: encryptedPath,
	}, nil
}
