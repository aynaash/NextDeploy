package cmd

// FEATURE: GIVE USER  ABILITY TO START FROM ZERO USING OUR OWN NEXTJS TEMPLATES

/*
TODO: REFACTOR & IMPROVE - INIT COMMAND

🚨 Structural / Architectural Concerns:
- Refactor monolithic `runInitCommand` into smaller decoupled flows.
- Move package detection logic out of `cmd` into reusable internal module.
- Separate Docker setup, gitignore update, and config creation into distinct concerns.
- Clean up semantic overlap between --yes, --force, and --default-config flags.
- Avoid side-effects inside command handlers—prefer returning changes to be printed/logged externally.

⚙️ Code Design / Maintenance:
- Split `DetectPackageManager` into composable scoring functions (env, FS, heuristics).
- Introduce interface abstraction for DockerManager to improve testability.
- Split `handleConfigSetup` and `handleDockerfileSetup`—they’re too large.
- Use typed config struct instead of raw JSON where possible.
- Centralize all user prompts into a dedicated `prompter` utility/package.

🧠 User Experience:
- Add `--package-manager` flag to skip detection.
- Log files that were overwritten when using --force or skipping prompts.
- Warn before overwriting existing `nextdeploy.yml`, even in non-interactive mode.
- Add a `--dry-run` flag to preview changes.
- Add true `--non-interactive` flag for scripting CI/CD flows.

📦 Project Clarity / Developer Experience:
- Split detection, prompting, and generation into isolated steps.
- Consider telemetry (optional/opt-in) for template & pkg manager usage insights.
- Improve .gitignore updater: handle duplicates and edge cases cleanly.
- Warn if `.gitignore` is missing entirely.
- Alert if directory is not a Git repo.

🧪 Testing / Reliability:
- Add test coverage for `DetectPackageManager`—currently untested.
- Handle `os.Getwd()` errors gracefully and test them.
- Validate generated `nextdeploy.yml` for schema/syntax correctness.
- Don't assume current dir for Dockerfile existence—use explicit pathing.

🌍 Platform Safety / Edge Cases:
- Ensure cross-platform compatibility (e.g. Windows paths, newline formats).
- Handle malformed or missing `package.json` gracefully.
- Replace direct `os.ReadFile()` with testable abstraction.
- Normalize scoring in package detection or provide confidence output.

Priority Fix Order:
1. Refactor `initCmd` into distinct steps/modules.
2. Move detection logic to internal reusable layer.
3. Add test coverage and interface abstractions.
4. Add --pkg-manager and non-interactive flags.
5. Modularize user prompt system.

*/
import (
	"bufio"
	"encoding/json"
	"fmt"
	"github.com/spf13/cobra"
	"nextdeploy/internal/config"
	"nextdeploy/internal/docker"
	"os"
	"path/filepath"
	"strings"
)

type PackageManager string

var (
	forceOverwrite bool
	skipPrompts    bool
	defaultConfig  bool
)

const (
	NPM     PackageManager = "npm"
	Yarn    PackageManager = "yarn"
	PNPM    PackageManager = "pnpm"
	Unknown PackageManager = "unknown"
)

