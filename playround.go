package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

// Config represents the structure of the nextdeploy.yaml file
type Config struct {
	Project     string   `yaml:"project"`
	Repository  string   `yaml:"repository"`
	Branch      string   `yaml:"branch"`
	Environment string   `yaml:"environment"`
	BuildCmd    string   `yaml:"buildCmd"`
	StartCmd    string   `yaml:"startCmd"`
	Envs        []string `yaml:"envs"`
	Database    struct {
		Type     string `yaml:"type"`
		Version  string `yaml:"version"`
		Name     string `yaml:"name"`
		Username string `yaml:"username"`
	} `yaml:"database"`
	Resources struct {
		CPUs   int    `yaml:"cpus"`
		Memory string `yaml:"memory"`
		Disk   string `yaml:"disk"`
	} `yaml:"resources"`
}

var rootCmd = &cobra.Command{
	Use:   "nextdeploy",
	Short: "NextDeploy - Deploy Next.js applications to DigitalOcean",
	Long:  `A CLI tool to deploy and manage Next.js applications on DigitalOcean VPS instances.`,
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new NextDeploy project",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Initializing NextDeploy project...")
		
		// Create default config
		config := Config{
			Project:     filepath.Base(getCurrentDir()),
			Repository:  "https://github.com/username/repo.git",
			Branch:      "main",
			Environment: "production",
			BuildCmd:    "npm run build",
			StartCmd:    "npm start",
			Envs:        []string{"NODE_ENV=production"},
		}
		
		config.Database.Type = "postgres"
		config.Database.Version = "14"
		config.Database.Name = "nextapp"
		config.Database.Username = "postgres"
		
		config.Resources.CPUs = 1
		config.Resources.Memory = "1GB"
		config.Resources.Disk = "25GB"
		
		// Create config file
		configYaml, err := yaml.Marshal(config)
		if err != nil {
			fmt.Printf("Error creating config: %v\n", err)
			return
		}
		
		err = os.WriteFile("nextdeploy.yaml", configYaml, 0644)
		if err != nil {
			fmt.Printf("Error writing config file: %v\n", err)
			return
		}
		
		fmt.Println("Created nextdeploy.yaml configuration file")
	},
}

var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploy your Next.js application",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Deploying Next.js application...")
		
		// Load config
		config, err := loadConfig()
		if err != nil {
			fmt.Printf("Error loading config: %v\n", err)
			return
		}
		
		// Connect to NextDeploy API
		apiKey := viper.GetString("api_key")
		if apiKey == "" {
			fmt.Println("API key not found. Please run 'nextdeploy login' first.")
			return
		}
		
		// Make API call to start deployment
		fmt.Printf("Starting deployment for %s...\n", config.Project)
		// Implementation of API call would go here
		
		fmt.Println("Deployment started! Run 'nextdeploy logs' to view deployment progress.")
	},
}

var logsCmd = &cobra.Command{
	Use:   "logs",
	Short: "View logs for your deployed application",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Fetching logs...")
		
		// Load config to get project name
		config, err := loadConfig()
		if err != nil {
			fmt.Printf("Error loading config: %v\n", err)
			return
		}
		
		// Connect to NextDeploy API
		apiKey := viper.GetString("api_key")
		if apiKey == "" {
			fmt.Println("API key not found. Please run 'nextdeploy login' first.")
			return
		}
		
		// Make API call to fetch logs
		fmt.Printf("Logs for %s:\n", config.Project)
		// Implementation of log fetching would go here
		
		fmt.Println("--- Sample log output ---")
		fmt.Println("2023-03-30T12:00:00Z [INFO] Building Next.js application")
		fmt.Println("2023-03-30T12:01:30Z [INFO] Build completed successfully")
		fmt.Println("2023-03-30T12:02:15Z [INFO] Deploying to DigitalOcean droplet")
		fmt.Println("2023-03-30T12:03:45Z [INFO] Application deployed successfully")
	},
}

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Login to NextDeploy",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Print("Enter your NextDeploy API key: ")
		var apiKey string
		fmt.Scanln(&apiKey)
		
		// Save API key in config
		viper.Set("api_key", apiKey)
		viper.WriteConfig()
		
		fmt.Println("Login successful!")
	},
}

