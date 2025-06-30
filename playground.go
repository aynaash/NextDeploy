//go:build ignore
// +build ignore

package playground

func createPolicy(iamClient *iam.Client, policyName string, policyDocument string) (string, error) {
	listPoliciesOutput, err := iamClient.ListPolicies(context.TODO(), &iam.ListPoliciesInput{
		Scope: aws.String("Local"),
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

	policyInput := &iam.CreatePolicyInput{
		PolicyName:     aws.String(policyName),
		PolicyDocument: aws.String(policyDocument),
	}

	policyOutput, err := iamClient.CreatePolicy(context.TODO(), policyInput)
	if err != nil {
		var alreadyExists *iamTypes.EntityAlreadyExistsException
		if errors.As(err, &alreadyExists) {
			ECRLogger.Info("Policy %s created by another process, looking up ARN", policyName)
			return getPolicyARN(iamClient, policyName)
		}
		return "", fmt.Errorf("failed to create policy: %w", err)
	}

	ECRLogger.Info("Created new policy %s", policyName)
	return aws.ToString(policyOutput.Policy.Arn), nil
}

func getPolicyARN(iamClient *iam.Client, policyName string) (string, error) {
	listPoliciesOutput, err := iamClient.ListPolicies(context.TODO(), &iam.ListPoliciesInput{
		Scope: aws.String("Local"),
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
		failfast.Failfast(err, failfast.Error, "Failed to create access key for ECR user")
	}
	ECRLogger.Info("Created access key for user %s", userName)
	return aws.ToString(output.AccessKey.AccessKeyId), aws.ToString(output.AccessKey.SecretAccessKey), nil
}

func VerifyECRAccess(region, accessKey, secretKey, sessionToken string) error {
	cfg, err := awsConfig.LoadDefaultConfig(context.TODO(),
		awsConfig.WithRegion(region),
		awsConfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKey, secretKey, sessionToken)))
	if err != nil {
		failfast.Failfast(err, failfast.Error, "Failed to load AWS config for ECR access verification")
	}

	ecrClient := ecr.NewFromConfig(cfg)
	_, err = ecrClient.DescribeRepositories(context.TODO(), &ecr.DescribeRepositoriesInput{
		MaxResults: aws.Int32(1),
	})
	if err != nil {
		failfast.Failfast(err, failfast.Error, "Failed to describe ECR repositories with provided credentials")
	}

	return nil
}
