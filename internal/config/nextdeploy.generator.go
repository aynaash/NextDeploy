package config

import (
	"bufio"
	"fmt"
	"strings"
	"os"
	"gopkg.in/yaml.v3"
)

// PromptForConfig collects user input for the nextdeploy configuration
func PromptForConfig(reader *bufio.Reader) (*NextDeployConfig, error) {
	cfg := &NextDeployConfig{
		Version: "1.0",
		App: AppConfig{
			Port: 3000,
		},
		Docker: DockerConfig{
			Build: DockerBuild{
				Context:    ".",
				Dockerfile: "Dockerfile",
				NoCache:    false,
				Args:       map[string]string{"NODE_ENV": "production"},
			},
			Push: true,
		},
		Deployment: Deployment{
			Container: Container{
				Restart: "always",
				Ports:   []string{"80:3000"},
			},
		},
	}

	// Basic app configuration
	if err := PromptAppConfig(reader, cfg); err != nil {
		return nil, fmt.Errorf("app configuration error: %w", err)
	}

	// Repository configuration
	if err := PromptRepositoryConfig(reader, cfg); err != nil {
		return nil, fmt.Errorf("repository configuration error: %w", err)
	}

	// Docker configuration
	if err := PromptDockerConfig(reader, cfg); err != nil {
		return nil, fmt.Errorf("docker configuration error: %w", err)
	}

	// Deployment configuration
	if err := PromptDeploymentConfig(reader, cfg); err != nil {
		return nil, fmt.Errorf("deployment configuration error: %w", err)
	}

	// Optional configurations
	if PromptYesNo(reader, "Configure database?") {
		dbConfig, err := PromptDatabaseConfig(reader)
		if err != nil {
			return nil, fmt.Errorf("database configuration error: %w", err)
		}
		cfg.Database = &dbConfig
	}

	if PromptYesNo(reader, "Configure monitoring?") {
		monConfig, err := PromptMonitoringConfig(reader)
		if err != nil {
			return nil, fmt.Errorf("monitoring configuration error: %w", err)
		}
		cfg.Monitoring = &monConfig
	}

	return cfg, nil
}

func PromptAppConfig(reader *bufio.Reader, cfg *NextDeployConfig) error {
	fmt.Print("Enter application name: ")
	name, err := ReadRequiredInput(reader)
	if err != nil {
		return err
	}
	cfg.App.Name = name

	fmt.Print("Environment (production/staging): ")
	env, err := ReadRequiredInput(reader)
	if err != nil {
		return err
	}
	cfg.App.Environment = env

	fmt.Print("Domain (leave empty if none): ")
	cfg.App.Domain, _ = reader.ReadString('\n')
	cfg.App.Domain = strings.TrimSpace(cfg.App.Domain)

	return nil
}

func PromptRepositoryConfig(reader *bufio.Reader, cfg *NextDeployConfig) error {
	fmt.Print("Git repository URL (e.g., git@github.com:user/repo.git): ")
	url, err := ReadRequiredInput(reader)
	if err != nil {
		return err
	}
	cfg.Repository.URL = url

	fmt.Print("Git branch (default: main): ")
	branch, _ := reader.ReadString('\n')
	branch = strings.TrimSpace(branch)
	if branch == "" {
		branch = "main"
	}
	cfg.Repository.Branch = branch

	cfg.Repository.AutoDeploy = PromptYesNo(reader, "Enable auto-deploy?")

	if cfg.Repository.AutoDeploy {
		fmt.Print("Webhook secret (leave empty to generate): ")
		secret, _ := reader.ReadString('\n')
		cfg.Repository.WebhookSecret = strings.TrimSpace(secret)
	}

	return nil
}

func PromptDockerConfig(reader *bufio.Reader, cfg *NextDeployConfig) error {
	fmt.Print("Docker image name (e.g., username/app): ")
	image, err := ReadRequiredInput(reader)
	if err != nil {
		return err
	}
	cfg.Docker.Image = image

	fmt.Print("Docker registry (leave empty for default): ")
	registry, _ := reader.ReadString('\n')
	cfg.Docker.Registry = strings.TrimSpace(registry)

	cfg.Docker.Build.NoCache = PromptYesNo(reader, "Build without cache?")

	return nil
}

func PromptDeploymentConfig(reader *bufio.Reader, cfg *NextDeployConfig) error {
	fmt.Print("Deployment server host (IP or domain): ")
	host, err := ReadRequiredInput(reader)
	if err != nil {
		return err
	}
	cfg.Deployment.Server.Host = host

	fmt.Print("Server SSH user: ")
	user, err := ReadRequiredInput(reader)
	if err != nil {
		return err
	}
	cfg.Deployment.Server.User = user

	fmt.Print("SSH key path (~/.ssh/key): ")
	keyPath, err := ReadRequiredInput(reader)
	if err != nil {
		return err
	}
	cfg.Deployment.Server.SSHKey = keyPath

	cfg.Deployment.Server.UseSudo = PromptYesNo(reader, "Require sudo for deployment?")

	fmt.Print("Container name: ")
	name, err := ReadRequiredInput(reader)
	if err != nil {
		return err
	}
	cfg.Deployment.Container.Name = name

	return nil
}

