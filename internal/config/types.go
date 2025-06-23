package config

import (
	"fmt"
	"gopkg.in/yaml.v3"
	"os"
)

// NextDeployConfig represents the complete deployment configuration
type NextDeployConfig struct {
	Version     string         `yaml:"version"`
	App         AppConfig      `yaml:"app"`
	Repository  Repository     `yaml:"repository"`
	Docker      DockerConfig   `yaml:"docker"`
	Deployment  Deployment     `yaml:"deployment"`
	Database    *Database      `yaml:"database,omitempty"`
	Monitoring  *Monitoring    `yaml:"monitoring,omitempty"`
	Secrets     SecretsConfig  `yaml:"secrets"`
	Logging     Logging        `yaml:"logging,omitempty"`
	Backup      *Backup        `yaml:"backup,omitempty"`
	SSL         *SSL           `yaml:"ssl,omitempty"`
	Webhooks    []Webhook      `yaml:"webhooks,omitempty"`
	Environment []EnvVariable  `yaml:"environment,omitempty"`
	Servers     []ServerConfig `yaml:"servers"`
	SSLConfig   *SSLConfig     `yaml:"ssl_config,omitempty"`
}
type SSLConfig struct {
	Domain      string `yaml:"domain"`
	Email       string `yaml:"email"`
	Staging     bool   `yaml:"staging"`
	Wildcard    bool   `yaml:"wildcard"`
	DNSProvider string `yaml:"dns_provider"`
	Force       bool   `yaml:"force"`
	SSL         struct {
		Enabled   bool   `yaml:"enabled"`
		Provider  string `yaml:"provider"`
		Email     string `yaml:"email"`
		AutoRenew bool   `yaml:"auto_renew"`
	} `yaml:"ssl"`
}

type ServerConfig struct {
	Name          string `yaml:"name"` // or json/xml depending on your config format
	Host          string `yaml:"host"`
	Port          int    `yaml:"port"` // default 22
	Username      string `yaml:"username"`
	Password      string `yaml:"password"`                 // Consider using SSH keys instead
	KeyPath       string `yaml:"key_path"`                 // Path to private key
	SSHKey        string `yaml:"ssh_key,omitempty"`        // Optional SSH key for authentication
	KeyPassphrase string `yaml:"key_passphrase,omitempty"` // Optional passphrase for SSH key
}

// AppConfig contains application-specific settings
type AppConfig struct {
	Name        string         `yaml:"name"`
	Port        int            `yaml:"port"`
	Environment string         `yaml:"environment"`
	Domain      string         `yaml:"domain,omitempty"`
	Secrets     *SecretsConfig `yaml:"secrets,omitempty"`
}

// Repository contains source control configuration
type Repository struct {
	URL           string `yaml:"url"`
	Branch        string `yaml:"branch"`
	AutoDeploy    bool   `yaml:"autoDeploy"`
	WebhookSecret string `yaml:"webhookSecret,omitempty"`
}

// DockerConfig contains containerization settings
type DockerConfig struct {
	Image        string      `yaml:"image"`
	Registry     string      `yaml:"registry,omitempty"`
	Build        DockerBuild `yaml:"build"`
	Push         bool        `yaml:"push"`
	Username     string      `yaml:"username,omitempty"`
	Password     string      `yaml:"password,omitempty"`
	AlwaysPull   bool        `yaml:"alwaysPull,omitempty"`
	Strategy     string      `yaml:"strategy,omitempty"` // e.g., "branch-commit", "timestamp"
	AutoPush     bool        `yaml:"autoPush,omitempty"` // Automatically push after build
	BuildArgs    []string    `yaml:"buildArgs,omitempty"`
	Platform     string      `yaml:"platform,omitempty"`     // e.g., "linux/amd64"
	NoCache      bool        `yaml:"noCache,omitempty"`      // Disable cache for BuildArg
	BuildContext string      `yaml:"buildContext,omitempty"` // Context for Docker build
	Target       string      `yaml:"target,omitempty"`       // Dockerfile target stage
}

// DockerBuild contains Docker build parameters
type DockerBuild struct {
	Context    string            `yaml:"context"`
	Dockerfile string            `yaml:"dockerfile"`
	NoCache    bool              `yaml:"noCache"`
	Args       map[string]string `yaml:"args,omitempty"`
}

// Deployment contains infrastructure deployment settings
type Deployment struct {
	Server    Server    `yaml:"server"`
	Container Container `yaml:"container"`
}

