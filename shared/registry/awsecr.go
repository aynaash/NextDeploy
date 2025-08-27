package registry

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"nextdeploy/shared"
	"nextdeploy/shared/config"
	"nextdeploy/shared/envstore"
	"nextdeploy/shared/git"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/ecr"
)

var (
	ECRLogger = shared.PackageLogger("ECR", "🅰️ ECR")
)

type ECRContext struct {
	ECRRepoName string
	ECRRegion   string
	AccessKey   string
	SecretKey   string
	Region      string
}

func NewECRContext() (*ECRContext, error) {
	cfg, err := config.Load()
	if err != nil {
		ECRLogger.Error("Failed to load configuration: %v", err)
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}

	store, err := envstore.New(
		envstore.WithEnvFile[string](".env"),
	)
	if err != nil {
		ECRLogger.Error("Failed to create env store: %v", err)
		return nil, fmt.Errorf("failed to create env store: %w", err)
	}

	accessKey, err := store.GetEnv("AWS_ACCESS_KEY_ID")
	if err != nil {
		ECRLogger.Error("Failed to get AWS_ACCESS_KEY_ID from env store: %v", err)
		return nil, fmt.Errorf("failed to get AWS_ACCESS_KEY_ID from env store: %w", err)
	}
	secretKey, err := store.GetEnv("AWS_SECRET_ACCESS_KEY")
	if err != nil {
		ECRLogger.Error("Failed to get AWS_SECRET_ACCESS_KEY from env store: %v", err)
		return nil, fmt.Errorf("failed to get AWS_SECRET_ACCESS_KEY from env store: %w", err)
	}
	if accessKey == "" || secretKey == "" {
		return nil, errors.New("AWS credentials not found in environment variables")
	}

	return &ECRContext{
		ECRRepoName: cfg.Docker.Image,
		Region:      cfg.Docker.RegistryRegion,
		AccessKey:   accessKey,
		SecretKey:   secretKey,
	}, nil
}
func (ctx ECRContext) ECRURL() string {
	cfg, err := config.Load()
	if err != nil {
		ECRLogger.Error("Failed to load configuration: %v", err)
		return ""
	}
	return cfg.Docker.Image
}

func (ctx ECRContext) FullImageName(image string) string {
	tag, err := git.GetCommitHash()
	if err != nil {
		ECRLogger.Error("Failed to get commit hash: %v", err)
		return fmt.Sprintf("%s:latest", image)
	}
	return image + ":" + tag
}

func (e *ECRContext) EnsureRepository() error {
	cfg, err := e.AwsConfig()
	if err != nil {
		ECRLogger.Error("Failed to load AWS config: %v", err)
		return fmt.Errorf("failed to load AWS config: %w", err)
	}
	configs, err := config.Load() // Replace with your profile name
	if err != nil {
		ECRLogger.Error("Failed to load configuration: %v", err)
		return fmt.Errorf("failed to load configuration: %w", err)
	}
	profile := configs.App.Domain

	identity, err := GetAWSIdentity(profile)
	if err != nil {
		ECRLogger.Error("Failed to get AWS identity: %v", err)
		return err
	}

	// Print formatted results
	fmt.Printf("✅ AWS Profile: %s\n", profile)
	fmt.Printf("   Account ID: %s\n", identity.Account)
	fmt.Printf("   User ID:    %s\n", identity.UserID)
	fmt.Printf("   ARN:        %s\n", identity.ARN)
	ecrClient := ecr.NewFromConfig(cfg)
	_, region, repoName, err := ExtractECRDetails(e.ECRRepoName)
	if err != nil {
		ECRLogger.Error("Failed to extract ECR details: %v", err)
		return fmt.Errorf("failed to extract ECR details: %w", err)
	}
	e.ECRRegion = region
	ECRLogger.Info("Ensuring ECR repository %s exists in region %s", repoName, e.ECRRegion)
	_, err = ecrClient.DescribeRepositories(context.TODO(), &ecr.DescribeRepositoriesInput{
		RepositoryNames: []string{repoName},
	})
	if err != nil && strings.Contains(err.Error(), "RepositoryNotFoundException") {
		ECRLogger.Info("ECR repository %s does not exist, creating it", e.ECRRepoName)
	}
	if err != nil {
		ECRLogger.Error("Failed to describe ECR repository: %v", err)
		return fmt.Errorf("failed to describe ECR repository, no such repo exits: %w", err)
	}

	ECRLogger.Info("ECR repository %s already exists", e.ECRRepoName)
	return nil
}

