package communication

type AppStatus struct {
	Name        string `json:"name"`
	ContainerID string `json:"container_id"`
	Status      string `json:"status"` // running, failed, rebuilding
	Port        int    `json:"port"`
	Domain      string `json:"domain"`
	Version     string `json:"version"`
	UpdatedAt   string `json:"updated_at"`
}

type DeployRequest struct {
	AppName   string            `json:"app_name"`
	Image     string            `json:"image"`
	EnvVars   map[string]string `json:"env"`
	Ports     []int             `json:"ports"`
	Domain    string            `json:"domain"`
	ProxyType string            `json:"proxy"` // caddy, nginx
}

type DaemonResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Payload interface{} `json:"payload,omitempty"`
}

type LogStream struct {
	AppName   string `json:"app_name"`
	LogLine   string `json:"log"`
	Timestamp string `json:"ts"`
}
