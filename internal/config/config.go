
package config

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"encoding/json"

	"gopkg.in/yaml.v3"
)

const (
	configFile      = "nextdeploy.yml"
	sampleConfigFile = "sample.nextdeploy.yml"
	emojiSuccess    = "✅"
	emojiWarning    = "⚠️"
	emojiInfo       = "ℹ️"
	emojiInput      = "🖊️"
	emojiQuestion   = "❓"
	emojiImportant  = "🔑"
	emojiNetwork    = "🌐"
	emojiContainer  = "🐳"
	emojiDatabase   = "💾"
)

func InteractiveConfigPrompt(reader *bufio.Reader) (*NextDeployConfig, error) {
	cfg := &NextDeployConfig{
		Version: "1.0",
	}

	fmt.Println("\n✨ Welcome to NextDeploy Configuration Wizard ✨")
	fmt.Println("Let's set up your deployment configuration step by step")

	// App Configuration
	fmt.Println("\n📱 Application Settings")
	fmt.Println("----------------------")
	cfg.App = promptAppConfig(reader)

	// Repository Configuration
	fmt.Println("\n📦 Repository Settings")
	fmt.Println("----------------------")
	cfg.Repository = promptRepositoryConfig(reader)

	// Docker Configuration
	fmt.Println("\n🐳 Docker Settings")
	fmt.Println("----------------------")
	cfg.Docker = promptDockerConfig(reader)

	// Deployment Configuration
	fmt.Println("\n🚀 Deployment Settings")
	fmt.Println("----------------------")
	cfg.Deployment = promptDeploymentConfig(reader)

	// Database Configuration
	if promptYesNo(reader, "Would you like to configure database settings?", false) {
		fmt.Println("\n💾 Database Settings")
		fmt.Println("----------------------")
		dbConfig := promptDatabaseConfig(reader)
		cfg.Database = &dbConfig
	}

	// Monitoring Configuration
	if promptYesNo(reader, "Would you like to configure monitoring?", false) {
		fmt.Println("\n👀 Monitoring Settings")
		fmt.Println("----------------------")
		monitoringConfig := promptMonitoringConfig(reader)
		cfg.Monitoring = &monitoringConfig
	}

	// Show configuration summary
	showConfigSummary(cfg)

	return cfg, nil
}

func promptAppConfig(reader *bufio.Reader) AppConfig {
	fmt.Printf("%s What's your application name? ", emojiInput)
	name, _ := reader.ReadString('\n')

	fmt.Printf("%s Which port does your app run on? (default: 3000) ", emojiInput)
	portStr, _ := reader.ReadString('\n')
	port, _ := strconv.Atoi(strings.TrimSpace(portStr))
	if port == 0 {
		port = 3000
	}

	fmt.Printf("%s What environment is this for? (dev/stage/prod): ", emojiInput)
	env, _ := reader.ReadString('\n')

	fmt.Printf("%s Your production domain (leave empty if none): ", emojiInput)
	domain, _ := reader.ReadString('\n')

	// Secrets configuration
	var secrets *SecretsConfig
	if promptYesNo(reader, "Would you like to configure secrets management?", false) {
		fmt.Printf("%s Secrets provider (aws/azure/gcp): ", emojiImportant)
		provider, _ := reader.ReadString('\n')

		fmt.Printf("%s Project/namespace for secrets: ", emojiImportant)
		project, _ := reader.ReadString('\n')

		fmt.Printf("%s Additional config path: ", emojiImportant)
		configPath, _ := reader.ReadString('\n')

		secrets = &SecretsConfig{
			Provider: strings.TrimSpace(provider),
			Project:  strings.TrimSpace(project),
			Config:   strings.TrimSpace(configPath),
		}
	}

	return AppConfig{
		Name:        strings.TrimSpace(name),
		Port:        port,
		Environment: strings.TrimSpace(env),
		Domain:      strings.TrimSpace(domain),
		Secrets:     secrets,
	}
}

