package cmd

import (
	"context"
	"fmt"
	"nextdeploy/shared"
	"nextdeploy/shared/config"
	"nextdeploy/shared/docker"
	"nextdeploy/shared/git"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var (
	buildlogger      = shared.PackageLogger("BUILD", "🧱 BUILD")
	ProvisionEcrUser bool
	fresh            = false // delete current exisiting user start a fresh
)

var buildCmd = &cobra.Command{
	Use:   "build",
	Short: "Build Docker image from the Dockerfile using metadata from nextcore collected by nextdeploy",
	Long: `
	Builds a Docker image with smart defaults and configuration data 
	collected by nextcore engine. 
	Our system that enables developer to use next all features in container 
	Envirenment built for cloud in a cloud agnostic way.
	.

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
	buildCmd.Flags().BoolVarP(&ProvisionEcrUser, "provision-ecr-user", "p", false, "Provision ECR user for pushing images")
}

func buildCmdFunction(cmd *cobra.Command, args []string) error {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Initialize components
	dm, err := docker.NewDockerClient(buildlogger)
	if err != nil {
		return fmt.Errorf("failed to create Docker client: %w", err)
	}
	// Determine environment (dev/prod)
	env := os.Getenv("NODE_ENV")
	if env == "" {
		env = "production"
	}
	var buildArgs map[string]string
	buildArgs = make(map[string]string)
	for k, v := range dm.GetBuildArgs() {
		// Check if the pointer is nil before dereferencing
		if v != nil {
			// log out the build args
			buildlogger.Info("Build arg: %s=%s\n", k, *v)
			// Skip empty values
			if *v != "" {
				buildArgs[k] = *v
			}
		} else {
			buildlogger.Warn("Build arg %s is nil, skipping", k)
		}
	}
	tag, _ := git.GetCommitHash()
	//Validate the tag
	if !ValidateDockerTag(tag) {
		buildlogger.Debug("Invalid Docker tag format: %s", tag)
		return fmt.Errorf("invalid Docker tag format: %s", tag)
	}

	imagename := cfg.Docker.Image + ":" + tag
	// Auto-configure build options based on environment
	opts := docker.BuildOptions{
		ImageName:        imagename,
		NoCache:          env == "production", // No cache in production
		Pull:             cfg.Docker.AlwaysPull || env == "production",
		Target:           cfg.Docker.Target,
		Platform:         cfg.Docker.Platform,
		BuildArgs:        buildArgs,
		ProvisionEcrUser: ProvisionEcrUser,
		Fresh:            fresh,
	}

	// Build options for image are
	// Log what we're doing
	cmd.Printf("Building %s image for %s environment\n", opts.ImageName, env)
	if opts.NoCache {
		cmd.Println("Cache disabled (production build)")
	}

	// Execute build
	ctx := context.Background()
	if err := dm.BuildAndDeploy(ctx, opts); err != nil {
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
	dm, err := docker.NewDockerClient(buildlogger)
	if err != nil {
		return fmt.Errorf("failed to create Docker client: %w", err)
	}

	// Validate Docker installation
	if err := dm.CheckDockerInstalled(); err != nil {
		return fmt.Errorf("docker is not installed or not functioning: %w", err)
	}
	// Check for Dockerfile
	// FIX::: we should generate the metdata need and use that to build the image
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
func isGitCommitHash(tag string) bool {
	if len(tag) < 7 || len(tag) > 40 {
		return false
	}
	matched, _ := regexp.MatchString(`^[a-f0-9]+$`, tag)
	return matched
}
func ValidateDockerTag(tag string) bool {
	// Check for simple git hash (7-40 hex chars)
	if isGitCommitHash(tag) {
		return true
	}

	// Check for git hash with suffix (separated by - or _)
	hashWithSuffix := regexp.MustCompile(`^[a-f0-9]{7,40}[-_][a-z0-9][a-z0-9._-]{0,126}$`)
	if hashWithSuffix.MatchString(tag) {
		return true
	}

	// Check for semantic version (v1.0.0)
	semVer := regexp.MustCompile(`^v\d+\.\d+\.\d+$`)
	if semVer.MatchString(tag) {
		return true
	}

	return false
}
