package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
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
	if err != nil {
		fmt.Printf("Error reading config: %v\n", err)
		os.Exit(1)
	}

	imageTag, err := git.GetCommitHash()
	if err != nil {
		fmt.Printf("Error getting git commit hash: %v\n", err)
		os.Exit(1)
	}
	sm, err := secrets.NewSecretManager()
	if err != nil {
		fmt.Printf("Error initializing SecretManager: %v\n", err)
		os.Exit(1)
	}
	runLogger.Debug("SecretManager initialized successfully")
	if sm.IsDopplerEnabled() {
		// TODO: integrate a nicer doppler secret management logic
		runLogger.Info("Doppler is enabled, downloading secrets...")
		err = downloadDopplerSecrets()
		if err != nil {
			fmt.Printf("Error downloading Doppler secrets: %v\n", err)
			os.Exit(1)
		}
	} else {
		runLogger.Warn("Doppler is not enabled, skipping secrets download.")
	}
	keyPath := sm.GetKey()

	key, err := os.ReadFile(keyPath)
	if err != nil {
		runLogger.Error("Error reading file for key master key")
		os.Exit(1)
	}

	// Get the key for decrypting the file
	cwd, err := sm.PrepareAppContext(string(key))
	if err != nil {
		runLogger.Error("Error preparing app run context:%s", err)
		os.Exit(1)
	}

	// the cwd is the current working directory
	runLogger.Debug("Current working directory: %s", cwd)
	if err != nil {
		fmt.Printf("Error preparing app context: %v\n", err)
		os.Exit(1)
	}
	// Load secrets

	// Run Docker container
	fullImage := fmt.Sprintf("%s:%s", config.Docker.Image, imageTag)
	err = runDockerContainer(fullImage)
	if err != nil {
		fmt.Printf("Error running Docker container: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Docker container started successfully")
}

func downloadDopplerSecrets() error {
	cmd := exec.Command("doppler", "secrets", "download", "--no-file", "--format", "env")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("doppler command failed: %w", err)
	}

	err = os.WriteFile(".env", output, 0644)
	if err != nil {
		return fmt.Errorf("could not write .env file: %w", err)
	}

	return nil
}

func runDockerContainer(image string) error {
	absPath, err := filepath.Abs(".env")
	if err != nil {
		return fmt.Errorf("could not get absolute path for .env: %w", err)
	}

	cmd := exec.Command("docker", "run", "-p", "3000:3000", "--env-file", absPath, image)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("docker run failed: %w", err)
	}
	runLogger.Success("Docker container started with image: %s", image)

	return nil
}
