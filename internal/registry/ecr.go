package registry

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"nextdeploy/internal/config"
	"nextdeploy/internal/git"
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
	ECRRegion   string
	AccessKey   string
	SecretKey   string
	Region      string
}

func NewECRContext(cfgFromEnv bool) (*ECRContext, error) {
	// Load from env or file manually
	var accessKey, secretKey string

	if cfgFromEnv {
		accessKey = strings.TrimSpace(os.Getenv("AWS_ACCESS_KEY_ID_FOR_ECR"))
		secretKey = strings.TrimSpace(os.Getenv("AWS_SECRET_ACCESS_KEY_FOR_ECR"))
	}

	if accessKey == "" || secretKey == "" {
		ECRLogger.Info("ECR credentials not found in environment variables, trying to read from .env file")
		envContent, err := os.ReadFile(".env")
		if os.IsNotExist(err) {
			return nil, errors.New(".env file not found and AWS credentials not set in environment variables")
		}
		if err != nil {
			return nil, fmt.Errorf("missing credentials in env and failed to read .env: %w", err)
		}

		lines := strings.Split(string(envContent), "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "AWS_ACCESS_KEY_ID_FOR_ECR=") {
				accessKey = strings.TrimSpace(strings.SplitN(line, "=", 2)[1])
			}
			if strings.HasPrefix(line, "AWS_SECRET_ACCESS_KEY_FOR_ECR=") {
				secretKey = strings.TrimSpace(strings.SplitN(line, "=", 2)[1])
			}
		}
	}

	if accessKey == "" || secretKey == "" {
		return nil, errors.New("ECR AWS credentials not found")
	}

	region := os.Getenv("ECR_REGION")
	repo := os.Getenv("ECR_REPO")
	if region == "" || repo == "" {
		return nil, errors.New("ECR_REGION or ECR_REPO missing")
	}

	return &ECRContext{
		Region:      region,
		ECRRepoName: repo,
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
	_, err = ecrClient.DescribeRepositories(context.TODO(), &ecr.DescribeRepositoriesInput{
		RepositoryNames: []string{e.ECRRepoName},
	})
	if err != nil {
		return fmt.Errorf("failed to describe ECR repository, no such repo exits: %w", err)
	}

	ECRLogger.Info("ECR repository %s already exists", e.ECRRepoName)
	return nil
}

func (e *ECRContext) AwsConfig() (aws.Config, error) {
	if e.Region == "" {
		return aws.Config{}, errors.New("ECR region is not set")
	}

	cfg, err := awsConfig.LoadDefaultConfig(context.TODO(),
		awsConfig.WithRegion(e.Region),
		awsConfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(e.AccessKey, e.SecretKey, "")),
	)
	if err != nil {
		return aws.Config{}, fmt.Errorf("unable to load AWS config: %w", err)
	}
	return cfg, nil
}

func (e *ECRContext) DockerLogin() error {
	cfg, err := e.AwsConfig()
	if err != nil {
		return fmt.Errorf("failed to load aws config: %w", err)
	}

	ecrClient := ecr.NewFromConfig(cfg)
	resp, err := ecrClient.GetAuthorizationToken(context.TODO(), &ecr.GetAuthorizationTokenInput{})
	if err != nil {
		return fmt.Errorf("failed to get ECR authorization token: %w", err)
	}

	if len(resp.AuthorizationData) == 0 {
		return errors.New("no authorization data returned from ECR")
	}

	token := resp.AuthorizationData[0].AuthorizationToken
	if token == nil {
		return errors.New("authorization token is nil")
	}

	endpoint := *resp.AuthorizationData[0].ProxyEndpoint
	decoded, err := base64.StdEncoding.DecodeString(*token)
	if err != nil {
		return fmt.Errorf("failed to decode ECR authorization token: %w", err)
	}

	parts := strings.SplitN(string(decoded), ":", 2)
	if len(parts) != 2 {
		return errors.New("invalid ECR authorization token format")
	}

	cmd := exec.Command("docker", "login", "-u", parts[0], "-p", parts[1], endpoint)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to run docker login command: %w", err)
	}

	ECRLogger.Success("Docker login to ECR repository %s successful", e.ECRRepoName)
	return nil
}

