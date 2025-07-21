package initialcommand

import (
	"bufio"
	"fmt"
	"nextdeploy/shared"
	"nextdeploy/shared/config"
	"nextdeploy/shared/docker"
	"os"

	"github.com/spf13/cobra"
)

func RunInitCommand(cmd *cobra.Command, args []string) error {
	dm, err := docker.NewDockerClient(shared.SharedLogger)
	if err != nil {
		return fmt.Errorf("failed to create Docker client: %w", err)
	}
	reader := bufio.NewReader(os.Stdin)
	log := shared.PackageLogger("Initialization", "Initialization")

	//	cmd.Println("🚀 NextDeploy Initialization")
	log.Info("🚀 NextDeploy Initialization")
	log.Info("----------------------------------------")
	if err := config.HandleConfigSetup(cmd, reader); err != nil {
		return fmt.Errorf("configuration setup failed: %w", err)
	}

	if err := docker.HandleDockerfileSetup(cmd, dm, reader); err != nil {
		return fmt.Errorf("docker setup failed: %w", err)
	}

	log.Info("\n🎉 Setup complete! Next steps:")
	if exists, _ := dm.DockerfileExists("."); exists {
		log.Info("- Review the generated Dockerfile")
	}
	log.Info("- Review your nextdeploy.yml configuration")
	log.Info("- Run 'nextdeploy build' to build the Docker image")

	return nil
}
