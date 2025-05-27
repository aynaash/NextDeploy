package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize your project with Dockerfile and nextdeploy.yml",
	Long:  "Scaffolds a Dockerfile and nextdeploy.yml to set up your Next.js project for deployment.",
	Run: func(cmd *cobra.Command, args []string) {
		// Check for existing Dockerfile
		dockerfilePath := filepath.Join(".", "Dockerfile")
		if _, err := os.Stat(dockerfilePath); os.IsNotExist(err) {
			fmt.Println("ℹ️  No Dockerfile found in current directory")

			reader := bufio.NewReader(os.Stdin)
			fmt.Print("Would you like to create a sample Next.js Dockerfile? (y/n): ")
			resp, _ := reader.ReadString('\n')
			resp = strings.TrimSpace(strings.ToLower(resp))

			if resp == "y" || resp == "yes" {
				// Currently defaulting to yarn, prompt is commented out for now
				pkgManager := "yarn"

				dockerfile := generateDockerfile(pkgManager)
				createFile("Dockerfile", dockerfile)
			} else {
				fmt.Println("ℹ️  Skipping Dockerfile creation")
			}
		} else {
			fmt.Println("✅ Dockerfile already exists")
		}

		// Always try to generate nextdeploy.yml
		createNextDeployConfig()
	},
}

func init() {
	// This init function runs automatically and registers the init command
	rootCmd.AddCommand(initCmd)
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

	fmt.Print("Enter your application name: ")
	appName, _ := reader.ReadString('\n')
	appName = strings.TrimSpace(appName)

	fmt.Print("Enter your server's SSH host (e.g., user@ip): ")
	sshHost, _ := reader.ReadString('\n')
	sshHost = strings.TrimSpace(sshHost)

	fmt.Print("Enter docker-compose path on server (/home/user/my-app/docker-compose.yml): ")
	composePath, _ := reader.ReadString('\n')
	composePath = strings.TrimSpace(composePath)

	configContent := fmt.Sprintf(`
app_name: %s
port: 3000
deploy:
  ssh_host: %s
  docker_compose_path: %s
`, appName, sshHost, composePath)

	createFile("nextdeploy.yml", configContent)
}

func generateDockerfile(pkgManager string) string {
	return fmt.Sprintf(`
# ---------- STAGE 1: Build ----------
FROM node:20-alpine AS builder

WORKDIR /app

# Copy dependency files
COPY package.json ./
COPY %s.lock ./

RUN %s install

# Copy rest of the project files
COPY . .

RUN %s run build

# ---------- STAGE 2: Runtime ----------
FROM node:20-alpine

WORKDIR /app

COPY --from=builder /app/public ./public
COPY --from=builder /app/.next/standalone ./
COPY --from=builder /app/.next/static ./.next/static

RUN adduser -D nextjs && chown -R nextjs:nextjs /app
USER nextjs

EXPOSE 3000
ENV PORT=3000

CMD ["node", "server.js"]
`, pkgManager, pkgManager, pkgManager)
}

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