func promptRepositoryConfig(reader *bufio.Reader) Repository {
	fmt.Printf("%s Git repository URL: ", emojiInput)
	url, _ := reader.ReadString('\n')

	fmt.Printf("%s Default branch: ", emojiInput)
	branch, _ := reader.ReadString('\n')

	autoDeploy := promptYesNo(reader, "Enable auto-deploy on push?", true)

	var webhookSecret string
	if autoDeploy {
		fmt.Printf("%s Webhook secret (leave empty to generate): ", emojiImportant)
		webhookSecret, _ = reader.ReadString('\n')
	}

	return Repository{
		URL:          strings.TrimSpace(url),
		Branch:       strings.TrimSpace(branch),
		AutoDeploy:   autoDeploy,
		WebhookSecret: strings.TrimSpace(webhookSecret),
	}
}

func promptDockerConfig(reader *bufio.Reader) DockerConfig {
	fmt.Printf("%s Docker image name: ", emojiContainer)
	image, _ := reader.ReadString('\n')

	fmt.Printf("%s Docker registry (leave empty for Docker Hub): ", emojiContainer)
	registry, _ := reader.ReadString('\n')

	// Docker build settings
	fmt.Println("\n🔨 Docker Build Settings")
	fmt.Printf("%s Build context path (default: .): ", emojiInput)
	context, _ := reader.ReadString('\n')

	fmt.Printf("%s Dockerfile path (default: Dockerfile): ", emojiInput)
	dockerfile, _ := reader.ReadString('\n')

	noCache := promptYesNo(reader, "Disable build cache?", false)
	push := promptYesNo(reader, "Push to registry after build?", true)

	// Build args
	args := make(map[string]string)
	if promptYesNo(reader, "Add build arguments?", false) {
		for {
			fmt.Printf("%s Build arg (KEY=value, leave empty to finish): ", emojiInput)
			arg, _ := reader.ReadString('\n')
			arg = strings.TrimSpace(arg)
			if arg == "" {
				break
			}
			parts := strings.SplitN(arg, "=", 2)
			if len(parts) == 2 {
				args[parts[0]] = parts[1]
			}
		}
	}

	return DockerConfig{
		Image:    strings.TrimSpace(image),
		Registry: strings.TrimSpace(registry),
		Build: DockerBuild{
			Context:    firstNonEmpty(strings.TrimSpace(context), "."),
			Dockerfile: firstNonEmpty(strings.TrimSpace(dockerfile), "Dockerfile"),
			NoCache:    noCache,
			Args:       args,
		},
		Push: push,
	}
}

func promptDeploymentConfig(reader *bufio.Reader) Deployment {
	fmt.Println("\n🖥️  Server Connection")
	fmt.Printf("%s Server host/IP: ", emojiNetwork)
	host, _ := reader.ReadString('\n')

	fmt.Printf("%s SSH username: ", emojiNetwork)
	user, _ := reader.ReadString('\n')

	fmt.Printf("%s SSH key path: ", emojiNetwork)
	sshKey, _ := reader.ReadString('\n')

	useSudo := promptYesNo(reader, "Require sudo for deployment?", false)

	fmt.Println("\n📦 Container Settings")
	fmt.Printf("%s Container name: ", emojiContainer)
	name, _ := reader.ReadString('\n')

	fmt.Printf("%s Restart policy (always/unless-stopped/no): ", emojiContainer)
	restart, _ := reader.ReadString('\n')

	// Port mappings
	var ports []string
	for {
		fmt.Printf("%s Port mapping (host:container, leave empty to finish): ", emojiNetwork)
		port, _ := reader.ReadString('\n')
		port = strings.TrimSpace(port)
		if port == "" {
			break
		}
		ports = append(ports, port)
	}

	return Deployment{
		Server: Server{
			Host:    strings.TrimSpace(host),
			User:    strings.TrimSpace(user),
			SSHKey:  strings.TrimSpace(sshKey),
			UseSudo: useSudo,
		},
		Container: Container{
			Name:    strings.TrimSpace(name),
			Restart: strings.TrimSpace(restart),
			Ports:   ports,
		},
	}
}

