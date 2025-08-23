package main

import (
	"fmt"
	"nextdeploy/shared/envstore"
	"os"
)

func handleDigitalOceanRegistryAuth() {
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Printf("Failed to get user home directory: %v\n", err)
		return
	}
	envPath := home + "app/.env"
	if err != nil {
		fmt.Printf("Failed to construct env file path: %v\n", err)
		return
	}
	_, err = os.Stat(envPath)
	if os.IsNotExist(err) {
		fmt.Printf(".env file does not exist at path: %s\n", envPath)
		return
	} else if err != nil {
		fmt.Printf("Failed to check .env file: %v\n", err)
		return
	}
	store, err := envstore.New(envstore.WithEnvFile[string](envPath))
	if err != nil {
		fmt.Printf("Failed to load .env file: %v\n", err)
		return
	}
	token, _ := store.GetEnv("DIGITALOCEAN_TOKEN")
	if token == "" {
		fmt.Println("DigitalOcean token is not set in .env file")
		return
	}
	fmt.Printf("DigitalOcean token: %s\n", token)

}
