// NOTE: cross compilation safe
package docker

import (
	"archive/tar"
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"nextdeploy/shared"
	"nextdeploy/shared/config"
	"nextdeploy/shared/nextcore"
	"nextdeploy/shared/registry"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/docker/docker/api/types/build"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/moby/term"
	"github.com/spf13/cobra"
)

const (
	DockerfileName = "Dockerfile"
	nodeVersion    = "20-alpine"
)

var (
	dlog = shared.PackageLogger("docker", "🐳 DOCKER")
)

var (
	forceOverwrite   bool
	skipPrompts      bool
	ProvisionEcrUser bool
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
	cli     *client.Client
	verbose bool
	logger  Logger // Interface for logging
}

type Logger interface {
	Debugf(format string, args ...interface{})
	Infof(format string, args ...interface{})
	Errorf(format string, args ...interface{})
}

type BuildOptions struct {
	ImageName        string
	NoCache          bool
	Pull             bool
	Target           string
	Platform         string            // Added platform support
	ProvisionEcrUser bool              // Flag to provision ECR user
	Fresh            bool              // Flag to indicate fresh build
	AddHost          []string          `json:"add-host"`        // Custom host-to-IP mappings
	Allow            []string          `json:"allow"`           // Extra privileged entitlements
	Annotations      []string          `json:"annotation"`      // Image annotations
	Attestations     []string          `json:"attest"`          // Attestation parameters
	BuildArgs        map[string]string `json:"build-arg"`       // Build-time variables
	BuildContexts    map[string]string `json:"build-context"`   // Additional build contexts
	Builder          string            `json:"builder"`         // Builder instance override
	CacheFrom        []string          `json:"cache-from"`      // External cache sources
	CacheTo          []string          `json:"cache-to"`        // Cache export destinations
	CgroupParent     string            `json:"cgroup-parent"`   // Parent cgroup for RUN
	Dockerfile       string            `json:"file"`            // Dockerfile name/path
	IIDFile          string            `json:"iidfile"`         // Image ID output file
	Labels           map[string]string `json:"label"`           // Image metadata
	Load             bool              `json:"load"`            // Output to docker
	MetadataFile     string            `json:"metadata-file"`   // Build metadata output
	Network          string            `json:"network"`         // Networking mode
	NoCacheFilter    []string          `json:"no-cache-filter"` // Stages to exclude from cache
	Outputs          []string          `json:"output"`          // Output destinations
	Platforms        []string          `json:"platform"`        // Target platforms
	Progress         string            `json:"progress"`        // Progress output type
	Provenance       string            `json:"provenance"`      // Provenance attestation
	Push             bool              `json:"push"`            // Push to registry
	Quiet            bool              `json:"quiet"`           // Suppress output
	SBOM             string            `json:"sbom"`            // SBOM attestation
	Secrets          []string          `json:"secret"`          // Build secrets
	ShmSize          string            `json:"shm-size"`        // /dev/shm size
	SSH              []string          `json:"ssh"`             // SSH agent/keys
	Tags             []string          `json:"tag"`             // Image name/tags
	Ulimits          []string          `json:"ulimit"`          // Ulimit options
}

func NewDockerClient(logger *shared.Logger) (*DockerManager, error) {
	cli, err := client.NewClientWithOpts(
		client.FromEnv,
		client.WithAPIVersionNegotiation(),
	)
	if err != nil {
		dlog.Error("Failed to create Docker client: %v", err)
		return nil, fmt.Errorf("failed to create Docker client: %w", err)
	}
	return &DockerManager{
		cli:     cli,
		verbose: true, // Set to true for verbose logging
	}, nil
}

// NewDockerManager creates a new DockerManager instance