func PrepareECRPushContext(ctx context.Context) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ecrCtx := ECRContext{
		ECRRepoName: cfg.Docker.Image,
		ECRRegion:   cfg.Docker.RegistryRegion,
		AccessKey:   os.Getenv("AWS_ACCESS_KEY_ID_FOR_ECR"),
		SecretKey:   os.Getenv("AWS_SECRET_ACCESS_KEY_FOR_ECR"),
		Region:      cfg.Docker.RegistryRegion,
	}

	ECRLogger.Info("Preparing ECR context for account %s in region %s", ecrCtx.ECRRepoName, ecrCtx.ECRRegion)

	// First try to get credentials from environment variables
	accessKey := strings.TrimSpace(os.Getenv("AWS_ACCESS_KEY_ID_FOR_ECR"))
	secretKey := strings.TrimSpace(os.Getenv("AWS_SECRET_ACCESS_KEY_FOR_ECR"))

	// If not found in env vars, try reading from .env file
	if accessKey == "" || secretKey == "" {
		envS, err := os.ReadFile(".env")
		if err != nil {
			return err
		}

		lines := strings.Split(string(envS), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "AWS_ACCESS_KEY_ID_FOR_ECR=") {
				parts := strings.SplitN(line, "=", 2)
				if len(parts) == 2 {
					accessKey = strings.TrimSpace(parts[1])
				}
			} else if strings.HasPrefix(line, "AWS_SECRET_ACCESS_KEY_FOR_ECR=") {
				parts := strings.SplitN(line, "=", 2)
				if len(parts) == 2 {
					secretKey = strings.TrimSpace(parts[1])
				}
			}
		}
	}

	if accessKey == "" || secretKey == "" {
		return errors.New("ecr AWS credentials not found in environment variables or .env file")
	}

	// Create an ECR client
	awsCfg, err := ecrCtx.AwsConfig()
	ECRLogger.Debug("AWS Config: %v", awsCfg)

	client := ecr.NewFromConfig(awsCfg)
	ECRLogger.Debug("ECR Client created successfully")

	// Check if repository exists
	_, err = client.DescribeRepositories(ctx, &ecr.DescribeRepositoriesInput{
		RepositoryNames: []string{ecrCtx.ECRRepoName},
	})

	if err != nil {
		if err != nil && strings.Contains(err.Error(), "RepositoryNotFoundException") {
			ECRLogger.Info("Repository does not exist, please use provision flag -p to create it")
			// FIX: err in logic
			var provisionEcrRepo bool = false
			if provisionEcrRepo {
				ECRLogger.Info("Creating ECR repository %s in region %s", ecrCtx.ECRRepoName, ecrCtx.ECRRegion)
				_, err = client.CreateRepository(ctx, &ecr.CreateRepositoryInput{
					RepositoryName: aws.String(ecrCtx.ECRRepoName),
				})
				if err != nil {
					return fmt.Errorf("failed to create ECR repository: %w", err)
				}
				ECRLogger.Success("ECR repository %s created successfully", ecrCtx.ECRRepoName)
			} else {
				ECRLogger.Error("ECR repository %s does not exist. Use -p flag to create it.", ecrCtx.ECRRepoName)
				return fmt.Errorf("ECR repository %s does not exist", ecrCtx.ECRRepoName)
			}
		}
	}

	// perform docker login
	err = ecrCtx.DockerLogin()
	if err != nil {
		return err
	}

	ECRLogger.Success("Docker login to ECR repository %s successful", ecrCtx.ECRRepoName)
	return nil
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

const (
	policyDocument = `{	
  	"Version": "2012-10-17",
		"Statement": [
		{
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
		}
		]
	}`
)

