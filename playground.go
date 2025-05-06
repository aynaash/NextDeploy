//go:build ignore
// +build ignore

package cmd

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var (
	imageName string
	registry  string
	noCache   bool
	tag       string
)

var buildCmd = &cobra.Command{
	Use:   "build",
	Short: "Build Docker image from the Dockerfile in the current directory",
	Long: `Builds a Docker image using the Dockerfile in the current directory.
Automatically tags the image using the latest Git commit hash, or you can provide a custom tag.
Warns if there are uncommitted changes.`,
	Example: `  # Build with default settings (auto-tag with commit hash)
  nextdeploy build

  # Build with custom image name and registry
  nextdeploy build --image dashboard --registry registry.digitalocean.com/nextdeploy

  # Build without cache and with manual tag
  nextdeploy build --no-cache --tag v1.2.3`,
	PreRun: func(cmd *cobra.Command, args []string) {
		if !dockerfileExists() {
			fmt.Println("❌ Error: No Dockerfile found in current directory.")
			fmt.Println("ℹ️  Run 'nextdeploy init' or navigate to a directory with a Dockerfile.")
			os.Exit(1)
		}

		if tag == "" {
			commitHash, err := getGitCommitHash()
			if err != nil {
				fmt.Println("❌ Error: Failed to get latest Git commit hash.")
				os.Exit(1)
			}
			tag = commitHash
			fmt.Printf("ℹ️  No tag provided. Using latest Git commit hash: %s\n", tag)
		}

		if isGitDirty() {
			fmt.Println("⚠️  Warning: You have uncommitted changes.")
			fmt.Println("💡 Commit changes to ensure accurate versioning.")
		}
	},
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("🚀 Building Docker image...")

		fullImageName := fmt.Sprintf("%s/%s:%s", registry, imageName, tag)

		if err := buildDockerImage(fullImageName, noCache); err != nil {
			fmt.Printf("❌ Error: Failed to build image: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("✅ Successfully built: %s\n", fullImageName)
	},
}

func init() {
	buildCmd.Flags().StringVarP(&imageName, "image", "i", "nextjs-app", "Name for the Docker image")
	buildCmd.Flags().StringVarP(&registry, "registry", "r", "registry.digitalocean.com/your-namespace", "Container registry URL")
	buildCmd.Flags().BoolVar(&noCache, "no-cache", false, "hld without using cache")
	buildCmd.Flags().StringVarP(&tag, "tag", "t", "", "Tag for the Docker image (default: latest Git commit)")

	rootCmd.AddCommand(buildCmd)
}

// dockerfileExists checks if a Dockerfile exists in the current directory
func dockerfileExists() bool {
	_, err := os.Stat(filepath.Join(".", "Dockerfile"))
	return !os.IsNotExist(err)
}

// getGitCommitHash returns the short hash of the latest Git commit
func getGitCommitHash() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--short", "HEAD")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	return strings.TrimSpace(out.String()), err
}

// isGitDirty returns true if there are uncommitted changes
func isGitDirty() bool {
	cmd := exec.Command("git", "status", "--porcelain")
	var out bytes.Buffer
	cmd.Stdout = &out
	_ = cmd.Run()
	return out.Len() > 0
}

// buildDockerImage executes the Docker build command
func buildDockerImage(image string, noCache bool) error {
	args := []string{"build", "-t", image, "."}
	if noCache {
		args = append(args, "--no-cache")
	}

	cmd := exec.Command("docker", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
