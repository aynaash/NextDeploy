package registry

import (
	"context"
	"errors"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
)

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
