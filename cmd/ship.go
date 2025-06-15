package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var manual bool

// deployCmd represents the deploy command
var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploy a Next.js app to a VPS",
	Long:  "This command builds Docker image, uploads to VPS, and starts the app.",
	Run: func(cmd *cobra.Command, args []string) {
		if manual {
			runStep("Building Docker image...", buildImage)
			runStep("Uploading to VPS...", uploadToVPS)
			runStep("Running containers...", startContainers)
		} else {
			fmt.Println("🚀 Running full deployment automatically...")
			buildImage()
			uploadToVPS()
			startContainers()
		}
	},
}

func runStep(description string, fn func()) {
	fmt.Println("\n🧱 " + description)
	fmt.Print("Press ENTER to run this step...")
	fmt.Scanln() // waits for user input
	fn()
}

func buildImage() {
	fmt.Println("🔨 Building Docker image...")
	// ... shell out to `docker build` or handle via Go
}

func uploadToVPS() {
	fmt.Println("📤 Uploading Docker image to VPS...")
	// ... scp / ssh handling here
}

func startContainers() {
	fmt.Println("🚦 Starting containers on VPS...")
	// ... docker-compose up or similar
}

func init() {
	rootCmd.AddCommand(deployCmd)
	deployCmd.Flags().BoolVarP(&manual, "manual", "m", false, "Run each deployment step manually")
}
