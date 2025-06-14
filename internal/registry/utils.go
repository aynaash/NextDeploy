package registry

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"nextdeploy/internal/config"
	"nextdeploy/internal/git"
	"nextdeploy/internal/logger"
	"os/exec"
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
	cfg      *config.NextDeployConfig
}

var (
	rlogger = logger.PackageLogger("RegistryValidator", "🅰️ REGISTRY VALIDATOR")
)

// NewRegistryValidator creates a new RegistryValidator instance
func NewRegistryValidator() *RegistryValidator {
	cfg, err := config.Load()
	if err != nil {
		rlogger.Error("Failed to load configuration: %v", err)
		return nil
	}
	// registry values from config
	registryname := cfg.Docker.Registry
	username := cfg.Docker.Username
	password := cfg.Docker.Password
	image := cfg.Docker.Image
	appName := cfg.App.Name

	return &RegistryValidator{
		cfg:      cfg,
		Registry: registryname,
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
	tag, err := git.GetCommitHash()
	if err != nil {
		rlogger.Error("Failed to get commit hash: %v", err)
		return "", fmt.Errorf("failed to retrieve commit hash: %w", err)
	}
	if err := rv.ValidateRegistryConfig(); err != nil {
		return "", fmt.Errorf("registry config validation failed: %w", err)
	}

	registry := strings.TrimPrefix(rv.Registry, "https://")
	registry = strings.TrimPrefix(registry, "http://")
	registry = strings.TrimSuffix(registry, "/")

	if strings.HasPrefix(rv.Image, registry+"/") {
		return rv.Image, nil
	}
	// The full values are
	// FIX: image name issue
	rlogger.Debug("Constructing full image tag with registry: %s, image: %s, tag: %s", registry, rv.Image, tag)
	fullTag := fmt.Sprintf("%s/%s%s", registry, rv.Image, tag)
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

// PushImageToRegistry pushes a local Docker image to the configured registry with proper tagging
func (rv *RegistryValidator) PushImageToRegistry(localImage, tag string) error {
	// Step 1: Validate config
	if err := rv.ValidateRegistryConfig(); err != nil {
		rlogger.Error("Invalid registry configuration: %v", err)
		return fmt.Errorf("registry validation failed: %w", err)
	}

	if !rv.cfg.Docker.Push {
		rlogger.Info("Image push is disabled by configuration")
		return nil
	}

	if err := rv.ValidateCredentials(); err != nil {
		rlogger.Error("Authentication validation failed: %v", err)
		return fmt.Errorf("authentication failed: %w", err)
	}

	// Step 2: Construct registry tag
	fullTag, err := rv.GetFullImageTag()
	if err != nil {
		rlogger.Error("Failed to construct image tag: %v", err)
		return fmt.Errorf("invalid image tag: %w", err)
	}

	// Step 3: Tag the local image with the full registry tag
	rlogger.Info("Tagging image: %s as %s", localImage, fullTag)
	tagCmd := exec.Command("docker", "tag", localImage, fullTag)
	if output, err := tagCmd.CombinedOutput(); err != nil {
		rlogger.Error("Failed to tag image: %v, output: %s", err, string(output))
		return fmt.Errorf("docker tag failed: %w", err)
	}

	// Step 4: Push the image and stream the logs
	rlogger.Info("Pushing image to registry: %s", fullTag)
	pushCmd := exec.Command("docker", "push", fullTag)

	stdout, err := pushCmd.StdoutPipe()
	if err != nil {
		rlogger.Error("Failed to capture push stdout: %v", err)
		return fmt.Errorf("push stdout pipe failed: %w", err)
	}

	stderr, err := pushCmd.StderrPipe()
	if err != nil {
		rlogger.Error("Failed to capture push stderr: %v", err)
		return fmt.Errorf("push stderr pipe failed: %w", err)
	}

	if err := pushCmd.Start(); err != nil {
		rlogger.Error("Failed to start docker push: %v", err)
		return fmt.Errorf("docker push start failed: %w", err)
	}

	// Step 5: Stream output
	go streamDockerOutput(stdout, "stdout")
	go streamDockerOutput(stderr, "stderr")

	if err := pushCmd.Wait(); err != nil {
		rlogger.Error("Docker push failed: %v", err)
		return fmt.Errorf("docker push error: %w", err)
	}

	rlogger.Success("Successfully pushed image to registry: %s", fullTag)
	return nil
}

func streamDockerOutput(reader io.Reader, label string) {
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "errorDetail") || strings.Contains(line, "unauthorized") {
			rlogger.Error("[%s] %s", label, line)
		} else {
			rlogger.Info("[%s] %s", label, line)
		}
	}
	if err := scanner.Err(); err != nil {
		rlogger.Error("[%s] scanner error: %v", label, err)
	}
}
