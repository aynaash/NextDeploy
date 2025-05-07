//go:build ignore
// +build ignore

package cmd

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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
		if !dockerfileExists() {
			fmt.Println("❌ Error: No Dockerfile found in current directory.")
			fmt.Println("ℹ️  Run 'nextdeploy init' or navigate to a directory with a Dockerfile.")
			os.Exit(1)
		}

		if tag == "" {
			commitHash, err := getGitCommitHash()
			if err != nil {
				fmt.Println("❌ Error: Failed to get latest Git commit hash.")
				os.Exit(1)
			}
			tag = commitHash
			fmt.Printf("ℹ️  No tag provided. Using latest Git commit hash: %s\n", tag)
		}

		if isGitDirty() {
			fmt.Println("⚠️  Warning: You have uncommitted changes.")
			fmt.Println("💡 Commit changes to ensure accurate versioning.")
		}
	},
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("🚀 Building Docker image...")

		fullImageName := fmt.Sprintf("%s/%s:%s", registry, imageName, tag)

		if err := buildDockerImage(fullImageName, noCache); err != nil {
			fmt.Printf("❌ Error: Failed to build image: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("✅ Successfully built: %s\n", fullImageName)
	},
}

func init() {
	buildCmd.Flags().StringVarP(&imageName, "image", "i", "nextjs-app", "Name for the Docker image")
	buildCmd.Flags().StringVarP(&registry, "registry", "r", "registry.digitalocean.com/your-namespace", "Container registry URL")
	buildCmd.Flags().BoolVar(&noCache, "no-cache", false, "hld without using cache")
	buildCmd.Flags().StringVarP(&tag, "tag", "t", "", "Tag for the Docker image (default: latest Git commit)")

	rootCmd.AddCommand(buildCmd)
}

// dockerfileExists checks if a Dockerfile exists in the current directory
func dockerfileExists() bool {
	_, err := os.Stat(filepath.Join(".", "Dockerfile"))
	return !os.IsNotExist(err)
}

// getGitCommitHash returns the short hash of the latest Git commit
func getGitCommitHash() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--short", "HEAD")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	return strings.TrimSpace(out.String()), err
}

// isGitDirty returns true if there are uncommitted changes
func isGitDirty() bool {
	cmd := exec.Command("git", "status", "--porcelain")
	var out bytes.Buffer
	cmd.Stdout = &out
	_ = cmd.Run()
	return out.Len() > 0
}

// buildDockerImage executes the Docker build command
func buildDockerImage(image string, noCache bool) error {
	args := []string{"build", "-t", image, "."}
	if noCache {
		args = append(args, "--no-cache")
	}

	cmd := exec.Command("docker", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}



// stored code 

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

// initCmd represents the init command
var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize your project with Dockerfile and nextdeploy.yml",
	Long:  "Scaffolds a Dockerfile and nextdeploy.yml to set up your Next.js project for deployment.",
	Run: func(cmd *cobra.Command, args []string) {
		// Check for existing Dockerfile
		dockerfilePath := filepath.Join(".", "Dockerfile")
		if _, err := os.Stat(dockerfilePath); os.IsNotExist(err) {
			fmt.Println("ℹ️  No Dockerfile found in current directory")
			
			// Prompt user to create a sample Dockerfile
			reader := bufio.NewReader(os.Stdin)
			fmt.Print("Would you like to create a sample Next.js Dockerfile? (y/n): ")
			response, _ := reader.ReadString('\n')
			response = strings.TrimSpace(strings.ToLower(response))

			if response == "y" || response == "yes" {
				// Get package manager preference
				pkgManager := promptForPackageManager()
				
				// Generate and create Dockerfile
				dockerfile := generateDockerfile(pkgManager)
				createFile("Dockerfile", dockerfile)
			} else {
				fmt.Println("ℹ️  Skipping Dockerfile creation")
			}
		} else {
			fmt.Println("✅ Dockerfile already exists")
		}

		// Generate nextdeploy.yml with required fields
		createNextDeployConfig()
	},
}

func promptForPackageManager() string {
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("Choose your package manager (npm/yarn/pnpm): ")
		pkgManager, _ := reader.ReadString('\n')
		pkgManager = strings.TrimSpace(strings.ToLower(pkgManager))
		
		switch pkgManager {
		case "npm", "yarn", "pnpm":
			return pkgManager
		default:
			fmt.Println("❌ Invalid choice. Please enter npm, yarn, or pnpm")
		}
	}
}

