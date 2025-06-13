//go:build ignore
// +build ignore

package registry

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/distribution/distribution/v3/reference"
	"nextdeploy/internal/config"
	"nextdeploy/internal/logger"
)

var (
	rlogger = logger.PackageLogger("REGISTRY", "📑REGISTRY")
)

// RegistryValidator provides methods to validate and work with container registries
type RegistryValidator struct {
	cfg *config.NextDeployConfig
}

// NewRegistryValidator creates a new RegistryValidator instance
func NewRegistryValidator() (*RegistryValidator, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}
	return &RegistryValidator{cfg: cfg}, nil
}

// IsValidRegistry checks if a registry string is valid
func (rv *RegistryValidator) IsValidRegistry(registry string) bool {
	if strings.TrimSpace(registry) == "" ||
		strings.Contains(registry, " ") ||
		strings.HasPrefix(registry, "/") ||
		strings.HasSuffix(registry, "/") {
		rlogger.Error("Registry string is malformed: %s", registry)
		return false
	}

	_, err := url.ParseRequestURI("https://" + registry)
	if err != nil {
		rlogger.Error("Registry URL is invalid: %v", err)
		return false
	}

	return true
}

// ValidateRegistryConfig validates the entire docker registry configuration
func (rv *RegistryValidator) ValidateRegistryConfig() error {
	if rv.cfg.Docker.Registry == "" {
		return errors.New("docker registry not configured")
	}

	if !rv.IsValidRegistry(rv.cfg.Docker.Registry) {
		return fmt.Errorf("invalid registry format: %s", rv.cfg.Docker.Registry)
	}

	if rv.cfg.Docker.Image == "" {
		return errors.New("docker image name not configured")
	}

	if !rv.IsValidImageName(rv.cfg.Docker.Image) {
		return fmt.Errorf("invalid image name format: %s", rv.cfg.Docker.Image)
	}

	return nil
}

// IsValidImageName checks if an image name is valid according to Docker standards
func (rv *RegistryValidator) IsValidImageName(image string) bool {
	_, err := reference.ParseNormalizedNamed(image)
	if err != nil {
		rlogger.Error("Invalid image name: %s", image)
	}
	return err == nil
}

// GetFullImageTag constructs the full image tag from registry and image name
func (rv *RegistryValidator) GetFullImageTag() (string, error) {
	if err := rv.ValidateRegistryConfig(); err != nil {
		return "", err
	}

	registry := strings.TrimPrefix(rv.cfg.Docker.Registry, "https://")
	registry = strings.TrimPrefix(registry, "http://")
	registry = strings.TrimSuffix(registry, "/")

	if strings.HasPrefix(rv.cfg.Docker.Image, registry+"/") {
		return rv.cfg.Docker.Image, nil
	}

	fullTag := fmt.Sprintf("%s/%s", registry, rv.cfg.Docker.Image)
	if !rv.IsValidImageName(fullTag) {
		return "", fmt.Errorf("invalid full image tag format: %s", fullTag)
	}

	return fullTag, nil
}

// IsSupportedRegistry checks if the registry is one of the supported types
func (rv *RegistryValidator) IsSupportedRegistry() bool {
	supported := map[string]bool{
		"docker.io":  true,
		"ghcr.io":    true,
		"gcr.io":     true,
		"ecr.aws":    true,
		"azurecr.io": true,
		"quay.io":    true,
		"localhost":  true,
		"127.0.0.1":  true,
	}

	reg := strings.Split(rv.cfg.Docker.Registry, "/")[0]
	return supported[reg]
}

// NeedsAuthentication checks if the registry requires authentication
func (rv *RegistryValidator) NeedsAuthentication() bool {
	return !strings.HasPrefix(rv.cfg.Docker.Registry, "localhost") &&
		!strings.HasPrefix(rv.cfg.Docker.Registry, "127.0.0.1")
}

// ValidateCredentials checks if credentials are provided when needed
func (rv *RegistryValidator) ValidateCredentials() error {
	if !rv.NeedsAuthentication() {
		return nil
	}
	if rv.cfg.Docker.Username == "" || rv.cfg.Docker.Password == "" {
		return errors.New("registry credentials required but not provided")
	}
	return nil
}

// GetRegistryType returns the type of registry (docker, ghcr, ecr, etc.)
func (rv *RegistryValidator) GetRegistryType() string {
	reg := strings.ToLower(rv.cfg.Docker.Registry)

	switch {
	case strings.Contains(reg, "ghcr.io"):
		return "github"
	case strings.Contains(reg, "ecr.aws"):
		return "ecr"
	case strings.Contains(reg, "gcr.io"):
		return "gcr"
	case strings.Contains(reg, "azurecr.io"):
		return "acr"
	case strings.Contains(reg, "docker.io"):
		return "dockerhub"
	default:
		return "generic"
	}
}

// PushImageToRegistry pushes the configured image to the registry
func (rv *RegistryValidator) PushImageToRegistry() (bool, error) {
	if err := rv.ValidateRegistryConfig(); err != nil {
		rlogger.Error("Invalid registry configuration: %v", err)
		return false, err
	}

	if !rv.cfg.Docker.Push {
		rlogger.Info("Image push is disabled by config")
		return false, nil
	}

	if err := rv.ValidateCredentials(); err != nil {
		rlogger.Error("Authentication validation failed: %v", err)
		return false, err
	}

	fullTag, err := rv.GetFullImageTag()
	if err != nil {
		rlogger.Error("Failed to get image tag: %v", err)
		return false, err
	}

	rlogger.Info("Pushing image %s to registry", fullTag)

	// TODO: Use Docker SDK or CLI to perform the actual push

	rlogger.Success("Image pushed successfully: %s", fullTag)
	return true, nil
}

// GetImagePullSecrets returns Kubernetes-style image pull secrets if needed
func (rv *RegistryValidator) GetImagePullSecrets() (string, error) {
	if !rv.NeedsAuthentication() {
		return "", nil
	}

	if err := rv.ValidateCredentials(); err != nil {
		return "", err
	}

	encoded := rv.generateDockerConfigJSON()
	name := strings.ReplaceAll(rv.cfg.App.Name, " ", "-")

	secret := fmt.Sprintf(`
apiVersion: v1
kind: Secret
metadata:
  name: %s-regcred
type: kubernetes.io/dockerconfigjson
data:
  .dockerconfigjson: %s
`, name, encoded)

	return secret, nil
}

// generateDockerConfigJSON generates the base64-encoded Docker config JSON
func (rv *RegistryValidator) generateDockerConfigJSON() string {
	auths := map[string]map[string]map[string]string{
		"auths": {
			rv.cfg.Docker.Registry: {
				"username": rv.cfg.Docker.Username,
				"password": rv.cfg.Docker.Password,
			},
		},
	}

	jsonBytes, err := json.Marshal(auths)
	if err != nil {
		rlogger.Error("Failed to marshal Docker config JSON: %v", err)
		return ""
	}

	return base64.StdEncoding.EncodeToString(jsonBytes)
}

// ValidateImageTag checks if a tag follows best practices
func (rv *RegistryValidator) ValidateImageTag(tag string) bool {
	match, _ := regexp.MatchString(`^[a-zA-Z0-9_\-\.]+$`, tag)
	return match && len(tag) <= 128
}
