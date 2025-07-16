//
//
// HERE : In this command we have s small idea of error handling and logging where
//       we pass go errors to a generic failfast function that handles the error
//       I have confned it  to this command mostly because i want it reviewed and made better by other devs
//       NOTE: Please check it out and let me know if you have any suggestions or improvements or you think it is a bad
//             idea to use this pattern in the project or in any go project

// TODO: fix logic to get env file and run with latest image just locally
package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"nextdeploy/internal/failfast"
	"nextdeploy/internal/git"
	"nextdeploy/internal/logger"
	"nextdeploy/internal/nextdeploy"
	"nextdeploy/internal/secrets"
	"os"
	"os/exec"
	"path/filepath"
)

type Config struct {
	Image string `yaml:"image"`
}

var (
	runLogger = logger.PackageLogger("RunImage::", "🚀 Run Image::")
)

var runimageCmd = &cobra.Command{
	Use:   "runimage",
	Short: "Run Docker image with configuration from YAML",
	Long: `Reads image name from YAML config, gets tag from git commit or flag,
and runs the Docker container with environment variables from Doppler.`,
	Run: func(cmd *cobra.Command, args []string) {
		runImage()
	},
}

func init() {
	rootCmd.AddCommand(runimageCmd)
}

func runImage() {
	// Read config file
	config, err := nextdeploy.Load("nextdeploy.yml")
	failfast.Failfast(err, failfast.Error, "Failed to load configuration file")

	imageTag, err := git.GetCommitHash()
	failfast.Failfast(err, failfast.Error, "Failed to get git commit hash")
	sm, err := secrets.NewSecretManager()
	failfast.Failfast(err, failfast.Error, "Failed to initialize SecretManager")
	runLogger.Debug("SecretManager initialized successfully")
	if sm.IsDopplerEnabled() {
		runLogger.Info("Doppler is enabled, downloading secrets...")
		failfast.Failfast(err, failfast.Error, "Failed to download Doppler secrets")
	} else {
		runLogger.Warn("Doppler is not enabled, skipping secrets download.")
	}
	// Load secrets

	// Run Docker container
	fullImage := fmt.Sprintf("%s:%s", config.Docker.Image, imageTag)
	runLogger.Debug("Full Docker image to run: %s", fullImage)
	err = runDockerContainer(fullImage)
	failfast.Failfast(err, failfast.Error, "Failed to run Docker container")

	fmt.Println("Docker container started successfully")
}

func runDockerContainer(image string) error {
	absPath, err := filepath.Abs(".env")
	runLogger.Debug("Absolute path for .env file: %s", absPath)
	failfast.Failfast(err, failfast.Error, "Failed to get absolute path for .env file")

	cmd := exec.Command("docker", "run", "-p", "3000:3000", "--env-file", absPath, image)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err = cmd.Run()
	failfast.Failfast(err, failfast.Error, "Failed to run Docker container")
	runLogger.Success("Docker container started with image: %s", image)

	return nil
}
