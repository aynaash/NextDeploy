package registry

import (
	"fmt"
	"nextdeploy/daemon/internal/config"
	"nextdeploy/shared/envstore"
	"os"
	"os/exec"
	"strings"
)

func HandleDigitalOceanRegistryAuth() error {
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Printf("Failed to get user home directory: %v\n", err)
		return err
	}
	envPath := home + "app/.env"
	_, err = os.Stat(envPath)
	if os.IsNotExist(err) {
		fmt.Printf(".env file does not exist at path: %s\n", envPath)
		return nil
	} else if err != nil {
		fmt.Printf("Failed to check .env file: %v\n", err)
		return err
	}
	store, err := envstore.New(envstore.WithEnvFile[string](envPath))
	if err != nil {
		fmt.Printf("Failed to load .env file: %v\n", err)
		return err
	}
	token, _ := store.GetEnv("DIGITALOCEAN_TOKEN")
	// create the ~/.docker/config.json and save the token there
	file, err := os.OpenFile(os.Getenv("HOME")+"/.docker/config.json", os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		fmt.Printf("Failed to open or create Docker config file: %v\n", err)
		return err
	}
	defer file.Close()
	dockerConfig := fmt.Sprintf(`{
			"auths": {
				"registry.digitalocean.com": {
					"auth": "%s"
				}
			}
	}`, token)
	// write the token to the file
	_, err = file.WriteString(dockerConfig)
	if err != nil {
		fmt.Printf("Failed to write to Docker config file: %v\n", err)
	}

	if token == "" {
		fmt.Println("DigitalOcean token is not set in .env file")
		return err
	}
	fmt.Printf("DigitalOcean token: %s\n", token)

	// read config file
	nextdeployPath := "~/app/nextdeploy.yml"
	cfg, err := config.ReadConfigInServer(nextdeployPath)
	if err != nil {
		fmt.Printf("Failed to read nextdeploy.yml file: %v\n", err)
		return err
	}
	fmt.Printf("NextDeploy config: %+v\n", cfg)
	// read the nextdeploy lock file
	filePath := "~/app/.nextdeploy/buil.lock"
	_, err = os.Stat(filePath)
	if os.IsNotExist(err) {
		return nil
	} else if err != nil {
		fmt.Printf("Failed to check build lock file: %v\n", err)
	}
	commit, err := config.GetGitCommit(filePath)
	if err != nil {
		fmt.Printf("Failed to get git commit from build lock file: %v\n", err)
		return err
	}
	fmt.Printf("Git commit from build lock file: %s\n", commit)
	fullImage := fmt.Sprintf("%s:%s", cfg.Docker.Image, commit)

	fmt.Printf("Full Docker image to pull: %s\n", fullImage)
	// pull the image from digitalocean registry
	err = pullDockerImage(fullImage)
	return nil

}

func GetRegistryType() string {
	nextdeployPath := "~/app/nextdeploy.yml"
	cfg, err := config.ReadConfigInServer(nextdeployPath)
	if err != nil {
		fmt.Printf("Failed to read nextdeploy.yml file: %v\n", err)
		return ""
	}
	registry := cfg.Docker.Registry
	digitalocean := strings.Contains(registry, "digitalocean")
	if digitalocean {
		return "digitalocean"
	}
	dockerhub := strings.Contains(registry, "docker.io")
	if dockerhub {
		return "dockerhub"
	}
	ecr := strings.Contains(registry, "amazonaws")
	if ecr {
		return "ecr"
	}
	ghr := strings.Contains(registry, "github")
	if ghr {
		return "ghcr"
	}
	return ""
}
func pullDockerImage(image string) error {
	cmd := exec.Command("docker", "pull", image)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		fmt.Printf("Failed to pull Docker image: %v\n", err)
		return err
	}
	fmt.Printf("Docker image pulled successfully: %s\n", image)
	return nil
}
