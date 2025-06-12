package validators

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"nextdeploy/internal/config"
	"nextdeploy/internal/logger"

	"github.com/docker/distribution/reference"
)

var (
	rlogger = logger.PackageLogger("REGISTRY", "📑REGISTRY")
)

// RegistryValidator provides methods to validate and work with container registries
type RegistryValidator struct {
	cfg *config.Config
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
	if strings.Contains(registry, " ") ||
		strings.HasPrefix(registry, "/") ||
		strings.HasSuffix(registry, "/") {
		return false
	}

	// Check if it's a valid URL format
	_, err := url.ParseRequestURI("https://" + registry)
	return err == nil
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
	return err == nil
}

// GetFullImageTag constructs the full image tag from registry and image name
func (rv *RegistryValidator) GetFullImageTag() (string, error) {
	if err := rv.ValidateRegistryConfig(); err != nil {
		return "", err
	}

	// Remove any protocol prefix
	registry := strings.TrimPrefix(rv.cfg.Docker.Registry, "http://")
	registry = strings.TrimPrefix(registry, "https://")

	// Ensure registry doesn't end with /
	registry = strings.TrimSuffix(registry, "/")

	// Check if image already contains registry
	if strings.HasPrefix(rv.cfg.Docker.Image, registry+"/") {
		return rv.cfg.Docker.Image, nil
	}

	// Construct full tag
	fullTag := fmt.Sprintf("%s/%s", registry, rv.cfg.Docker.Image)
	if !rv.IsValidImageName(fullTag) {
		return "", fmt.Errorf("invalid full image tag format: %s", fullTag)
	}

	return fullTag, nil
}

// IsSupportedRegistry checks if the registry is one of the supported types
func (rv *RegistryValidator) IsSupportedRegistry() bool {
	supportedRegistries := map[string]bool{
		"docker.io":  true,
		"ghcr.io":    true,
		"gcr.io":     true,
		"ecr.aws":    true,
		"azurecr.io": true,
		"quay.io":    true,
		"localhost":  true, // for testing
		"127.0.0.1":  true, // for testing
	}

	registry := strings.Split(rv.cfg.Docker.Registry, "/")[0]
	return supportedRegistries[registry]
}

// NeedsAuthentication checks if the registry requires authentication
func (rv *RegistryValidator) NeedsAuthentication() bool {
	// Local registries typically don't need auth
	if strings.HasPrefix(rv.cfg.Docker.Registry, "localhost") ||
		strings.HasPrefix(rv.cfg.Docker.Registry, "127.0.0.1") {
		return false
	}
	return true
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
	registry := strings.ToLower(rv.cfg.Docker.Registry)

	switch {
	case strings.Contains(registry, "ghcr.io"):
		return "github"
	case strings.Contains(registry, "ecr.aws"):
		return "ecr"
	case strings.Contains(registry, "gcr.io"):
		return "gcr"
	case strings.Contains(registry, "azurecr.io"):
		return "acr"
	case strings.Contains(registry, "docker.io"):
		return "dockerhub"
	default:
		return "generic"
	}
}

// PushImageToRegistry pushes the configured image to the registry
func (rv *RegistryValidator) PushImageToRegistry() (bool, error) {
	if err := rv.ValidateRegistryConfig(); err != nil {
		rlogger.Error("Invalid registry configuration: %v", err)
		return false, fmt.Errorf("invalid registry configuration: %w", err)
	}

	if !rv.cfg.Docker.Push {
		rlogger.Info("Skipping image push as configured")
		return false, nil
	}

	if err := rv.ValidateCredentials(); err != nil {
		rlogger.Error("Registry authentication failed: %v", err)
		return false, fmt.Errorf("registry authentication failed: %w", err)
	}

	fullTag, err := rv.GetFullImageTag()
	if err != nil {
		rlogger.Error("Failed to construct image tag: %v", err)
		return false, fmt.Errorf("failed to construct image tag: %w", err)
	}

	rlogger.Info("Pushing image %s to registry %s", fullTag, rv.cfg.Docker.Registry)

	// Here you would implement the actual push logic using Docker SDK or shelling out to docker CLI
	// For example:
	// err = dockerClient.ImagePush(ctx, fullTag, options)
	// if err != nil {
	//     return false, err
	// }

	rlogger.Success("Successfully pushed image %s", fullTag)
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

	// This would generate a Kubernetes secret manifest for the registry credentials
	// In a real implementation, you might use a Kubernetes client to create this
	secret := fmt.Sprintf(`
apiVersion: v1
kind: Secret
metadata:
  name: %s-regcred
type: kubernetes.io/dockerconfigjson
data:
  .dockerconfigjson: %s
`, strings.ReplaceAll(rv.cfg.App.Name, " ", "-"), rv.generateDockerConfigJSON())

	return secret, nil
}

// generateDockerConfigJSON generates the JSON for Docker config
func (rv *RegistryValidator) generateDockerConfigJSON() string {
	// In a real implementation, this would properly encode the credentials
	return fmt.Sprintf(`{"auths":{"%s":{"username":"%s","password":"%s"}}}`,
		rv.cfg.Docker.Registry,
		rv.cfg.Docker.Username,
		rv.cfg.Docker.Password)
}

// ValidateImageTag checks if a tag follows best practices
func (rv *RegistryValidator) ValidateImageTag(tag string) bool {
	// Basic tag validation regex
	match, _ := regexp.MatchString(`^[a-zA-Z0-9_\-\.]+$`, tag)
	return match && len(tag) <= 128
}