func (pm PackageManager) String() string {
	return string(pm)
}
func DetectPackageManager(projectPath string) (PackageManager, error) {
	// Define all possible indicators with weights
	indicators := map[string]struct {
		manager PackageManager
		weight  int
	}{
		// Lock files (highest confidence)
		"pnpm-lock.yaml":    {PNPM, 100},
		"yarn.lock":         {Yarn, 100},
		"package-lock.json": {NPM, 100},

		// Configuration files
		".npmrc":              {NPM, 40},
		".yarnrc":             {Yarn, 40},
		"pnpm-workspace.yaml": {PNPM, 40},

		// Directories
		".yarn":              {Yarn, 30},
		".pnpm-store":        {PNPM, 30},
		"node_modules/.yarn": {Yarn, 30},

		// Script patterns in package.json (we'll check these separately)
	}

	// 1. Check for package.json first
	pkgPath := filepath.Join(projectPath, "package.json")
	if _, err := os.Stat(pkgPath); os.IsNotExist(err) {
		return Unknown, fmt.Errorf("not a Node.js project (no package.json found)")
	}

	// 2. Check file system indicators
	scores := make(map[PackageManager]int)
	for filename, data := range indicators {
		if _, err := os.Stat(filepath.Join(projectPath, filename)); err == nil {
			scores[data.manager] += data.weight
		}
	}

	// 3. Check package.json scripts and engines
	pkgJson, err := os.ReadFile(pkgPath)
	if err == nil {
		content := string(pkgJson)

		// Check for package manager specific scripts
		if strings.Contains(content, "pnpm") {
			scores[PNPM] += 20
		}
		if strings.Contains(content, "yarn") {
			scores[Yarn] += 20
		}

		// Check engines field
		if strings.Contains(content, `"pnpm"`) {
			scores[PNPM] += 50
		}
		if strings.Contains(content, `"yarn"`) {
			scores[Yarn] += 50
		}
	}

	// 4. Check for process.env (in CI environments)
	// This would require environment variable checks
	if os.Getenv("PNPM_HOME") != "" {
		scores[PNPM] += 30
	}
	if os.Getenv("YARN_VERSION") != "" {
		scores[Yarn] += 30
	}

	// 5. Determine result
	var result PackageManager
	maxScore := 0
	for manager, score := range scores {
		if score > maxScore {
			maxScore = score
			result = manager
		}
	}

	// 6. Fallback checks when no clear winner
	if maxScore == 0 {
		// Check for package manager binaries in node_modules/.bin
		if _, err := os.Stat(filepath.Join(projectPath, "node_modules", ".bin", "pnpm")); err == nil {
			return PNPM, nil
		}
		if _, err := os.Stat(filepath.Join(projectPath, "node_modules", ".bin", "yarn")); err == nil {
			return Yarn, nil
		}

		// Default to npm if nothing else found
		return NPM, nil
	}

	return result, nil
}

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
	// Infer the needed data from the current working directory
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
	// TODO: secret management integration here
	if err := handleConfigSetup(cmd, reader); err != nil {
		return fmt.Errorf("configuration setup failed: %w", err)
	}

	if err := handleDockerfileSetup(cmd, dm, reader); err != nil {
		return fmt.Errorf("docker setup failed: %w", err)
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
		//TODO:  add the logic for secrets management here
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
			cwd, err := os.Getwd()
			detectManager, err := DetectPackageManager(cwd)
			if err != nil {
				return fmt.Errorf("failed to detect package manager: %w", err)
			}
			pkgManager = detectManager.String()
		}

		if err := dm.GenerateDockerfile(".", pkgManager, true); err != nil {
			return fmt.Errorf("failed to generate Dockerfile: %w", err)
		}
		cmd.Println("✅ Dockerfile created")
		// Add .dockerignore file creation
		if skipPrompts || config.PromptYesNo(reader, "Create .dockerignore file?") {
			if err := createDockerignore(); err != nil {
				cmd.Printf("⚠️ Couldn't create .dockerignore: %v\n", err)
			} else {
				cmd.Println("✅ Created .dockerignore")
			}
		} else {
			cmd.Println("ℹ️ Skipping .dockerignore creation")
		}

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
	if err := f.Close(); err != nil {
		return fmt.Errorf("failed to close .gitignore file: %w", err)
	}

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

func createDockerignore() error {
	patterns := []string{
		"# NextDeploy Dockerignore: auto-generated file",
		"",
		"# Node modules and build directories",
		"node_modules",
		".next",
		"dist",
		"",
		"# Environment files",
		".env*",
		"",
		"# Logs",
		"npm-debug.log*",
		"yarn-debug.log*",
		"yarn-error.log*",
		"",
		"# Git",
		".git",
		".gitignore",
		"",
		"# IDE",
		".idea",
		".vscode",
		"",
		"# OS generated files",
		".DS_Store",
		"Thumbs.db",
	}
	// Check if file already exists
	if _, err := os.Stat(".dockerignore"); err == nil {
		// File exists, don't overwrite unless forced
		if !forceOverwrite && !skipPrompts {
			return fmt.Errorf(".dockerignore already exists (use --force to overwrite)")
		}
	}
	// Write the file
	content := strings.Join(patterns, "\n")
	if err := os.WriteFile(".dockerignore", []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write .dockerignore: %w", err)
	}
	return nil
}