func CheckUserExists() (bool, error) {
	ECRLogger.Info("Checking if ECR user exists")
	cfg, err := config.Load()
	if err != nil {
		return false, err
	}

	awsCfg, err := awsConfig.LoadDefaultConfig(context.TODO(),
		awsConfig.WithRegion(cfg.Docker.RegistryRegion),
	)
	if err != nil {
		return false, err
	}

	iamClient := iam.NewFromConfig(awsCfg)
	userName := cfg.App.Domain

	_, err = iamClient.GetUser(context.TODO(), &iam.GetUserInput{
		UserName: aws.String(userName),
	})
	if err != nil {
		if strings.Contains(err.Error(), "NoSuchEntity") {
			return false, nil // User does not exist
		}
		return false, fmt.Errorf("failed to check if user exists: %w", err)
	}

	return true, nil // User exists
}
func DeleteECRUserAndPolicy() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ECRLogger.Info("Deleting ECR user and policy")

	awsCfg, err := awsConfig.LoadDefaultConfig(context.TODO(),
		awsConfig.WithRegion(cfg.Docker.RegistryRegion))
	if err != nil {
		return err
	}

	iamClient := iam.NewFromConfig(awsCfg)
	user := cfg.App.Domain

	err = cleanupUser(iamClient, user)
	if err != nil {
		return err
	}

	ECRLogger.Success("ECR user and policy deleted successfully")
	return nil
}

func cleanupUser(iamClient *iam.Client, userName string) error {
	if err := deleteAccessKeys(iamClient, userName); err != nil {
		return err
	}

	if err := detachManagedPolicies(iamClient, userName); err != nil {
		return err
	}

	if err := deleteInlinePolicies(iamClient, userName); err != nil {
		return err
	}

	if err := removeFromGroups(iamClient, userName); err != nil {
		return err
	}

	_, err := iamClient.DeleteUser(context.TODO(), &iam.DeleteUserInput{
		UserName: aws.String(userName),
	})
	if err != nil {
		ECRLogger.Warn("User %s does not exist, skipping deletion", userName)
	}

	return nil
}

func deleteAccessKeys(iamClient *iam.Client, userName string) error {
	output, err := iamClient.ListAccessKeys(context.TODO(), &iam.ListAccessKeysInput{
		UserName: aws.String(userName),
	})
	if err != nil {
		return err
	}

	for _, key := range output.AccessKeyMetadata {
		_, err := iamClient.DeleteAccessKey(context.TODO(), &iam.DeleteAccessKeyInput{
			UserName:    aws.String(userName),
			AccessKeyId: key.AccessKeyId,
		})
		if err != nil {
			return err
		}
		ECRLogger.Info("Deleted access key %s for user %s", *key.AccessKeyId, userName)
	}
	return nil
}

func detachManagedPolicies(iamClient *iam.Client, userName string) error {
	output, err := iamClient.ListAttachedUserPolicies(context.TODO(), &iam.ListAttachedUserPoliciesInput{
		UserName: aws.String(userName),
	})
	if err != nil {
		return err
	}

	for _, policy := range output.AttachedPolicies {
		_, err := iamClient.DetachUserPolicy(context.TODO(), &iam.DetachUserPolicyInput{
			UserName:  aws.String(userName),
			PolicyArn: policy.PolicyArn,
		})
		if err != nil {
			return err
		}
		ECRLogger.Info("Detached policy %s from user %s", *policy.PolicyName, userName)
	}
	return nil
}

func deleteInlinePolicies(iamClient *iam.Client, userName string) error {
	output, err := iamClient.ListUserPolicies(context.TODO(), &iam.ListUserPoliciesInput{
		UserName: aws.String(userName),
	})
	if err != nil {
		return err
	}

	for _, policyName := range output.PolicyNames {
		_, err := iamClient.DeleteUserPolicy(context.TODO(), &iam.DeleteUserPolicyInput{
			UserName:   aws.String(userName),
			PolicyName: aws.String(policyName),
		})
		if err != nil {
			return err
		}
		ECRLogger.Info("Deleted inline policy %s for user %s", policyName, userName)
	}
	return nil
}

func removeFromGroups(iamClient *iam.Client, userName string) error {
	output, err := iamClient.ListGroupsForUser(context.TODO(), &iam.ListGroupsForUserInput{
		UserName: aws.String(userName),
	})
	if err != nil {
		return err
	}

	for _, group := range output.Groups {
		_, err := iamClient.RemoveUserFromGroup(context.TODO(), &iam.RemoveUserFromGroupInput{
			GroupName: aws.String(*group.GroupName),
			UserName:  aws.String(userName),
		})
		if err != nil {
			return err
		}
		ECRLogger.Info("Removed user %s from group %s", userName, *group.GroupName)
	}
	return nil
}

