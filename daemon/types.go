package daemon

type AppStatus struct {
	AppName   string   `json:"app_name"`
	Image     string   `json:"image"`
	Status    string   `json:"status"` // e.g., "running", "stopped", "deploying", "error", "pending" "updating"
	Port      string   `json:"port"`
	Domains   []string `json:"domains"`
	UpdatedAt string   `json:"updated_at"`
}

type DeployRequest struct {
	AppName     string   `json:"app_name"`
	Image       string   `json:"image"`
	EnvVars     []string `json:"env_vars"`
	Port        string   `json:"port"`
	Domains     []string `json:"domains"`
	BuildTarget string   `json:"build_target"`
	ProxyRoute  string   `json:"proxy_route"` // cadyy or nginx
}

type DaemonResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Payload interface{} `json:"payload,omitempty"`
}

type SystemMetrics struct {
	CPUUsage    float64 `json:"cpu"`
	MemoryUsage float64 `json:"memory"`
	DiskUsage   float64 `json:"disk"`
	Uptime      string  `json:"uptime"` // e.g., "3h17m"
}

type ProxyRoute struct {
	RouteName string `json:"route_name"`
	Port      int    `json:"port"`
	Domain    string `json:"domain"`
	App       string `json:"app"` // e.g., "myapp"
}
