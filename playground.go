//go:build ignore
// +build ignore

package playground

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"nextdeploy/internal/config"
	"nextdeploy/internal/logger"
	"os"
	"os/exec"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/iam/types"
)

var (
	ECRLogger = logger.PackageLogger("ECR", "🅰️ ECR")
)

type ECRContext struct {
	ECRRepoName string
	Region      string
	AccessKey   string
	SecretKey   string
}

// NewECRContext creates a new ECR context by loading credentials from environment or .env file
func NewECRContext() (*ECRContext, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}

	accessKey := strings.TrimSpace(os.Getenv("AWS_ACCESS_KEY_ID"))
	secretKey := strings.TrimSpace(os.Getenv("AWS_SECRET_ACCESS_KEY"))

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

// FullImageName returns the full ECR image name with commit hash tag
func (ctx ECRContext) FullImageName() (string, error) {
	tag, err := git.GetCommitHash()
	if err != nil {
		return "", fmt.Errorf("failed to get git commit hash: %w", err)
	}
	return fmt.Sprintf("%s:%s", ctx.ECRRepoName, tag), nil
}

// EnsureRepository checks if the ECR repository exists and creates it if needed
func (e *ECRContext) EnsureRepository() error {
	cfg, err := e.awsConfig()
	if err != nil {
		return fmt.Errorf("failed to load AWS config: %w", err)
	}

	client := ecr.NewFromConfig(cfg)
	_, err = client.DescribeRepositories(context.TODO(), &ecr.DescribeRepositoriesInput{
		RepositoryNames: []string{e.ECRRepoName},
	})

	if err != nil {
		_, createErr := client.CreateRepository(context.TODO(), &ecr.CreateRepositoryInput{
			RepositoryName: aws.String(e.ECRRepoName),
		})
		if createErr != nil {
			return fmt.Errorf("failed to create ECR repository: %w", createErr)
		}
		ECRLogger.Success("Created ECR repository: %s", e.ECRRepoName)
	}

	return nil
}

// Login authenticates Docker with ECR
func (e *ECRContext) Login() error {
	token, err := GetECRToken(e.Region, e.AccessKey, e.SecretKey)
	if err != nil {
		return fmt.Errorf("failed to get ECR token: %w", err)
	}

	if err := DockerLoginWithToken(token, e.ECRRepoName); err != nil {
		return fmt.Errorf("docker login failed: %w", err)
	}

	ECRLogger.Success("Successfully authenticated with ECR")
	return nil
}

// awsConfig creates an AWS config with the stored credentials
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

// GetECRToken retrieves an ECR login token using AWS CLI
func GetECRToken(region, accessKey, secretKey string) (string, error) {
	cmd := exec.Command("aws", "ecr", "get-login-password", "--region", region)

	cmd.Env = []string{
		"AWS_ACCESS_KEY_ID=" + accessKey,
		"AWS_SECRET_ACCESS_KEY=" + secretKey,
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to get ECR token: %v\nstderr: %s", err, stderr.String())
	}

	return strings.TrimSpace(stdout.String()), nil
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

// PrepareECRPushContext prepares the environment for pushing to ECR
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

// IAM Management Functions (unchanged but could be moved to separate package)
const (
	policyDocument = `{
		"Version": "2012-10-17",
		"Statement": [{
			"Effect": "Allow",
			"Action": [
				"ecr:GetAuthorizationToken",
				"ecr:BatchCheckLayerAvailability",
				"ecr:GetDownloadUrlForLayer",
				"ecr:GetRepositoryPolicy",
				"ecr:DescribeRepositories",
				"ecr:ListImages",
				"ecr:DescribeImages",
				"ecr:BatchGetImage",
				"ecr:InitiateLayerUpload",
				"ecr:UploadLayerPart",
				"ecr:CompleteLayerUpload",
				"ecr:PutImage"
			],
			"Resource": "*"
		}]
	}`
)

// CreateECRUserAndPolicy creates an IAM user with ECR access (unchanged)
func CreateECRUserAndPolicy() (*types.User, error) {
	// ... existing implementation ...
}

// CheckUserExists checks if the IAM user exists (unchanged)
func CheckUserExists() (bool, error) {
	// ... existing implementation ...
}

// DeleteECRUserAndPolicy deletes the IAM user and policy (unchanged)
func DeleteECRUserAndPolicy() error {
	// ... existing implementation ...
}
