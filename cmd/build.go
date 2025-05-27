package cmd

import (
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

var buildCmd = &cobra.Command{
	Use:   "build",
	Short: "Build Docker image from the Dockerfile",
	PreRun: func(cmd *cobra.Command, args []string) {
		if !docker.DockerfileExists() {
			fmt.Println("❌ No Dockerfile found.")
			os.Exit(1)
		}
		if !docker.IsValidImageName(imageName) {
			fmt.Println("❌ Invalid image name format.")
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

		if err := docker.BuildImage(fullImage, noCache); err != nil {
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
