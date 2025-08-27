package registry

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"nextdeploy/shared"
	"nextdeploy/shared/config"
	"nextdeploy/shared/git"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

var (
	rlogger = shared.PackageLogger("RegistryValidator", "🅰️ REGISTRY VALIDATOR")

	// imageNameRegex validates complete Docker image names
	imageNameRegex = regexp.MustCompile(`^([a-zA-Z0-9\-\.]+(?::[0-9]+)?/)?[a-z0-9]+(?:[._-][a-z0-9]+)*(/[a-z0-9]+(?:[._-][a-z0-9]+)*)*(?::[a-zA-Z0-9_\-\.]+)?$`)

	// tagRegex validates standalone image tags
	tagRegex = regexp.MustCompile(`^[a-zA-Z0-9_\-\.]+$`)
)

type RegistryValidator struct {
	cfg      *config.NextDeployConfig
	registry string
	image    string
	username string
	password string
	appName  string
}

// New creates a new RegistryValidator instance
func New() (*RegistryValidator, error) {
	cfg, err := config.Load()
	if err != nil {
		rlogger.Error("Failed to load configuration: %v", err)
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}
	if cfg == nil {
		return nil, errors.New("configuration cannot be nil")
	}

	rv := &RegistryValidator{
		cfg:      cfg,
		registry: cfg.Docker.Registry,
		image:    cfg.Docker.Image,
		username: cfg.Docker.Username,
		password: cfg.Docker.Password,
		appName:  cfg.App.Name,
	}

	if err := rv.ValidateConfig(); err != nil {
		rlogger.Error("Invalid registry configuration: %v", err)
		return nil, fmt.Errorf("invalid registry configuration: %w", err)
	}

	return rv, nil
}

// ValidateConfig checks all required registry configuration
func (rv *RegistryValidator) ValidateConfig() error {
	if err := validateRequired("registry", rv.registry); err != nil {
		rlogger.Error("Registry validation failed: %v", err)
		return err
	}
	if !rv.IsValidRegistry(rv.registry) {
		return fmt.Errorf("invalid registry format: %s", rv.registry)
	}

	if err := validateRequired("image", rv.image); err != nil {
		rlogger.Error("Image validation failed: %v", err)
		return err
	}
	if !isValidImageName(rv.image) {
		return fmt.Errorf("invalid image name format: %s", rv.image)
	}

	if rv.NeedsAuth() {
		if err := validateRequired("username", rv.username); err != nil {
			rlogger.Error("Username validation failed: %v", err)
			return err
		}
		if err := validateRequired("password", rv.password); err != nil {
			rlogger.Error("Password validation failed: %v", err)
			return err
		}
	}

	return nil
}

// PushImage handles the complete image push workflow
func (rv *RegistryValidator) PushImage(localImage string) error {
	if localImage == "" {
		var err error
		localImage, err = rv.findLocalImage()
		if err != nil {
			rlogger.Error("Failed to find local image: %v", err)
			return fmt.Errorf("failed to find local image: %w", err)
		}
	}

	if err := validateLocalImage(localImage); err != nil {
		rlogger.Error("Invalid local image: %v", err)
		return fmt.Errorf("invalid local image: %w", err)
	}

	targetTag, err := rv.BuildTargetTag()
	if err != nil {
		rlogger.Error("Failed to build target tag: %v", err)
		return fmt.Errorf("failed to build target tag: %w", err)
	}

	if err := rv.tagImage(localImage, targetTag); err != nil {
		rlogger.Error("Failed to tag image: %v", err)
		return fmt.Errorf("failed to tag image: %w", err)
	}

	if err := rv.pushImage(targetTag); err != nil {
		rlogger.Error("Failed to push image: %v", err)
		return fmt.Errorf("failed to push image: %w", err)
	}

	rlogger.Success("Successfully pushed image %s to %s", localImage, targetTag)
	return nil
}

// BuildTargetTag constructs the complete target image tag
func (rv *RegistryValidator) BuildTargetTag() (string, error) {
	tag, err := git.GetCommitHash()
	if err != nil {
		rlogger.Error("Failed to get commit hash: %v", err)
		return "", fmt.Errorf("failed to get commit hash: %w", err)
	}

	if !isValidTag(tag) {
		return "", fmt.Errorf("invalid tag format: %s", tag)
	}

	registry := normalizeRegistry(rv.registry)
	image := stripTag(rv.image)

	fullTag := fmt.Sprintf("%s/%s:%s", registry, image, tag)
	if !imageNameRegex.MatchString(fullTag) {
		return "", fmt.Errorf("invalid image tag format: %s", fullTag)
	}

	return fullTag, nil
}

