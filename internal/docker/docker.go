package docker

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"github.com/spf13/cobra"
	"nextdeploy/internal/config"
	"nextdeploy/internal/detect"
	"nextdeploy/internal/logger"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	DockerfileName = "Dockerfile"
	nodeVersion    = "20-alpine"
)

var (
	dlog = logger.PackageLogger("docker", "🐳 DOCKER")
)

var (
	forceOverwrite bool
	skipPrompts    bool
)

var (
	ErrDockerfileExists   = errors.New("dockerfile already exists")
	ErrDockerNotInstalled = errors.New("docker not installed")
	ErrInvalidImageName   = errors.New("invalid Docker image name")
	ErrDockerfileNotFound = errors.New("dockerfile not found")
	ErrBuildFailed        = errors.New("docker build failed")
	ErrPushFailed         = errors.New("docker push failed")
)

type DockerManager struct {
	verbose bool
	logger  Logger // Interface for logging
}

type Logger interface {
	Debugf(format string, args ...interface{})
	Infof(format string, args ...interface{})
	Errorf(format string, args ...interface{})
}

type BuildOptions struct {
	ImageName string
	NoCache   bool
	Pull      bool
	Target    string
	BuildArgs []string
	Platform  string // Added platform support
}

// NewDockerManager creates a new DockerManager instance
func NewDockerManager(verbose bool, dlog Logger) *DockerManager {

	return &DockerManager{
		verbose: verbose,
		logger:  dlog,
	}
}

// DockerfileExists checks if a Dockerfile exists in the specified directory
func (dm *DockerManager) DockerfileExists(dir string) (bool, error) {
	path := filepath.Join(dir, DockerfileName)
	dlog.Debug("Checking for Dockerfile at: %s", path)

	stat, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		dlog.Error("Error checking Dockerfile existence: %v", err)
		return false, fmt.Errorf("failed to check Dockerfile existence: %w", err)
	}

	if stat.IsDir() {
		dlog.Error("Dockerfile path is a directory, expected a file")
		return false, fmt.Errorf("dockerfile path is a directory")
	}

	return true, nil
}

// GenerateDockerfile creates a new Dockerfile with content tailored to the package manager
func (dm *DockerManager) GenerateDockerfile(dir, pkgManager string, overwrite bool) error {
	exists, err := dm.DockerfileExists(dir)
	if err != nil {
		dlog.Error("Error checking Dockerfile existence: %v", err)
		return fmt.Errorf("failed to check Dockerfile existence: %w", err)
	}
	if exists && !overwrite {
		return ErrDockerfileExists
	}

	content, err := dm.generateDockerfileContent(pkgManager)
	if err != nil {
		dlog.Error("Error generating Dockerfile content: %v", err)
		return fmt.Errorf("failed to generate Dockerfile content: %w", err)
	}

	return dm.WriteDockerfile(dir, content)
}

// WriteDockerfile writes content to Dockerfile in the specified directory
func (dm *DockerManager) WriteDockerfile(dir, content string) error {
	content = strings.TrimSpace(content) + "\n"
	dlog.Debug("Writing Dockerfile with content:\n%s", content)

	path := filepath.Join(dir, DockerfileName)
	err := os.WriteFile(path, []byte(content), 0644)
	if err != nil {
		dlog.Error("Error writing Dockerfile: %v", err)
		return fmt.Errorf("failed to write Dockerfile: %w", err)
	}

	dlog.Success("Dockerfile created successfully at %s", path)
	return nil
}

