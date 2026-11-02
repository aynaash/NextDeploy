package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/aynaash/nextdeploy/shared/secrets"
)

func main() {
	sm, err := secrets.NewSecretManager()
	if err != nil {
		fmt.Println("Failed to initialize SecretManager:", err)
		return
	}

	// Check if Doppler is enabled
	if sm.IsDopplerEnabled() {
		fmt.Println("Doppler is enabled, no need for this whole process...")
		return
	}
	// Check if the key exists
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Println("Failed to get home directory:", err)
		return
	}
	keyPath := filepath.Join(home, "app", ".nextdeploykeys", "master.key")
	master, err := os.ReadFile(keyPath)
	if err != nil {
		fmt.Printf("Failed to read encryption key from %s: %v\n", keyPath, err)
		return
	}
	// Decrypt the nextdeploy.yml file
	nextdeployyml, err := sm.DecryptFile("nextdeploy.yml.enc", master)
	if err != nil {
		fmt.Println("Failed to decrypt nextdeploy.yml:", err)
		return
	}
	// Decrypt the .env file
	envfile, err := sm.DecryptFile(".env.enc", master)
	if err != nil {
		fmt.Println("Failed to decrypt .env file:", err)
		return
	}
	// save the decrypted files
	err = os.WriteFile("nextdeploy.yml", []byte(nextdeployyml), 0600)
	if err != nil {
		fmt.Println("Failed to write decrypted nextdeploy.yml:", err)
		return
	}
	err = os.WriteFile(".env", []byte(envfile), 0600)
	if err != nil {
		fmt.Println("Failed to write decrypted .env file:", err)
		return
	}
	// Print success message
	fmt.Println("Files decrypted successfully with mode 0600.")
}
