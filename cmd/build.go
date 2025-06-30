package cmd

import (
	"context"
	"fmt"
	"nextdeploy/internal/config"
	"nextdeploy/internal/docker"
	"nextdeploy/internal/git"
	"nextdeploy/internal/logger"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var (
	buildlogger      = logger.PackageLogger("BUILD", "🧱 BUILD")
	ProvisionEcrUser bool
	fresh            = false // delete current exisiting user start a fresh
)

var buildCmd = &cobra.Command{
	Use:   "build",
	Short: "Build Docker image from the Dockerfile",
	Long: `Builds a Docker image with smart defaults and configuration.

The command automatically:
- Detects your Git repository state
- Uses appropriate naming conventions
- Applies environment-specific settings
- Handles caching appropriately
- 

Configuration:
  Create a 'nextdeploy.yml' file to customize behavior:

  docker:
    image: "my-app"             # Base image name
    registry: "ghcr.io/myorg"   # Container registry
    strategy: "branch-commit"   # Naming strategy
    alwaysPull: true            # Always pull base images
    platform: "linux/amd64"     # Target platform

Examples:
  # Basic build (auto-detects everything)
  nextdeploy build

  # Build for production (uses different defaults)
  NEXTDEPLOY_ENV=production nextdeploy build

`,
	PreRunE: checkBuildConditionsMet,
	RunE:    buildCmdFunction,
}

type DockerConfig struct {
	Image    string
	Registry string
	Strategy string // "commit-hash", "branch-commit", or "simple"
}

func GenerateImageName(config DockerConfig, gitInfo *git.RepositoryInfo, env string) string {
	// Start with registry if provided
	var parts []string
	if config.Registry != "" {
		parts = append(parts, config.Registry)
	}

	// Add image name or default
	imageName := config.Image
	if imageName == "" {
		imageName = "app"
	}
	parts = append(parts, imageName)

	// Generate tag based on strategy
	tag := ""
	switch config.Strategy {
	case "branch-commit":
		sanitizedBranch := strings.ReplaceAll(gitInfo.BranchName, "/", "-")
		tag = fmt.Sprintf("%s-%s", sanitizedBranch, gitInfo.CommitHash)
	case "simple":
		tag = "latest"
	default: // "commit-hash" or fallback
		tag = gitInfo.CommitHash
	}

	// Add timestamp for production builds
	if env == "production" {
		tag = fmt.Sprintf("%s-%s", tag, time.Now().Format("20060102-150405"))
	}

	// Append tag
	if tag != "" {
		parts = append(parts, tag)
	}

	return strings.Join(parts, ":")
}

func init() {
	// No flags needed - everything comes from config or auto-detection
	rootCmd.AddCommand(buildCmd)
	// provision-ecr-user --fresh
	buildCmd.Flags().BoolVarP(&fresh, "fresh", "f", false, "Delete current existing user and start fresh")
	buildCmd.Flags().BoolVar(&ProvisionEcrUser, "provision-ecr-user", false, "Provision ECR user for pushing images")
}

func buildCmdFunction(cmd *cobra.Command, args []string) error {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Initialize components
	dm := docker.NewDockerManager(true, nil)
	// Determine environment (dev/prod)
	env := os.Getenv("NODE_ENV")
	if env == "" {
		env = "development"
	}
	var builArgs []string
	for k, v := range cfg.Docker.BuildArgs {
		builArgs = append(builArgs, fmt.Sprintf("--build-arg=%v=%s", k, v))
		buildlogger.Debug("Build args look like this %v", builArgs)
	}
	tag, _ := git.GetCommitHash()
	imagename := cfg.Docker.Image + ":" + tag
	// Auto-configure build options based on environment
	opts := docker.BuildOptions{
		ImageName:        imagename,
		NoCache:          env == "production", // No cache in production
		Pull:             cfg.Docker.AlwaysPull || env == "production",
		Target:           cfg.Docker.Target,
		Platform:         cfg.Docker.Platform,
		BuildArgs:        cfg.Docker.BuildArgs,
		ProvisionEcrUser: ProvisionEcrUser,
		Fresh:            fresh,
	}

	// Build options for image are
	buildlogger.Info("Build Options: %v", opts)
	// Log what we're doing
	cmd.Printf("Building %s image for %s environment\n", opts.ImageName, env)
	if opts.NoCache {
		cmd.Println("Cache disabled (production build)")
	}

	// Execute build
	ctx := context.Background()
	if err := dm.BuildImage(ctx, ".", opts); err != nil {
		buildlogger.Error("Build failed: %v", err)
		return fmt.Errorf("build failed: %w", err)
	}

	cmd.Printf("Successfully built: %s\n", opts.ImageName)

	// Handle push if configured
	if cfg.Docker.Push {
		cmd.Println("Pushing image to registry...")
		if err := dm.PushImage(ctx, opts.ImageName, opts.ProvisionEcrUser, opts.Fresh); err != nil {
			return fmt.Errorf("push failed: %w", err)
		}
		cmd.Println("Image pushed successfully")
	}

	return nil
}

func checkBuildConditionsMet(cmd *cobra.Command, args []string) error {
	dm := docker.NewDockerManager(true, nil)

	// Validate Docker installation
	if err := dm.CheckDockerInstalled(); err != nil {
		return fmt.Errorf("docker is not installed or not functioning: %w", err)
	}

	// Check for Dockerfile
	if exists, err := dm.DockerfileExists("."); err != nil {
		return fmt.Errorf("failed to check for Dockerfile: %w", err)
	} else if !exists {
		return fmt.Errorf("no Dockerfile found in current directory")
	}

	// Check for uncommitted changes (warning only)
	if git.IsDirty() {
		cmd.Printf("Warning: Building with uncommitted changes\n")
	}

	return nil
}