func getCurrentDir() string {
	dir, err := os.Getwd()
	if err != nil {
		fmt.Printf("Error getting current directory: %v\n", err)
		return ""
	}
	return dir
}

func loadConfig() (Config, error) {
	var config Config
	
	configData, err := os.ReadFile("nextdeploy.yaml")
	if err != nil {
		return config, fmt.Errorf("could not read config file: %w", err)
	}
	
	err = yaml.Unmarshal(configData, &config)
	if err != nil {
		return config, fmt.Errorf("could not parse config file: %w", err)
	}
	
	return config, nil
}

func init() {
	cobra.OnInitialize(initConfig)
	
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(deployCmd)
	rootCmd.AddCommand(logsCmd)
	rootCmd.AddCommand(loginCmd)
}

func initConfig() {
	// Find home directory
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	
	// Search config in home directory with name ".nextdeploy"
	viper.AddConfigPath(home)
	viper.SetConfigName(".nextdeploy")
	viper.SetConfigType("yaml")
	
	viper.AutomaticEnv()
	
	// If a config file is found, read it in
	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	} else {
		// Create config file if it doesn't exist
		err = viper.SafeWriteConfig()
		if err != nil {
			fmt.Println("Error creating config file:", err)
		}
	}
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}


package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/digitalocean/godo"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/golang-jwt/jwt"
	"github.com/joho/godotenv"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// Models
type User struct {
	gorm.Model
	Email        string `json:"email" gorm:"unique"`
	PasswordHash string `json:"-"`
	APIKey       string `json:"api_key" gorm:"unique"`
	Projects     []Project
}

type Project struct {
	gorm.Model
	UserID         uint   `json:"user_id"`
	Name           string `json:"name"`
	RepositoryURL  string `json:"repository_url"`
	Branch         string `json:"branch" gorm:"default:main"`
	Environment    string `json:"environment" gorm:"default:production"`
	BuildCommand   string `json:"build_command" gorm:"default:npm run build"`
	StartCommand   string `json:"start_command" gorm:"default:npm start"`
	Deployments    []Deployment
	DatabaseConfig DatabaseConfig
}

type DatabaseConfig struct {
	gorm.Model
	ProjectID uint   `json:"project_id"`
	Type      string `json:"type" gorm:"default:postgres"`
	Version   string `json:"version" gorm:"default:14"`
	Name      string `json:"name"`
	Username  string `json:"username"`
	Password  string `json:"-"`
}

type Deployment struct {
	gorm.Model
	ProjectID       uint      `json:"project_id"`
	Status          string    `json:"status" gorm:"default:pending"`
	DockerImage     string    `json:"docker_image"`
	DeployedAt      time.Time `json:"deployed_at"`
	DropletID       int       `json:"droplet_id"`
	DatabaseID      int       `json:"database_id"`
	DeploymentLogs  string    `json:"deployment_logs" gorm:"type:text"`
	PublicIPAddress string    `json:"public_ip_address"`
}

// Request/Response Types
type UserCreateRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Token  string `json:"token"`
	APIKey string `json:"api_key"`
	UserID uint   `json:"user_id"`
}

type ProjectCreateRequest struct {
	Name          string `json:"name"`
	RepositoryURL string `json:"repository_url"`
	Branch        string `json:"branch"`
	Environment   string `json:"environment"`
	BuildCommand  string `json:"build_command"`
	StartCommand  string `json:"start_command"`
	DatabaseType  string `json:"database_type"`
	DatabaseName  string `json:"database_name"`
}

type DeploymentCreateRequest struct {
	ProjectID uint `json:"project_id"`
}

// JWT Claims
type JWTClaims struct {
	UserID uint `json:"user_id"`
	jwt.StandardClaims
}

// DigitalOcean Client
var doClient *godo.Client

// Database instance
var db *gorm.DB

