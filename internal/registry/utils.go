package registry

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

// RegistryValidator provides methods to validate and work with container registries
type RegistryValidator struct {
	Registry string
	Image    string
	Username string
	Password string
	AppName  string
}

// NewRegistryValidator creates a new RegistryValidator instance
func NewRegistryValidator(registry, image, username, password, appName string) *RegistryValidator {
	return &RegistryValidator{
		Registry: registry,
		Image:    image,
		Username: username,
		Password: password,
		AppName:  appName,
	}
}

// ValidateRegistryConfig validates the entire docker registry configuration
func (rv *RegistryValidator) ValidateRegistryConfig() error {
	if rv.Registry == "" {
		return errors.New("docker registry not configured")
	}

	if !rv.IsValidRegistry(rv.Registry) {
		return fmt.Errorf("invalid registry format: %s", rv.Registry)
	}

	if rv.Image == "" {
		return errors.New("docker image name not configured")
	}

	if !rv.IsValidImageName(rv.Image) {
		return fmt.Errorf("invalid image name format: %s", rv.Image)
	}

	return nil
}

// IsValidRegistry checks if a registry string is valid
func (rv *RegistryValidator) IsValidRegistry(registry string) bool {
	if strings.TrimSpace(registry) == "" ||
		strings.Contains(registry, " ") ||
		strings.HasPrefix(registry, "/") ||
		strings.HasSuffix(registry, "/") {
		return false
	}

	_, err := url.ParseRequestURI("https://" + registry)
	return err == nil
}

// IsValidImageName checks if an image name is valid according to Docker standards
func (rv *RegistryValidator) IsValidImageName(image string) bool {
	// Basic validation - can be enhanced with more specific rules
	if strings.TrimSpace(image) == "" {
		return false
	}

	// Check for invalid characters
	if strings.ContainsAny(image, " \t\n\r") {
		return false
	}

	// Check for valid structure (simplified)
	parts := strings.Split(image, "/")
	for _, part := range parts {
		if part == "" {
			return false
		}
	}

	return true
}

// ValidateImageTag checks if a tag follows best practices
func (rv *RegistryValidator) ValidateImageTag(tag string) bool {
	match, _ := regexp.MatchString(`^[a-zA-Z0-9_\-\.]+$`, tag)
	return match && len(tag) <= 128
}

// GetFullImageTag constructs the full image tag from registry and image name
func (rv *RegistryValidator) GetFullImageTag() (string, error) {
	if err := rv.ValidateRegistryConfig(); err != nil {
		return "", fmt.Errorf("registry config validation failed: %w", err)
	}

	registry := strings.TrimPrefix(rv.Registry, "https://")
	registry = strings.TrimPrefix(registry, "http://")
	registry = strings.TrimSuffix(registry, "/")

	if strings.HasPrefix(rv.Image, registry+"/") {
		return rv.Image, nil
	}

	fullTag := fmt.Sprintf("%s/%s", registry, rv.Image)
	if !rv.IsValidImageName(fullTag) {
		return "", fmt.Errorf("invalid full image tag format: %s", fullTag)
	}

	return fullTag, nil
}

// GetRegistryType returns the type of registry (docker, ghcr, ecr, etc.)
func (rv *RegistryValidator) GetRegistryType() string {
	reg := strings.ToLower(rv.Registry)

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
	case strings.Contains(reg, "digitalocean"):
		return "do-registry"
	default:
		return "unknown"
	}
}

// NeedsAuthentication checks if the registry requires authentication
func (rv *RegistryValidator) NeedsAuthentication() bool {
	return !strings.HasPrefix(rv.Registry, "localhost") &&
		!strings.HasPrefix(rv.Registry, "127.0.0.1")
}

// ValidateCredentials checks if credentials are provided when needed
func (rv *RegistryValidator) ValidateCredentials() error {
	if !rv.NeedsAuthentication() {
		return nil
	}
	if rv.Username == "" || rv.Password == "" {
		return errors.New("registry credentials required but not provided")
	}
	return nil
}

// GetImagePullSecrets returns Kubernetes-style image pull secrets if needed
func (rv *RegistryValidator) GetImagePullSecrets() (string, error) {
	if !rv.NeedsAuthentication() {
		return "", nil
	}

	if err := rv.ValidateCredentials(); err != nil {
		return "", fmt.Errorf("credentials validation failed: %w", err)
	}

	encoded := rv.generateDockerConfigJSON()
	name := strings.ReplaceAll(rv.AppName, " ", "-")

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
			rv.Registry: {
				"username": rv.Username,
				"password": rv.Password,
			},
		},
	}

	jsonBytes, _ := json.Marshal(auths)
	return base64.StdEncoding.EncodeToString(jsonBytes)
}

// validator := registry.NewRegistryValidator(
// 	"docker.io",
// 	"myorg/myimage",
// 	"username",
// 	"password",
// 	"myapp",
// )
//
// // Validate configuration
// if err := validator.ValidateRegistryConfig(); err != nil {
// 	// handle error
// }
//
// // Get full image tag
// fullTag, err := validator.GetFullImageTag()
// if err != nil {
// 	// handle error
// }
//
// // Generate Kubernetes pull secret if needed
// secret, err := validator.GetImagePullSecrets()
// if err != nil {
// 	// handle error
// }
