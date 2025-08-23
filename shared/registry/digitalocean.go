package registry

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"nextdeploy/shared"
	"nextdeploy/shared/config"
	"nextdeploy/shared/envstore"
	"nextdeploy/shared/git"
	"os"
	"os/exec"
	"strings"
)

var (
	oceanlogs = shared.PackageLogger("DIGITALOCEAN", "DIGITALOCEAN")
)

func DigitalOceanRegistry(ctx context.Context) error {
	// Ensure the registry is set up
	if err := ensureRegistrySetup(); err != nil {
		oceanlogs.Error("Failed to ensure registry setup: %v", err)
		return fmt.Errorf("failed to ensure registry setup: %w", err)
	}
	// Push the Docker image to the DigitalOcean registry
	if err := pushDockerImage(); err != nil {
		oceanlogs.Error("Failed to push Docker image: %v", err)
		return fmt.Errorf("failed to push Docker image: %w", err)
	}
	return nil
}
func ensureRegistrySetup() error {
	cfg, err := config.Load()
	if err != nil {
		oceanlogs.Error("Failed to load config: %v", err)
		return fmt.Errorf("failed to load config: %w", err)
	}
	// Check if the registry is already set up
	registryURL := cfg.Docker.Registry
	digitalocean := strings.Contains(registryURL, "digitalocean.com")

	if digitalocean && registryURL != "" {
		fmt.Println("DigitalOcean registry is already set up:", registryURL)
		return nil
	}
	store, err := envstore.New(envstore.WithEnvFile[string](".env"))
	oceanlogs.Info("Loading .env file for DigitalOcean registry setup:%v", store)
	if err != nil {
		oceanlogs.Error("Failed to load .env file: %v", err)
		return fmt.Errorf("failed to load .env file: %w", err)
	}
	token, _ := store.GetEnv("DIGITALOCEAN_TOKEN")
	oceanlogs.Debug("DigitalOcean token: %s", token)
	if token == "" {
		oceanlogs.Error("DigitalOcean token is not set in .env file")
		return errors.New("digitalocean token is not set in .env file")
	}
	result, err := GetRegistryInfo(token)
	oceanlogs.Info("DigitalOcean registry info: %s", string(result))
	if err != nil {
		oceanlogs.Error("Failed to get registry info: %v", err)
		return fmt.Errorf("failed to get registry info: %w", err)
	}

	fmt.Println("DigitalOcean registry info:", string(result))
	return nil
}

func GetRegistryInfo(token string) ([]byte, error) {
	// create a http clinent
	client := &http.Client{}
	// create request
	req, err := http.NewRequest("GET", "https://api.digitalocean.com/v2/registry", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	// set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))

	// send request
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get registry info: %s", resp.Status)
	}
	// read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}
	return body, nil
}
func pushDockerImage() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	registryURL := cfg.Docker.Registry
	if registryURL == "" {
		return errors.New("registry URL is not set")
	}
	commit, err := git.GetCommitHash()
	name := cfg.Docker.Image + ":" + commit
	oceanlogs.Info("Pushing Docker image to DigitalOcean registry: %s", name)
	imageName := fmt.Sprintf("%s/%s", registryURL, name)
	fmt.Println("Pushing Docker image:", imageName)

	// tag the Docker image
	cmdTag := exec.Command("docker", "tag", name, imageName)
	cmdTag.Stdout = os.Stdout
	cmdTag.Stderr = os.Stderr
	if err := cmdTag.Run(); err != nil {
		return fmt.Errorf("failed to tag Docker image: %w", err)
	}

	// Push the Docker image to the DigitalOcean registry
	cmd := exec.Command("docker", "push", imageName)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to push Docker image: %w", err)
	}

	fmt.Println("Docker image pushed successfully:", imageName)
	return nil
}