func PromptDatabaseConfig(reader *bufio.Reader) (Database, error) {
	var db Database

	fmt.Print("Database type (mysql/postgres): ")
	dbType, err := ReadRequiredInput(reader)
	if err != nil {
		return db, err
	}
	db.Type = dbType

	fmt.Print("Database host (leave empty for localhost): ")
	host, _ := reader.ReadString('\n')
	host = strings.TrimSpace(host)
	if host == "" {
		host = "localhost"
	}
	db.Host = host

	fmt.Print("Database port: ")
	port, err := ReadRequiredInput(reader)
	fmt.Println(port)
	if err != nil {
		return db, err
	}
	// Convert port to int (error handling omitted for brevity)
	// db.Port = portInt

	fmt.Print("Database username: ")
	username, err := ReadRequiredInput(reader)
	if err != nil {
		return db, err
	}
	db.Username = username

	fmt.Print("Database password: ")
	password, err := ReadRequiredInput(reader)
	if err != nil {
		return db, err
	}
	db.Password = password

	fmt.Print("Database name: ")
	name, err := ReadRequiredInput(reader)
	if err != nil {
		return db, err
	}
	db.Name = name

	return db, nil
}

func PromptMonitoringConfig(reader *bufio.Reader) (Monitoring, error) {
	var mon Monitoring

	mon.Enabled = true

	fmt.Print("Monitoring type (prometheus/grafana): ")
	monType, err := ReadRequiredInput(reader)
	if err != nil {
		return mon, err
	}
	mon.Type = monType

	fmt.Print("Monitoring endpoint (leave empty for default): ")
	endpoint, _ := reader.ReadString('\n')
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		if mon.Type == "prometheus" {
			endpoint = "http://localhost:9090"
		} else {
			endpoint = "http://localhost:3000"
		}
	}
	mon.Endpoint = endpoint

	return mon, nil
}

func PromptYesNo(reader *bufio.Reader, question string) bool {
	fmt.Printf("%s (y/n): ", question)
	resp, _ := reader.ReadString('\n')
	resp = strings.TrimSpace(strings.ToLower(resp))
	return resp == "y" || resp == "yes"
}

func ReadRequiredInput(reader *bufio.Reader) (string, error) {
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)
	if input == "" {
		return "", fmt.Errorf("this field is required")
	}
	return input, nil
}

// WriteConfig writes the configuration to a YAML file
func WriteConfig(filename string, cfg *NextDeployConfig) error {
	// Implementation using yaml.Marshal would go here
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}
	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}
	fmt.Printf("Configuration saved to %s\n", filename)
	return nil
}

func PromptForConfigs(reader *bufio.Reader) (*NextDeployConfig, error) {
	cfg := &NextDeployConfig{
		Version: "1.0",
		App: AppConfig{
			Port: 3000,
		},
		Docker: DockerConfig{
			Build: DockerBuild{
				Context:    ".",
				Dockerfile: "Dockerfile",
				NoCache:    false,
				Args:       map[string]string{"NODE_ENV": "production"},
			},
			Push: true,
		},
		Deployment: Deployment{
			Container: Container{
				Restart: "always",
				Ports:   []string{"80:3000"},
			},
		},
	}

	// Basic app configuration
	if err := PromptAppConfig(reader, cfg); err != nil {
		return nil, fmt.Errorf("app configuration error: %w", err)
	}

	// Repository configuration
	if err := PromptRepositoryConfig(reader, cfg); err != nil {
		return nil, fmt.Errorf("repository configuration error: %w", err)
	}

	// Docker configuration
	if err := PromptDockerConfig(reader, cfg); err != nil {
		return nil, fmt.Errorf("docker configuration error: %w", err)
	}

	// Deployment configuration
	if err := PromptDeploymentConfig(reader, cfg); err != nil {
		return nil, fmt.Errorf("deployment configuration error: %w", err)
	}

	// Optional configurations
	if PromptYesNo(reader, "Configure database?") {
		dbConfig, err := PromptDatabaseConfig(reader)
		if err != nil {
			return nil, fmt.Errorf("database configuration error: %w", err)
		}
		cfg.Database = &dbConfig
	}

	if PromptYesNo(reader, "Configure monitoring?") {
		monConfig, err := PromptMonitoringConfig(reader)
		if err != nil {
			return nil, fmt.Errorf("monitoring configuration error: %w", err)
		}
		cfg.Monitoring = &monConfig
	}

	return cfg, nil
}