func promptDatabaseConfig(reader *bufio.Reader) Database {
	fmt.Printf("%s Database type (postgres/mysql/mongodb): ", emojiDatabase)
	dbType, _ := reader.ReadString('\n')

	fmt.Printf("%s Database host: ", emojiDatabase)
	host, _ := reader.ReadString('\n')

	fmt.Printf("%s Database port: ", emojiDatabase)
	port, _ := reader.ReadString('\n')

	fmt.Printf("%s Database username: ", emojiDatabase)
	username, _ := reader.ReadString('\n')

	fmt.Printf("%s Database password: ", emojiDatabase)
	password, _ := reader.ReadString('\n')

	fmt.Printf("%s Database name: ", emojiDatabase)
	name, _ := reader.ReadString('\n')

	return Database{
		Type:     strings.TrimSpace(dbType),
		Host:     strings.TrimSpace(host),
		Port:     strings.TrimSpace(port),
		Username: strings.TrimSpace(username),
		Password: strings.TrimSpace(password),
		Name:     strings.TrimSpace(name),
	}
}

func promptMonitoringConfig(reader *bufio.Reader) Monitoring {
	fmt.Printf("%s Monitoring type (prometheus/datadog/newrelic): ", emojiInput)
	monType, _ := reader.ReadString('\n')

	fmt.Printf("%s Monitoring endpoint: ", emojiInput)
	endpoint, _ := reader.ReadString('\n')

	return Monitoring{
		Enabled:  true,
		Type:     strings.TrimSpace(monType),
		Endpoint: strings.TrimSpace(endpoint),
	}
}

func promptYesNo(reader *bufio.Reader, question string, defaultYes bool) bool {
	options := "(y/N)"
	if defaultYes {
		options = "(Y/n)"
	}

	for {
		fmt.Printf("%s %s %s: ", emojiQuestion, question, options)
		answer, _ := reader.ReadString('\n')
		answer = strings.TrimSpace(strings.ToLower(answer))

		if answer == "" {
			return defaultYes
		}
		if answer == "y" || answer == "yes" {
			return true
		}
		if answer == "n" || answer == "no" {
			return false
		}
		fmt.Printf("%s Please answer with 'y' or 'n'\n", emojiWarning)
	}
}

func showConfigSummary(cfg *NextDeployConfig) {
	fmt.Println("\n" + strings.Repeat("=", 50))
	fmt.Println("🎉 Configuration Summary")
	fmt.Println(strings.Repeat("=", 50))

	fmt.Printf("\n📱 Application: %s\n", cfg.App.Name)
	fmt.Printf("🌐 Port: %d | Env: %s\n", cfg.App.Port, cfg.App.Environment)
	if cfg.App.Domain != "" {
		fmt.Printf("🔗 Domain: %s\n", cfg.App.Domain)
	}

	fmt.Printf("\n📦 Repository: %s (%s)\n", cfg.Repository.URL, cfg.Repository.Branch)
	if cfg.Repository.AutoDeploy {
		fmt.Println("🤖 Auto-deploy: Enabled")
	}

	fmt.Printf("\n🐳 Docker Image: %s\n", cfg.Docker.Image)
	fmt.Printf("📦 Build Context: %s\n", cfg.Docker.Build.Context)

	fmt.Printf("\n🚀 Deployment Server: %s@%s\n", cfg.Deployment.Server.User, cfg.Deployment.Server.Host)
	fmt.Printf("📦 Container: %s (Restart: %s)\n", cfg.Deployment.Container.Name, cfg.Deployment.Container.Restart)

	if cfg.Database != nil {
		fmt.Printf("\n💾 Database: %s@%s:%s\n", cfg.Database.Username, cfg.Database.Host, cfg.Database.Port)
	}

	if cfg.Monitoring != nil {
		fmt.Printf("\n👀 Monitoring: %s (%s)\n", cfg.Monitoring.Type, cfg.Monitoring.Endpoint)
	}

	fmt.Println("\n" + strings.Repeat("=", 50))
}

func Save(cfg *NextDeployConfig, path string) error {
    data, err := json.MarshalIndent(cfg, "", "  ")
    if err != nil {
        return err
    }
    return os.WriteFile(path, data, 0644)
}
func Load() (*NextDeployConfig, error) {
	data, err := os.ReadFile(configFile)
	if err != nil {
		return nil, fmt.Errorf("%s Config file not found: %w", emojiWarning, err)
	}

	var cfg NextDeployConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("%s Invalid config format: %w", emojiWarning, err)
	}

	fmt.Printf("%s Configuration loaded successfully\n", emojiSuccess)
	return &cfg, nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
