package registry

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"nextdeploy/internal/config"
	"nextdeploy/internal/envstore"
	"nextdeploy/internal/git"
	"nextdeploy/internal/logger"
	"os"
	"os/exec"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
)

var (
	ECRLogger = logger.PackageLogger("ECR", "🅰️ ECR")
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
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}

	store, err := envstore.New(
		envstore.WithEnvFile[string](".env"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create env store: %w", err)
	}

	accessKey, err := store.GetEnv("AWS_ACCESS_KEY_ID")
	if err != nil {
		return nil, fmt.Errorf("failed to get AWS_ACCESS_KEY_ID from env store: %w", err)
	}
	ECRLogger.Debug("AWS_ACCESS_KEY_ID found: %s", accessKey)
	secretKey, err := store.GetEnv("AWS_SECRET_ACCESS_KEY")
	if err != nil {
		return nil, fmt.Errorf("failed to get AWS_SECRET_ACCESS_KEY from env store: %w", err)
	}
	ECRLogger.Debug("AWS_SECRET_ACCESS_KEY found: %s", secretKey)

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
		return ""
	}
	return cfg.Docker.Image
}

func (ctx ECRContext) FullImageName(image string) string {
	tag, err := git.GetCommitHash()
	if err != nil {
		return fmt.Sprintf("%s:latest", image)
	}
	return image + ":" + tag
}

func (e *ECRContext) EnsureRepository() error {
	cfg, err := e.AwsConfig()
	if err != nil {
		return fmt.Errorf("failed to load AWS config: %w", err)
	}

	ecrClient := ecr.NewFromConfig(cfg)
	// log out the config data first
	ECRLogger.Debug("ECR Client Config: %v", &cfg)
	_, err = ecrClient.DescribeRepositories(context.TODO(), &ecr.DescribeRepositoriesInput{
		RepositoryNames: []string{e.ECRRepoName},
		MaxResults:      aws.Int32(1),
	})
	// let us GetAuthorizationToken first

	if err != nil && strings.Contains(err.Error(), "RepositoryNotFoundException") {
		ECRLogger.Info("ECR repository %s does not exist, creating it", e.ECRRepoName)

		_, err = ecrClient.CreateRepository(context.TODO(), &ecr.CreateRepositoryInput{
			RepositoryName: aws.String(e.ECRRepoName),
		})
		if err != nil {
			return fmt.Errorf("failed to create ECR repository: %w", err)
		}

		ECRLogger.Success("ECR repository %s created successfully", e.ECRRepoName)
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to describe ECR repository, no such repo exits: %w", err)
	}

	ECRLogger.Info("ECR repository %s already exists", e.ECRRepoName)
	return nil
}

func (e *ECRContext) Login() error {
	token, err := GetECRToken(e.Region, e.AccessKey, e.SecretKey, "")
	if err != nil {
		return fmt.Errorf("failed to get ECR token: %w", err)
	}

	if err := DockerLoginWithToken(token, e.ECRRepoName); err != nil {
		return fmt.Errorf("docker login failed: %w", err)
	}

	ECRLogger.Success("Successfully authenticated with ECR")
	return nil
}
func (e *ECRContext) awsConfig() (aws.Config, error) {
	return awsConfig.LoadDefaultConfig(context.TODO(),
		awsConfig.WithRegion(e.Region),
		awsConfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			e.AccessKey,
			e.SecretKey,
			"", // No session token needed
		)),
	)
}

// DockerLoginWithToken performs docker login with an ECR token
func DockerLoginWithToken(token string, registryURL string) error {
	cmd := exec.Command("docker", "login", "-u", "aws", "--password-stdin", registryURL)
	cmd.Stdin = strings.NewReader(token)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker login failed: %v\nstderr: %s", err, stderr.String())
	}

	return nil
}

func PrepareECRPushContext(ctx context.Context, createRepo bool) error {
	ecrCtx, err := NewECRContext()
	if err != nil {
		return fmt.Errorf("failed to create ECR context: %w", err)
	}

	if createRepo {
		if err := ecrCtx.EnsureRepository(); err != nil {
			return fmt.Errorf("failed to ensure repository exists: %w", err)
		}
	}

	return ecrCtx.Login()
}
func PrepareECRPullContext(ctx context.Context, ecrCtx ECRContext) (string, error) {
	ECRLogger.Info("Preparing ECR pull context")

	loginCommand := exec.CommandContext(ctx, "aws", "ecr", "get-login-password", "--region", ecrCtx.ECRRegion)
	var stdout, stderr bytes.Buffer
	loginCommand.Stdout = &stdout
	loginCommand.Stderr = &stderr

	err := loginCommand.Run()
	if err != nil {
		return "", fmt.Errorf("failed to get ECR login password: %w\n%s", err, stderr.String())
	}

	return stdout.String(), nil
}

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
		return fmt.Errorf("failed to create or open .env file: %w", err)
	}
	defer file.Close()

	for key, value := range existingContent {
		_, err = file.WriteString(fmt.Sprintf("%s=%s\n", key, value))
		if err != nil {
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
		return fmt.Errorf("failed to add AWS profile: %w", err)
	}

	ECRLogger.Info("Wrote AWS credentials to %s", envFile)
	return nil
}

func GetECRToken(region, accessKey, secretKey, sessionToken string) (string, error) {
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
	fmt.Println("AWS_ACCESS_KEY_ID:", accessKey)
	fmt.Println("AWS_SECRET_ACCESS_KEY:", secretKey)
	fmt.Println("AWS_SESSION_TOKEN:", sessionToken)
	fmt.Println("AWS Region:", region)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to get ECR token here is the issue: %v, stderr: %s", err, stderr.String())
	}
	return strings.TrimSpace(stdout.String()), nil
}
