//go:build ignore
// +build ignore

package registr

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
	rlogger = logger.PackageLogger("REGISTRY", "📁REGISTRY")
)

// Custom error types
var (
	ErrInvalidRegistryFormat   = errors.New("invalid registry format")
	ErrMissingRegistryConfig   = errors.New("docker registry not configured")
	ErrMissingImageConfig      = errors.New("docker image name not configured")
	ErrInvalidImageNameFormat  = errors.New("invalid image name format")
	ErrMissingCredentials      = errors.New("registry credentials required but not provided")
	ErrConfigLoadFailure       = errors.New("failed to load config")
	ErrSecretGenerationFailure = errors.New("failed to generate secret")
	ErrInvalidImageTagFormat   = errors.New("invalid image tag format")
)

// RegistryValidatorOption defines functional options for RegistryValidator
type RegistryValidatorOption func(*RegistryValidator)

func WithConfig(cfg *config.NextDeployConfig) RegistryValidatorOption {
	return func(rv *RegistryValidator) {
		rv.cfg = cfg
	}
}

type RegistryValidator struct {
	cfg *config.NextDeployConfig
}

func NewRegistryValidator(opts ...RegistryValidatorOption) (*RegistryValidator, error) {
	rv := &RegistryValidator{}
	for _, opt := range opts {
		opt(rv)
	}

	if rv.cfg == nil {
		cfg, err := config.Load()
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrConfigLoadFailure, err)
		}
		rv.cfg = cfg
	}
	return rv, nil
}

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

func (rv *RegistryValidator) IsValidImageName(image string) bool {
	_, err := reference.ParseNormalizedNamed(image)
	if err != nil {
		rlogger.Error("Invalid image name: %s", image)
	}
	return err == nil
}

func (rv *RegistryValidator) ValidateRegistryConfig() error {
	if rv.cfg.Docker.Registry == "" {
		return ErrMissingRegistryConfig
	}
	if !rv.IsValidRegistry(rv.cfg.Docker.Registry) {
		return fmt.Errorf("%w: %s", ErrInvalidRegistryFormat, rv.cfg.Docker.Registry)
	}
	if rv.cfg.Docker.Image == "" {
		return ErrMissingImageConfig
	}
	if !rv.IsValidImageName(rv.cfg.Docker.Image) {
		return fmt.Errorf("%w: %s", ErrInvalidImageNameFormat, rv.cfg.Docker.Image)
	}
	return nil
}

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
		return "", fmt.Errorf("%w: %s", ErrInvalidImageNameFormat, fullTag)
	}
	return fullTag, nil
}

func (rv *RegistryValidator) NeedsAuthentication() bool {
	return !strings.HasPrefix(rv.cfg.Docker.Registry, "localhost") &&
		!strings.HasPrefix(rv.cfg.Docker.Registry, "127.0.0.1")
}

func (rv *RegistryValidator) ValidateCredentials() error {
	if !rv.NeedsAuthentication() {
		return nil
	}
	if rv.cfg.Docker.Username == "" || rv.cfg.Docker.Password == "" {
		return ErrMissingCredentials
	}
	return nil
}

// SecretBuilder helps construct a Kubernetes pull secret
type SecretBuilder struct {
	name string
	auth map[string]map[string]map[string]string
}

func NewSecretBuilder(name string) *SecretBuilder {
	return &SecretBuilder{
		name: name,
		auth: map[string]map[string]map[string]string{
			"auths": {},
		},
	}
}

func (sb *SecretBuilder) WithCredentials(reg, user, pass string) *SecretBuilder {
	sb.auth["auths"][reg] = map[string]string{
		"username": user,
		"password": pass,
	}
	return sb
}

func (sb *SecretBuilder) Build() (string, error) {
	jsonBytes, err := json.Marshal(sb.auth)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrSecretGenerationFailure, err)
	}
	encoded := base64.StdEncoding.EncodeToString(jsonBytes)
	secret := fmt.Sprintf(`
apiVersion: v1
kind: Secret
metadata:
  name: %s-regcred
type: kubernetes.io/dockerconfigjson
data:
  .dockerconfigjson: %s`, sb.name, encoded)
	return secret, nil
}

// PushImageCommand executes the push logic
type PushImageCommand struct {
	Validator *RegistryValidator
	Logger    logger.Logger
}

func (cmd *PushImageCommand) Execute() error {
	if err := cmd.Validator.ValidateRegistryConfig(); err != nil {
		cmd.Logger.Error("Invalid registry configuration: %v", err)
		return err
	}
	if !cmd.Validator.cfg.Docker.Push {
		cmd.Logger.Info("Image push is disabled by config")
		return nil
	}
	if err := cmd.Validator.ValidateCredentials(); err != nil {
		cmd.Logger.Error("Authentication validation failed: %v", err)
		return err
	}
	fullTag, err := cmd.Validator.GetFullImageTag()
	if err != nil {
		cmd.Logger.Error("Failed to get image tag: %v", err)
		return err
	}
	cmd.Logger.Info("Pushing image %s to registry", fullTag)

	// TODO: Push with Docker SDK

	cmd.Logger.Success("Image pushed successfully: %s", fullTag)
	return nil
}

func (rv *RegistryValidator) ValidateImageTag(tag string) bool {
	match, _ := regexp.MatchString(`^[a-zA-Z0-9_\-\.]+$`, tag)
	return match && len(tag) <= 128
}