func (dm *DockerManager) BuildImage(ctx context.Context, opts BuildOptions) error {
	// generate and validate metadata first
	metadata, err := nextcore.GenerateMetadata()
	if err != nil {
		dlog.Error("Metadata generation failed: %v", err)
		return fmt.Errorf("metadata generation  failed:%w", err)
	}
	// validate
	if err := nextcore.ValidateBuildState(); err != nil {
		dlog.Error("Failed to validate build state: %v", err)
		return fmt.Errorf("failed to validate build state: %w", err)
	}
	// create build context
	buildContext, err := dm.createBuildContext()
	if err != nil {
		dlog.Error("Failed to create build context: %v", err)
		return fmt.Errorf("failed to create build context: %w", err)
	}
	defer buildContext.Close()

	// configure build options
	buildOptions := build.ImageBuildOptions{
		Tags:        []string{opts.ImageName},
		Dockerfile:  "Dockerfile",
		Remove:      true,
		ForceRemove: true,
		NoCache:     opts.NoCache,
		PullParent:  opts.Pull,
		Platform:    opts.Platform,
		Target:      opts.Target,
	}
	// add build args from metadata
	if envVars := dm.GetBuildArgs(); envVars == nil {
		dlog.Error("Failed to get build args: %v", err)
		return fmt.Errorf("failed to get build args: %w", err)
	}
	// Execute build
	resp, err := dm.cli.ImageBuild(ctx, buildContext, buildOptions)
	if err != nil {
		dlog.Error("Failed to build Docker image: %v", err)
		return fmt.Errorf("failed to build Docker image: %w", err)
	}
	defer resp.Body.Close()
	// Display build output
	fd, isTerminal := term.GetFdInfo(os.Stdout)
	//TODO: we can build the container here using .nextdeploy/metadata.json
	nextruntime, err := nextcore.NewNextRuntime(&metadata)
	if err != nil {
		dlog.Error("error creating next runtime:%s", err)
		return err
	}
	ID, err := nextruntime.CreateContainer(ctx)
	if err != nil {
		dlog.Error("Failed to create container: %v", err)
		return fmt.Errorf("failed to create container: %w", err)
	}
	dlog.Success("Docker image built successfully with ID: %s", ID)
	return jsonmessage.DisplayJSONMessagesStream(resp.Body, os.Stdout, fd, isTerminal, nil)
}
func (dm *DockerManager) createBuildContext() (io.ReadCloser, error) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	defer tw.Close()

	// Add Dockerfile
	cwd, err := os.Getwd()
	if err != nil {
		dlog.Error("Failed to get current directory: %v", err)
		return nil, fmt.Errorf("failed to get current directory: %w", err)
	}
	packageManger, err := nextcore.DetectPackageManager(cwd)
	dockerfileContent, err := dm.generateDockerfileContent(packageManger.String())
	dlog.Debug("Generated Dockerfile content:\n%s", dockerfileContent)
	if err != nil {
		dlog.Error("Failed to generate Dockerfile content: %v", err)
		return nil, err
	}
	if err := addFileToTar(tw, "Dockerfile", []byte(dockerfileContent), 0644); err != nil {
		dlog.Error("Failed to add Dockerfile to tar: %v", err)
		return nil, err
	}

	// Add application files
	if err := dm.addAppFiles(tw); err != nil {
		dlog.Error("Failed to add application files to tar: %v", err)
		return nil, err
	}

	// Add metadata and assets
	if err := dm.addMetadataAndAssets(tw); err != nil {
		dlog.Error("Failed to add metadata and assets to tar: %v", err)
		return nil, err
	}

	return io.NopCloser(&buf), nil
}

func (dm DockerManager) addAppFiles(tw *tar.Writer) error {
	// Get current working directory
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %w", err)
	}

	// Detect package manager
	packageManager, err := nextcore.DetectPackageManager(cwd)
	if err != nil {
		dlog.Error("Failed to detect package manager: %v", err)
		return fmt.Errorf("failed to detect package manager: %w", err)
	}

	// Determine lock files based on package manager
	var packageFiles []string
	switch packageManager {
	case "yarn":
		packageFiles = []string{"package.json", "yarn.lock"}
	case "pnpm":
		packageFiles = []string{"package.json", "pnpm-lock.yaml"}
	default: // npm
		packageFiles = []string{"package.json", "package-lock.json"}
	}

	// Add package files
	for _, file := range packageFiles {
		if err := dm.addFileIfExists(tw, file); err != nil {
			dlog.Error("Failed to add package file %s: %v", file, err)
			return fmt.Errorf("failed to add package file %s: %w", file, err)
		}
	}

	// Add configuration files
	configFiles := []string{"next.config.js", "next.config.mjs", ".env", ".env.local"}
	for _, file := range configFiles {
		if err := dm.addFileIfExists(tw, file); err != nil {
			dlog.Error("Failed to add config file %s: %v", file, err)
			return fmt.Errorf("failed to add config file %s: %w", file, err)
		}
	}

	// Add source directories
	dirs := []string{"pages", "app", "components", "public", "src"}
	for _, dir := range dirs {
		if _, err := os.Stat(dir); err == nil {
			if err := addDirectoryToTar(tw, dir, dir); err != nil {
				dlog.Error("Failed to add directory %s: %v", dir, err)
				return fmt.Errorf("failed to add directory %s: %w", dir, err)
			}
		}
	}

	return nil
}

