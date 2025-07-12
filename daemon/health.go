package daemon
import (
	"context"
)
func ProbeContainerHealth(containerID string) (string, error) {
	inspect, err := dockerCli.ContainerInspect(context.Background(), containerID)
	if err != nil {
		return "error", err
	}
	if inspect.State != nil && inspect.State.Health != nil {
		return inspect.State.Health.Status, nil
	}
	return "unknown", nil
}

func SystemHealth() map[string]string {
	// Use gopsutil here later for RAM, CPU
	return map[string]string{
		"uptime": "12h",
		"cpu":    "8%",
		"mem":    "34%",
	}
}
