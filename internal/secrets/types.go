
package secrets


type Config struct {
	Version     string           `yaml:"version"`
	App         AppConfig        `yaml:"app"`
	Repository  RepositoryConfig `yaml:"repository"`
	Docker      DockerConfig     `yaml:"docker"`
	Deployment  DeploymentConfig `yaml:"deployment"`
	Database    DatabaseConfig   `yaml:"database"`
	Logging     LoggingConfig    `yaml:"logging"`
	Monitoring  MonitoringConfig `yaml:"monitoring"`
	Backup      BackupConfig     `yaml:"backup"`
	SSL         SSLConfig        `yaml:"ssl"`
	Webhook     WebhookConfig    `yaml:"webhook"`
}

type AppConfig struct {
	Name        string `yaml:"name"`
	Environment string `yaml:"environment"`
	Domain      string `yaml:"domain"`
	Port        int    `yaml:"port"`
}

type RepositoryConfig struct {
	URL           string `yaml:"url"`
	Branch        string `yaml:"branch"`
	AutoDeploy    bool   `yaml:"auto_deploy"`
	WebhookSecret string `yaml:"webhook_secret"`
}

type DockerConfig struct {
	Build    DockerBuildConfig `yaml:"build"`
	Image    string            `yaml:"image"`
	Registry string            `yaml:"registry"`
	Push     bool              `yaml:"push"`
}

type DockerBuildConfig struct {
	Context     string            `yaml:"context"`
	Dockerfile string            `yaml:"dockerfile"`
	Args        map[string]string `yaml:"args"`
	NoCache     bool              `yaml:"no_cache"`
}

type DeploymentConfig struct {
	Server    ServerConfig    `yaml:"server"`
	Container ContainerConfig `yaml:"container"`
}

type ServerConfig struct {
	Host       string `yaml:"host"`
	Port       int    `yaml:"port"`
	SSHKeyPath string `yaml:"ssh_key_path"`
	SSHKey     string `yaml:"ssh_key"`
	Username   string `yaml:"username"`
	UseSudo    bool   `yaml:"use_sudo"`
}

type ContainerConfig struct {
	Name        string             `yaml:"name"`
	Restart     string             `yaml:"restart"`
	Volumes     []string           `yaml:"volumes"`
	Ports       []string           `yaml:"ports"`
	HealthCheck HealthCheckConfig  `yaml:"health_check"`
}

type HealthCheckConfig struct {
	Path     string `yaml:"path"`
	Interval string `yaml:"interval"`
	Timeout  string `yaml:"timeout"`
	Retries  int    `yaml:"retries"`
}

type LoggingConfig struct {
	Enabled    bool     `yaml:"enabled"`
	Provider   string   `yaml:"provider"`
	StreamLogs []string `yaml:"streams"`
	LogPath    string   `yaml:"log_path"`
}

type MonitoringConfig struct {
	Enabled          bool                  `yaml:"enabled"`
	CPUThreshold    int                   `yaml:"cpu_threshold"`
	MemoryThreshold int                   `yaml:"memory_threshold"`
	DiskThreshold   int                   `yaml:"disk_threshold"`
	Alert           MonitoringAlertConfig `yaml:"alert"`
}

type MonitoringAlertConfig struct {
	Email        string `yaml:"email"`
	SlackWebhook string `yaml:"slack_webhook"`
	NotifyOnError bool   `yaml:"notify_on_error"`
}

type BackupConfig struct {
	Enabled       bool          `yaml:"enabled"`
	Frequency     string        `yaml:"frequency"`
	RetentionDays int           `yaml:"retention_days"`
	Storage       StorageConfig `yaml:"storage"`
}

type StorageConfig struct {
	Provider  string `yaml:"provider"`
	Bucket    string `yaml:"bucket"`
	Region    string `yaml:"region"`
	AccessKey string `yaml:"access_key"`
	SecretKey string `yaml:"secret_key"`
}

type SSLConfig struct {
	Enabled         bool   `yaml:"enabled"`
	Provider        string `yaml:"provider"`
	CertificatePath string `yaml:"certificate_path"`
	KeyPath         string `yaml:"key_path"`
	AutoRenew       bool   `yaml:"auto_renew"`
}

type WebhookConfig struct {
	URL       string   `yaml:"url"`
	OnSuccess []string `yaml:"on_success"`
	OnFailure []string `yaml:"on_failure"`
}

type DatabaseConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	Name     string `yaml:"name"`
}

type Secret struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}