// Initialize database connection
func initDB() {
	var err error
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable TimeZone=UTC",
		os.Getenv("DB_HOST"),
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_NAME"),
		os.Getenv("DB_PORT"),
	)
	
	db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	
	// Auto migrate schemas
	db.AutoMigrate(&User{}, &Project{}, &Deployment{}, &DatabaseConfig{})
	
	log.Println("Database connection established")
}

// Initialize DigitalOcean client
func initDigitalOceanClient() {
	doToken := os.Getenv("DO_API_TOKEN")
	if doToken == "" {
		log.Fatal("DO_API_TOKEN environment variable is required")
	}
	
	doClient = godo.NewFromToken(doToken)
	
	// Test the client
	ctx := context.TODO()
	_, _, err := doClient.Account.Get(ctx)
	if err != nil {
		log.Fatalf("Failed to connect to DigitalOcean API: %v", err)
	}
	
	log.Println("DigitalOcean client initialized")
}

// Middleware for JWT auth
func jwtAuth(c *fiber.Ctx) error {
	token := c.Get("Authorization")
	if token == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Missing authorization token",
		})
	}
	
	// Remove 'Bearer ' prefix if present
	if len(token) > 7 && token[:7] == "Bearer " {
		token = token[7:]
	}
	
	// Parse and validate token
	claims := &JWTClaims{}
	parsedToken, err := jwt.ParseWithClaims(token, claims, func(token *jwt.Token) (interface{}, error) {
		return []byte(os.Getenv("JWT_SECRET")), nil
	})
	
	if err != nil || !parsedToken.Valid {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Invalid or expired token",
		})
	}
	
	// Set user ID in context
	c.Locals("userID", claims.UserID)
	
	return c.Next()
}

// API Key Auth middleware
func apiKeyAuth(c *fiber.Ctx) error {
	apiKey := c.Get("X-API-Key")
	if apiKey == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Missing API key",
		})
	}
	
	// Find user by API key
	var user User
	result := db.Where("api_key = ?", apiKey).First(&user)
	if result.Error != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Invalid API key",
		})
	}
	
	// Set user ID in context
	c.Locals("userID", user.ID)
	
	return c.Next()
}

// Generate JWT token
func generateToken(userID uint) (string, error) {
	claims := JWTClaims{
		UserID: userID,
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: time.Now().Add(time.Hour * 24 * 7).Unix(), // 7 days
			IssuedAt:  time.Now().Unix(),
		},
	}
	
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(os.Getenv("JWT_SECRET")))
}

// Generate random API key
func generateAPIKey() string {
	// Implementation would generate a random string
	// For simplicity, using a timestamp-based string here
	return fmt.Sprintf("nd_%d", time.Now().UnixNano())
}

// Main API setup
func setupAPI() *fiber.App {
	app := fiber.New()
	
	// Middleware
	app.Use(logger.New())
	app.Use(cors.New())
	
	// Public routes
	app.Post("/api/register", handleRegister)
	app.Post("/api/login", handleLogin)
	
	// Protected routes - Web UI
	api := app.Group("/api", jwtAuth)
	api.Get("/projects", handleGetProjects)
	api.Post("/projects", handleCreateProject)
	api.Get("/projects/:id", handleGetProject)
	api.Post("/projects/:id/deploy", handleDeployProject)
	api.Get("/deployments/:id", handleGetDeployment)
	api.Get("/deployments/:id/logs", handleGetDeploymentLogs)
	
	// Protected routes - CLI
	cli := app.Group("/cli", apiKeyAuth)
	cli.Post("/deploy", handleCLIDeploy)
	cli.Get("/logs/:projectId", handleCLILogs)
	cli.Get("/status/:projectId", handleCLIStatus)
	
	// Webhook route for GitHub/GitLab
	app.Post("/webhook/:projectId", handleWebhook)
	
	return app
}

