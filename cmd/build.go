package cmd
import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
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
		// Validate Dockerfile exists
		if !dockerfileExists() {
			fmt.Println("❌ Error: No Dockerfile found in current directory.")
			fmt.Println("ℹ️  Run 'nextdeploy init' or navigate to a directory with a Dockerfile.")
			os.Exit(1)
		}

		// Validate image name format
		if !isValidImageName(imageName) {
			fmt.Println("❌ Error: Invalid image name. Must be lowercase alphanumeric with hyphens/underscores")
			os.Exit(1)
		}

		// Validate registry format
		if registry != "" && !isValidRegistry(registry) {
			fmt.Println("❌ Error: Invalid registry format. Should be [domain/]namespace")
			os.Exit(1)
		}

		// Set default tag if not provided
		if tag == "" {
			commitHash, err := getGitCommitHash()
			if err != nil {
				fmt.Printf("❌ Error: Failed to get Git commit hash: %v\n", err)
				fmt.Println("💡 Either initialize a Git repo or provide a tag with --tag")
				os.Exit(1)
			}
			tag = commitHash
			// Safely display the hash (minimum 7 characters, show up to 8 if available)
			displayLength := len(tag)
			if displayLength > 8 {
				displayLength = 8
			}
			fmt.Printf("ℹ️  Using Git commit hash as tag: %s\n", tag[:displayLength])
		}

		// Check for uncommitted changes
		if isGitDirty() {
			fmt.Println("⚠️  Warning: You have uncommitted changes in Git.")
			fmt.Println("💡 Consider committing changes for reproducible builds.")
		}
	},
	Run: func(cmd *cobra.Command, args []string) {
		// Construct full image name
		fullImageName := strings.TrimSuffix(registry, "/") + "/" + imageName + ":" + tag
		fmt.Printf("🚀 Building Docker image: %s\n", fullImageName)

		// Build the image
		if err := buildDockerImage(fullImageName, noCache); err != nil {
			fmt.Printf("❌ Error: Failed to build image: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("\n✅ Successfully built: %s\n", fullImageName)
		fmt.Println("ℹ️  You can now push this image with: nextdeploy push")
	},
}

func init() {
	buildCmd.Flags().StringVarP(&imageName, "image", "i", "nextjs-app", "Name for the Docker image")
	buildCmd.Flags().StringVarP(&registry, "registry", "r", "registry.digitalocean.com/your-namespace", "Container registry URL")
	buildCmd.Flags().BoolVar(&noCache, "no-cache", false, "Build without using cache")
	buildCmd.Flags().StringVarP(&tag, "tag", "t", "", "Tag for the Docker image (default: latest Git commit hash)")
	rootCmd.AddCommand(buildCmd)
}

// dockerfileExists checks if Dockerfile exists in current directory
func dockerfileExists() bool {
	_, err := os.Stat(filepath.Join(".", "Dockerfile"))
	return !os.IsNotExist(err)
}

// getGitCommitHash returns the short commit hash
func getGitCommitHash() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--short=7", "HEAD")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("git command failed: %v", err)
	}
	return strings.TrimSpace(out.String()), nil
}

// isGitDirty checks for uncommitted changes
func isGitDirty() bool {
	cmd := exec.Command("git", "status", "--porcelain")
	var out bytes.Buffer
	cmd.Stdout = &out
	_ = cmd.Run()
	return out.Len() > 0
}

// buildDockerImage executes the docker build command
func buildDockerImage(imageName string, noCache bool) error {
	args := []string{"build", "-t", imageName, "."}
	if noCache {
		args = append(args, "--no-cache")
	}

	cmd := exec.Command("docker", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// isValidImageName checks if the name follows Docker conventions
func isValidImageName(name string) bool {
	matched, _ := regexp.MatchString(`^[a-z0-9]+(?:[._-][a-z0-9]+)*$`, name)
	return matched
}

// isValidRegistry checks basic registry format
func isValidRegistry(registry string) bool {
	// Basic validation - can be enhanced
	return !strings.Contains(registry, " ") && 
		!strings.HasPrefix(registry, "/") && 
		!strings.HasSuffix(registry, "/")
}
