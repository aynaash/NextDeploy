package cmd
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
# Use official Node.js image
FROM node:20-alpine

# Set working directory
WORKDIR /app

# 1. Copy package files for dependency installation
COPY package.json package-lock.json ./

# 2. Install dependencies (production-only)
RUN npm ci --only=production

# 3. Copy all files (except those in .dockerignore)
COPY . .

# 4. Build the app
RUN npm run build

# 5. Use lightweight web server for production
FROM node:20-alpine
WORKDIR /app

# Copy only necessary files from builder
COPY --from=0 /app/public ./public
COPY --from=0 /app/.next/standalone ./
COPY --from=0 /app/.next/static ./.next/static

# 6. Run as non-root user
RUN adduser -D nextjs && chown -R nextjs:nextjs /app
USER nextjs

# 7. Start the app
EXPOSE 3000
ENV PORT=3000
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
