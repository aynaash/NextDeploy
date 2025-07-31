package docker

type ImageSpec struct {
	Name        string
	Tag         string
	NodeVersion string
	PackageMgr  string
	Entrypoint  []string
	ContextPath string
}

type ContainerSpec struct {
	Name       string
	Image      string
	Env        map[string]string
	Ports      map[string]string
	Volumes    map[string]string
	Entrypoint []string
	WorkingDir string
}

type ContainerHealth struct {
	Status     string
	StartedAt  string
	FinishedAt string
	Logs       []string
}

type ContainerStats struct {
	CPUPercentage  float64
	MemoryUsageMB  float64
	MemoryLimitMB  float64
	MemoryPercent  float64
	NetworkRxBytes int64
	NetworkTxBytes int64
	DiskReadBytes  int64
	DiskWriteBytes int64
}
