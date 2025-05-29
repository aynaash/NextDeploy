package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"encoding/json"
	"path/filepath"
	"nextdeploy/internal/config"
	"nextdeploy/internal/docker"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize your Next.js deployment configuration",
	Long: `Scaffolds all necessary files for deploying a Next.js application including:
- Dockerfile for containerization
- nextdeploy.yml for deployment configuration
- Optional sample configuration`,
	Run: func(cmd *cobra.Command, args []string) {
		// check if current directory is a Next.js project
		cwd, err := os.Getwd()
		if err != nil {
			fmt.Printf("❌ Error getting current directory: %v\n", err)
			os.Exit(1)
		}
		isNextJS, err := isNextJSProject(cwd)
		if err != nil {
			fmt.Printf("❌ Error checking project type: %v\n", err)
			os.Exit(1)
		}
		if !isNextJS {
			fmt.Println("❌ We only support Next.js projects at the moment.")
			os.Exit(1)
		}
		// Create a buffered reader for user input
		reader := bufio.NewReader(os.Stdin)
		
		// Welcome message
		fmt.Println("🚀 NextDeploy Initialization")
		fmt.Println("This will help you set up your Next.js project for deployment")
		fmt.Println("----------------------------------------")

		// Step 1: Offer to generate sample config
		if config.PromptYesNo(reader, "Would you like to generate a sample configuration file for reference?") {
			// NOTE: works
			if err := config.GenerateSampleConfig(); err != nil {
				fmt.Printf("❌ Error generating sample config: %v\n", err)
			} else {
				fmt.Println("✅ sample.nextdeploy.yml created")
				fmt.Println("You can review this file and customize it as needed")
			}
		}

		// Step 2: Interactive configuration
		if config.PromptYesNo(reader, "Would you like to create a customized nextdeploy.yml now?") {
			cfg, err := config.PromptForConfig(reader)
			if err != nil {
				fmt.Printf("❌ Error getting configuration: %v\n", err)
				os.Exit(1)
			}

			// print out the config of nextdeploy.yml
			fmt.Println("Here is your nextdeploy.yml configuration:")
			fmt.Println("------------------------------------------------")
			fmt.Printf("The config looks like this %+v\n", cfg)

			if err := config.WriteConfig("nextdeploy.yml", cfg); err != nil {
				fmt.Printf("❌ Error writing configuration: %v\n", err)
				os.Exit(1)
			}
			fmt.Println("✅ nextdeploy.yml created with your custom settings")
		}

		// Step 3: Dockerfile setup
		if config.PromptYesNo(reader, "Would you like to set up a Dockerfile for your project?") {
			if err := handleDockerfileSetup(reader); err != nil {
				fmt.Printf("❌ Error setting up Dockerfile: %v\n", err)
				os.Exit(1)
			}
		}

		// Completion message
		fmt.Println("\n🎉 Setup complete! Next steps:")
		if docker.DockerfileExists() {
			fmt.Println("- Review the generated Dockerfile")
		}
		fmt.Println("- Review your nextdeploy.yml configuration")
		fmt.Println("- Run 'nextdeploy build' to build the docker image")
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}

func handleDockerfileSetup(reader *bufio.Reader) error {
	// Check for existing Dockerfile
	if docker.DockerfileExists() {
		if !config.PromptYesNo(reader, "A Dockerfile already exists. Overwrite it?") {
			fmt.Println("ℹ️ Using existing Dockerfile")
			return nil
		}
	}

	// Determine package manager
	pkgManager := promptForPackageManager(reader)
	
	// Generate and write Dockerfile
	content := docker.GenerateDockerfile(pkgManager)
  if content != nil {
		// 
		fmt.Print(content)
	}
	// Offer to add to gitignore
	if config.PromptYesNo(reader, "Would you like to add .env and node_modules to .gitignore?") {
		if err := addToGitignore(); err != nil {
			fmt.Printf("⚠️ Couldn't update .gitignore: %v\n", err)
		}
	}

	return nil
}

func promptForPackageManager(reader *bufio.Reader) string {
	for {
		fmt.Print("Choose your package manager (npm/yarn/pnpm): ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(strings.ToLower(input))
		switch input {
		case "npm", "yarn", "pnpm":
			return input
		default:
			fmt.Println("❌ Invalid choice. Please choose: npm, yarn, or pnpm.")
		}
	}
}

func addToGitignore() error {
	// Check if entries already exist
	content, err := os.ReadFile(".gitignore")
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	entries := []string{"\n# NextDeploy", ".env", "node_modules"}
	for _, entry := range entries {
		if strings.Contains(string(content), entry) {
			continue
		}

		f, err := os.OpenFile(".gitignore", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return err
		}
		defer func(){
			closedErr := f.Close()
			if closedErr != nil {
				fmt.Printf("⚠️ Error closing .gitignore file: %v\n", closedErr)
			}
		}()
		// Write the entry to .gitignore
		if _, err := f.WriteString(entry + "\n"); err != nil {
			return err
		}
	}

	fmt.Println("✅ Updated .gitignore")
	return nil
}

type PackageJSON struct {
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"devDependencies"`
	Scripts         map[string]string `json:"scripts"`
}

func isNextJSProject(dir string) (bool, error) {
	// Check for package.json
	packageJsonPath := filepath.Join(dir, "package.json")
	if _, err := os.Stat(packageJsonPath); os.IsNotExist(err) {
		return false, nil
	}

	// Parse package.json
	content, err := os.ReadFile(packageJsonPath)
	if err != nil {
		return false, fmt.Errorf("error reading package.json: %w", err)
	}

	var pkg PackageJSON
	if err := json.Unmarshal(content, &pkg); err != nil {
		return false, fmt.Errorf("error parsing package.json: %w", err)
	}

	// Check dependencies
	checkDeps := func(deps map[string]string) bool {
		for dep := range deps {
			if dep == "next" {
				return true
			}
		}
		return false
	}

	if checkDeps(pkg.Dependencies) || checkDeps(pkg.DevDependencies) {
		return true, nil
	}

	// Check scripts for Next.js commands
	for _, script := range pkg.Scripts {
		if strings.Contains(script, "next ") || 
		   strings.Contains(script, "next build") ||
		   strings.Contains(script, "next dev") {
			return true, nil
		}
	}

	// Check for Next.js specific files/directories
	nextFiles := []string{
		"next.config.js",
		"next.config.mjs",
		filepath.Join("src", "pages"),
		"pages",
		"app", // Next.js 13+
		".next",
	}

	for _, file := range nextFiles {
		if _, err := os.Stat(filepath.Join(dir, file)); err == nil {
			return true, nil
		}
	}

	return false, nil
}