func createNextDeployConfig() {
	reader := bufio.NewReader(os.Stdin)
	
	// Get required configuration values
	fmt.Print("Enter your application name: ")
	appName, _ := reader.ReadString('\n')
	appName = strings.TrimSpace(appName)
	
	fmt.Print("Enter SSH host (user@your-vps-ip): ")
	sshHost, _ := reader.ReadString('\n')
	sshHost = strings.TrimSpace(sshHost)
	
	fmt.Print("Enter Docker compose path on server (e.g. /home/user/my-app/docker-compose.yml): ")
	composePath, _ := reader.ReadString('\n')
	composePath = strings.TrimSpace(composePath)
	
	// Generate the config content
	configContent := fmt.Sprintf(`
app_name: %s
port: 3000
deploy:
  ssh_host: %s
  docker_compose_path: %s
`, appName, sshHost, composePath)
	
	createFile("nextdeploy.yml", configContent)
}

// Function to generate Dockerfile based on package manager choice
func generateDockerfile(pkgManager string) string {
	dockerfileContent := fmt.Sprintf(`
# syntax=docker.io/docker/dockerfile:1
### ---- BASE IMAGE ---- ###
FROM node:22-slim AS base
WORKDIR /app
ENV NODE_ENV=production
ENV NEXT_TELEMETRY_DISABLED=1
RUN apk add --no-cache libc6-compat bash

### ---- DEPENDENCIES ---- ###
FROM base AS deps

# Copy only the files needed to determine lockfile
COPY package.json yarn.lock* package-lock.json* pnpm-lock.yaml* .npmrc* ./

RUN bash -c '\
  echo "Checking for lockfiles..." && \
  [ -f yarn.lock ] && echo "✅ yarn.lock found" || echo "❌ yarn.lock not found" && \
  [ -f package-lock.json ] && echo "✅ package-lock.json found" || echo "❌ package-lock.json not found" && \
  [ -f pnpm-lock.yaml ] && echo "✅ pnpm-lock.yaml found" || echo "❌ pnpm-lock.yaml not found" && \
  \
  if [ -f yarn.lock ]; then \
    echo "🔨 Building with Yarn" && \
    (command -v yarn >/dev/null 2>&1 || npm install -g yarn) && \
    yarn build; \
  elif [ -f package-lock.json ]; then \
    echo "🔨 Building with NPM" && \
    npm run build; \
  elif [ -f pnpm-lock.yaml ]; then \
    echo "🔨 Building with PNPM" && \
    (command -v pnpm >/dev/null 2>&1 || npm install -g pnpm) && \
    pnpm run build; \
  else \
    echo "❌ No lockfile found. Build cannot continue."; \
    exit 1; \
  fi'

### ---- BUILDER ---- ###
FROM base AS builder
COPY --from=deps /app/node_modules ./node_modules
COPY . .

# Optional: Add build script fallback
RUN bash -c '\
  if [ -f yarn.lock ]; then \
    yarn build; \
  elif [ -f package-lock.json ]; then \
    npm run build; \
  elif [ -f pnpm-lock.yaml ]; then \
    corepack enable pnpm && pnpm run build; \
  else \
    echo "⚠️  No lockfile found. Skipping build."; \
  fi'

### ---- RUNTIME ---- ###
FROM base AS runner

# Setup app directory and user
RUN addgroup -S nodejs && adduser -S nextjs -G nodejs
WORKDIR /app

# Copy only needed files
COPY --from=builder /app/public ./public
COPY --from=builder /app/.next ./.next

# Handle both standalone and traditional builds safely
RUN bash -c '\
  mkdir -p .next && \
  if [ -d /app/.next/standalone ]; then \
    echo "🚀 Using standalone build"; \
    cp -r /app/.next/standalone/* ./ && \
    cp -r /app/.next/static ./public/static; \
  else \
    echo "📦 Using traditional .next build"; \
    mkdir -p .next && \
    cp -r /app/.next/* .next/; \
  fi'
USER nextjs
EXPOSE 3000
ENV PORT=3000
ENV HOSTNAME=0.0.0.0

CMD ["node", "server.js"]
`)

	return dockerfileContent
}

// Function to create files if they don't already exist
func createFile(name, content string) {
	path := filepath.Join(".", name)
	if _, err := os.Stat(path); err == nil {
		fmt.Printf("⚠️  %s already exists, skipping...\n", name)
		return
	}

	err := os.WriteFile(path, []byte(content), 0644)
	if err != nil {
		fmt.Printf("❌ Failed to create %s: %v\n", name, err)
	} else {
		fmt.Printf("✅ Created %s\n", name)
	}
}

func init() {
	rootCmd.AddCommand(initCmd)
}
