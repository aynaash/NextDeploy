package registry

import (
	"context"
	"errors"
	"fmt"
	"nextdeploy/internal/config"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
)

func (e *ECRContext) AwsConfig() (aws.Config, error) {
	if e.Region == "" {
		return aws.Config{}, errors.New("ECR region is not set")
	}
	nextconfig, err := config.Load()
	if err != nil {
		ECRLogger.Error("Failed to load configuration: %v", err)
		return aws.Config{}, fmt.Errorf("failed to load configuration: %w", err)
	}
	profile := nextconfig.App.Domain

	cfg, err := awsConfig.LoadDefaultConfig(context.TODO(),
		awsConfig.WithRegion(e.Region),
		awsConfig.WithSharedConfigProfile(profile),
	)
	if err != nil {
		ECRLogger.Error("Unable to load AWS config: %v", err)
		return aws.Config{}, fmt.Errorf("unable to load AWS config: %w", err)
	}
	return cfg, nil
}
