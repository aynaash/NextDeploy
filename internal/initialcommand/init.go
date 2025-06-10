package initialcommand

import (
	"bufio"
	"fmt"
	"github.com/spf13/cobra"
	"nextdeploy/internal/config"
	"nextdeploy/internal/docker"
	"nextdeploy/internal/logger"
	"os"
)

func RunInitCommand(cmd *cobra.Command, args []string) error {
	dm := docker.NewDockerManager(true, nil)
	reader := bufio.NewReader(os.Stdin)
	log := logger.DefaultLogger()

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

// pack  VGVGage initflow
//
// import (
// 	"github.com/yourorg/nextdeploy/types"
// 	"nextdeploy/internal/detect"
// 	"nextdeploy/internal/fs"
// 	"nextdeploy/internal/prompt"
// 	"nextdeploy/internal/secrets"
// )
//
// func InitFlow(opts types.InitOptions, prompter prompt.Prompter) error {
// 	// Detect package manager
// 	pm, err := detect.DetectPackageManager(".")
// 	if err != nil {
// 		return err
// 	}
//
// 	if opts.PackageManager == "" {
// 		opts.PackageManager = pm
// 	}
//
// 	// Confirm overwrites
// 	fw := fs.NewFileWriter(opts.Force, opts.DryRun)
// 	if err := fw.Write("Dockerfile", []byte("...docker content...")); err != nil {
// 		return err
// 	}
//
// 	if err := fw.Write("nextdeploy.yml", []byte("...config...")); err != nil {
// 		return err
// 	}
//
// 	// Secret bootstrapping
// 	if opts.SecretsProvider != "" {
// 		sp := secrets.NewProvider(opts.SecretsProvider)
// 		if err := sp.BootstrapSecrets("."); err != nil {
// 			return err
// 		}
// 	}
//
// 	return nil
// }
