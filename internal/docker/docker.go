package docker 

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
)



func DockerfileExists() bool {
	_, err := os.Stat(filepath.Join(".", "Dockerfile"))

	return !os.IsNotExist(err)
}



func IsValidImageName(name string)bool {
	matched, _ := regexp.MatchString(`^[a-z0-9]+(?:[._-][a-z0-9]+)*s`, name)
	return matched
}


func BuildImage(imageName string, noCache bool) error {
	args := []string{"build", "-t", imageName, "."}

	if noCache {
		args = append(args, "--no-cache")
	}


	cmd := exec.Command("docker", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
