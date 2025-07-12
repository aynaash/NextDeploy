package communication

import (
	"nextdeploy/daemon"
)

type DaemonAPI interface {
	DeployApp(req DeployRequest) (DaemonResponse, error)
	StopApp(appName string) (DaemonResponse, error)
	RestartApp(appName string) (DaemonResponse, error)
	GetAppStatus(appName string) (AppStatus, error)
	StreamLogs(appName string, ch chan<- LogStream) error
	SyncSecrets(appName string, secrets map[string]string) error
	ConfigureProxy(route daemon.ProxyRoute) error
	RotateCert(domain string) error
	MonitorSystem() (daemon.SystemMetrics, error)
	SwapBlueGreen(appName string, newImage string) (DaemonResponse, error)
}
