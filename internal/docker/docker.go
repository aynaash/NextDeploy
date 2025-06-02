package docker

import (
	"context"
	"errors"
	"fmt"
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
	ErrDockerfileExists    = errors.New("dockerfile already exists")
	ErrDockerNotInstalled  = errors.New("docker not installed")
	ErrInvalidImageName    = errors.New("invalid Docker image name")
	ErrDockerfileNotFound  = errors.New("dockerfile not found")
	ErrBuildFailed         = errors.New("docker build failed")
	ErrPushFailed          = errors.New("docker push failed")
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
func NewDockerManager(verbose bool, logger Logger) *DockerManager {
	if logger == nil {
		logger = &defaultLogger{verbose: verbose}
	}
	return &DockerManager{
		verbose: verbose,
		logger:  logger,
	}
}

type defaultLogger struct {
	verbose bool
}

func (l *defaultLogger) Debugf(format string, args ...interface{}) {
	if l.verbose {
		fmt.Printf("[DEBUG] "+format+"\n", args...)
	}
}

func (l *defaultLogger) Infof(format string, args ...interface{}) {
	fmt.Printf("[INFO] "+format+"\n", args...)
}

func (l *defaultLogger) Errorf(format string, args ...interface{}) {
	fmt.Printf("[ERROR] "+format+"\n", args...)
}

// DockerfileExists checks if a Dockerfile exists in the specified directory
func (dm *DockerManager) DockerfileExists(dir string) (bool, error) {
	path := filepath.Join(dir, DockerfileName)
	dm.logger.Debugf("Checking for Dockerfile at: %s", path)
	
	stat, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("error checking Dockerfile existence: %w", err)
	}
	
	if stat.IsDir() {
		return false, fmt.Errorf("dockerfile path is a directory")
	}
	
	return true, nil
}

// GenerateDockerfile creates a new Dockerfile with content tailored to the package manager
func (dm *DockerManager) GenerateDockerfile(dir, pkgManager string, overwrite bool) error {
	exists, err := dm.DockerfileExists(dir)
	if err != nil {
		return fmt.Errorf("failed to check Dockerfile existence: %w", err)
	}
	if exists && !overwrite {
		return ErrDockerfileExists
	}

	content, err := dm.generateDockerfileContent(pkgManager)
	if err != nil {
		return fmt.Errorf("failed to generate Dockerfile content: %w", err)
	}

	return dm.WriteDockerfile(dir, content)
}

// WriteDockerfile writes content to Dockerfile in the specified directory
func (dm *DockerManager) WriteDockerfile(dir, content string) error {
	content = strings.TrimSpace(content) + "\n"
	dm.logger.Debugf("Writing Dockerfile with content:\n%s", content)
	
	path := filepath.Join(dir, DockerfileName)
	err := os.WriteFile(path, []byte(content), 0644)
	if err != nil {
		return fmt.Errorf("failed to write Dockerfile: %w", err)
	}
	
	dm.logger.Infof("Dockerfile created successfully at %s", path)
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

RUN yarn install --frozen-lockfile

COPY . .

RUN yarn build

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
		dm.logger.Debugf("Unknown package manager '%s', defaulting to npm", pkgManager)
		template = templates["npm"]
	}

	return fmt.Sprintf(template, nodeVersion, nodeVersion), nil
}

// ValidateImageName checks if a Docker image name is valid
func (dm *DockerManager) ValidateImageName(name string) error {
	if len(name) == 0 {
		return fmt.Errorf("%w: image name cannot be empty", ErrInvalidImageName)
	}
	
	if len(name) > 255 {
		return fmt.Errorf("%w: exceeds 255 characters", ErrInvalidImageName)
	}

	// Regex from Docker's implementation
	validImageName := regexp.MustCompile(`^[a-z0-9]+(?:[._-][a-z0-9]+)*(?:/[a-z0-9]+(?:[._-][a-z0-9]+)*)*$`)
	validTag := regexp.MustCompile(`^[\w][\w.-]{0,127}$`)

	parts := strings.SplitN(name, ":", 2)
	if !validImageName.MatchString(parts[0]) {
		return fmt.Errorf("%w: invalid repository name", ErrInvalidImageName)
	}

	if len(parts) == 2 {
		if !validTag.MatchString(parts[1]) {
			return fmt.Errorf("%w: invalid tag format", ErrInvalidImageName)
		}
	}

	dm.logger.Debugf("Image name validation passed: %s", name)
	return nil
}

// CheckDockerInstalled verifies Docker is available
func (dm *DockerManager) CheckDockerInstalled() error {
	dm.logger.Debugf("Checking if Docker is installed...")
	
	path, err := exec.LookPath("docker")
	if err != nil {
		return ErrDockerNotInstalled
	}
	
	dm.logger.Debugf("Docker found at: %s", path)
	
	// Verify docker is actually working
	cmd := exec.Command("docker", "version")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker is installed but not functioning: %w", err)
	}
	
	return nil
}

// BuildImage builds a Docker image with options
func (dm *DockerManager) BuildImage(ctx context.Context, dir string, opts BuildOptions) error {
	if err := dm.ValidateImageName(opts.ImageName); err != nil {
		return fmt.Errorf("invalid image name: %w", err)
	}

	if err := dm.CheckDockerInstalled(); err != nil {
		return fmt.Errorf("docker check failed: %w", err)
	}

	exists, err := dm.DockerfileExists(dir)
	if err != nil {
		return fmt.Errorf("dockerfile check failed: %w", err)
	}
	if !exists {
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

	dm.logger.Infof("Building Docker image with args: %v", args)
	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%w: %v", ErrBuildFailed, err)
	}

	dm.logger.Infof("Image built successfully: %s", opts.ImageName)
	return nil
}

// PushImage pushes a Docker image to registry
func (dm *DockerManager) PushImage(ctx context.Context, imageName string) error {
	if err := dm.ValidateImageName(imageName); err != nil {
		return fmt.Errorf("invalid image name: %w", err)
	}

	if err := dm.CheckDockerInstalled(); err != nil {
		return fmt.Errorf("docker check failed: %w", err)
	}

	dm.logger.Infof("Pushing Docker image: %s", imageName)
	cmd := exec.CommandContext(ctx, "docker", "push", imageName)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%w: %v", ErrPushFailed, err)
	}

	dm.logger.Infof("Image pushed successfully: %s", imageName)
	return nil
}
