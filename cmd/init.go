package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"nextdeploy/internal/config"
	"nextdeploy/internal/docker"
	"github.com/spf13/cobra"
)

var (
	forceOverwrite bool
	skipPrompts    bool
	defaultConfig  bool
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize Next.js deployment configuration",
	Long: `Scaffolds deployment configuration for Next.js applications including:
- Dockerfile for containerization
- nextdeploy.yml configuration
- Optional sample files and gitignore updates`,
	PreRunE: validateNextJSProject,
	RunE:    runInitCommand,
}

func init() {
	initCmd.Flags().BoolVarP(&forceOverwrite, "force", "f", false, 
		"Overwrite existing files without prompting")
	initCmd.Flags().BoolVarP(&skipPrompts, "yes", "y", false, 
		"Skip all prompts and use default values")
	initCmd.Flags().BoolVar(&defaultConfig, "default-config", false, 
		"Generate with default configuration only")
	
	rootCmd.AddCommand(initCmd)
}

func validateNextJSProject(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	isNextJS, err := isNextJSProject(cwd)
	if err != nil {
		return fmt.Errorf("project validation failed: %w", err)
	}

	if !isNextJS {
		return fmt.Errorf("current directory doesn't appear to be a Next.js project")
	}
	return nil
}

func runInitCommand(cmd *cobra.Command, args []string) error {
	dm := docker.NewDockerManager(true, nil)
	reader := bufio.NewReader(os.Stdin)

	cmd.Println("🚀 NextDeploy Initialization")
	cmd.Println("----------------------------------------")

	if err := handleConfigSetup(cmd, reader); err != nil {
		return fmt.Errorf("configuration setup failed: %w", err)
	}

	if err := handleDockerfileSetup(cmd, dm, reader); err != nil {
		return fmt.Errorf("Docker setup failed: %w", err)
	}

	cmd.Println("\n🎉 Setup complete! Next steps:")
	if exists, _ := dm.DockerfileExists("."); exists {
		cmd.Println("- Review the generated Dockerfile")
	}
	cmd.Println("- Review your nextdeploy.yml configuration")
	cmd.Println("- Run 'nextdeploy build' to build the Docker image")
	
	return nil
}

func handleConfigSetup(cmd *cobra.Command, reader *bufio.Reader) error {
	if defaultConfig || (!skipPrompts && config.PromptYesNo(reader, "Generate sample configuration file for reference?")) {
		if err := config.GenerateSampleConfig(); err != nil {
			return fmt.Errorf("failed to generate sample config: %w", err)
		}
		cmd.Println("✅ sample.nextdeploy.yml created")
	}

	if defaultConfig {
		return nil
	}

	if skipPrompts || config.PromptYesNo(reader, "Create customized nextdeploy.yml?") {
		cfg, err := config.InteractiveConfigPrompt(reader)
		if err != nil {
			return fmt.Errorf("failed to get configuration: %w", err)
		}

		if err := config.WriteConfig("nextdeploy.yml", cfg); err != nil {
			return fmt.Errorf("failed to write configuration: %w", err)
		}
		cmd.Println("✅ nextdeploy.yml created with your settings")
	}
	
	return nil
}

func handleDockerfileSetup(cmd *cobra.Command, dm *docker.DockerManager, reader *bufio.Reader) error {
	if skipPrompts || config.PromptYesNo(reader, "Set up Dockerfile for your project?") {
		exists, err := dm.DockerfileExists(".")
		if err != nil {
			return fmt.Errorf("failed to check Dockerfile existence: %w", err)
		}

		if exists {
			if !forceOverwrite && !skipPrompts && !config.PromptYesNo(reader, "Dockerfile exists. Overwrite?") {
				cmd.Println("ℹ️ Using existing Dockerfile")
				return nil
			}
		}

		pkgManager := "npm"
		if !skipPrompts {
			pkgManager = promptForPackageManager(reader)
		}

		if err := dm.GenerateDockerfile(".", pkgManager, true); err != nil {
			return fmt.Errorf("failed to generate Dockerfile: %w", err)
		}
		cmd.Println("✅ Dockerfile created")

		if skipPrompts || config.PromptYesNo(reader, "Add .env and node_modules to .gitignore?") {
			if err := updateGitignore(); err != nil {
				cmd.Printf("⚠️ Couldn't update .gitignore: %v\n", err)
			} else {
				cmd.Println("✅ Updated .gitignore")
			}
		}
	}
	
	return nil
}

func promptForPackageManager(reader *bufio.Reader) string {
	for {
		fmt.Print("Choose package manager (npm/yarn/pnpm): ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(strings.ToLower(input))
		
		switch input {
		case "npm", "yarn", "pnpm":
			return input
		default:
			fmt.Println("Invalid choice. Please enter: npm, yarn, or pnpm")
		}
	}
}

func updateGitignore() error {
	entries := []string{
		"\n# NextDeploy",
		".env",
		"node_modules",
		".next",
		"dist",
	}

	content := ""
	if file, err := os.ReadFile(".gitignore"); err == nil {
		content = string(file)
	}

	toAdd := []string{}
	for _, entry := range entries {
		if !strings.Contains(content, entry) {
			toAdd = append(toAdd, entry)
		}
	}

	if len(toAdd) == 0 {
		return nil
	}

	f, err := os.OpenFile(".gitignore", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err := f.WriteString(strings.Join(toAdd, "\n") + "\n"); err != nil {
		return err
	}

	return nil
}

type PackageJSON struct {
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"devDependencies"`
	Scripts         map[string]string `json:"scripts"`
}

func isNextJSProject(dir string) (bool, error) {
	packagePath := filepath.Join(dir, "package.json")
	data, err := os.ReadFile(packagePath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("error reading package.json: %w", err)
	}

	var pkg PackageJSON
	if err := json.Unmarshal(data, &pkg); err != nil {
		return false, fmt.Errorf("error parsing package.json: %w", err)
	}

	hasNextDependency := func(deps map[string]string) bool {
		_, exists := deps["next"]
		return exists
	}

	if hasNextDependency(pkg.Dependencies) || hasNextDependency(pkg.DevDependencies) {
		return true, nil
	}

	for _, script := range pkg.Scripts {
		if strings.Contains(script, "next") {
			return true, nil
		}
	}

	nextjsFiles := []string{
		"next.config.js",
		"next.config.mjs",
		filepath.Join("src", "pages"),
		"pages",
		"app",
	}

	for _, file := range nextjsFiles {
		if _, err := os.Stat(filepath.Join(dir, file)); err == nil {
			return true, nil
		}
	}

	return false, nil
}
