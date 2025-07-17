
package docker

import (
	"fmt"
	"os/exec"
)

func Run(image string, env map[string]string, port int) (string, error) {
	args := []string{"run", "-d", "-p", fmt.Sprintf("%d:%d", port, port)}
	for k, v := range env {
		args = append(args, "-e", fmt.Sprintf("%s=%s", k, v))
	}
	args = append(args, image)

	cmd := exec.Command("docker", args...)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}

	containerID := string(out[:12])
	fmt.Println("🧪 Started container:", containerID)
	return containerID, nil
}

func Stop(containerID string) {
	_ = exec.Command("docker", "stop", containerID).Run()
}
