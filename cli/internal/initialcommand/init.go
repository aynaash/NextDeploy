package initialcommand

import (
	"bufio"
	"fmt"
	"nextdeploy/shared"
	"nextdeploy/shared/config"
	"nextdeploy/shared/docker"
	"nextdeploy/shared/nextcore"
	"os"

	"github.com/spf13/cobra"
)

func RunInitCommand(cmd *cobra.Command, args []string) error {
	dm := docker.NewDockerManager(true, nil)
	reader := bufio.NewReader(os.Stdin)
	log := shared.PackageLogger("Initialization", "Initialization")

	buildMetaData, err := nextcore.GenerateMetadata()
	if err != nil {
		return err
	}
	log.Debug("The build meta generated is: %v", buildMetaData)
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
