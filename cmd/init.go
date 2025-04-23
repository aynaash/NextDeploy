package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"github.com/spf13/cobra"
)

type   initoptions  struct {
packageManager  string
cloudprovider string
}

// initCmd represents the init command
var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize your project with Dockerfile and nextdeploy.yml",
	Long:  "Scaffolds a Dockerfile and nextdeploy.yml to set up your Next.js project for deployment.",
	Run: func(cmd *cobra.Command, args []string) {
		// Get the package manager preference from flags
		pkgManager, _ := cmd.Flags().GetString("package-manager")

		// Generate the Dockerfile and nextdeploy.yml
		dockerfile := generateDockerfile(pkgManager)
		createFile("Dockerfile", dockerfile)
		createFile("nextdeploy.yml", deployConfigContent)
	},
}

// Set default content for nextdeploy.yml
const deployConfigContent = `
app_name: my-nextjs-app
port: 3000
deploy:
  ssh_host: user@your-vps-ip
  docker_compose_path: /home/user/my-nextjs-app/docker-compose.yml
`

// Function to generate Dockerfile based on package manager choice
func generateDockerfile(pkgManager string) string {
	if pkgManager == "" {
		pkgManager = "npm" // Default to npm if no flag is provided
	}

	dockerfileContent := fmt.Sprintf(`
# syntax=docker.io/docker/dockerfile:1

FROM node:18-alpine AS base

# Install dependencies only when needed
FROM base AS deps
RUN apk add --no-cache libc6-compat
WORKDIR /app

# Install dependencies based on the preferred package manager
COPY package.json yarn.lock* package-lock.json* pnpm-lock.yaml* .npmrc* ./
RUN \
  if [ -f yarn.lock ]; then yarn --frozen-lockfile; \
  elif [ -f package-lock.json ]; then npm ci; \
  elif [ -f pnpm-lock.yaml ]; then corepack enable pnpm && pnpm i --frozen-lockfile; \
  else echo "Lockfile not found." && exit 1; \
  fi

# Rebuild the source code only when needed
FROM base AS builder
WORKDIR /app
COPY --from=deps /app/node_modules ./node_modules
COPY . .

RUN \
  if [ -f yarn.lock ]; then yarn run build; \
  elif [ -f package-lock.json ]; then npm run build; \
  elif [ -f pnpm-lock.yaml ]; then corepack enable pnpm && pnpm run build; \
  else echo "Lockfile not found." && exit 1; \
  fi

# Production image, copy all the files and run next
FROM base AS runner
WORKDIR /app

ENV NODE_ENV=production
RUN addgroup --system --gid 1001 nodejs
RUN adduser --system --uid 1001 nextjs

COPY --from=builder /app/public ./public
COPY --from=builder --chown=nextjs:nodejs /app/.next/standalone ./
COPY --from=builder --chown=nextjs:nodejs /app/.next/static ./.next/static

USER nextjs
EXPOSE 3000
ENV PORT=3000
ENV HOSTNAME="0.0.0.0"
CMD ["node", "server.js"]
`, pkgManager)

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
	// Adding 'package-manager' flag to specify which package manager to use
	initCmd.Flags().StringP("package-manager", "p", "", "Specify the package manager (npm, yarn, pnpm)")

	// Adding init command to rootCmd
	rootCmd.AddCommand(initCmd)
}
