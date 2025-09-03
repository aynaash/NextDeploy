package daemon

import (
	"nextdeploy/daemon/internal/types"
	"os/exec"
	"strings"
)

type DockerClient struct {
	config *types.DaemonConfig
}

func NewDockerClient(config *types.DaemonConfig) *DockerClient {
	return &DockerClient{config: config}
}
func (dc *DockerClient) ContainerExists(container string) bool {
	cmd := exec.Command("docker", "ps", "-a", "--filter", "name="+container, "--format", "{{.Names}}")
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(output)) == container
}

func (dc *DockerClient) CheckDockerAccess() error {
	cmd := exec.Command("docker", "version")
	return cmd.Run()
}

func (dc *DockerClient) ExecuteCommand(args ...string) error {
	cmd := exec.Command("docker", args...)
	return cmd.Run()
}

func (dc *DockerClient) ExecuteCommandWithOutput(args ...string) ([]byte, error) {
	cmd := exec.Command("docker", args...)
	return cmd.Output()
}

func (dc *DockerClient) GetContainerInfo(containerName string) (*types.ContainerInfo, error) {
	cmd := exec.Command("docker", "inspect", "--format", "{{.State.Status}}", containerName)
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	return &types.ContainerInfo{
		Name:   containerName,
		Status: string(output),
	}, nil
}

func (cd *DockerClient) ParseContainerList(output string) []map[string]string {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	var containers []map[string]string

	if len(lines) < 1 {
		return containers
	}

	for _, line := range lines[1:] {
		if strings.TrimSpace(line) == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) >= 6 {
			containers = append(containers, map[string]string{
				"id":      fields[0],
				"name":    fields[1],
				"image":   fields[2],
				"status":  fields[3],
				"ports":   fields[4],
				"created": strings.Join(fields[5:], " "),
			})
		}
	}
	return containers
}