// Handler for user registration
func handleRegister(c *fiber.Ctx) error {
	var req UserCreateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}
	
	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to hash password",
		})
	}
	
	// Create user
	user := User{
		Email:        req.Email,
		PasswordHash: string(hashedPassword),
		APIKey:       generateAPIKey(),
	}
	
	result := db.Create(&user)
	if result.Error != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to create user, email might already be taken",
		})
	}
	
	// Generate token
	token, err := generateToken(user.ID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to generate token",
		})
	}
	
	return c.Status(fiber.StatusCreated).JSON(LoginResponse{
		Token:  token,
		APIKey: user.APIKey,
		UserID: user.ID,
	})
}

// Handler for user login
func handleLogin(c *fiber.Ctx) error {
	var req LoginRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}
	
	// Find user by email
	var user User
	result := db.Where("email = ?", req.Email).First(&user)
	if result.Error != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Invalid email or password",
		})
	}
	
	// Check password
	err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password))
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Invalid email or password",
		})
	}
	
	// Generate token
	token, err := generateToken(user.ID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to generate token",
		})
	}
	
	return c.JSON(LoginResponse{
		Token:  token,
		APIKey: user.APIKey,
		UserID: user.ID,
	})
}

// Handler to get user's projects
func handleGetProjects(c *fiber.Ctx) error {
	userID := c.Locals("userID").(uint)
	
	var projects []Project
	result := db.Where("user_id = ?", userID).Find(&projects)
	if result.Error != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch projects",
		})
	}
	
	return c.JSON(projects)
}

// Handler to create a new project
func handleCreateProject(c *fiber.Ctx) error {
	userID := c.Locals("userID").(uint)
	
	var req ProjectCreateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}
	
	// Create project
	project := Project{
		UserID:        userID,
		Name:          req.Name,
		RepositoryURL: req.RepositoryURL,
		Branch:        req.Branch,
		Environment:   req.Environment,
		BuildCommand:  req.BuildCommand,
		StartCommand:  req.StartCommand,
	}
	
	tx := db.Begin()
	
	if err := tx.Create(&project).Error; err != nil {
		tx.Rollback()
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to create project",
		})
	}
	
	// Create database config
	dbConfig := DatabaseConfig{
		ProjectID: projee  ct.ID,
		Type:      req.DatabaseType,
		Name:      req.DatabaseName,
		Username:  "nextdeploy_" + req.Name,
		Password:  generateRandomPassword(),
	}
	
	if err := tx.Create(&dbConfig).Error; err != nil {
		tx.Rollback()
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to create database config",
		})
	}
	
	tx.Commit()
	
	return c.Status(fiber.StatusCreated).JSON(project)
}

// Generate random password for database
func generateRandomPassword() string {
	// Implementation would generate a secure random password
	// For simplicity, using a timestamp-based string here
	return fmt.Sprintf("pw_%d", time.Now().UnixNano())
}

// Handler to get a specific project
func handleGetProject(c *fiber.Ctx) error {
	userID := c.Locals("userID").(uint)
	projectID := c.Params("id")
	
	var project Project
	result := db.Where("id = ? AND user_id = ?", projectID, userID).Preload("Deployments").First(&project)
	if result.Error != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Project not found",
		})
	}
	
	// Also load database config
	var dbConfig DatabaseConfig
	db.Where("project_id = ?", project.ID).First(&dbConfig)
	project.DatabaseConfig = dbConfig
	
	return c.JSON(project)
}

// Handler to deploy a project
func handleDeployProject(c *fiber.Ctx) error {
	userID := c.Locals("userID").(uint)
	projectID := c.Params("id")
	
	// Verify project belongs to user
	var project Project
	result := db.Where("id = ? AND user_id = ?", projectID, userID).First(&project)
	if result.Error != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Project not found",
		})
	}
	
	// Create deployment record
	deployment := Deployment{
		ProjectID:  project.ID,
		Status:     "pending",
		DeployedAt: time.Now(),
	}
	
	result = db.Create(&deployment)
	if result.Error != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to create deployment",
		})
	}
	
	// Start deployment process in background
	go deployProject(project, deployment)
	
	return c.Status(fiber.StatusAccepted).JSON(deployment)
}