// generateDockerfileContent creates Dockerfile content based on package manager
func (dm *DockerManager) generateDockerfileContent(pkgManager string) (string, error) {
	// Predefined templates for different package managers
	templates := map[string]string{
		"npm": `# ---------- STAGE 1: Build ----------
FROM node:%s AS builder

WORKDIR /app

COPY package.json ./
COPY package-lock.json ./

RUN npm ci --production=false

COPY . .

RUN npm run build

# ---------- STAGE 2: Runtime ----------
FROM node:%s

WORKDIR /app

COPY --from=builder /app/public ./public
COPY --from=builder /app/.next/standalone ./
COPY --from=builder /app/.next/static ./.next/static

RUN adduser -D nextjs && chown -R nextjs:nextjs /app
USER nextjs

EXPOSE 3000
ENV PORT=3000
ENV NODE_ENV=production

CMD ["node", "server.js"]`,

		"yarn": `# ---------- STAGE 1: Build ----------
FROM node:%s AS builder

WORKDIR /app

COPY package.json ./
COPY yarn.lock ./
RUN corepack enable && corepack prepare yarn@4.9.1 --activate
RUN yarn install --frozen-lockfile
ENV NEXT_TELEMETRY_DISABLED=1
COPY . .

RUN  npm run build
# ---------- STAGE 2: Runtime ----------
FROM node:%s

WORKDIR /app

COPY --from=builder /app/public ./public
COPY --from=builder /app/.next/standalone ./
COPY --from=builder /app/.next/static ./.next/static

RUN adduser -D nextjs && chown -R nextjs:nextjs /app
USER nextjs

EXPOSE 3000
ENV PORT=3000
ENV NODE_ENV=production

CMD ["node", "server.js"]`,

		"pnpm": `# ---------- STAGE 1: Build ----------
FROM node:%s AS builder

WORKDIR /app

COPY package.json ./
COPY pnpm-lock.yaml ./

RUN pnpm install --frozen-lockfile

COPY . .

RUN pnpm run build

# ---------- STAGE 2: Runtime ----------
FROM node:%s

WORKDIR /app

COPY --from=builder /app/public ./public
COPY --from=builder /app/.next/standalone ./
COPY --from=builder /app/.next/static ./.next/static

RUN adduser -D nextjs && chown -R nextjs:nextjs /app
USER nextjs

EXPOSE 3000
ENV PORT=3000
ENV NODE_ENV=production

CMD ["node", "server.js"]`,
	}

	template, ok := templates[pkgManager]
	if !ok {
		dlog.Debug("Unknown package manager '%s', defaulting to npm", pkgManager)
		template = templates["npm"]
	}

	return fmt.Sprintf(template, nodeVersion, nodeVersion), nil
}

// ValidateImageName checks if a Docker image name is valid
func (dm *DockerManager) ValidateImageName(name string) error {
	if len(name) == 0 {
		dlog.Error("Image name cannot be empty")
		return fmt.Errorf("%w: image name cannot be empty", ErrInvalidImageName)
	}

	if len(name) > 255 {
		dlog.Error("Image name exceeds 255 characters")
		return fmt.Errorf("%w: exceeds 255 characters", ErrInvalidImageName)
	}

	// Regex from Docker's implementation
	validImageName := regexp.MustCompile(`^[a-z0-9]+(?:[._-][a-z0-9]+)*(?:/[a-z0-9]+(?:[._-][a-z0-9]+)*)*$`)
	validTag := regexp.MustCompile(`^[\w][\w.-]{0,127}$`)

	parts := strings.SplitN(name, ":", 2)
	if !validImageName.MatchString(parts[0]) {
		dlog.Error("Invalid repository name: %s", parts[0])
		return fmt.Errorf("%w: invalid repository name", ErrInvalidImageName)
	}

	if len(parts) == 2 {
		if !validTag.MatchString(parts[1]) {
			dlog.Error("Invalid tag format: %s", parts[1])
			return fmt.Errorf("%w: invalid tag format", ErrInvalidImageName)
		}
	}

	dlog.Success("Image name validation passed: %s", name)
	return nil
}

// CheckDockerInstalled verifies Docker is available
func (dm *DockerManager) CheckDockerInstalled() error {
	dlog.Info("Checking if Docker is installed...")

	path, err := exec.LookPath("docker")
	if err != nil {
		dlog.Warn("Docker not found in PATH")
		return ErrDockerNotInstalled
	}

	dlog.Success("Docker found at: %s", path)

	// Verify docker is actually working
	cmd := exec.Command("docker", "version")
	if err := cmd.Run(); err != nil {
		dlog.Error("Docker is installed but not functioning: %v", err)
		return fmt.Errorf("docker is installed but not functioning: %w", err)
	}

	return nil
}

