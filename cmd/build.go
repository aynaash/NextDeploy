
package cmd

import (
	"context"
	"fmt"
	"os"
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
	tag       string
)
/*
* This commands needs to do the following 
* 1. Check if Dockerfile DockerfileExists
* 2. Validate the image name format
* 3. Validate the registry format
* 4. If tag is not provided, use the Git commit hash as the tag
* 5. Check if there are uncommitted changes in the Git repository
* 6. Build the Docker image using the provided or default IsValidRegistry
* 7. Print success or error messages accordingly
* * Usage:
* nextdeploy build [flags]
* Flags:
* --image string       Name for the Docker image (default "nextjs-app")
* --registry string    Container registry URL (default "registry.digitalocean.com/your-namespace")
* --no-cache           Build without using cache
* --tag string         Tag for the Docker image (default is Git commit hash)
* Example:
* nextdeploy build --image my-nextjs-app --registry registry.digitalocean.com/my-namespace --no-cache --tag v1.0.0
*
*
*
*/
var buildCmd = &cobra.Command{
	Use:   "build",
	Short: "Build Docker image from the Dockerfile",
	PreRun: func(cmd *cobra.Command, args []string) {
		// check if Docker is installed
		if err := docker.CheckDockerInstalled(); err != nil {
			fmt.Println("❌ Docker is not installed or not in PATH.")
			os.Exit(1)
		}

// check if Dockerfile exists 
		if !docker.DockerfileExists() { 
			fmt.Println("❌ Dockerfile not found in the current directory. Please run 'nextdeploy init' to create one.")
			os.Exit(1)
		}
		if err := docker.ValidateImageName(imageName); err != nil {
			fmt.Printf("❌ Invalid image name: %v\n", err)
			os.Exit(1)
		}
		if registry != "" && !validators.IsValidRegistry(registry) {
			fmt.Println("❌ Invalid registry format.")
			os.Exit(1)
		}
		if tag == "" {
			hash, err := gitutils.GetCommitHash()
			if err != nil {
				fmt.Printf("❌ Failed to get commit hash: %v\n", err)
	 		os.Exit(1)
			}
			tag = hash
			fmt.Printf("ℹ️  Using Git commit hash as tag: %s\n", tag)
		}
		if gitutils.IsDirty() {
			fmt.Println("⚠️  Uncommitted changes present.")
		}
	},
	Run: func(cmd *cobra.Command, args []string) {
		fullImage := strings.TrimSuffix(registry, "/") + "/" + imageName + ":" + tag
		fmt.Printf("🚀 Building image: %s\n", fullImage)

		// build the Docker image 
		 ctx := context.Background()
		 if err := docker.BuildImage(ctx, fullImage, noCache); err != nil {
			 fmt.Printf("❌ Build failed: %v\n", err)
			 os.Exit(1)
		 }
		fmt.Println("✅ Build successful.")
	},
}

func init() {
	buildCmd.Flags().StringVarP(&imageName, "image", "i", "nextjs-app", "Name for the Docker image")
	buildCmd.Flags().StringVarP(&registry, "registry", "r", "registry.digitalocean.com/your-namespace", "Container registry URL")
	buildCmd.Flags().BoolVar(&noCache, "no-cache", false, "Build without using cache")
	buildCmd.Flags().StringVarP(&tag, "tag", "t", "", "Tag for the Docker image")
	rootCmd.AddCommand(buildCmd)
}
	