func deleteLoginProfile(iamClient *iam.Client, userName string) error {
	_, err := iamClient.DeleteLoginProfile(context.TODO(), &iam.DeleteLoginProfileInput{
		UserName: aws.String(userName),
	})
	if err != nil {
		ECRLogger.Warn("Login profile for user %s does not exist, skipping deletion", userName)
	}

	ECRLogger.Info("Deleted login profile for user %s", userName)
	return nil
}

func CreateECRUserAndPolicy() (*types.User, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}

	awsCfg, err := awsConfig.LoadDefaultConfig(context.TODO(),
		awsConfig.WithRegion(cfg.Docker.RegistryRegion),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	iamClient := iam.NewFromConfig(awsCfg)
	name := cfg.App.Domain

	user, err := createUser(iamClient, name)
	if err != nil {
		return nil, fmt.Errorf("failed to create IAM user for ECR access: %w", err)
	}

	policyArn, err := createPolicy(iamClient, name+"-ecr-policy", policyDocument)
	if err != nil {
		return nil, fmt.Errorf("failed to create IAM policy for ECR access: %w", err)
	}

	err = attachPolicyToUser(iamClient, policyArn, name, name+"-ecr-policy")
	if err != nil {
		return nil, fmt.Errorf("failed to attach policy %s to user %s: %w", name+"-ecr-policy", name, err)
	}

	ECRLogger.Success("ECR user and policy created successfully")

	accessKey, secretKey, err := createAccessKey(iamClient, name)
	if err != nil {
		return nil, fmt.Errorf("failed to create access key for ECR user: %w", err)
	}

	ECRLogger.Success("ECR user access key created successfully")

	err = writeCredentialsToEnvFile(accessKey, secretKey)
	if err != nil {
		return nil, fmt.Errorf("failed to write ECR credentials to .env file: %w", err)
	}

	ECRLogger.Success("ECR credentials written to env file successfully")
	return user, nil
}

func createUser(iamClient *iam.Client, userName string) (*types.User, error) {
	output, err := iamClient.CreateUser(context.TODO(), &iam.CreateUserInput{
		UserName: aws.String(userName),
	})
	if err != nil {
		return nil, err
	}

	ECRLogger.Debug("User created looks like this: %v", output.User)
	return output.User, nil
}

func createPolicy(iamClient *iam.Client, policyName string, policyDocument string) (string, error) {
	var maxPolicies int32 = 1000
	listPoliciesOutput, err := iamClient.ListPolicies(context.TODO(), &iam.ListPoliciesInput{
		MaxItems: aws.Int32(maxPolicies),
	})
	if err != nil {
		return "", fmt.Errorf("failed to list policies: %w", err)
	}

	for _, policy := range listPoliciesOutput.Policies {
		if aws.ToString(policy.PolicyName) == policyName {
			ECRLogger.Info("Policy %s already exists, reusing ARN: %s", policyName, aws.ToString(policy.Arn))
			return aws.ToString(policy.Arn), nil
		}
	}

	policyOutput, err := iamClient.CreatePolicy(context.TODO(), &iam.CreatePolicyInput{
		PolicyName:     aws.String(policyName),
		PolicyDocument: aws.String(policyDocument),
	})
	if err != nil {
		ECRLogger.Info("Policy %s created by another process, looking up ARN", policyName)
		return getPolicyARN(iamClient, policyName)
	}

	ECRLogger.Info("Created new policy %s", policyName)
	return aws.ToString(policyOutput.Policy.Arn), nil
}

func getPolicyARN(iamClient *iam.Client, policyName string) (string, error) {
	var maxPolicies int32 = 1000
	listPoliciesOutput, err := iamClient.ListPolicies(context.TODO(), &iam.ListPoliciesInput{
		MaxItems: aws.Int32(maxPolicies),
	})
	if err != nil {
		return "", fmt.Errorf("failed to list policies: %w", err)
	}

	for _, policy := range listPoliciesOutput.Policies {
		if aws.ToString(policy.PolicyName) == policyName {
			return aws.ToString(policy.Arn), nil
		}
	}

	return "", fmt.Errorf("policy %s not found after creation conflict", policyName)
}

