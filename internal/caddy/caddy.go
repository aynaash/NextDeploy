package caddy

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// CaddyManager provides simplified Caddy server configuration management
type CaddyManager struct {
	adminAPI string       // Caddy admin API endpoint (e.g., "http://localhost:2019")
	client   *http.Client // HTTP client for API requests
}

// New creates a new CaddyManager instance
func New(adminAPI string) *CaddyManager {
	return &CaddyManager{
		adminAPI: strings.TrimSuffix(adminAPI, "/"),
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Config represents a Caddy configuration (can be either JSON or Caddyfile format)
type Config struct {
	Content string // The configuration content
	Format  string // "json" or "caddyfile"
}

// GetConfig retrieves the current Caddy configuration
func (cm *CaddyManager) GetConfig(ctx context.Context) (*Config, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", cm.adminAPI+"/config/", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := cm.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get config: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	return &Config{
		Content: string(body),
		Format:  "json", // Caddy admin API always returns JSON
	}, nil
}

// LoadConfig loads a configuration from a file
func (cm *CaddyManager) LoadConfig(ctx context.Context, filePath string) (*Config, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	format := "caddyfile"
	if strings.HasSuffix(filePath, ".json") {
		format = "json"
	}

	return &Config{
		Content: string(content),
		Format:  format,
	}, nil
}

// SaveConfig saves the configuration to a file
func (cm *CaddyManager) SaveConfig(ctx context.Context, config *Config, filePath string) error {
	return os.WriteFile(filePath, []byte(config.Content), 0644)
}

// ApplyConfig applies a new configuration to Caddy
func (cm *CaddyManager) ApplyConfig(ctx context.Context, config *Config) error {
	var (
		url  string
		body io.Reader
	)

	switch config.Format {
	case "json":
		url = cm.adminAPI + "/config/"
		body = strings.NewReader(config.Content)
	case "caddyfile":
		url = cm.adminAPI + "/load"
		body = strings.NewReader(config.Content)
	default:
		return fmt.Errorf("unsupported config format: %s", config.Format)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, body)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	if config.Format == "caddyfile" {
		req.Header.Set("Content-Type", "text/caddyfile")
	} else {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := cm.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to apply config: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errorBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to apply config (status %d): %s", resp.StatusCode, string(errorBody))
	}

	return nil
}

// ValidateConfig validates a configuration without applying it
func (cm *CaddyManager) ValidateConfig(ctx context.Context, config *Config) error {
	if config.Format != "caddyfile" {
		return fmt.Errorf("validation is only supported for caddyfile format")
	}

	req, err := http.NewRequestWithContext(ctx, "POST", cm.adminAPI+"/validate", strings.NewReader(config.Content))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "text/caddyfile")

	resp, err := cm.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to validate config: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errorBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("config validation failed (status %d): %s", resp.StatusCode, string(errorBody))
	}

	return nil
}

// GetConfigAsCaddyfile retrieves the current config in Caddyfile format
func (cm *CaddyManager) GetConfigAsCaddyfile(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", cm.adminAPI+"/config/caddyfile", nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := cm.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to get caddyfile: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	return string(body), nil
}

// PatchConfig applies a partial configuration update
func (cm *CaddyManager) PatchConfig(ctx context.Context, path string, config interface{}) error {
	jsonData, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "PATCH", cm.adminAPI+path, strings.NewReader(string(jsonData)))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := cm.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to patch config: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errorBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to patch config (status %d): %s", resp.StatusCode, string(errorBody))
	}

	return nil
}