func addDirectoryToTar(tw *tar.Writer, srcDir, tarDir string) error {
	// Walk through the directory and add files to tar
	return filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			dlog.Error("Error walking path %s: %v", path, err)
			return fmt.Errorf("error walking path %s: %w", path, err)
		}
		if info.IsDir() {
			return nil // Skip directories
		}

		// Read file content
		content, err := os.ReadFile(path)
		if err != nil {
			dlog.Error("Failed to read file %s: %v", path, err)
			return fmt.Errorf("failed to read file %s: %w", path, err)
		}

		// Create tar header
		header := &tar.Header{
			Name: filepath.Join(tarDir, strings.TrimPrefix(path, srcDir+"/")),
			Mode: int64(info.Mode()),
			Size: int64(len(content)),
		}

		if err := tw.WriteHeader(header); err != nil {
			dlog.Error("Failed to write header for %s: %v", path, err)
			return fmt.Errorf("failed to write header for %s: %w", path, err)
		}

		if _, err := tw.Write(content); err != nil {
			dlog.Error("Error writing content for %s: %v", path, err)
			return fmt.Errorf("failed to write content for %s: %w", path, err)
		}

		return nil
	})
}
func (dm *DockerManager) addMetadataAndAssets(tw *tar.Writer) error {
	// Add metadata files
	metadataFiles := []string{".nextdeploy/metadata.json", ".nextdeploy/build.lock"}
	for _, file := range metadataFiles {
		if content, err := os.ReadFile(file); err == nil {
			if err := addFileToTar(tw, file, content, 0644); err != nil {
				dlog.Error("Failed to add metadata file %s: %v", file, err)
				return err
			}
		}
	}

	// Add static assets
	if err := addDirectoryToTar(tw, ".nextdeploy/assets", ".nextdeploy/assets"); err != nil {
		dlog.Error("Failed to add static assets: %v", err)
		return fmt.Errorf("failed to add static assets: %w", err)
	}

	return nil
}

// TODO: build args are static we need load from somewhere that is user defined
func (dc *DockerManager) GetBuildArgs() map[string]string {
	return map[string]string{
		"NODE_ENV":     "production",
		"NODE_VERSION": "20-alpine",
		"APP_ENV":      "production",
	}
} // Helper function to add a file to tar if it exists
func (dm DockerManager) addFileIfExists(tw *tar.Writer, filename string) error {
	content, err := os.ReadFile(filename)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist - skip silently
			dlog.Debug("File %s does not exist, skipping", filename)
			return nil
		}
		return err
	}

	if err := addFileToTar(tw, filename, content, 0644); err != nil {
		dlog.Error("Failed to add file %s to tar: %v", filename, err)
		return fmt.Errorf("failed to add file %s to tar: %w", filename, err)
	}
	return nil
}

// Helper functions
func addFileToTar(tw *tar.Writer, name string, content []byte, mode int64) error {
	hdr := &tar.Header{
		Name: name,
		Mode: mode,
		Size: int64(len(content)),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		dlog.Error("Failed to write header for %s: %v", name, err)
		return err
	}
	_, err := tw.Write(content)
	if err != nil {
		dlog.Error("Failed to write content for %s: %v", name, err)
		// Return a more descriptive error
		return fmt.Errorf("failed to write content for %s: %w", name, err)
	}
	return err
}