// Server contains target server connection details
type Server struct {
	Host    string `yaml:"host"`
	User    string `yaml:"user"`
	SSHKey  string `yaml:"sshKey,omitempty"`
	UseSudo bool   `yaml:"useSudo"`
}

// Container contains runtime configuration
type Container struct {
	Name        string       `yaml:"name"`
	Restart     string       `yaml:"restart"`
	Ports       []string     `yaml:"ports"`
	HealthCheck *HealthCheck `yaml:"healthCheck,omitempty"`
}

// HealthCheck defines container health monitoring
type HealthCheck struct {
	Test     []string `yaml:"test,omitempty"`
	Interval string   `yaml:"interval,omitempty"`
	Timeout  string   `yaml:"timeout,omitempty"`
	Retries  int      `yaml:"retries,omitempty"`
}

// Database contains persistence layer configuration
type Database struct {
	Type     string `yaml:"type"`
	Host     string `yaml:"host"`
	Port     string `yaml:"port"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	Name     string `yaml:"name"`
}

// Monitoring contains observability settings
type Monitoring struct {
	Enabled  bool   `yaml:"enabled"`
	Type     string `yaml:"type"`
	Endpoint string `yaml:"endpoint"`
	Alerts   Alerts `yaml:"alerts,omitempty"`
}

// Alerts defines monitoring notification rules
type Alerts struct {
	CPUThreshold    int    `yaml:"cpuThreshold"`
	MemoryThreshold int    `yaml:"memoryThreshold"`
	Email           string `yaml:"email,omitempty"`
	SlackWebhook    string `yaml:"slackWebhook,omitempty"`
}

// SecretsConfig defines secret management
type SecretsConfig struct {
	Provider string         `yaml:"provider"`
	Doppler  *DopplerConfig `yaml:"doppler,omitempty"`
	Vault    *VaultConfig   `yaml:"vault,omitempty"`
	Files    []SecretFile   `yaml:"files,omitempty"`
	Project  string         `yaml:"project,omitempty"`
	Config   string         `yaml:"config,omitempty"`
	token    string         `yaml:"token,omitempty"`
}

// DopplerConfig contains Doppler-specific settings
type DopplerConfig struct {
	Project string `yaml:"project"`
	Config  string `yaml:"config"`
	Token   string `yaml:"token,omitempty"`
}

// VaultConfig contains HashiCorp Vault settings
type VaultConfig struct {
	Address string `yaml:"address"`
	Token   string `yaml:"token"`
	Path    string `yaml:"path"`
}

// SecretFile defines file-based secrets
type SecretFile struct {
	Path   string `yaml:"path"`
	Secret string `yaml:"secret"`
}

// Logging contains log management configuration
type Logging struct {
	Driver  string            `yaml:"driver"`
	Options map[string]string `yaml:"options,omitempty"`
}

// Backup defines data backup policies
type Backup struct {
	Enabled   bool    `yaml:"enabled"`
	Schedule  string  `yaml:"schedule"`
	Retention int     `yaml:"retentionDays"`
	Storage   Storage `yaml:"storage"`
}

// Storage contains backup storage details
type Storage struct {
	Type      string `yaml:"type"`
	Endpoint  string `yaml:"endpoint,omitempty"`
	Bucket    string `yaml:"bucket"`
	AccessKey string `yaml:"accessKey,omitempty"`
	SecretKey string `yaml:"secretKey,omitempty"`
}

// SSL contains certificate management
type SSL struct {
	Enabled     bool     `yaml:"enabled"`
	Provider    string   `yaml:"provider"`
	Domains     []string `yaml:"domains"`
	Email       string   `yaml:"email"`
	Wildcard    bool     `yaml:"wildcard"`
	DNSProvider string   `yaml:"dns_provider"`
	Staging     bool     `yaml:"staging"`
	Force       bool     `yaml:"force"`
	AutoRenew   bool     `yaml:"auto_renew"`
	Domain      string   `yaml:"domain,omitempty"`
}

// Webhook defines deployment webhooks
type Webhook struct {
	Name   string   `yaml:"name"`
	URL    string   `yaml:"url"`
	Events []string `yaml:"events"`
	Secret string   `yaml:"secret,omitempty"`
}

// EnvVariable contains environment variables
type EnvVariable struct {
	Name   string `yaml:"name"`
	Value  string `yaml:"value"`
	Secret bool   `yaml:"secret,omitempty"`
}

// SaveConfig writes the configuration to a file
func SaveConfig(path string, cfg *NextDeployConfig) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}
