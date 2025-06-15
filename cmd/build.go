package cmd

import (
	"context"
	"fmt"
	"nextdeploy/internal/build"
	"nextdeploy/internal/config"
	"nextdeploy/internal/docker"
	"nextdeploy/internal/git"
	"nextdeploy/internal/logger"
	"nextdeploy/internal/registry"

	"github.com/spf13/cobra"
)

var (
	buildlogger = logger.PackageLogger("BUILD", "🧱 BUILD")
)
var (
	imageName    string
	noCache      bool
	pull         bool
	tag          string
	target       string
	platform     string
	buildArgs    []string
	nameStrategy string
	useTimestamp bool
	branchName   string
	registryname string
)

var buildCmd = &cobra.Command{
	Use:   "build",
	Short: "Build Docker image from the Dockerfile",
	Long: `Builds a Docker image based on the provided Dockerfile in the current directory.
	Build Docker image with flexible naming strategies.

Examples:
  # Build with default naming (commit hash + timestamp)
  nextdeploy build

  # Build for specific branch with commit hash
  nextdeploy build --name-strategy branch-commit

  # Build with custom registry and simple 'latest' tag
  nextdeploy build --registry docker.io/myorg --name-strategy simple

  # Build with specific branch name override
  nextdeploy build --name-strategy branch-commit --branch feat/new-auth

  # Build with all custom options
  nextdeploy build \
    --image customer-portal \
    --registry ghcr.io/mycompany \
    --name-strategy branch-commit \
    --branch main \
    --no-cache \
    --pull

  # Build with semantic version tag (if implemented)
  nextdeploy build --name-strategy semver --tag v1.2.3`,
	PreRunE: checkBuildCondtionsmet,
	RunE:    buildCmdFunction,
}

func init() {
	// Image Configuration Flags
	buildCmd.Flags().StringVarP(&imageName, "image", "i", "nextjs-app",
		`Base name for the Docker image (without registry)
Examples:
  - "user-service"
  - "frontend-app"
  - "api-gateway"`)

	buildCmd.Flags().StringVarP(&registryname, "registry", "r", "",
		`Container registry URL where the image will be pushed
Examples:
  - "docker.io/myorg" (Docker Hub)
  - "ghcr.io/company" (GitHub Container Registry)
  - "123456789012.dkr.ecr.region.amazonaws.com" (AWS ECR)
  - "localhost:5000" (Local registry)`)

	buildCmd.Flags().StringVarP(&tag, "tag", "t", "",
		`Tag for the Docker image (auto-generated if empty)
Default behavior:
  - Uses Git commit hash (short format)
  - Appends timestamp if --timestamp=true
Examples:
  - "v1.2.3" (semantic version)
  - "prod-20240101" (environment-date)
  - "feature-auth-abc123" (branch-commit)`)

	// Naming Strategy Flags
	buildCmd.Flags().StringVar(&nameStrategy, "name-strategy", "commit-hash",
		`Image tagging strategy:
  commit-hash   : Uses Git commit hash (default)
  branch-commit : Includes branch name and commit hash
  semver        : For semantic versioning (requires --tag)
  simple        : Uses static "latest" tag
Examples:
  - "--name-strategy branch-commit"
  - "--name-strategy simple"`)

	buildCmd.Flags().BoolVar(&useTimestamp, "timestamp", true,
		`Append timestamp to auto-generated tags
Format: YYYYMMDD-HHMMSS
Examples:
  - "main-abc123-20240101-142536"
  - "feat-auth-def456-20240101"`)

	buildCmd.Flags().StringVar(&branchName, "branch", "",
		`Override current Git branch name for tagging
Note: Automatically sanitized (special chars replaced with '-')
Examples:
  - "--branch release/v1.2.0" becomes "release-v1.2.0"
  - "--branch feat/new-auth"`)

	// Build Optimization Flags
	buildCmd.Flags().BoolVar(&noCache, "no-cache", false,
		`Force a clean build by disabling cache
Use cases:
  - When dependency versions have changed
  - For production builds where reproducibility is critical`)

	buildCmd.Flags().BoolVar(&pull, "pull", false,
		`Always attempt to pull newer base images
Recommended for:
  - CI/CD pipelines
  - Ensuring latest security updates`)

	// Advanced Build Flags
	buildCmd.Flags().StringVar(&target, "target", "",
		`Build specific stage from multi-stage Dockerfile
Examples:
  - "--target builder"
  - "--target production"`)

	buildCmd.Flags().StringVar(&platform, "platform", "",
		`Set target platform for multi-architecture builds
Format: os/arch[/variant]
Examples:
  - "--platform linux/amd64"
  - "--platform linux/arm64"
  - "--platform windows/amd64"`)

	buildCmd.Flags().StringArrayVar(&buildArgs, "build-arg", []string{},
		`Set build-time variables (can be repeated)
Format: KEY=VALUE
Examples:
  - "--build-arg VERSION=1.2.3"
  - "--build-arg ENV=production"
  - "--build-arg NODE_ENV=production"`)

	rootCmd.AddCommand(buildCmd)
}

func buildCmdFunction(cmd *cobra.Command, args []string) error {
	dm := docker.NewDockerManager(true, nil)
	imageName := build.ConstructImageName(tag)

	// Construct image name using builder pattern
	// builder := build.NewImageNameBuilder(imageName).
	// 	WithTag(tag).
	// 	WithNameStrategy(nameStrategy).
	// 	WithTimestamp(useTimestamp).
	// 	WithBranch(branchName)
	//
	// fullImage, err := builder.Build()
	// if err != nil {
	// 	return fmt.Errorf("failed to construct image name: %w", err)
	// }

	cmd.Printf("Building image: %s\n", imageName)

	// Prepare build options
	opts := docker.BuildOptions{
		ImageName: imageName,
		NoCache:   noCache,
		Pull:      pull,
		Target:    target,
		Platform:  platform,
		BuildArgs: buildArgs,
	}

	// Execute build with context
	ctx := context.Background()
	if err := dm.BuildImage(ctx, ".", opts); err != nil {
		buildlogger.Error("Failed to build image: %v", err)
		return fmt.Errorf("build failed: %w", err)
	}

	cmd.Printf("Successfully built image: %s\n", imageName)
	// push image if push it set to true
	config, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	push := config.Docker.Push
	if !push {
		cmd.Printf("Skipping image push as 'push' is set to false in configuration\n")
		return nil
	}
	err = dm.PushImage(ctx, imageName)
	return nil
}

// Function to check all build condtions are met before building
func checkBuildCondtionsmet(cmd *cobra.Command, args []string) error {
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
		return fmt.Errorf("dockerfile not found in current directory")
	}
	validatorRegistry, err := registry.New()

	if err != nil {
		return fmt.Errorf("failed to initialize registry validator: %w", err)
	}

	// Validate registry if provided
	isvalidRegistry := validatorRegistry.IsValidRegistry(registryname)
	if registryname != "" && !isvalidRegistry {
		return fmt.Errorf("invalid registry format: %s", registryname)
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
		cmd.Printf("Warning: Uncommitted changes present in working directory.Please commit the changes to build latest version of app\n")
	}

	return nil
}
