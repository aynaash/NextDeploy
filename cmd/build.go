
package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"nextdeploy/internal/docker"
	"nextdeploy/internal/git"
	"nextdeploy/internal/validators"
)

var (
	imageName string
	registry  string
	noCache   bool
	pull      bool
	tag       string
	target    string
	platform  string
	buildArgs []string
)

var buildCmd = &cobra.Command{
	Use:   "build",
	Short: "Build Docker image from the Dockerfile",
	PreRunE: func(cmd *cobra.Command, args []string) error {
		dm := docker.NewDockerManager(true, nil)

		// Validate Docker installation
		if err := dm.CheckDockerInstalled(); err != nil {
			return fmt.Errorf("docker is not installed or not functioning: %w", err)
		}

		// Check for Dockerfile
		exists, err := dm.DockerfileExists(".")
		if err != nil {
			return fmt.Errorf("failed to check for Dockerfile: %w", err)
		}
		if !exists {
			return fmt.Errorf("Dockerfile not found in current directory")
		}

		// Validate registry if provided
		if registry != "" && !validators.IsValidRegistry(registry) {
			return fmt.Errorf("invalid registry format: %s", registry)
		}

		// Set default tag from Git if not provided
		if tag == "" {
			hash, err := git.GetCommitHash()
			if err != nil {
				return fmt.Errorf("failed to get git commit hash: %w", err)
			}
			tag = hash
			cmd.Printf("Using Git commit hash as tag: %s\n", tag)
		}

		// Warn about uncommitted changes
		if git.IsDirty() {
			cmd.Printf("Warning: Uncommitted changes present in working directory\n")
		}

		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		dm := docker.NewDockerManager(true, nil)
  //
		 // Construct full image name with registry and tag
		fullImage := constructImageName(imageName, registry, tag)
		cmd.Printf("Building image: %s\n", fullImage)

		// Prepare build options
		opts := docker.BuildOptions{
			ImageName: fullImage,
			NoCache:   noCache,
			Pull:      pull,
			Target:    target,
			Platform:  platform,
			BuildArgs: buildArgs,
		}

		// Execute build with context
		ctx := context.Background()
		if err := dm.BuildImage(ctx, ".", opts); err != nil {
			return fmt.Errorf("build failed: %w", err)
		}

		cmd.Printf("Successfully built image: %s\n", fullImage)
		return nil
	},
}

func init() {
	// Image configuration flags
	buildCmd.Flags().StringVarP(&imageName, "image", "i", "nextjs-app", 
		"Name for the Docker image (without registry)")
	buildCmd.Flags().StringVarP(&registry, "registry", "r", "", 
		"Container registry URL (e.g., 'docker.io/myorg')")
	buildCmd.Flags().StringVarP(&tag, "tag", "t", "", 
		"Tag for the Docker image (default: Git commit hash)")

	// Build optimization flags
	buildCmd.Flags().BoolVar(&noCache, "no-cache", false, 
		"Build without using cache")
	buildCmd.Flags().BoolVar(&pull, "pull", false, 
		"Always attempt to pull a newer version of the base image")

	// Advanced build flags
	buildCmd.Flags().StringVar(&target, "target", "", 
		"Set the target build stage to build")
	buildCmd.Flags().StringVar(&platform, "platform", "", 
		"Set platform if server is multi-platform capable")
	buildCmd.Flags().StringArrayVar(&buildArgs, "build-arg", []string{}, 
		"Set build-time variables")

	rootCmd.AddCommand(buildCmd)
}

func constructImageName(name, registry, tag string) string {
	fullImage := name
	if registry != "" {
		fullImage = strings.TrimSuffix(registry, "/") + "/" + name
	}
	return fullImage + ":" + tag
}
