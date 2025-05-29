package config

// NextDeployConfig represents the complete configuration structure
type NextDeployConfig struct {
	Version     string         `yaml:"version"`
	App         AppConfig      `yaml:"app"`
	Repository  Repository     `yaml:"repository"`
	Docker      DockerConfig   `yaml:"docker"`
	Deployment  Deployment     `yaml:"deployment"`
	Database    *Database      `yaml:"database,omitempty"`
	Monitoring  *Monitoring    `yaml:"monitoring,omitempty"`
}

// AppConfig contains application-specific settings
type AppConfig struct {
	Name        string `yaml:"name"`
	Port        int    `yaml:"port"`
	Environment string `yaml:"environment"`
	Domain      string `yaml:"domain,omitempty"`
	Secrets    *SecretsConfig `yaml:"secrets,omitempty"`
}

// Repository contains git repository settings
type Repository struct {
	URL          string `yaml:"url"`
	Branch       string `yaml:"branch"`
	AutoDeploy   bool   `yaml:"autoDeploy"`
	WebhookSecret string `yaml:"webhookSecret,omitempty"`
}

// DockerConfig contains Docker-related settings
type DockerConfig struct {
	Image  string     `yaml:"image"`
	Registry string   `yaml:"registry,omitempty"`
	Build  DockerBuild `yaml:"build"`
	Push   bool       `yaml:"push"`
}

// DockerBuild contains Docker build settings
type DockerBuild struct {
	Context    string            `yaml:"context"`
	Dockerfile string            `yaml:"dockerfile"`
	NoCache    bool              `yaml:"noCache"`
	Args       map[string]string `yaml:"args"`
}

// Deployment contains deployment settings
type Deployment struct {
	Server    Server    `yaml:"server"`
	Container Container `yaml:"container"`
}

// Server contains server connection details
type Server struct {
	Host    string `yaml:"host"`
	User    string `yaml:"user"`
	SSHKey  string `yaml:"sshKey"`
	UseSudo bool   `yaml:"useSudo"`
}

// Container contains container runtime settings
type Container struct {
	Name    string   `yaml:"name"`
	Restart string   `yaml:"restart"`
	Ports   []string `yaml:"ports"`
}

// Database contains database connection settings
type Database struct {
	Type     string `yaml:"type"`
	Host     string `yaml:"host"`
	Port     string `yaml:"port"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	Name     string `yaml:"name"`
}

// Monitoring contains monitoring settings
type Monitoring struct {
	Enabled  bool   `yaml:"enabled"`
	Type     string `yaml:"type"`
	Endpoint string `yaml:"endpoint"`
}

type SecretsConfig struct {
	Provider string `yaml:"provider"`
	Project  string `yaml:"project"`
	Config   string `yaml:"config"`
}
// PromptForConfig collects user input for the nextdeploy configuration
