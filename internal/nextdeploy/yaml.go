package nextdeploy

import (
	"errors"
	"fmt"
	"gopkg.in/yaml.v3"
	"nextdeploy/internal/logger"
	"os"
	"path/filepath"
)

// Config represents the structure of a nextdeploy.yml file
// New creates a new Config with default values
var (
	ylogger = logger.PackageLogger("YAML", "↪ YAML")
)

func New() *Config {
	return &Config{
		Version: "1.0",
		App: AppConfig{
			Environment: "production",
			Port:        3000,
		},
		Repository: Repository{
			Branch:     "main",
			AutoDeploy: true,
		},
		Docker: DockerConfig{
			Build: DockerBuild{
				Context:    ".",
				Dockerfile: "Dockerfile",
				NoCache:    false,
				Args:       make(map[string]string),
			},
			Push: true,
		},
		Deployment: Deployment{
			Server: Server{
				UseSudo: false,
			},
			Container: Container{
				Restart: "always",
				Healthcheck: ContainerHealthcheck{
					Interval: "30s",
					Timeout:  "5s",
					Retries:  3,
				},
			},
		},
		Logging: Logging{
			Enabled:    true,
			Provider:   "nextdeploy",
			StreamLogs: true,
		},
		Monitoring: Monitoring{
			Enabled:         true,
			CPUThreshold:    80,
			MemoryThreshold: 75,
			DiskThreshold:   90,
		},
		Backup: Backup{
			Enabled:       true,
			Frequency:     "daily",
			RetentionDays: 7,
		},
		SSL: SSL{
			Enabled:   true,
			Provider:  "letsencrypt",
			AutoRenew: true,
		},
	}
}

// Load reads a nextdeploy.yml file and returns a Config struct
func Load(filename string) (*Config, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	config := New()
	err = yaml.Unmarshal(data, config)
	if err != nil {
		ylogger.Error("Failed to parse YAML")
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	return config, nil
}

// Save writes the Config to a nextdeploy.yml file
func (c *Config) Save(filename string) error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("failed to marshal config to YAML: %w", err)
	}

	// Ensure the directory exists
	dir := filepath.Dir(filename)
	if dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}
	}

	err = os.WriteFile(filename, data, 0644)
	if err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// UpdateApp updates the app configuration
func (c *Config) UpdateApp(name, environment, domain string, port int) {
	c.App.Name = name
	c.App.Environment = environment
	c.App.Domain = domain
	c.App.Port = port
}

// UpdateRepository updates the repository configuration
func (c *Config) UpdateRepository(url, branch string, autoDeploy bool, webhookSecret string) {
	c.Repository.URL = url
	c.Repository.Branch = branch
	c.Repository.AutoDeploy = autoDeploy
	c.Repository.WebhookSecret = webhookSecret
}

// AddDockerBuildArg adds or updates a Docker build argument
func (c *Config) AddDockerBuildArg(key, value string) {
	if c.Docker.Build.Args == nil {
		c.Docker.Build.Args = make(map[string]string)
	}
	c.Docker.Build.Args[key] = value
}

// RemoveDockerBuildArg removes a Docker build argument
func (c *Config) RemoveDockerBuildArg(key string) {
	delete(c.Docker.Build.Args, key)
}

// AddContainerVolume adds a volume to the container configuration
func (c *Config) AddContainerVolume(volume string) error {
	for _, v := range c.Deployment.Container.Volumes {
		if v == volume {
			return errors.New("volume already exists")
		}
	}
	c.Deployment.Container.Volumes = append(c.Deployment.Container.Volumes, volume)
	return nil
}

// RemoveContainerVolume removes a volume from the container configuration
func (c *Config) RemoveContainerVolume(volume string) error {
	for i, v := range c.Deployment.Container.Volumes {
		if v == volume {
			c.Deployment.Container.Volumes = append(c.Deployment.Container.Volumes[:i], c.Deployment.Container.Volumes[i+1:]...)
			return nil
		}
	}
	return errors.New("volume not found")
}

// AddContainerPort adds a port mapping to the container configuration
func (c *Config) AddContainerPort(port string) error {
	for _, p := range c.Deployment.Container.Ports {
		if p == port {
			return errors.New("port already exists")
		}
	}
	c.Deployment.Container.Ports = append(c.Deployment.Container.Ports, port)
	return nil
}

// RemoveContainerPort removes a port mapping from the container configuration
func (c *Config) RemoveContainerPort(port string) error {
	for i, p := range c.Deployment.Container.Ports {
		if p == port {
			c.Deployment.Container.Ports = append(c.Deployment.Container.Ports[:i], c.Deployment.Container.Ports[i+1:]...)
			return nil
		}
	}
	return errors.New("port not found")
}

// AddSuccessWebhook adds a webhook to be triggered on successful deployment
func (c *Config) AddSuccessWebhook(url string) {
	c.Webhook.OnSuccess = append(c.Webhook.OnSuccess, url)
}

// RemoveSuccessWebhook removes a webhook from the success list
func (c *Config) RemoveSuccessWebhook(url string) error {
	for i, u := range c.Webhook.OnSuccess {
		if u == url {
			c.Webhook.OnSuccess = append(c.Webhook.OnSuccess[:i], c.Webhook.OnSuccess[i+1:]...)
			return nil
		}
	}
	return errors.New("webhook not found")
}

// AddFailureWebhook adds a webhook to be triggered on failed deployment
func (c *Config) AddFailureWebhook(url string) {
	c.Webhook.OnFailure = append(c.Webhook.OnFailure, url)
}

// RemoveFailureWebhook removes a webhook from the failure list
func (c *Config) RemoveFailureWebhook(url string) error {
	for i, u := range c.Webhook.OnFailure {
		if u == url {
			c.Webhook.OnFailure = append(c.Webhook.OnFailure[:i], c.Webhook.OnFailure[i+1:]...)
			return nil
		}
	}
	return errors.New("webhook not found")
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	if c.App.Name == "" {
		return errors.New("app name is required")
	}
	if c.App.Domain == "" {
		return errors.New("app domain is required")
	}
	if c.Repository.URL == "" {
		return errors.New("repository URL is required")
	}
	if c.Deployment.Server.Host == "" {
		return errors.New("deployment server host is required")
	}
	return nil
}

// func (c *Config) PushConfigToDoppler() error {
// 	if c.Doppler == nil {
// 		return errors.New("Doppler configuration is not set")
// 	}
//
// 	if c.Doppler.Token == "" {
// 		return errors.New("Doppler token is required")
// 	}
//
// 	if c.Doppler.Project == "" {
// 		return errors.New("Doppler project is required")
// 	}
//
// 	if c.Doppler.Config == "" {
// 		return errors.New("Doppler config is required")
// 	}
//
// 	ylogger.Info("Pushing configuration to Doppler...")
// 	// Here you would implement the logic to push the configuration to Doppler
// 	// This is a placeholder for the actual implementation
// 	return nil
// }