// Function to handle the actual deployment process
func deployProject(project Project, deployment Deployment) {
	// Update deployment status
	updateDeploymentStatus(&deployment, "building", "Starting build process...")
	
	// 1. Create droplet
	dropletID, err := createDroplet(project)
	if err != nil {
		updateDeploymentStatus(&deployment, "failed", fmt.Sprintf("Failed to create droplet: %v", err))
		return
	}
	
	deployment.DropletID = dropletID
	db.Save(&deployment)
	
	// 2. Create database
	dbID, dbConnectionString, err := createDatabase(project.DatabaseConfig)
	if err != nil {
		updateDeploymentStatus(&deployment, "failed", fmt.Sprintf("Failed to create database: %v", err))
		return
	}
	
	deployment.DatabaseID = dbID
	db.Save(&deployment)
	
	// 3. Get droplet IP
	ip, err := getDropletIP(dropletID)
	if err != nil {
		updateDeploymentStatus(&deployment, "failed", fmt.Sprintf("Failed to get droplet IP: %v", err))
		return
	}
	
	deployment.PublicIPAddress = ip
	db.Save(&deployment)
	
	// 4. SSH into droplet and set up environment
	updateDeploymentStatus(&deployment, "configuring", "Configuring server environment...")
	err = configureDroplet(deployment.PublicIPAddress, project, dbConnectionString)
	if err != nil {
		updateDeploymentStatus(&deployment, "failed", fmt.Sprintf("Failed to configure droplet: %v", err))
		return
	}
	
	// 5. Clone repository and build
	updateDeploymentStatus(&deployment, "building", "Cloning repository and building application...")
	err = buildApplication(deployment.PublicIPAddress, project)
	if err != nil {
		updateDeploymentStatus(&deployment, "failed", fmt.Sprintf("Failed to build application: %v", err))
		return
	}
	
	// 6. Start application
	updateDeploymentStatus(&deployment, "starting", "Starting the application...")
	err = startApplication(deployment.PublicIPAddress, project)
	if err != nil {
		updateDeploymentStatus(&deployment, "failed", fmt.Sprintf("Failed to start application: %v", err))
		return
	}
	
	// 7. Final setup
	updateDeploymentStatus(&deployment, "configuring_nginx", "Configuring Nginx...")
	err = configureNginx(deployment.PublicIPAddress, project)
	if err != nil {
		updateDeploymentStatus(&deployment, "failed", fmt.Sprintf("Failed to configure Nginx: %v", err))
		return
	}
	
	// Update deployment as successful
	updateDeploymentStatus(&deployment, "deployed", "Application deployed successfully!")
}

// Helper to update deployment status and logs
func updateDeploymentStatus(deployment *Deployment, status string, logMessage string) {
	deployment.Status = status
	deployment.DeploymentLogs += fmt.Sprintf("[%s] %s: %s\n", time.Now().Format(time.RFC3339), status, logMessage)
	db.Save(deployment)
}

// Create a droplet on DigitalOcean
func createDroplet(project Project) (int, error) {
	ctx := context.TODO()
	
	// Create droplet
	createRequest := &godo.DropletCreateRequest{
		Name:   fmt.Sprintf("nextdeploy-%s-%d", project.Name, time.Now().Unix()),
		Region: "nyc1", // Default region
		Size:   "s-1vcpu-1gb", // Default size
		Image: godo.DropletCreateImage{
			Slug: "ubuntu-20-04-x64",
		},
		SSHKeys: []godo.DropletCreateSSHKey{
			{Fingerprint: os.Getenv("DO_SSH_FINGERPRINT")},
		},
		Tags: []string{"nextdeploy", fmt.Sprintf("project:%d", project.ID)},
	}
	
	droplet, _, err := doClient.Droplets.Create(ctx, createRequest)
	if err != nil {
		return 0, err
	}
	
	// Wait for droplet to be active
	for {
		d, _, err := doClient.Droplets.Get(ctx, droplet.ID)
		if err != nil {
			return 0, err
		}
		
		if d.Status == "active" {
			break
		}
		
		time.Sleep(5 * time.Second)
	}
	
	return droplet.ID, nil
}

// Create a database