// DockerfileExists checks if a Dockerfile exists in the specified directory
func (dm *DockerManager) DockerfileExists(dir string) (bool, error) {
	path := filepath.Join(dir, DockerfileName)
	dlog.Debug("Checking for Dockerfile at: %s", path)

	stat, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			dlog.Debug("Dockerfile does not exist at: %s", path)
			return false, nil
		}
		dlog.Error("Error checking Dockerfile: %v", err)
		return false, fmt.Errorf("error checking Dockerfile: %w", err)
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
		dlog.Error("Failed to check Dockerfile existence: %v", err)
		return fmt.Errorf("failed to check Dockerfile existence: %w", err)
	}
	if exists && !overwrite {
		return ErrDockerfileExists
	}
	content, err := dm.generateDockerfileContent(pkgManager)
	if err != nil {
		dlog.Error("Failed to generate Dockerfile content: %v", err)
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
		dlog.Error("Failed to write Dockerfile: %v", err)
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

	// Updated regex patterns with better support for git commit hashes
	validImageName := regexp.MustCompile(`^[a-z0-9]+(?:[._-][a-z0-9]+)*(?:/[a-z0-9]+(?:[._-][a-z0-9]+)*)*$`)
	validTag := regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,127}$`)

	parts := strings.SplitN(name, ":", 2)
	if !validImageName.MatchString(parts[0]) {
		dlog.Error("Invalid repository name: %s", parts[0])
		return fmt.Errorf("%w: invalid repository name", ErrInvalidImageName)
	}

	if len(parts) == 2 {
		tag := parts[1]
		if len(tag) > 128 {
			dlog.Error("Tag exceeds maximum length (128 characters)")
			return fmt.Errorf("%w: tag exceeds maximum length", ErrInvalidImageName)
		}

		// Special case for git commit hashes (7-40 hex chars)
		if isGitCommitHash(tag) {
			return nil
		}

		if !validTag.MatchString(tag) {
			dlog.Error("Invalid tag format: %s", tag)
			return fmt.Errorf("%w: invalid tag format", ErrInvalidImageName)
		}
	}

	return nil
}

// isGitCommitHash checks if the tag is a valid git commit hash (7-40 hex characters)
func isGitCommitHash(tag string) bool {
	if len(tag) < 7 || len(tag) > 40 {
		return false
	}
	matched, _ := regexp.MatchString(`^[a-f0-9]+$`, tag)
	return matched
}

// CheckDockerInstalled verifies Docker is available
func (dm *DockerManager) CheckDockerInstalled() error {
	dlog.Info("Checking if Docker is installed...")
	path, err := exec.LookPath("docker")
	if err != nil {
		dlog.Error("Docker not found in PATH: %v", err)
		return fmt.Errorf("%w: %s", ErrDockerNotInstalled, err)
	}

	dlog.Success("Docker found at: %s", path)

	// Verify docker is actually working
	cmd := exec.Command("docker", "version")
	err = cmd.Run()
	if err != nil {
		dlog.Error("Docker command failed: %v", err)
		return fmt.Errorf("%w: %s", ErrDockerNotInstalled, err)
	}
	return nil
}
func (dm *DockerManager) PushImage(ctx context.Context, imageName string, ProvisionECRUser bool, Fresh bool) error {
	err := dm.ValidateImageName(imageName)
	if err != nil {
		dlog.Error("Invalid Docker image name: %s", imageName)
		return fmt.Errorf("%w: %s", ErrInvalidImageName, err)
	}
	err = dm.CheckDockerInstalled()
	if err != nil {
		dlog.Error("Docker is not installed or not functioning: %v", err)
		return fmt.Errorf("%w: %s", ErrDockerNotInstalled, err)
	}
	cfg, err := config.Load()
	if err != nil {
		dlog.Error("Failed to load configuration: %v", err)
		return fmt.Errorf("failed to load configuration: %w", err)
	}
	if cfg.Docker.Registry == "ecr" {
		dlog.Info("Preparing ECR context for image push")
		ecrContext := registry.ECRContext{
			ECRRepoName: cfg.Docker.Image,
			ECRRegion:   cfg.Docker.RegistryRegion,
		}
		if ProvisionECRUser {
			if Fresh {
				dlog.Info("Provisioning new ECR user and policy and new old ones for name conflict")
				// before deleting we we need to confirm user to deleted exists in order to avoid try to non-existing user deletion
				exists, err := registry.CheckUserExists()
				if err != nil {
					dlog.Error("Failed to check if ECR user exists: %v", err)
					return fmt.Errorf("failed to check if ECR user exists: %w", err)
				}
				if exists {
					err = registry.DeleteECRUserAndPolicy()
					if err != nil {
						dlog.Error("Failed to delete old ECR user and policy: %v", err)
						return fmt.Errorf("failed to delete old ECR user and policy: %w", err)
					} else {
						dlog.Info("Old ECR user and policy deleted successfully")
					}
				}
			}
			user, err := registry.CreateECRUserAndPolicy()
			if err != nil {
				dlog.Error("Failed to create ECR user and policy: %v", err)
				return fmt.Errorf("failed to create ECR user and policy: %w", err)
			}
			dlog.Info("ECR user created: %v", user)
		}
		dlog.Debug("ECR context: %+v", ecrContext)
		// prepare ecr push context
		err = registry.PrepareECRPushContext(ctx, Fresh)
		if err != nil {
			dlog.Error("Failed to prepare ECR push context: %v", err)
			return fmt.Errorf("failed to prepare ECR push context: %w", err)
		}
	}

	dlog.Info("Pushing Docker image: %s", imageName)
	cmd := exec.CommandContext(ctx, "docker", "push", imageName)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err = cmd.Run()
	if err != nil {
		dlog.Error("Failed to push Docker image: %v", err)
		return fmt.Errorf("%w: %s", ErrPushFailed, err)
	}
	dlog.Success("Image pushed successfully: %s", imageName)
	return nil
}

func HandleDockerfileSetup(cmd *cobra.Command, dm *DockerManager, reader *bufio.Reader) error {
	if skipPrompts || config.PromptYesNo(reader, "Set up Dockerfile for your project?") {
		exists, err := dm.DockerfileExists(".")
		if err != nil {
			dlog.Error("Failed to check for Dockerfile: %v", err)
			return fmt.Errorf("failed to check for Dockerfile: %w", err)
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
			detectManager, err := nextcore.DetectPackageManager(cwd)
			if err != nil {
				dlog.Error("Failed to detect package manager: %v", err)
				return fmt.Errorf("failed to detect package manager: %w", err)
			}
			pkgManager = detectManager.String()
		}

		err = dm.GenerateDockerfile(".", pkgManager, true)
		if err != nil {
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
		return fmt.Errorf("failed to open .gitignore file: %w", err)
	}

	err = f.Close()
	if err != nil {
		return fmt.Errorf("failed to close .gitignore file: %w", err)
	}

	_, err = f.WriteString(strings.Join(toAdd, "\n") + "\n")
	if err != nil {
		return fmt.Errorf("failed to write to .gitignore file: %w", err)
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
		// Read existing content
		existingContent, err := os.ReadFile(".dockerignore")
		if err != nil {
			return fmt.Errorf("failed to read existing .dockerignore: %w", err)
		}

		// Split existing content into lines
		existingLines := strings.Split(string(existingContent), "\n")

		// Create a map for existing patterns for quick lookup
		existingPatterns := make(map[string]bool)
		for _, line := range existingLines {
			line = strings.TrimSpace(line)
			if line != "" && !strings.HasPrefix(line, "#") {
				existingPatterns[line] = true
			}
		}

		// Add only new patterns that don't exist already
		var newPatterns []string
		for _, line := range patterns {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				newPatterns = append(newPatterns, line)
			} else if !existingPatterns[line] {
				newPatterns = append(newPatterns, line)
			}
		}

		// Combine existing content with new patterns
		content := string(existingContent)
		if len(content) > 0 && !strings.HasSuffix(content, "\n") {
			content += "\n"
		}
		content += strings.Join(newPatterns, "\n")

		// Write the merged file
		err = os.WriteFile(".dockerignore", []byte(content), 0644)
		if err != nil {
			return fmt.Errorf("failed to update .dockerignore: %w", err)
		}
		return nil
	}

	// Write the file if it doesn't exist
	content := strings.Join(patterns, "\n")
	err := os.WriteFile(".dockerignore", []byte(content), 0644)
	if err != nil {
		return fmt.Errorf("failed to create .dockerignore: %w", err)
	}
	return nil
}