// BuildImage builds a Docker image with options
func (dm *DockerManager) BuildImage(ctx context.Context, dir string, opts BuildOptions) error {
	// print out the options for debugging
	dlog.Debug("Build options: %+v", opts)
	if err := dm.ValidateImageName(opts.ImageName); err != nil {
		dlog.Error("Invalid image name: %v", err)
		return fmt.Errorf("invalid image name: %w", err)
	}

	if err := dm.CheckDockerInstalled(); err != nil {
		dlog.Error("Docker check failed: %v", err)
		return fmt.Errorf("docker check failed: %w", err)
	}

	exists, err := dm.DockerfileExists(dir)
	if err != nil {
		dlog.Error("Error checking Dockerfile existence: %v", err)
		return fmt.Errorf("dockerfile check failed: %w", err)
	}
	if !exists {
		dlog.Error("Dockerfile not found in directory: %s", dir)
		return ErrDockerfileNotFound
	}

	args := []string{"build"}
	if opts.NoCache {
		args = append(args, "--no-cache")
	}
	if opts.Pull {
		args = append(args, "--pull")
	}
	if opts.Target != "" {
		args = append(args, "--target", opts.Target)
	}
	if opts.Platform != "" {
		args = append(args, "--platform", opts.Platform)
	}
	for _, buildArg := range opts.BuildArgs {
		args = append(args, "--build-arg", buildArg)
	}

	args = append(args, "-t", opts.ImageName, ".")

	dlog.Info("Building Docker image with args: %v", args)
	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		dlog.Error("Docker build failed: %v", err)
		return fmt.Errorf("%w: %v", ErrBuildFailed, err)
	}

	dlog.Success("Image built successfully: %s", opts.ImageName)
	return nil
}

// PushImage pushes a Docker image to registry
func (dm *DockerManager) PushImage(ctx context.Context, imageName string) error {
	if err := dm.ValidateImageName(imageName); err != nil {
		dlog.Error("Invalid image name: %v", err)
		return fmt.Errorf("invalid image name: %w", err)
	}

	if err := dm.CheckDockerInstalled(); err != nil {
		dlog.Error("Docker check failed: %v", err)
		return fmt.Errorf("docker check failed: %w", err)
	}

	dlog.Info("Pushing Docker image: %s", imageName)
	cmd := exec.CommandContext(ctx, "docker", "push", imageName)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%w: %v", ErrPushFailed, err)
	}

	dlog.Success("Image pushed successfully: %s", imageName)
	return nil
}

func HandleDockerfileSetup(cmd *cobra.Command, dm *DockerManager, reader *bufio.Reader) error {
	if skipPrompts || config.PromptYesNo(reader, "Set up Dockerfile for your project?") {
		exists, err := dm.DockerfileExists(".")
		if err != nil {
			dlog.Error("Error checking Dockerfile existence: %v", err)
			return fmt.Errorf("failed to check Dockerfile existence: %w", err)
		}

		if exists {
			if !forceOverwrite && !skipPrompts && !config.PromptYesNo(reader, "Dockerfile exists. Overwrite?") {
				dlog.Info("Skipping Dockerfile creation")
				cmd.Println("ℹ️ Using existing Dockerfile")
				return nil
			}
		}

		pkgManager := "npm"
		if !skipPrompts {
			cwd, err := os.Getwd()
			if err != nil {
				dlog.Error("Failed to get current directory: %v", err)
				return fmt.Errorf("failed to get current directory: %w", err)
			}
			detectManager, err := detect.DetectPackageManager(cwd)
			if err != nil {
				dlog.Error("Failed to detect package manager: %v", err)
				return fmt.Errorf("failed to detect package manager: %w", err)
			}
			pkgManager = detectManager.String()
		}

		if err := dm.GenerateDockerfile(".", pkgManager, true); err != nil {
			dlog.Error("Failed to generate Dockerfile: %v", err)
			return fmt.Errorf("failed to generate Dockerfile: %w", err)
		}
		dlog.Success("Dockerfile created successfully")
		// Add .dockerignore file creation
		if skipPrompts || config.PromptYesNo(reader, "Create .dockerignore file?") {
			if err := createDockerignore(); err != nil {

				dlog.Error("⚠️ Couldn't create .dockerignore: %v\n", err)
			} else {
				dlog.Success("✅ Created .dockerignore")
				cmd.Println("✅ Created .dockerignore")
			}
		} else {
			dlog.Info("ℹ️ Skipping .dockerignore creation")
		}

		if skipPrompts || config.PromptYesNo(reader, "Add .env and node_modules to .gitignore?") {
			if err := updateGitignore(); err != nil {
				dlog.Error("⚠️ Couldn't update .gitignore: %v\n", err)
				cmd.Printf("⚠️ Couldn't update .gitignore: %v\n", err)
			} else {
				dlog.Success("✅ Updated .gitignore")
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
		dlog.Error("Failed to close .gitignore file: %v", err)
		return fmt.Errorf("failed to close .gitignore file: %w", err)
	}

	if _, err := f.WriteString(strings.Join(toAdd, "\n") + "\n"); err != nil {
		return err
	}

	return nil
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
