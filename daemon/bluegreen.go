package main

import (
	"fmt"
	"log"
	"math"
	"net/http"
	"nextdeploy/shared/config"
	"os/exec"
	"strings"
	"time"
)

func SetupBlueGreenDeployment() error {
	cfg, err := config.ReadConfigInServer("/home/app/.nextdeplo.yaml")
	if err != nil {
		return fmt.Errorf("failed to read config: %w", err)
	}

	// Determine current deployment color
	currentColor, err := detectCurrentDeploymentColor(cfg.App.Name)
	if err != nil {
		return fmt.Errorf("failed to detect current deployment: %w", err)
	}

	newColor := getNextColor(currentColor)
	log.Printf("Current color: %s, New color: %s", currentColor, newColor)

	// Deploy new container
	if err := deployNewContainer(cfg, newColor); err != nil {
		return fmt.Errorf("failed to deploy new container: %w", err)
	}

	// Verify new container health
	if err := verifyContainerHealth(newColor, getPortForColor(newColor)); err != nil {
		return fmt.Errorf("new container failed health check: %w", err)
	}

	// Update load balancer configuration
	if err := updateLoadBalancerConfig(cfg, newColor); err != nil {
		return fmt.Errorf("failed to update load balancer: %w", err)
	}

	// Clean up old deployment
	if currentColor != "" {
		if err := cleanupOldDeployment(cfg.App.Name, currentColor); err != nil {
			log.Printf("Warning: failed to clean up old deployment: %v", err)
		}
	}

	return nil
}

// Helper functions for better modularity and testing
func detectCurrentDeploymentColor(appName string) (string, error) {
	cmd := exec.Command("docker", "ps", "--format", "{{.Names}}", "--filter", fmt.Sprintf("name=%s", appName))
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to list containers: %w", err)
	}

	containers := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, container := range containers {
		if strings.Contains(container, "blue") {
			return "blue", nil
		} else if strings.Contains(container, "green") {
			return "green", nil
		}
	}
	return "", nil // No current deployment
}

func getNextColor(currentColor string) string {
	if currentColor == "blue" {
		return "green"
	}
	return "blue"
}

func getPortForColor(color string) string {
	if color == "blue" {
		return "8080"
	}
	return "8081"
}

func getImageForColor(baseImage, color string) string {
	return fmt.Sprintf("%s:%s", baseImage, color)
}

func deployNewContainer(cfg *config.NextDeployConfig, color string) error {
	containerName := fmt.Sprintf("%s-%s", cfg.App.Name, color)
	port := getPortForColor(color)
	image := getImageForColor(cfg.Docker.Image, color)

	cmd := exec.Command("docker", "run",
		"-d",
		"--name", containerName,
		"-p", fmt.Sprintf("%s:3000", port),
		"--health-cmd", "curl -f http://localhost:3000/health || exit 1",
		"--health-interval", "5s",
		"--health-timeout", "3s",
		"--health-retries", "3",
		image,
	)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to start container %s: %w", containerName, err)
	}

	log.Printf("New container %s started on port %s", containerName, port)
	return nil
}

func verifyContainerHealth(color, port string) error {
	healthURL := fmt.Sprintf("http://localhost:%s/health", port)

	// Retry with exponential backoff
	retries := 5
	for i := 0; i < retries; i++ {
		resp, err := http.Get(healthURL)
		if err == nil && resp.StatusCode == http.StatusOK {
			resp.Body.Close()
			log.Printf("Container %s health check passed", color)
			return nil
		}

		if resp != nil {
			resp.Body.Close()
		}

		time.Sleep(time.Duration(math.Pow(2, float64(i))) * time.Second)
	}

	return fmt.Errorf("container %s failed health check after %d retries", color, retries)
}

func updateLoadBalancerConfig(cfg *config.NextDeployConfig, newColor string) error {
	// Update Caddyfile or other load balancer configuration
	// This should be more sophisticated in production
	cmd := exec.Command("caddy", "reload", "--config", "/etc/caddy/Caddyfile")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to reload load balancer: %w", err)
	}
	return nil
}

func cleanupOldDeployment(appName, oldColor string) error {
	containerName := fmt.Sprintf("%s-%s", appName, oldColor)

	// Stop container
	stopCmd := exec.Command("docker", "stop", containerName)
	if err := stopCmd.Run(); err != nil {
		return fmt.Errorf("failed to stop container %s: %w", containerName, err)
	}

	// Remove container
	rmCmd := exec.Command("docker", "rm", containerName)
	if err := rmCmd.Run(); err != nil {
		return fmt.Errorf("failed to remove container %s: %w", containerName, err)
	}

	log.Printf("Cleaned up old container: %s", containerName)
	return nil
}
