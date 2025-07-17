package registry

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/iam/types"
	"nextdeploy/internal/config"
	"strings"
)

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
		ECRLogger.Error("Failed to load configuration: %v", err)
		return false, err
	}

	awsCfg, err := awsConfig.LoadDefaultConfig(context.TODO(),
		awsConfig.WithRegion(cfg.Docker.RegistryRegion),
		awsConfig.WithSharedConfigProfile("default"),
	)
	if err != nil {
		ECRLogger.Error("Failed to load AWS config: %v", err)
		return false, err
	}

	iamClient := iam.NewFromConfig(awsCfg)
	userName := cfg.App.Domain

	_, err = iamClient.GetUser(context.TODO(), &iam.GetUserInput{
		UserName: aws.String(userName),
	})
	if err != nil {
		ECRLogger.Error("Failed to get user %s: %v", userName, err)
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
		ECRLogger.Error("Failed to load configuration: %v", err)
		return err
	}

	ECRLogger.Info("Deleting ECR user and policy")

	awsCfg, err := awsConfig.LoadDefaultConfig(context.TODO(),
		awsConfig.WithRegion(cfg.Docker.RegistryRegion))
	if err != nil {
		ECRLogger.Error("Failed to load AWS config: %v", err)
		return err
	}

	iamClient := iam.NewFromConfig(awsCfg)
	user := cfg.App.Domain

	err = cleanupUser(iamClient, user)
	if err != nil {
		ECRLogger.Error("Failed to clean up user %s: %v", user, err)
		return err
	}

	ECRLogger.Success("ECR user and policy deleted successfully")
	return nil
}
func cleanupUser(iamClient *iam.Client, userName string) error {
	if err := deleteAccessKeys(iamClient, userName); err != nil {
		ECRLogger.Error("Failed to delete access keys for user %s: %v", userName, err)
		return err
	}

	if err := detachManagedPolicies(iamClient, userName); err != nil {
		ECRLogger.Error("Failed to detach managed policies for user %s: %v", userName, err)
		return err
	}

	if err := deleteInlinePolicies(iamClient, userName); err != nil {
		ECRLogger.Error("Failed to delete inline policies for user %s: %v", userName, err)
		return err
	}

	if err := removeFromGroups(iamClient, userName); err != nil {
		ECRLogger.Error("Failed to remove user %s from groups: %v", userName, err)
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
		ECRLogger.Error("Failed to list access keys for user %s: %v", userName, err)
		return err
	}

	for _, key := range output.AccessKeyMetadata {
		_, err := iamClient.DeleteAccessKey(context.TODO(), &iam.DeleteAccessKeyInput{
			UserName:    aws.String(userName),
			AccessKeyId: key.AccessKeyId,
		})
		if err != nil {
			ECRLogger.Error("Failed to delete access key %s for user %s: %v", *key.AccessKeyId, userName, err)
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
		ECRLogger.Error("Failed to list attached policies for user %s: %v", userName, err)
		return err
	}

	for _, policy := range output.AttachedPolicies {
		_, err := iamClient.DetachUserPolicy(context.TODO(), &iam.DetachUserPolicyInput{
			UserName:  aws.String(userName),
			PolicyArn: policy.PolicyArn,
		})
		if err != nil {
			ECRLogger.Error("Failed to detach policy %s from user %s: %v", *policy.PolicyName, userName, err)
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
		ECRLogger.Error("Failed to list inline policies for user %s: %v", userName, err)
		return err
	}

	for _, policyName := range output.PolicyNames {
		_, err := iamClient.DeleteUserPolicy(context.TODO(), &iam.DeleteUserPolicyInput{
			UserName:   aws.String(userName),
			PolicyName: aws.String(policyName),
		})
		if err != nil {
			ECRLogger.Error("Failed to delete inline policy %s for user %s: %v", policyName, userName, err)
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
		ECRLogger.Error("Failed to list groups for user %s: %v", userName, err)
		return err
	}

	for _, group := range output.Groups {
		_, err := iamClient.RemoveUserFromGroup(context.TODO(), &iam.RemoveUserFromGroupInput{
			GroupName: aws.String(*group.GroupName),
			UserName:  aws.String(userName),
		})
		if err != nil {
			ECRLogger.Error("Failed to remove user %s from group %s: %v", userName, *group.GroupName, err)
			return err
		}
		ECRLogger.Info("Removed user %s from group %s", userName, *group.GroupName)
	}
	return nil
}

func DeleteLoginProfile(iamClient *iam.Client, userName string) error {
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
		ECRLogger.Error("Failed to load configuration: %v", err)
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}
	awsCfg, err := awsConfig.LoadDefaultConfig(context.TODO(),
		awsConfig.WithRegion(cfg.Docker.RegistryRegion),
		awsConfig.WithSharedConfigProfile("default"),
	)
	if err != nil {
		ECRLogger.Error("Failed to load AWS config: %v", err)
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	iamClient := iam.NewFromConfig(awsCfg)
	name := cfg.App.Domain

	user, err := createUser(iamClient, name)
	if err != nil {
		ECRLogger.Error("Failed to create IAM user for ECR access: %v", err)
		return nil, fmt.Errorf("failed to create IAM user for ECR access: %w", err)
	}

	policyArn, err := createPolicy(iamClient, name+"-ecr-policy", policyDocument)
	if err != nil {
		ECRLogger.Error("Failed to create IAM policy for ECR access: %v", err)
		return nil, fmt.Errorf("failed to create IAM policy for ECR access: %w", err)
	}

	err = attachPolicyToUser(iamClient, policyArn, name, name+"-ecr-policy")
	if err != nil {
		ECRLogger.Error("Failed to attach policy to user %s: %v", name, err)
		return nil, fmt.Errorf("failed to attach policy %s to user %s: %w", name+"-ecr-policy", name, err)
	}

	ECRLogger.Success("ECR user and policy created successfully")

	accessKey, secretKey, err := createAccessKey(iamClient, name)
	if err != nil {
		ECRLogger.Error("Failed to create access key for ECR user: %v", err)
		return nil, fmt.Errorf("failed to create access key for ECR user: %w", err)
	}

	ECRLogger.Success("ECR user access key created successfully")

	err = writeCredentialsToEnvFile(accessKey, secretKey)
	if err != nil {
		ECRLogger.Error("Failed to write ECR credentials to .env file: %v", err)
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
		ECRLogger.Error("Failed to create user %s: %v", userName, err)
		return nil, err
	}

	return output.User, nil
}

func createPolicy(iamClient *iam.Client, policyName string, policyDocument string) (string, error) {
	var maxPolicies int32 = 1000
	listPoliciesOutput, err := iamClient.ListPolicies(context.TODO(), &iam.ListPoliciesInput{
		MaxItems: aws.Int32(maxPolicies),
	})
	if err != nil {
		ECRLogger.Error("Failed to list policies: %v", err)
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
		ECRLogger.Error("Failed to list policies: %v", err)
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
		ECRLogger.Error("Failed to attach policy %s to user %s: %v", policyName, userName, err)
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
			ECRLogger.Error("Failed to create access key for user %s: %v", userName, err)
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
		ECRLogger.Error("Unable to load AWS config: %v", err)
		return fmt.Errorf("unable to load AWS config: %w", err)
	}

	ecrClient := ecr.NewFromConfig(cfg)
	_, err = ecrClient.DescribeRepositories(context.TODO(), &ecr.DescribeRepositoriesInput{})
	if err != nil {
		ECRLogger.Error("Failed to describe ECR repositories: %v", err)
		if strings.Contains(err.Error(), "AccessDeniedException") {
			return fmt.Errorf("access denied to ECR in region %s with provided credentials", region)
		}
		return fmt.Errorf("failed to describe ECR repositories: %w", err)
	}

	return nil
}
