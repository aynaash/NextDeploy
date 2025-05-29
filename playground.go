//go:build ignore
// +build ignore


// internal/config/template.go
package config

import (
	"os"
	"path/filepath"
)

const sampleConfig = `# nextdeploy.yml
version: "1.0"

app:
  name: example-app
  environment: production
  domain: app.example.com
  port: 3000

repository:
  url: git@github.com:username/example-app.git
  branch: main
  auto_deploy: true
  webhook_secret: your_webhook_secret  # Optional if using webhook triggers

docker:
  build:
    context: .
    dockerfile: Dockerfile
    args:
      NODE_ENV: production
    no_cache: false
  image: username/example-app:latest
  registry: ghcr.io  # or docker.io, or ECR/GCR if you support it
  push: true  # Push after successful build

# ... rest of the sample config
`

func GenerateSampleConfig() error {
	// Write the sample config to sample.nextdeploy.yml in the current directory
	path := filepath.Join(".", "sample.nextdeploy.yml")
	return os.WriteFile(path, []byte(sampleConfig), 0644)
}

// cmd/init.go
package cmd

import (
	"bufio"
	"fmt"
	"os"
	"nextdeploy/internal/config"
	"nextdeploy/internal/docker"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize your project configuration",
	Run: func(cmd *cobra.Command, args []string) {
		reader := bufio.NewReader(os.Stdin)

		// Ask if they want to generate a sample config
		if config.PromptYesNo(reader, "Would you like to generate a sample configuration file?") {
			if err := config.GenerateSampleConfig(); err != nil {
				fmt.Printf("Error generating sample config: %v\n", err)
			} else {
				fmt.Println("✅ sample.nextdeploy.yml created")
			}
		}

		// Ask if they want to create a custom config
		if config.PromptYesNo(reader, "Would you like to create a custom nextdeploy.yml?") {
			cfg, err := config.PromptForConfig(reader)
			if err != nil {
				fmt.Printf("Error getting configuration: %v\n", err)
				os.Exit(1)
			}

			if err := config.WriteConfig("nextdeploy.yml", cfg); err != nil {
				fmt.Printf("Error writing config: %v\n", err)
				os.Exit(1)
			}
			fmt.Println("✅ nextdeploy.yml created")
		}

		// Handle Dockerfile creation
		if config.PromptYesNo(reader, "Would you like to create a Dockerfile?") {
			if err := docker.HandleDockerfileCreation(reader); err != nil {
				fmt.Printf("Error creating Dockerfile: %v\n", err)
				os.Exit(1)
			}
		}
	},
}


// internal/docker/docker.go
package docker

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

func HandleDockerfileCreation(reader *bufio.Reader) error {
	if DockerfileExists() {
		if !PromptYesNo(reader, "Dockerfile exists. Overwrite?") {
			return nil
		}
	}

	pkgManager := promptPackageManager(reader)
	content := GenerateDockerfile(pkgManager)
	return WriteFile("Dockerfile", content)
}

// ... other Docker-related functions