// DockerLoginWithToken performs docker login with an ECR token

func PrepareECRPushContext(ctx context.Context, createRepo bool) error {
	ecrCtx, err := NewECRContext()
	if err != nil {
		ECRLogger.Error("Failed to create ECR context: %v", err)
		return fmt.Errorf("failed to create ECR context: %w", err)
	}

	if createRepo {
		if err := ecrCtx.EnsureRepository(); err != nil {
			ECRLogger.Error("Failed to ensure ECR repository exists: %v", err)
			return fmt.Errorf("failed  to ensure repository exists: %w", err)
		}
	}

	return ecrCtx.Login()
}
func (e *ECRContext) Login() error {
	ECRLogger.Info("Logging in to ECR repository %s in region %s", e.ECRRepoName, e.Region)

	// Validate inputs first
	if e.ECRRepoName == "" || e.Region == "" {
		return fmt.Errorf("missing required ECR parameters (repo: %s, region: %s)",
			e.ECRRepoName, e.Region)
	}

	// Get ECR token with retry logic
	token, err := GetECRTokenWithRetry(e.Region, e.AccessKey, e.SecretKey, "", 3)
	if err != nil {
		ECRLogger.Error("Failed to get ECR token after retries: %v", err)
		return fmt.Errorf("failed to get ECR token: %w", err)
	}

	// Mask token in logs for security
	// Attempt Docker login with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := DockerLoginWithToken(ctx, token, e.ECRRepoName); err != nil {
		// Check for common error patterns
		if strings.Contains(err.Error(), "401 Unauthorized") {
			ECRLogger.Error("Authentication failed - check AWS permissions and credentials")
			return fmt.Errorf("ecr authentication failed: %w", err)
		}
		return fmt.Errorf("docker login failed: %w", err)
	}

	ECRLogger.Success("Successfully authenticated with ECR")
	return nil
}

func GetECRTokenWithRetry(region, accessKey, secretKey, sessionToken string, maxRetries int) (string, error) {
	var lastErr error
	for i := 0; i < maxRetries; i++ {
		token, err := GetECRToken(accessKey, secretKey, sessionToken)
		if err == nil {
			return token, nil
		}
		lastErr = err
		time.Sleep(time.Duration(i+1) * time.Second) // Exponential backoff
	}
	return "", fmt.Errorf("after %d retries: %w", maxRetries, lastErr)
}

func DockerLoginWithToken(ctx context.Context, token string, registryURL string) error {
	// Validate registry URL format
	if !strings.HasPrefix(registryURL, "http") {
		registryURL = "https://" + registryURL
	}

	cmd := exec.CommandContext(ctx, "docker", "login", "-u", "AWS", "--password-stdin", registryURL)
	cmd.Stdin = strings.NewReader(token)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		// Enhanced error analysis
		errMsg := stderr.String()
		switch {
		case strings.Contains(errMsg, "401 Unauthorized"):
			return fmt.Errorf("invalid credentials or insufficient permissions")
		case strings.Contains(errMsg, "no such host"):
			return fmt.Errorf("network error - cannot reach ECR endpoint")
		case strings.Contains(errMsg, "certificate"):
			return fmt.Errorf("TLS certificate error - check system time")
		default:
			return fmt.Errorf("docker login failed: %v\nstderr: %s", err, errMsg)
		}
	}

	return nil
}

