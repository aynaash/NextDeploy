package registry

import (
	"bytes"
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/session"
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"nextdeploy/internal/config"
	"nextdeploy/internal/failfast"
	"nextdeploy/internal/git"
	"nextdeploy/internal/logger"
	"os"
	"os/exec"
	"strings"
)

var (
	ECRLogger = logger.PackageLogger("ECR", "🅰️ ECR")
)

type ECRContext struct {
	ECRRepoName string
	ECRRegion   string
}

func (ctx ECRContext) ECRURL() string {
	cfg, err := config.Load()
	if err != nil {
		failfast.Failfast(err, failfast.Error, "Failed to config at ecr context preps")
	}
	return cfg.Docker.Image
}
func (ctx ECRContext) FullImageName(image string) string {
	tag, err := git.GetCommitHash()
	if err != nil {
		failfast.Failfast(err, failfast.Error, "Failed to get commit hash")
	}
	// faull image name
	fullImage := image + ":" + tag
	return fullImage
}
func PrepareECRPushContext(ctx context.Context, Ecr ECRContext) error {
	ECRLogger.Info("Preparing ECR context for account %s in region %s", Ecr.ECRRepoName, Ecr.ECRRegion)

	// First try to get credentials from environment variables
	accessKey := strings.TrimSpace(os.Getenv("AWS_ACCESS_KEY_ID_FOR_ECR"))
	secretKey := strings.TrimSpace(os.Getenv("AWS_SECRET_ACCESS_KEY_FOR_ECR"))

	// If not found in env vars, try reading from .env file
	if accessKey == "" || secretKey == "" {
		envS, err := os.ReadFile(".env")
		if err != nil {
			failfast.Failfast(err, failfast.Error, "Failed to read .env file for ECR credentials")
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
		failfast.Failfast(nil, failfast.Error, "AWS credentials for ECR are not set in environment variables or .env file")
	}

	// Create a new AWS session with the provided credentials
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(Ecr.ECRRegion),
		Credentials: credentials.NewStaticCredentials(
			accessKey,
			secretKey,
			"", // session token is optional
		),
	})
	if err != nil {
		failfast.Failfast(err, failfast.Error, "Failed to create AWS session for ECR push context preparation")
	}

	// Create an ECR client
	ecrClient := ecr.New(sess)

	// Check if the repository exists, if not create it
	cfg, err := config.Load()
	if err != nil {
		failfast.Failfast(err, failfast.Error, "Failed to load configuration for ECR push context preparation")
	}
	repoName := cfg.Docker.Image

	token, err := GetShortECRToken(Ecr.ECRRegion, accessKey, secretKey, repoName)
	if err != nil {
		failfast.Failfast(err, failfast.Error, "Failed to get ECR token for repository")
	}
	ECRLogger.Info("ECR token for repository %s: %s", repoName, token)

	// Check if repository exists
	_, err = ecrClient.DescribeRepositories(&ecr.DescribeRepositoriesInput{
		RepositoryNames: []*string{aws.String(repoName)},
	})

	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case ecr.ErrCodeRepositoryNotFoundException:
				// Repository doesn't exist, create it
				ECRLogger.Info("Repository %s not found, creating a new one", repoName)
				_, err = ecrClient.CreateRepository(&ecr.CreateRepositoryInput{
					RepositoryName:     aws.String(repoName),
					ImageTagMutability: aws.String("MUTABLE"),
					ImageScanningConfiguration: &ecr.ImageScanningConfiguration{
						ScanOnPush: aws.Bool(true),
					},
				})
				if err != nil {
					failfast.Failfast(err, failfast.Error, "Failed to create ECR repository")
				}
				ECRLogger.Success("ECR repository %s created successfully", repoName)
			default:
				failfast.Failfast(err, failfast.Error, "Failed to describe ECR repository")
			}
		} else {
			failfast.Failfast(err, failfast.Error, "Failed to describe ECR repository")
		}
	} else {
		ECRLogger.Info("ECR repository %s already exists", repoName)
	}

	return nil
}
func PrepareECRPullContext(ctx context.Context, ecr ECRContext) (token string, error error) {
	ECRLogger.Info("Preparing ECR pull context")
	// Get login password from aws CLI
	loginCommand := exec.CommandContext(ctx, "aws", "ecr", "get-login-password", "--region", ecr.ECRRegion)
	var stdout, stderr bytes.Buffer
	loginCommand.Stdout = &stdout
	loginCommand.Stderr = &stderr
	err := loginCommand.Run()
	if err != nil {
		failfast.Failfast(err, failfast.Error, "Failed to get ECR login password: %s")
	}
	password := stdout.String()
	// pip token to docker login
	return password, nil
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

func DeleteECRUserAndPolicy() error {
	cfg, err := config.Load()
	if err != nil {
		failfast.Failfast(err, failfast.Error, "Error loading config for ecr policy deletion")
	}
	ECRLogger.Info("Deleting ECR user and policy")
	region := cfg.Docker.RegistryRegion
	session, err := session.NewSession(&aws.Config{
		Region: aws.String(region),
	})
	if err != nil {
		failfast.Failfast(err, failfast.Error, "Failed to create AWS session for ECR user and policy deletion")
	}
	svc := iam.New(session)
	user := cfg.App.Domain
	err = cleanupUser(svc, user)
	if err != nil {
		failfast.Failfast(err, failfast.Error, "Failed to delete ECR user and policy")
	}
	ECRLogger.Success("ECR user and policy deleted successfully")
	return nil
}
func cleanupUser(svc *iam.IAM, userName string) error {
	// 1. Delete access keys
	if err := deleteAccessKeys(svc, userName); err != nil {
		failfast.Failfast(err, failfast.Error, "Failed to delete access keys for user")
	}
	// 2. Detach managed policies
	if err := detachManagedPolicies(svc, userName); err != nil {
		failfast.Failfast(err, failfast.Error, "Failed to detach managed policies for user")
	}
	// 3. Delete inline policies
	if err := deleteInlinePolicies(svc, userName); err != nil {
		failfast.Failfast(err, failfast.Error, "Failed to delete inline policies for user")
	}
	// 4. Remove from groups
	if err := removeFromGroups(svc, userName); err != nil {
		failfast.Failfast(err, failfast.Error, "Failed to remove user from groups")
	}
	// 5. Delete login profile
	if err := deleteLoginProfile(svc, userName); err != nil {
		failfast.Failfast(err, failfast.Error, "Failed to delete login profile for user")
	}
	// 6. Delete the userName
	_, err := svc.DeleteUser(&iam.DeleteUserInput{
		UserName: aws.String(userName),
	})
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok && aerr.Code() == iam.ErrCodeNoSuchEntityException {
			ECRLogger.Warn("User %s does not exist, skipping deletion", userName)
			return nil // User does not exist, nothing to delete
		}
		failfast.Failfast(err, failfast.Error, "Failed to delete user")
	}
	ECRLogger.Info("Successfully deleted user %s", userName)
	return nil

}
func deleteAccessKeys(svc *iam.IAM, userName string) error {
	output, err := svc.ListAccessKeys(&iam.ListAccessKeysInput{
		UserName: aws.String(userName),
	})
	if err != nil {
		return err
	}

	for _, key := range output.AccessKeyMetadata {
		_, err := svc.DeleteAccessKey(&iam.DeleteAccessKeyInput{
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
func detachManagedPolicies(svc *iam.IAM, userName string) error {
	output, err := svc.ListAttachedUserPolicies(&iam.ListAttachedUserPoliciesInput{
		UserName: aws.String(userName),
	})
	if err != nil {
		return err
	}

	for _, policy := range output.AttachedPolicies {
		_, err := svc.DetachUserPolicy(&iam.DetachUserPolicyInput{
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
func deleteInlinePolicies(svc *iam.IAM, userName string) error {
	output, err := svc.ListUserPolicies(&iam.ListUserPoliciesInput{
		UserName: aws.String(userName),
	})
	if err != nil {
		return err
	}

	for _, policyName := range output.PolicyNames {
		_, err := svc.DeleteUserPolicy(&iam.DeleteUserPolicyInput{
			UserName:   aws.String(userName),
			PolicyName: aws.String(*policyName),
		})
		if err != nil {
			return err
		}
		ECRLogger.Info("Deleted inline policy %s for user %s", *policyName, userName)
	}
	return nil
}
func removeFromGroups(svc *iam.IAM, userName string) error {
	output, err := svc.ListGroupsForUser(&iam.ListGroupsForUserInput{
		UserName: aws.String(userName),
	})
	if err != nil {
		return err
	}

	for _, group := range output.Groups {
		_, err := svc.RemoveUserFromGroup(&iam.RemoveUserFromGroupInput{
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
func deleteLoginProfile(svc *iam.IAM, userName string) error {
	_, err := svc.DeleteLoginProfile(&iam.DeleteLoginProfileInput{
		UserName: aws.String(userName),
	})
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok && aerr.Code() == iam.ErrCodeNoSuchEntityException {
			ECRLogger.Warn("Login profile for user %s does not exist, skipping deletion", userName)
			return nil // Login profile does not exist, nothing to delete
		}
		return err
	}
	ECRLogger.Info("Deleted login profile for user %s", userName)
	return nil
}
func CreateECRUserAndPolicy() (string, error) {
	cfg, err := config.Load()
	if err != nil {
		failfast.Failfast(err, failfast.Error, "Error loading config for ecr policy creation")
	}

	region := cfg.Docker.RegistryRegion

	session, err := session.NewSession(&aws.Config{
		Region: aws.String(region),
	})
	if err != nil {
		failfast.Failfast(err, failfast.Error, "Failed to create AWS session for ECR user and policy creation")
	}
	// create Iam and ecr clients
	iamClient := iam.New(session)
	//	ecrClient := ecr.New(session)

	name := cfg.App.Domain
	// Step one create an iam user
	user, err := createUser(iamClient, name)
	if err != nil {
		failfast.Failfast(err, failfast.Error, "Failed to create IAM user for ECR access")
	}
	// Step two create a policy
	policyArn, err := createPolicy(iamClient, name+"-ecr-policy", policyDocument)
	if err != nil {
		failfast.Failfast(err, failfast.Error, "Failed to create IAM policy for ECR access")
	}
	// Step three attach the policy to the user
	err = attachPolicyToUser(iamClient, policyArn, name, name+"-ecr-policy")
	if err != nil {
		failfast.Failfast(err, failfast.Error, "Failed to attach IAM policy to user for ECR access")
	}
	ECRLogger.Success("ECR user and policy created successfully")
	// step 4 create access keys
	accessKey, secretKey, err := createAccessKey(iamClient, name)
	if err != nil {
		failfast.Failfast(err, failfast.Error, "Failed to create access key for ECR user")
	}
	ECRLogger.Success("ECR user access key created successfully")
	// Step 5 ::: Write the crentials to env file
	err = writeCredentialsToEnvFile(accessKey, secretKey)
	if err != nil {
		failfast.Failfast(err, failfast.Error, "Failed to write ECR credentials to env file")
	}
	ECRLogger.Success("ECR credentials written to env file successfully")

	return user, nil
}

func createUser(iamClient *iam.IAM, userName string) (string, error) {
	// Call CreateUser API
	createUserOutput, err := iamClient.CreateUser(&iam.CreateUserInput{
		UserName: aws.String(userName),
	})
	if err != nil {
		failfast.Failfast(err, failfast.Error, "Failed to create IAM user for ECR access")
		return "", err
	}

	// Extract relevant information from the response
	userARN := aws.StringValue(createUserOutput.User.Arn)
	createdUserName := aws.StringValue(createUserOutput.User.UserName)

	// Log the successful creation
	ECRLogger.Info("Created IAM user %s with ARN %s for ECR access", createdUserName, userARN)

	// Return the ARN (or whatever you need)
	return userARN, nil
}

func createPolicy(iamClient *iam.IAM, policyName string, policyDocument string) (string, error) {
	// First try to find existing policy
	listPoliciesOutput, err := iamClient.ListPolicies(&iam.ListPoliciesInput{
		Scope:    aws.String("Local"),
		MaxItems: aws.Int64(1000),
	})
	if err != nil {
		return "", fmt.Errorf("failed to list policies: %w", err)
	}

	// Check if policy already exists
	for _, policy := range listPoliciesOutput.Policies {
		if aws.StringValue(policy.PolicyName) == policyName {
			ECRLogger.Info("Policy %s already exists, reusing ARN: %s",
				policyName, aws.StringValue(policy.Arn))
			return aws.StringValue(policy.Arn), nil
		}
	}

	// Create new policy if not found
	policyInput := &iam.CreatePolicyInput{
		PolicyName:     aws.String(policyName),
		PolicyDocument: aws.String(policyDocument),
	}

	policyOutput, err := iamClient.CreatePolicy(policyInput)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok && aerr.Code() == iam.ErrCodeEntityAlreadyExistsException {
			// Handle race condition where policy was created between list and create calls
			ECRLogger.Info("Policy %s created by another process, looking up ARN", policyName)
			return getPolicyARN(iamClient, policyName)
		}
		return "", fmt.Errorf("failed to create policy: %w", err)
	}

	ECRLogger.Info("Created new policy %s", policyName)
	return aws.StringValue(policyOutput.Policy.Arn), nil
}

func getPolicyARN(iamClient *iam.IAM, policyName string) (string, error) {
	listPoliciesOutput, err := iamClient.ListPolicies(&iam.ListPoliciesInput{
		Scope:    aws.String("Local"),
		MaxItems: aws.Int64(1000),
	})
	if err != nil {
		return "", fmt.Errorf("failed to list policies: %w", err)
	}

	for _, policy := range listPoliciesOutput.Policies {
		if aws.StringValue(policy.PolicyName) == policyName {
			return aws.StringValue(policy.Arn), nil
		}
	}

	return "", fmt.Errorf("policy %s not found after creation conflict", policyName)
}
func attachPolicyToUser(iamClient *iam.IAM, policyArn string, userName string, policyName string) error {
	_, err := iamClient.AttachUserPolicy(&iam.AttachUserPolicyInput{
		UserName:  aws.String(userName),
		PolicyArn: aws.String(policyArn),
	})
	if err != nil {
		return err
	}
	ECRLogger.Info("Attached policy %s to user %s for ECR access", policyName, userName)
	return nil
}

func createAccessKey(iamClient *iam.IAM, userName string) (string, string, error) {
	output, err := iamClient.CreateAccessKey(&iam.CreateAccessKeyInput{
		UserName: aws.String(userName),
	})
	if err != nil {
		failfast.Failfast(err, failfast.Error, "Failed to create access key for ECR user")
	}
	ECRLogger.Info("Created access key for user %s", userName)
	return *output.AccessKey.AccessKeyId, *output.AccessKey.SecretAccessKey, nil
}

func VerifyECRAccess(region, accessKey, secretKey, sessionToken string) error {
	// Create a new session with the provided credentials
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(region),
		Credentials: credentials.NewStaticCredentials(
			accessKey,
			secretKey,
			sessionToken, // session token is optional
		),
	})
	if err != nil {
		failfast.Failfast(err, failfast.Error, "Failed to create AWS session for ECR access verification")
	}

	ecrSvc := ecr.New(sess)

	// Try a simple ECR API call to verify access
	_, err = ecrSvc.DescribeRepositories(&ecr.DescribeRepositoriesInput{
		MaxResults: aws.Int64(1), // Limit to 1 repository to minimize API calls
	})
	if err != nil {
		failfast.Failfast(err, failfast.Error, "Failed to describe ECR repositories with provided credentials")
	}

	return nil
}
func writeCredentialsToEnvFile(accessKey, secretKey string) error {
	ECRLogger.Info("Writing AWS credentials to .env file")
	envFile := ".env"

	// Read existing file content if it exists
	existingContent := make(map[string]string)
	if _, err := os.Stat(envFile); err == nil {
		fileContent, err := os.ReadFile(envFile)
		if err != nil {
			failfast.Failfast(err, failfast.Error, "Failed to read existing env file")
		}

		// Parse existing content
		for _, line := range strings.Split(string(fileContent), "\n") {
			if strings.HasPrefix(line, "AWS_ACCESS_KEY_ID_FOR_ECR=") ||
				strings.HasPrefix(line, "AWS_SECRET_ACCESS_KEY_FOR_ECR=") {
				continue // Skip existing credentials
			}
			if strings.Contains(line, "=") {
				parts := strings.SplitN(line, "=", 2)
				existingContent[parts[0]] = parts[1]
			}
		}
	}

	// Create or truncate the file
	file, err := os.Create(envFile)
	if err != nil {
		failfast.Failfast(err, failfast.Error, "Failed to create env file:")
	}
	defer file.Close()

	// Write all existing content (except our credentials)
	for key, value := range existingContent {
		_, err = file.WriteString(fmt.Sprintf("%s=%s\n", key, value))
		if err != nil {
			failfast.Failfast(err, failfast.Error, "Failed to write existing content to env file")
		}
	}

	// Write new credentials
	_, err = file.WriteString("AWS_ACCESS_KEY_ID_FOR_ECR=" + accessKey + "\n")
	if err != nil {
		failfast.Failfast(err, failfast.Error, "Failed to write AWS_ACCESS_KEY_ID to env file")
	}
	_, err = file.WriteString("AWS_SECRET_ACCESS_KEY_FOR_ECR=" + secretKey + "\n")
	if err != nil {
		failfast.Failfast(err, failfast.Error, "Failed to write AWS_SECRET_ACCESS_KEY to env file")
	}

	ECRLogger.Info("Wrote AWS credentials to %s", envFile)
	return nil
}

func GetShortECRToken(region string, accessKey string, secretKey string, repoName string) (string, error) {
	cfg, err := awsConfig.LoadDefaultConfig(context.TODO(),
		awsConfig.WithRegion(region),
		awsConfig.WithCredentialsProvider(aws.CredentialsProviderFunc(func(ctx context.Context) (aws.Credentials, error) {
			return aws.Credentials{
				AccessKeyID:     accessKey,
				SecretAccessKey: secretKey,
			}, nil
		})),
	)
	if err != nil {
		return "", fmt.Errorf("unable to load AWS config: %w", err)
	}
	ecrClient := ecr.NewFromConfig(cfg)
	output, err := ecrClient.GetAuthorizationToken(context.TODO(), &ecr.GetAuthorizationTokenInput{})
	if err != nil {
		return "", fmt.Errorf("failed to get ECR token: %w", err)
	}
	// The token comes base64 encoded with "AWS:" prefix
	token := *output.AuthorizationData[0].AuthorizationToken
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