// GeneratePullSecret creates Kubernetes image pull secret manifest
func (rv *RegistryValidator) GeneratePullSecret() (string, error) {
	if !rv.NeedsAuth() {
		return "", nil
	}

	authConfig := map[string]interface{}{
		"auths": map[string]interface{}{
			rv.registry: map[string]string{
				"username": rv.username,
				"password": rv.password,
				"auth":     base64.StdEncoding.EncodeToString([]byte(rv.username + ":" + rv.password)),
			},
		},
	}

	configJSON, err := json.Marshal(authConfig)
	if err != nil {
		rlogger.Error("Failed to marshal auth config: %v", err)
		return "", fmt.Errorf("failed to marshal auth config: %w", err)
	}

	secretName := strings.ReplaceAll(rv.appName, " ", "-") + "-regcred"

	return fmt.Sprintf(`
apiVersion: v1
kind: Secret
metadata:
  name: %s
  creationTimestamp: %s
type: kubernetes.io/dockerconfigjson
data:
  .dockerconfigjson: %s
`, secretName, time.Now().Format(time.RFC3339), base64.StdEncoding.EncodeToString(configJSON)), nil
}

// NeedsAuth checks if registry requires authentication
func (rv *RegistryValidator) NeedsAuth() bool {
	return !strings.HasPrefix(rv.registry, "localhost") &&
		!strings.HasPrefix(rv.registry, "127.0.0.1")
}

// RegistryType identifies the registry provider
func (rv *RegistryValidator) RegistryType() string {
	reg := strings.ToLower(rv.registry)

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

// Helper methods
func (rv *RegistryValidator) findLocalImage() (string, error) {
	cmd := exec.Command("docker", "images", "--format", "{{.Repository}}:{{.Tag}}", stripTag(rv.image))
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("docker images failed: %w\nOutput: %s", err, string(output))
	}

	images := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, img := range images {
		if img != "" && isValidImageName(img) {
			return img, nil
		}
	}

	return "", errors.New("no valid local images found")
}

func (rv *RegistryValidator) tagImage(source, target string) error {
	rlogger.Info("Tagging %s as %s", source, target)

	cmd := exec.Command("docker", "tag", source, target)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("docker tag failed: %w\nOutput: %s", err, string(output))
	}
	return nil
}

func (rv *RegistryValidator) pushImage(image string) error {
	rlogger.Info("Pushing image %s", image)

	// cmd := exec.Command("docker", "push", image)
	// stdout, _ := cmd.StdoutPipe()
	// stderr, _ := cmd.StderrPipe()

	// if err := cmd.Start(); err != nil {
	// 	return fmt.Errorf("docker push failed to start: %w", err)
	// }
	//
	// go streamOutput(stdout, "stdout")
	// go streamOutput(stderr, "stderr")
	//
	// if err := cmd.Wait(); err != nil {
	// 	return fmt.Errorf("docker push failed: %w", err)
	// }
	//
	return nil
}

func StreamOutput(reader io.Reader, label string) {
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "error") {
			rlogger.Error("[%s] %s", label, line)
		} else {
			rlogger.Debug("[%s] %s", label, line)
		}
	}
}

// Validation functions
func validateRequired(name, value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%s is required", name)
	}
	return nil
}

func (rv *RegistryValidator) IsValidRegistry(registry string) bool {
	registry = strings.TrimSpace(registry)
	return registry != "" &&
		!strings.Contains(registry, " ") &&
		!strings.HasPrefix(registry, "/") &&
		!strings.HasSuffix(registry, "/") &&
		isValidURL("https://"+registry)
}

func isValidURL(rawURL string) bool {
	_, err := url.ParseRequestURI(rawURL)
	return err == nil
}

func isValidImageName(image string) bool {
	return strings.TrimSpace(image) != "" &&
		!strings.ContainsAny(image, " \t\n\r") &&
		imageNameRegex.MatchString(image)
}

func isValidTag(tag string) bool {
	return len(tag) <= 128 && tagRegex.MatchString(tag)
}

func validateLocalImage(image string) error {
	if !isValidImageName(image) && !strings.HasPrefix(image, "sha256:") {
		return fmt.Errorf("invalid local image reference: %s", image)
	}
	return nil
}

func normalizeRegistry(registry string) string {
	registry = strings.TrimPrefix(registry, "https://")
	registry = strings.TrimPrefix(registry, "http://")
	return strings.TrimSuffix(registry, "/")
}

func stripTag(image string) string {
	return strings.Split(image, ":")[0]

}