// func PrepareECRPullContext(ctx context.Context, ecrCtx ECRContext) (string, error) {
// 	ECRLogger.Info("Preparing ECR pull context")
//
// 	loginCommand := exec.CommandContext(ctx, "aws", "ecr", "get-login-password", "--region", ecrCtx.ECRRegion)
// 	var stdout, stderr bytes.Buffer
// 	loginCommand.Stdout = &stdout
// 	loginCommand.Stderr = &stderr
//
// 	err := loginCommand.Run()
// 	if err != nil {
// 		ECRLogger.Error("Failed to get ECR login password: %v", err)
// 		return "", fmt.Errorf("failed to get ECR login password: %w\n%s", err, stderr.String())
// 	}
//
// 	return stdout.String(), nil
// }

func writeCredentialsToEnvFile(accessKey, secretKey string) error {
	ECRLogger.Info("Writing AWS credentials to .env file")
	envFile := ".env"

	existingContent := make(map[string]string)
	if _, err := os.Stat(envFile); err == nil {
		fileContent, err := os.ReadFile(envFile)
		if err != nil {
			return fmt.Errorf("failed to read existing .env file: %w", err)
		}

		for _, line := range strings.Split(string(fileContent), "\n") {
			if strings.HasPrefix(line, "AWS_ACCESS_KEY_ID=") ||
				strings.HasPrefix(line, "AWS_SECRET_ACCESS_KEY=") {
				continue
			}
			if strings.Contains(line, "=") {
				parts := strings.SplitN(line, "=", 2)
				existingContent[parts[0]] = parts[1]
			}
		}
	}

	file, err := os.Create(envFile)
	if err != nil {
		ECRLogger.Error("Failed to create or open .env file: %v", err)
		return fmt.Errorf("failed to create or open .env file: %w", err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			ECRLogger.Error("Failed to close .env file: %v", closeErr)
		}
	}()

	for key, value := range existingContent {
		_, err = file.WriteString(fmt.Sprintf("%s=%s\n", key, value))
		if err != nil {
			ECRLogger.Error("Failed to write existing content to .env file: %v", err)
			return fmt.Errorf("failed to write existing content to .env file: %w", err)
		}
	}

	_, err = file.WriteString("AWS_ACCESS_KEY_ID=" + accessKey + "\n")
	if err != nil {
		return fmt.Errorf("failed to write AWS_ACCESS_KEY_ID to env file: %w", err)
	}

	_, err = file.WriteString("AWS_SECRET_ACCESS_KEY=" + secretKey + "\n")
	if err != nil {
		return fmt.Errorf("failed to write AWS_SECRET_ACCESS_KEY to env file: %w", err)
	}

	// CREATE THE PROFILE NOW
	cfg, _ := config.Load()
	profile := cfg.App.Domain
	err = AddAWSProfile(
		profile,
		accessKey,
		secretKey,
	)
	if err != nil {
		ECRLogger.Error("Failed to add AWS profile: %v", err)
		return fmt.Errorf("failed to add AWS profile: %w", err)
	}

	ECRLogger.Info("Wrote AWS credentials to %s", envFile)
	return nil
}

func GetECRToken(accessKey, secretKey, sessionToken string) (string, error) {
	cfg, err := config.Load()
	if err != nil {
		ECRLogger.Error("Failed to load configuration: %v", err)
		return "", fmt.Errorf("failed to load configuration: %w", err)
	}
	image := cfg.Docker.Image
	_, region, repoName, err := ExtractECRDetails(image)
	if err != nil {
		ECRLogger.Error("Failed to extract ECR details: %v", err)
		return "", fmt.Errorf("failed to extract ECR details: %w", err)
	}
	ECRLogger.Debug("Extracted ECR details - Region: %s, Repository Name: %s", region, repoName)
	args := []string{
		"ecr",
		"get-login-password",
		"--region", region,
	}

	cmd := exec.Command("aws", args...)

	// Setup env
	env := os.Environ()
	env = append(env, "AWS_ACCESS_KEY_ID="+accessKey)
	env = append(env, "AWS_SECRET_ACCESS_KEY="+secretKey)
	if sessionToken != "" {
		env = append(env, "AWS_SESSION_TOKEN="+sessionToken)
	}
	cmd.Env = env

	fmt.Println("Running AWS CLI with environment:")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to get ECR token here is the issue: %v, stderr: %s", err, stderr.String())
	}
	return strings.TrimSpace(stdout.String()), nil
}