func attachPolicyToUser(iamClient *iam.Client, policyArn string, userName string, policyName string) error {
	_, err := iamClient.AttachUserPolicy(context.TODO(), &iam.AttachUserPolicyInput{
		UserName:  aws.String(userName),
		PolicyArn: aws.String(policyArn),
	})
	if err != nil {
		return err
	}

	ECRLogger.Info("Attached policy %s to user %s for ECR access", policyName, userName)
	return nil
}

func createAccessKey(iamClient *iam.Client, userName string) (string, string, error) {
	output, err := iamClient.CreateAccessKey(context.TODO(), &iam.CreateAccessKeyInput{
		UserName: aws.String(userName),
	})
	if err != nil {
		if strings.Contains(err.Error(), "LimitExceeded") {
			ECRLogger.Warn("Access key limit exceeded for user %s, deleting existing keys", userName)
			if err := deleteAccessKeys(iamClient, userName); err != nil {
				return "", "", fmt.Errorf("failed to delete existing access keys: %w", err)
			}
			output, err = iamClient.CreateAccessKey(context.TODO(), &iam.CreateAccessKeyInput{
				UserName: aws.String(userName),
			})
		}
		if err != nil {
			return "", "", fmt.Errorf("failed to create access key for user %s: %w", userName, err)
		}
	}

	ECRLogger.Info("Created access key for user %s", userName)
	return aws.ToString(output.AccessKey.AccessKeyId), aws.ToString(output.AccessKey.SecretAccessKey), nil
}

func VerifyECRAccess(region, accessKey, secretKey, sessionToken string) error {
	cfg, err := awsConfig.LoadDefaultConfig(context.TODO(),
		awsConfig.WithRegion(region),
		awsConfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKey, secretKey, sessionToken)),
	)
	if err != nil {
		return fmt.Errorf("unable to load AWS config: %w", err)
	}

	ecrClient := ecr.NewFromConfig(cfg)
	_, err = ecrClient.DescribeRepositories(context.TODO(), &ecr.DescribeRepositoriesInput{
		MaxResults: aws.Int32(1),
	})
	if err != nil {
		if strings.Contains(err.Error(), "AccessDeniedException") {
			return fmt.Errorf("access denied to ECR in region %s with provided credentials", region)
		}
		return fmt.Errorf("failed to describe ECR repositories: %w", err)
	}

	return nil
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
			if strings.HasPrefix(line, "AWS_ACCESS_KEY_ID_FOR_ECR=") ||
				strings.HasPrefix(line, "AWS_SECRET_ACCESS_KEY_FOR_ECR=") {
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

	_, err = file.WriteString("AWS_ACCESS_KEY_ID_FOR_ECR=" + accessKey + "\n")
	if err != nil {
		return fmt.Errorf("failed to write AWS_ACCESS_KEY_ID to env file: %w", err)
	}

	_, err = file.WriteString("AWS_SECRET_ACCESS_KEY_FOR_ECR=" + secretKey + "\n")
	if err != nil {
		return fmt.Errorf("failed to write AWS_SECRET_ACCESS_KEY to env file: %w", err)
	}

	ECRLogger.Info("Wrote AWS credentials to %s", envFile)
	return nil
}

func GetShortECRToken(region string, accessKey string, secretKey string, repoName string) (string, error) {
	cfg, err := awsConfig.LoadDefaultConfig(context.TODO(),
		awsConfig.WithRegion(region),
		awsConfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKey, secretKey, "")),
	)
	if err != nil {
		return "", fmt.Errorf("unable to load AWS config: %w", err)
	}

	ecrClient := ecr.NewFromConfig(cfg)
	output, err := ecrClient.GetAuthorizationToken(context.TODO(), &ecr.GetAuthorizationTokenInput{})
	if err != nil {
		ECRLogger.Error("Error getting authorization token: %v", err)
		return "", fmt.Errorf("failed to get ECR authorization token: %w", err)
	}

	token := aws.ToString(output.AuthorizationData[0].AuthorizationToken)
	decodedToken := strings.TrimPrefix(token, "AWS:")

	cmd := exec.Command("docker", "login", "--username", "AWS", "--password-stdin", repoName)
	cmd.Stdin = strings.NewReader(decodedToken)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("docker login failed: %w", err)
	}

	ECRLogger.Info("Successfully logged in to ECR repository %s", repoName)
	return decodedToken, nil
}
